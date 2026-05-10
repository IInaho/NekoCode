package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nekocode/bot/ctxmgr"
	"nekocode/bot/tools"
	"nekocode/llm"
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

func (e *Engine) Run(ctx context.Context, cfg RunConfig) (string, error) {
	ctxMgr := ctxmgr.New(cfg.AgentType.SystemPrompt)
	ctxMgr.SetTokenBudget(cfg.AgentType.TokenBudget)
	ctxMgr.SetSummarizer(e.makeSummarizer(ctx))

	if cfg.Cwd != "" {
		ctxMgr.Add("system", "Working directory: "+cfg.Cwd)
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

	for step := 0; step < cfg.AgentType.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Proactive auto-compact before each LLM call.
		ctxMgr.AutoCompactIfNeeded(ctxMgr.GetAutoCompactConfig(), nil)

		calls, text, err := e.reason(ctx, ctxMgr, cfg.AgentType.Tools, cfg.AddTokens, phase)
		if err != nil {
			return "", err
		}
		if len(calls) == 0 {
			phase("done")
			return text, nil
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
		phase("Waiting")
	}

	return e.forceSynthesize(ctx, ctxMgr), nil
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

		firstToken := true
		for token := range tokenCh {
			if token.Content != "" {
				if firstToken {
					firstToken = false
					if phase != nil {
						phase(tools.PhaseThinking)
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
	var text string
	_ = llm.Retry(ctx, llm.DefaultRetryConfig, func() error {
		mgr.AutoCompactIfNeeded(mgr.GetAutoCompactConfig(), nil)
		messages := mgr.Build(false)
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: "Based on the information collected above, provide your final conclusion directly. Do NOT call any more tools. Keep it concise, under 300 chars.",
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
	if text == "" {
		return "Unable to generate summary"
	}
	return text
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
