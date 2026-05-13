package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"nekocode/bot/ctxmgr"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin"
	"nekocode/llm"
)

// StopReason categorizes why the agent loop stopped.
type StopReason int

const (
	StopCompleted          StopReason = iota // normal completion — model returned text without tools
	StopInterrupted                          // user aborted
	StopDoomLoop                             // identical tool calls detected
	StopDiminishingReturns                   // consecutive turns with minimal progress
	StopHookPrevented                        // shouldStop hook returned true
)

func (s StopReason) String() string {
	switch s {
	case StopCompleted:
		return "completed"
	case StopInterrupted:
		return "interrupted"
	case StopDoomLoop:
		return "doom_loop"
	case StopDiminishingReturns:
		return "diminishing_returns"
	case StopHookPrevented:
		return "hook_prevented"
	default:
		return "unknown"
	}
}

// StopInfo is passed to ShouldStopFunc after each tool-execution turn.
type StopInfo struct {
	Step             int
	State            *stepState
	TokensUsed       int  // total prompt+completion tokens this session
	TokenBudget      int  // configured token budget
	BudgetPressure   bool // true if context is over 80% of token budget
	ConsecutiveTurns int  // number of turns since last meaningful progress
}

// ShouldStopFunc is a configurable stop condition.
// Return true to force synthesis and stop. The StopInfo provides full context
// so hooks can make nuanced decisions (e.g., "stop if >10 turns AND budget pressure").
type ShouldStopFunc func(info StopInfo) bool

// ContextTransform allows extensions to inspect/modify messages before LLM calls,
// like Pi's transformContext hook.
type ContextTransform func(messages []llm.Message) []llm.Message

// StreamCallback receives incremental content text during LLM streaming.
type StreamCallback func(delta string, isToolCall bool)

// ReasoningCallback receives DeepSeek reasoning_content tokens during streaming.
type ReasoningCallback func(delta string)

type Agent struct {
	ctxMu                sync.Mutex
	parentCtx            context.Context
	ctx                  context.Context
	cancel               context.CancelFunc
	ctxMgr               *ctxmgr.Manager
	llmClient            llm.LLM
	toolRegistry         *tools.Registry
	executor             *tools.Executor
	phaseFn              tools.PhaseFunc
	lastReasoningContent string
	currentStep          int
	finished             bool
	stopReason           StopReason // why the last run stopped
	shouldStop           ShouldStopFunc
	transformContext     ContextTransform
	streamFn             StreamCallback
	reasoningFn          ReasoningCallback
	steeringCh           chan string
	doomLoopHistory      []string // last few tool call signatures for doom loop detection
	synthesizePrompt     string
	tokenPrompt          atomic.Int64
	tokenCompletion      atomic.Int64
	startTime            time.Time
	// Diminishing returns tracking (Claude Code checkTokenBudget pattern).
	// Tracks consecutive turns where token output was below the threshold.
	diminishingStreak int
	lastTurnTokens    int64 // tokens used in the most recent turn
	// Budget pressure: when context fills up, inject meta-messages to push
	// the model toward synthesis rather than continued exploration.
	budgetPressureInjected bool
}

const (
	steeringChBuffer = 4
	// Diminishing returns: if N consecutive turns each produce fewer than
	// this many completion tokens, the model is spinning its wheels.
	diminishingThreshold = 3   // consecutive low-output turns
	minCompletionTokens  = 200 // minimum completion tokens per turn to count as "progress"
	// Budget pressure: when context exceeds this fraction of the token budget,
	// inject a meta-message urging synthesis.
	budgetPressureRatio = 0.8
)

func New(
	ctx context.Context,
	ctxMgr *ctxmgr.Manager,
	llmClient llm.LLM,
	toolRegistry *tools.Registry,
) *Agent {
	agentCtx, cancel := context.WithCancel(ctx)
	return &Agent{
		parentCtx:        ctx,
		ctx:              agentCtx,
		cancel:           cancel,
		ctxMgr:           ctxMgr,
		llmClient:        llmClient,
		toolRegistry:     toolRegistry,
		executor:         tools.NewExecutor(toolRegistry),
		steeringCh:       make(chan string, steeringChBuffer),
		synthesizePrompt: "Based on the information collected above, provide a final answer. Do NOT call any more tools. Output your conclusion directly.",
	}
}

func (a *Agent) SetConfirmFn(fn tools.ConfirmFunc) { a.executor.SetConfirmFn(fn) }
func (a *Agent) SetPhaseFn(fn tools.PhaseFunc)     { a.phaseFn = fn; a.executor.SetPhaseFn(fn) }
func (a *Agent) PhaseFn() tools.PhaseFunc          { return a.phaseFn }
func (a *Agent) SetPlanMode(on bool)               { a.executor.SetPlanMode(on) }

// StopReason returns why the last run stopped.
func (a *Agent) StopReason() StopReason { return a.stopReason }
func (a *Agent) WireTodoWrite(fn tools.TodoFunc) {
	if t, err := a.toolRegistry.Get("todo_write"); err == nil {
		t.(*builtin.TodoWriteTool).SetUpdateFn(fn)
	}
}
func (a *Agent) SetShouldStop(fn ShouldStopFunc)           { a.shouldStop = fn }
func (a *Agent) SetContextTransform(fn ContextTransform)   { a.transformContext = fn }
func (a *Agent) SetSynthesizePrompt(prompt string)         { a.synthesizePrompt = prompt }
func (a *Agent) SetStreamFn(fn StreamCallback)             { a.streamFn = fn }
func (a *Agent) SetReasoningStreamFn(fn ReasoningCallback) { a.reasoningFn = fn }

// getCtx atomically reads the current context (safe for concurrent Steer/Abort).
func (a *Agent) getCtx() context.Context {
	a.ctxMu.Lock()
	defer a.ctxMu.Unlock()
	return a.ctx
}

// replaceCtx atomically cancels the current context and replaces it with a fresh one.
func (a *Agent) replaceCtx() {
	a.ctxMu.Lock()
	defer a.ctxMu.Unlock()
	a.cancel()
	a.ctx, a.cancel = context.WithCancel(a.parentCtx)
}

// Steer injects a user message mid-loop and interrupts the ongoing LLM call.
func (a *Agent) Steer(msg string) {
	writeAgentLog("Steer: msg=%q", msg)
	select {
	case a.steeringCh <- msg:
	default:
	}
	a.replaceCtx()
	writeAgentLog("Steer: context replaced")
}

// Abort cancels the agent's context, causing LLM calls and tool execution to stop.
func (a *Agent) Abort() {
	a.finished = true
	a.ctxMu.Lock()
	a.cancel()
	a.ctxMu.Unlock()
}

func (a *Agent) AddTokens(prompt, completion int) {
	a.tokenPrompt.Add(int64(prompt))
	a.tokenCompletion.Add(int64(completion))
	writeAgentLog("AddTokens(+%d,+%d) total: p=%d c=%d", prompt, completion,
		a.tokenPrompt.Load(), a.tokenCompletion.Load())
}

func (a *Agent) ResetTokens() {
	a.tokenPrompt.Store(0)
	a.tokenCompletion.Store(0)
}

func (a *Agent) TokenUsage() (prompt, completion int) {
	return int(a.tokenPrompt.Load()), int(a.tokenCompletion.Load())
}

// ContextTokens returns the estimated context size for the current messages.
func (a *Agent) ContextTokens() int {
	_, tokens, _ := a.ctxMgr.Stats()
	return tokens
}

func (a *Agent) Duration() time.Duration {
	if a.startTime.IsZero() {
		return 0
	}
	return time.Since(a.startTime)
}

func (a *Agent) Reset() {
	a.ctxMu.Lock()
	if a.ctx.Err() != nil {
		a.ctx, a.cancel = context.WithCancel(a.parentCtx)
	}
	a.ctxMu.Unlock()
	a.currentStep = 0
	a.finished = false
	a.stopReason = StopCompleted
	a.lastReasoningContent = ""
	a.doomLoopHistory = nil
	a.diminishingStreak = 0
	a.lastTurnTokens = 0
	a.budgetPressureInjected = false
	a.tokenPrompt.Store(0)
	a.tokenCompletion.Store(0)
	a.startTime = time.Now()
}
