package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nekocode/bot/ctxmgr"
	"nekocode/bot/tools"
	"nekocode/llm"
)

const (
	thoroughQuick  = "quick"
	thoroughDeep   = "very thorough"
)

// Engine runs a sub-agent loop. Fully self-contained — does not import agent.
type Engine struct {
	llmClient    llm.LLM
	toolRegistry *tools.Registry
	executor     *tools.Executor
}

func NewEngine(llmClient llm.LLM, registry *tools.Registry) *Engine {
	e := tools.NewExecutor(registry)
	// Auto-approve write-level tools for subagents — the main agent already
	// obtained user approval for the delegated task. Destructive tools (LevelDestructive,
	// LevelForbidden) are still blocked by the executor's level check.
	e.SetConfirmFn(func(req tools.ConfirmRequest) bool {
		return req.Level <= tools.LevelWrite
	})
	return &Engine{llmClient: llmClient, toolRegistry: registry, executor: e}
}


// toolCallItem mirrors agent.ToolCallItem to avoid circular imports.
type toolCallItem struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// Run executes a subagent and returns a structured Result.
// Pattern from Claude Code's runAgent() + finalizeAgentTool():
//   - Independent context with fresh file cache
//   - Structured output parsing after completion
//   - Safety classification before handoff
//   - Partial result recovery on error/interrupt
func (e *Engine) Run(ctx context.Context, cfg RunConfig) (*Result, error) {
	// Apply thoroughness-based overrides for the explore agent.
	maxSteps := cfg.AgentType.MaxSteps
	tokenBudget := cfg.AgentType.TokenBudget
	systemPrompt := cfg.AgentType.SystemPrompt
	if cfg.AgentType.Name == "explore" {
		switch cfg.Thoroughness {
		case thoroughQuick:
			maxSteps = 1
			tokenBudget = 4000
		case thoroughDeep:
			maxSteps = 4
			tokenBudget = 16000
			systemPrompt = strings.Replace(systemPrompt,
				"Focus on the specific question, no exhaustive searches",
				"Search across multiple directories and naming conventions. Be thorough.", 1)
		}
	}

	// Sub-agents have isolated contexts — they can't reference file content
	// from the main agent's conversation. Swap in a fresh cache so their
	// first read of any file returns full content, not a stub.
	savedCache := tools.GlobalFileCache
	tools.GlobalFileCache = tools.NewFileStateCache()
	defer func() { tools.GlobalFileCache = savedCache }()

	ctxMgr := ctxmgr.New(systemPrompt)
	ctxMgr.SetTokenBudget(tokenBudget)
	ctxMgr.SetSummarizer(e.makeSummarizer(ctx))

	if cfg.Cwd != "" {
		ctxMgr.Add("system", "Working directory: "+cfg.Cwd)
	}
	if cfg.ProjectContext != "" && !cfg.AgentType.OmitProjectContext {
		ctxMgr.Add("system", cfg.ProjectContext)
	}
	if cfg.DisableThinking {
		ctxMgr.Add("system", "Stop when done. Don't over-analyze — just execute. Keep output short (≤100 chars).")
	}
	ctxMgr.Add("user", cfg.Prompt)

	phase := func(p string) {
		if cfg.OnPhase != nil {
			cfg.OnPhase(p)
		}
	}
	phase("Waiting")

	var readOnlyStreak int
	var lastText string // last assistant text content (for partial result recovery)

	localAddTokens := cfg.AddTokens

	for step := 0; step < maxSteps; step++ {
		select {
		case <-ctx.Done():
			return buildPartialResult(lastText), ctx.Err()
		default:
		}

		// Proactive auto-compact before each LLM call.
		ctxMgr.AutoCompactIfNeeded(ctxMgr.GetAutoCompactConfig(), nil)

		calls, text, err := e.reason(ctx, ctxMgr, cfg.AgentType.Tools, localAddTokens, phase)
		if err != nil {
			if lastText != "" {
				return buildPartialResult(lastText), nil
			}
			return buildFailedResult(err.Error()), err
		}

		if text != "" {
			lastText = text
		}

		if len(calls) == 0 {
			phase("done")
			return buildResult(text), nil
		}

		for _, c := range calls {
			phase("Running " + c.Name)
		}
		items := make([]tools.ToolCallItem, len(calls))
		for i, c := range calls {
			items[i] = tools.ToolCallItem{ID: c.ID, Name: c.Name, Args: c.Args}
		}
		results := e.executor.ExecuteBatch(ctx, items)
		msgs := make([]llm.Message, len(results))
		for i, r := range results {
			content := r.Output
			if r.Error != "" {
				content = r.Error
			}
			msgs[i] = llm.Message{Content: content, ToolCallID: r.ID}
		}
		ctxMgr.AddToolResultsBatch(msgs)

		// Read-only spiral check — inject AFTER tool results so the
		// assistant→tool message chain stays contiguous.
		if subAllExploration(calls) {
			readOnlyStreak++
			if readOnlyStreak >= 3 {
				ctxMgr.Add("user", "[System] You've been reading without acting. Summarize your findings now — don't read any more files.")
				readOnlyStreak = 0
			}
		} else {
			readOnlyStreak = 0
		}

		phase("Waiting")
	}

	// Max steps reached — force synthesize.
	lastText = e.forceSynthesize(ctx, ctxMgr)
	return buildResult(lastText), nil
}

// buildResult constructs a Result from a completed subagent run.
func buildResult(rawOutput string) *Result {
	content, keyFiles, filesChanged, issues := parseStructuredOutput(rawOutput)

	if content == "" {
		content = rawOutput
	}

	r := &Result{
		Status:   StatusCompleted,
		Content:  content,
		KeyFiles: keyFiles,
		Issues:   issues,
	}

	r.classification = classifyHandoff(rawOutput, filesChanged, keyFiles)

	return r
}

// buildPartialResult creates a Result for interrupted/killed subagents.
func buildPartialResult(lastText string) *Result {
	content, keyFiles, filesChanged, issues := parseStructuredOutput(lastText)
	if content == "" {
		content = lastText
	}
	r := &Result{
		Status:   StatusPartial,
		Content:  content,
		KeyFiles: keyFiles,
		Issues:   append(issues, "subagent was interrupted before completion"),
	}

	r.classification = classifyHandoff(lastText, filesChanged, keyFiles)

	return r
}

// buildFailedResult creates a Result for subagents that produced no output.
func buildFailedResult(errMsg string) *Result {
	return &Result{
		Status:         StatusFailed,
		Content:        errMsg,
		Issues:         []string{errMsg},
		classification: classUnavailable,
	}
}

func (e *Engine) makeSummarizer(ctx context.Context) ctxmgr.Summarizer {
	return func(msgs []llm.Message, prevSummary string) (string, error) {
		prompt := ctxmgr.BuildPrompt(msgs, prevSummary)
		resp, err := e.llmClient.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, nil)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) > 0 {
			return resp.Choices[0].Message.Content, nil
		}
		return "", nil
	}
}

func (e *Engine) reason(ctx context.Context, mgr *ctxmgr.Manager, allowed []string, addTokens func(int, int), phase func(string)) ([]toolCallItem, string, error) {
	var calls []toolCallItem
	var textContent string
	var reasoningContent string

	firstAttempt := true
	err := llm.Retry(ctx, llm.DefaultRetryConfig, func() error {
		mgr.AutoCompactIfNeeded(mgr.GetAutoCompactConfig(), nil)
		messages := mgr.Build(true)
		toolDefs := e.filteredToolDefs(allowed)

		tokenCh, errCh := e.llmClient.ChatStream(ctx, messages, toolDefs)
		if tokenCh == nil {
			select {
			case err := <-errCh:
				return err
			default:
				return fmt.Errorf("chat stream failed")
			}
		}

		var textBuf strings.Builder
		var reasoningBuf strings.Builder
		tcAccum := make(map[int]*toolAccum)

		promptChars := 0
		for _, m := range messages {
			promptChars += len(m.Content) + len(m.Role)
		}
		if firstAttempt && addTokens != nil {
			addTokens(promptChars/4, 0)
			firstAttempt = false
		}

		firstReasoning := true
		phaseThink := true
		for token := range tokenCh {
			if firstReasoning && token.ReasoningContent != "" {
				firstReasoning = false
				if phase != nil {
					phase(tools.PhaseThinking)
				}
			}
			if token.Content != "" {
				if phaseThink {
					phaseThink = false
					if phase != nil {
						phase(tools.PhaseReasoning)
					}
				}
				textBuf.WriteString(token.Content)
				if addTokens != nil {
					addTokens(0, 1)
				}
			}
			if token.ReasoningContent != "" {
				reasoningBuf.WriteString(token.ReasoningContent)
			}
			if token.ToolCallDelta != nil {
				if phaseThink {
					phaseThink = false
					if phase != nil {
						phase(tools.PhaseReasoning)
					}
				}
				idx := token.ToolCallDelta.Index
				acc := tcAccum[idx]
				if acc == nil {
					acc = &toolAccum{}
					tcAccum[idx] = acc
				}
				if token.ToolCallDelta.ID != "" {
					acc.id = token.ToolCallDelta.ID
				}
				if token.ToolCallDelta.Name != "" {
					acc.name = token.ToolCallDelta.Name
				}
				acc.args.WriteString(token.ToolCallDelta.Arguments)
				if addTokens != nil {
					addTokens(0, 1)
				}
			}
		}

		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		default:
		}

		textContent = tools.StripAnsi(textBuf.String())
		reasoningContent = reasoningBuf.String()

		if len(tcAccum) == 0 {
			return nil
		}

		calls = make([]toolCallItem, 0, len(tcAccum))
		for i := 0; i < len(tcAccum); i++ {
			acc := tcAccum[i]
			if acc == nil {
				continue
			}
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(acc.args.String()), &args); err != nil {
				return fmt.Errorf("failed to parse tool arguments: %v", err)
			}
			calls = append(calls, toolCallItem{ID: acc.id, Name: acc.name, Args: args})
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	if len(calls) > 0 {
		mgr.AddAssistantToolCall(textContent, reasoningContent, toLLMToolCalls(calls))
	}
	return calls, textContent, nil
}

func (e *Engine) forceSynthesize(ctx context.Context, mgr *ctxmgr.Manager) string {
	// Primary path: full context with retries.
	var text string
	_ = llm.Retry(ctx, llm.DefaultRetryConfig, func() error {
		mgr.AutoCompactIfNeeded(mgr.GetAutoCompactConfig(), nil)
		messages := mgr.Build(false)
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: "Based on the information collected above, provide your final conclusion using the required output format (Scope/Result/Key files/Issues). Do NOT call any more tools. Keep it concise, under 300 chars.",
		})
		tokenCh, errCh := e.llmClient.ChatStream(ctx, messages, nil)
		if tokenCh == nil {
			return fmt.Errorf("chat stream failed")
		}
		var textBuf strings.Builder
		for token := range tokenCh {
			if token.Content != "" {
				textBuf.WriteString(token.Content)
			}
		}
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		default:
		}
		text = tools.StripAnsi(textBuf.String())
		return nil
	})
	if text != "" {
		return text
	}
	// Emergency fallback: one last attempt with minimal context.
	mgr.ForceCompact()
	msgs := mgr.BuildMinimal()
	msgs = append(msgs, llm.Message{
		Role:    "user",
		Content: "Provide your final conclusion based on the context above. Keep it concise, under 300 chars.",
	})
	ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tokenCh, errCh := e.llmClient.ChatStream(ctx2, msgs, nil)
	if tokenCh != nil {
		var buf strings.Builder
		for token := range tokenCh {
			if token.Content != "" {
				buf.WriteString(token.Content)
			}
		}
		select {
		case <-errCh:
		default:
		}
		if t := tools.StripAnsi(buf.String()); t != "" {
			return t
		}
	}
	return "Unable to generate summary"
}

func (e *Engine) filteredToolDefs(allowed []string) []llm.ToolDef {
	all := e.toolRegistry.Descriptors()
	set := make(map[string]bool, len(allowed))
	for _, n := range allowed {
		set[n] = true
	}
	var filtered []tools.Descriptor
	for _, d := range all {
		if set[d.Name] {
			filtered = append(filtered, d)
		}
	}
	return tools.ToToolDefs(filtered)
}

// --- helpers ---

type toolAccum struct {
	id   string
	name string
	args strings.Builder
}

func toLLMToolCalls(calls []toolCallItem) []llm.ToolCall {
	out := make([]llm.ToolCall, len(calls))
	for i, c := range calls {
		args, _ := json.Marshal(c.Args)
		out[i] = llm.ToolCall{
			ID:       c.ID,
			Type:     "function",
			Function: llm.FunctionCall{Name: c.Name, Arguments: string(args)},
		}
	}
	return out
}

// subAllExploration returns true if every call is read-only exploration.
func subAllExploration(calls []toolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		switch c.Name {
		case "read", "grep", "glob", "list", "web_search", "web_fetch":
			continue
		default:
			return false
		}
	}
	return true
}
