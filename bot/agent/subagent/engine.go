package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"primusbot/bot/ctxmgr"
	"primusbot/bot/tools"
	"primusbot/bot/types"
	"primusbot/llm"
)

// Engine runs a sub-agent loop. Fully self-contained — does not import agent.
type Engine struct {
	llmClient    llm.LLM
	toolRegistry *tools.Registry
}

func NewEngine(llmClient llm.LLM, registry *tools.Registry) *Engine {
	return &Engine{llmClient: llmClient, toolRegistry: registry}
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
		ctxMgr.Add("system", "工作目录: "+cfg.Cwd)
	}
	if cfg.DisableThinking {
		ctxMgr.Add("system", "完成就停。不要过度分析，直接动手执行。输出简短（≤100字）。")
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

		// Auto-compact when approaching the token budget.
		if ctxMgr.NeedsSummarization() {
			_ = ctxMgr.Summarize()
		}

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
		results := e.executeBatch(ctx, calls)
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
	messages := mgr.Build(true)
	toolDefs := e.filteredToolDefs(allowed)

	tokenCh, errCh := e.llmClient.ChatStream(ctx, messages, toolDefs)
	if tokenCh == nil {
		select {
		case err := <-errCh:
			return nil, "", err
		default:
			return nil, "", fmt.Errorf("chat stream failed")
		}
	}

	var textBuf strings.Builder
	var reasoningBuf strings.Builder
	tcAccum := make(map[int]*toolAccum)

	promptChars := 0
	for _, m := range messages {
		promptChars += len(m.Content) + len(m.Role)
	}
	if addTokens != nil {
		addTokens(promptChars/4, 0)
	}

	firstToken := true
	for token := range tokenCh {
		if token.Content != "" {
			if firstToken {
				firstToken = false
				if phase != nil {
					phase(types.PhaseThinking)
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
		}
	}

	select {
	case err := <-errCh:
		if err != nil {
			return nil, "", err
		}
	default:
	}

	textContent := tools.StripAnsi(textBuf.String())

	if len(tcAccum) == 0 {
		return nil, textContent, nil
	}

	calls := make([]toolCallItem, 0, len(tcAccum))
	for i := 0; i < len(tcAccum); i++ {
		acc := tcAccum[i]
		if acc == nil {
			continue
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(acc.args.String()), &args); err != nil {
			return nil, "", fmt.Errorf("解析工具参数失败: %v", err)
		}
		calls = append(calls, toolCallItem{ID: acc.id, Name: acc.name, Args: args})
	}

	mgr.AddAssistantToolCall(textContent, reasoningBuf.String(), toLLMToolCalls(calls))
	return calls, textContent, nil
}

func (e *Engine) executeBatch(ctx context.Context, calls []toolCallItem) []tools.ToolCallResult {
	items := make([]tools.ToolCallItem, len(calls))
	for i, c := range calls {
		items[i] = tools.ToolCallItem{ID: c.ID, Name: c.Name, Args: c.Args}
	}

	var ro, mw []tools.ToolCallItem
	for _, c := range items {
		t, err := e.toolRegistry.Get(c.Name)
		if err != nil || t.ExecutionMode(c.Args) == tools.ModeSequential {
			mw = append(mw, c)
		} else {
			ro = append(ro, c)
		}
	}

	results := make([]tools.ToolCallResult, 0, len(calls))
	if len(ro) > 0 {
		results = append(results, e.execTools(ctx, ro, true)...)
	}
	if len(mw) > 0 {
		results = append(results, e.execTools(ctx, mw, false)...)
	}
	return results
}

func (e *Engine) execTools(ctx context.Context, calls []tools.ToolCallItem, parallel bool) []tools.ToolCallResult {
	n := len(calls)
	if n == 0 {
		return nil
	}
	results := make([]tools.ToolCallResult, n)
	if parallel && n > 1 {
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, c := range calls {
			select {
			case <-ctx.Done():
				results[i] = tools.ToolCallResult{ID: c.ID, Name: c.Name, Error: ctx.Err().Error()}
				continue
			default:
			}
			wg.Add(1)
			go func(idx int, tc tools.ToolCallItem) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				results[idx] = execOne(ctx, e.toolRegistry, tc)
			}(i, c)
		}
		wg.Wait()
	} else {
		for i, c := range calls {
			results[i] = execOne(ctx, e.toolRegistry, c)
		}
	}
	return results
}

func execOne(ctx context.Context, reg *tools.Registry, tc tools.ToolCallItem) tools.ToolCallResult {
	tool, err := reg.Get(tc.Name)
	if err != nil {
		return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}
	output, err := tool.Execute(ctx, tc.Args)
	if err != nil {
		return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}
	return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: output}
}

func (e *Engine) forceSynthesize(ctx context.Context, mgr *ctxmgr.Manager) string {
	messages := mgr.Build(false)
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: "以上是你收集到的信息。请直接输出最终结论，不要再调用工具。保持精简，控制在 300 字以内。",
	})
	tokenCh, errCh := e.llmClient.ChatStream(ctx, messages, nil)
	if tokenCh == nil {
		return "无法生成总结"
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
			return "总结失败"
		}
	default:
	}
	return tools.StripAnsi(textBuf.String())
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
	return descriptorsToToolDefs(filtered)
}

// --- helpers ---

type toolAccum struct {
	id   string
	name string
	args strings.Builder
}

func descriptorsToToolDefs(descs []tools.Descriptor) []llm.ToolDef {
	defs := make([]llm.ToolDef, len(descs))
	for i, d := range descs {
		props := make(map[string]llm.Property)
		var required []string
		for _, p := range d.Parameters {
			props[p.Name] = llm.Property{Type: p.Type, Description: p.Description}
			if p.Required {
				required = append(required, p.Name)
			}
		}
		defs[i] = llm.ToolDef{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        d.Name,
				Description: d.Description,
				Parameters: llm.Parameters{
					Type: "object", Properties: props, Required: required,
				},
			},
		}
	}
	return defs
}

func toLLMToolCalls(calls []toolCallItem) []llm.ToolCall {
	out := make([]llm.ToolCall, len(calls))
	for i, c := range calls {
		args, _ := json.Marshal(c.Args)
		out[i] = llm.ToolCall{
			ID:   c.ID,
			Type: "function",
			Function: llm.FunctionCall{Name: c.Name, Arguments: string(args)},
		}
	}
	return out
}
