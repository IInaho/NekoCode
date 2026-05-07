package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"primusbot/bot/tools"
	"primusbot/llm"
)

type ActionType int

const (
	ActionChat ActionType = iota
	ActionExecuteTool
	ActionFinish
)

type ToolCallItem struct {
	ID   string
	Name string
	Args map[string]interface{}
}

type ReasoningResult struct {
	Thought     string
	Action      ActionType
	ActionInput string
	ToolCallID  string
	ToolCalls   []ToolCallItem
	TextContent string
	IsFinal     bool
	Interrupted bool // context was canceled mid-stream (steer, not abort)
}

func (a *Agent) Reason(state *stepState) *ReasoningResult {
	if a.phaseFn != nil {
		a.phaseFn("Thinking")
	}
	if strings.HasPrefix(state.Input, "/") {
		return &ReasoningResult{
			Thought: "用户输入了命令", Action: ActionFinish, IsFinal: true,
		}
	}

	toolCalls, textContent, err := a.callLLMForTool()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &ReasoningResult{
				Thought: "用户中断", Action: ActionChat,
				Interrupted: true,
			}
		}
		return &ReasoningResult{
			Thought: "LLM调用失败", Action: ActionChat,
			ActionInput: fmt.Sprintf("抱歉，调用失败: %v", err), IsFinal: true,
		}
	}

	if len(toolCalls) == 0 {
		if textContent == "" {
			textContent = "抱歉，我无法确定要做什么"
		}
		return &ReasoningResult{
			Thought: "直接回复", Action: ActionChat,
			ActionInput: textContent, IsFinal: true,
		}
	}

	if len(toolCalls) == 1 {
		tc := toolCalls[0]
		return &ReasoningResult{
			Thought: "调用工具: " + tc.Name, Action: ActionExecuteTool,
			ActionInput: tc.Name + ":" + formatArgs(tc.Args),
			ToolCallID: tc.ID, ToolCalls: toolCalls, TextContent: textContent, IsFinal: false,
		}
	}

	var names []string
	for _, tc := range toolCalls {
		names = append(names, tc.Name)
	}
	return &ReasoningResult{
		Thought: "并行调用工具: " + strings.Join(names, ", "),
		Action: ActionExecuteTool, ToolCalls: toolCalls, TextContent: textContent, IsFinal: false,
	}
}

func (a ActionType) String() string {
	switch a {
	case ActionChat:
		return "chat"
	case ActionExecuteTool:
		return "execute_tool"
	case ActionFinish:
		return "finish"
	default:
		return "unknown"
	}
}

func (a *Agent) callLLMForTool() ([]ToolCallItem, string, error) {
	toolDefs := descriptorsToToolDefs(a.toolRegistry.Descriptors())

	var items []ToolCallItem
	var textContent string

	err := withRetry(a.getCtx(), func() error {
		messages := a.ctxMgr.Build(true)
		if a.transformContext != nil {
			messages = a.transformContext(messages)
		}

		tokenCh, errCh := a.llmClient.ChatStream(a.getCtx(), messages, toolDefs)
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

		ctxChars := 0
		for _, m := range messages {
			ctxChars += len(m.Content) + len(m.Role)
		}
		a.AddTokens(ctxChars/4, 0)

		for token := range tokenCh {
			if token.Content != "" {
				if textBuf.Len() == 0 && a.phaseFn != nil {
					a.phaseFn("Reasoning")
				}
				textBuf.WriteString(token.Content)
				if a.streamFn != nil {
					a.streamFn(token.Content, false)
				}
				a.AddTokens(0, 1)
			}
			if token.Usage != nil && (token.Usage.PromptTokens > 0 || token.Usage.CompletionTokens > 0) {
				a.ResetTokens()
				a.AddTokens(token.Usage.PromptTokens, token.Usage.CompletionTokens)
			}
			if token.ReasoningContent != "" {
				reasoningBuf.WriteString(token.ReasoningContent)
				writeAgentLog("ReasoningContent[%d]: %q", len(token.ReasoningContent), token.ReasoningContent)
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
				return err
			}
		default:
		}

		textContent = tools.StripAnsi(textBuf.String())
		if reasoningBuf.Len() > 0 {
			a.lastReasoningContent = reasoningBuf.String()
		}

		if len(tcAccum) == 0 {
			return nil
		}

		var toolCalls []llm.ToolCall
		for i := 0; i < len(tcAccum); i++ {
			acc := tcAccum[i]
			if acc == nil {
				continue
			}
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   acc.id,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      acc.name,
					Arguments: acc.args.String(),
				},
			})
		}

		items = make([]ToolCallItem, 0, len(toolCalls))
		for _, tc := range toolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return fmt.Errorf("解析工具参数失败: %v", err)
			}
			items = append(items, ToolCallItem{
				ID: tc.ID, Name: tc.Function.Name, Args: args,
			})
		}

		a.ctxMgr.AddAssistantToolCall(textContent, a.lastReasoningContent, toolCalls)
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return items, textContent, nil
}

type toolAccum struct {
	id   string
	name string
	args strings.Builder
}

func (a *Agent) forceSynthesize() string {
	var text string
	err := withRetry(a.getCtx(), func() error {
		messages := a.ctxMgr.Build(false)
		messages = append(messages, llm.Message{
			Role: "user", Content: a.synthesizePrompt,
		})
		tokenCh, errCh := a.llmClient.ChatStream(a.getCtx(), messages, nil)
		if tokenCh == nil {
			select {
			case err := <-errCh:
				return err
			default:
				return fmt.Errorf("chat stream failed")
			}
		}

		var textBuf strings.Builder
		ctxChars := 0
		for _, m := range messages {
			ctxChars += len(m.Content) + len(m.Role)
		}
		a.AddTokens(ctxChars/4, 0)

		for token := range tokenCh {
			if token.Content != "" {
				textBuf.WriteString(token.Content)
				if a.streamFn != nil {
					a.streamFn(token.Content, false)
				}
				a.AddTokens(0, 1)
			}
			if token.Usage != nil && (token.Usage.PromptTokens > 0 || token.Usage.CompletionTokens > 0) {
				a.ResetTokens()
				a.AddTokens(token.Usage.PromptTokens, token.Usage.CompletionTokens)
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
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return "已中断"
		}
		return "抱歉，信息收集完成但总结失败"
	}
	if text != "" {
		return text
	}
	return "抱歉，无法生成总结"
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
