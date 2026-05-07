package agent

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"primusbot/bot/tools"
	"primusbot/bot/types"
	"primusbot/bot/ctxmgr"
	"primusbot/llm"
)

// StopInfo is passed to ShouldStopFunc, mirroring Pi's ShouldStopAfterTurnContext.
type StopInfo struct {
	Step      int
	State     *stepState
	Reasoning *ReasoningResult
}

// ShouldStopFunc is a configurable stop condition, like Pi's shouldStopAfterTurn.
type ShouldStopFunc func(info StopInfo) bool

// ContextTransform allows extensions to inspect/modify messages before LLM calls,
// like Pi's transformContext hook.
type ContextTransform func(messages []llm.Message) []llm.Message

// StreamCallback receives incremental text and tool call signals during LLM streaming.
type StreamCallback func(delta string, isToolCall bool)

// TokenStats tracks actual API token usage.
type TokenStats struct {
	Prompt      int
	Completion  int
	TotalCalls  int
}

type Agent struct {
	ctxMu                sync.Mutex
	ctx                  context.Context
	cancel               context.CancelFunc
	ctxMgr               *ctxmgr.Manager
	llmClient            llm.LLM
	toolRegistry         *tools.Registry
	executor             *Executor
	phaseFn              types.PhaseFunc
	lastReasoningContent string
	maxIterations        int
	currentStep          int
	finished             bool
	shouldStop           ShouldStopFunc
	transformContext     ContextTransform
	streamFn             StreamCallback
	steeringCh           chan string
	synthesizePrompt     string
	tokensMu             sync.Mutex
	tokens               TokenStats
	startTime            time.Time
}

func New(
	ctx context.Context,
	ctxMgr *ctxmgr.Manager,
	llmClient llm.LLM,
	toolRegistry *tools.Registry,
) *Agent {
	agentCtx, cancel := context.WithCancel(ctx)
	return &Agent{
		ctx:              agentCtx,
		cancel:           cancel,
		ctxMgr:           ctxMgr,
		llmClient:        llmClient,
		toolRegistry:     toolRegistry,
		executor:         NewExecutor(toolRegistry),
		maxIterations:    15,
		steeringCh:       make(chan string, 4),
		synthesizePrompt: "以上是你收集到的信息。请根据这些信息给出最终回答，不要再调用工具。直接输出结论。",
	}
}

func (a *Agent) SetConfirmFn(fn types.ConfirmFunc) { a.executor.SetConfirmFn(fn) }
func (a *Agent) SetPhaseFn(fn types.PhaseFunc)     { a.phaseFn = fn; a.executor.SetPhaseFn(fn) }
func (a *Agent) SetShouldStop(fn ShouldStopFunc)    { a.shouldStop = fn }
func (a *Agent) SetContextTransform(fn ContextTransform) { a.transformContext = fn }
func (a *Agent) SetSynthesizePrompt(prompt string)  { a.synthesizePrompt = prompt }
func (a *Agent) SetStreamFn(fn StreamCallback)       { a.streamFn = fn }

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
	a.ctx, a.cancel = context.WithCancel(context.Background())
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
	a.tokensMu.Lock()
	a.tokens.Prompt += prompt
	a.tokens.Completion += completion
	a.tokens.TotalCalls++
	sumP := a.tokens.Prompt
	sumC := a.tokens.Completion
	a.tokensMu.Unlock()
	writeAgentLog("AddTokens(+%d,+%d) total: p=%d c=%d", prompt, completion, sumP, sumC)
}

func (a *Agent) ResetTokens() {
	a.tokensMu.Lock()
	a.tokens.Prompt = 0
	a.tokens.Completion = 0
	a.tokensMu.Unlock()
}

func (a *Agent) TokenUsage() (prompt, completion int) {
	a.tokensMu.Lock()
	p := a.tokens.Prompt
	c := a.tokens.Completion
	a.tokensMu.Unlock()
	return p, c
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
		a.ctx, a.cancel = context.WithCancel(context.Background())
	}
	a.ctxMu.Unlock()
	a.currentStep = 0
	a.finished = false
	a.lastReasoningContent = ""
	a.tokensMu.Lock()
	a.tokens = TokenStats{}
	a.tokensMu.Unlock()
	a.startTime = time.Now()
}


func writeAgentLog(format string, args ...interface{}) {
	f, err := os.OpenFile("/tmp/primusbot-debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil { return }
	defer f.Close()
	fmt.Fprintf(f, format+"\n", args...)
}