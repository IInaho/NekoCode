package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nekocode/bot/tools"
	"nekocode/llm"
)

type ActionType int

const (
	ActionChat ActionType = iota
	ActionExecuteTool
)

const maxTokensCeiling = 64000 // max_tokens limit for finish_reason=length escalation

type ToolCallItem struct {
	ID   string
	Name string
	Args map[string]interface{}
}

type ReasoningResult struct {
	Thought     string
	Action      ActionType
	ActionInput string
	ToolCalls   []ToolCallItem
	TextContent string
	Interrupted bool // context was canceled mid-stream (steer, not abort)
}

func (a *Agent) Reason(state *stepState) *ReasoningResult {
	// Drain any BTW steering messages that arrived since the last loop-top drain,
	// minimizing the race window between drainSteering and the LLM call.
	a.drainSteering()

	if a.phaseFn != nil {
		a.phaseFn(tools.PhaseThinking)
	}
		// Skip pure slash commands (no arguments), but let commands with
		// content through — e.g., /kami generate a resume is a skill invocation.
	if strings.HasPrefix(state.Input, "/") && !strings.Contains(state.Input, " ") {
		return &ReasoningResult{
			Thought: "User entered a command", Action: ActionChat,
		}
	}

	toolCalls, textContent, err := a.callLLMForTool()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &ReasoningResult{
				Thought: "User interrupted", Action: ActionChat,
				Interrupted: true,
			}
		}
		// If we have partial text, surface it so the user isn't left with
		// nothing — but mark it clearly as truncated.
		if textContent != "" && !isGarbledToolCall(textContent) {
			return &ReasoningResult{
				Thought: "Truncated reply", Action: ActionChat,
				ActionInput: textContent,
			}
		}
		return &ReasoningResult{
			Thought: "LLM call failed", Action: ActionChat,
			ActionInput: fmt.Sprintf("调用失败了，原因：%v。可以再试一次吗？", err),
		}
	}

	if len(toolCalls) == 0 {
		if textContent == "" {
			textContent = "Sorry, I couldn't determine what to do"
		}
		return &ReasoningResult{
			Thought: "Direct reply", Action: ActionChat,
			ActionInput: textContent,
		}
	}

	if len(toolCalls) == 1 {
		tc := toolCalls[0]
		return &ReasoningResult{
			Thought: "Call tool: " + tc.Name, Action: ActionExecuteTool,
			ActionInput: tc.Name + ":" + formatArgs(tc.Args),
			ToolCalls: toolCalls, TextContent: textContent,
		}
	}

	var names []string
	for _, tc := range toolCalls {
		names = append(names, tc.Name)
	}
	return &ReasoningResult{
		Thought: "Parallel tool calls: " + strings.Join(names, ", "),
		Action:  ActionExecuteTool, ToolCalls: toolCalls, TextContent: textContent,
	}
}

func (a ActionType) String() string {
	switch a {
	case ActionChat:
		return "chat"
	case ActionExecuteTool:
		return "execute_tool"

	default:
		return "unknown"
	}
}

func (a *Agent) callLLMForTool() ([]ToolCallItem, string, error) {
	toolDefs := tools.ToToolDefs(a.toolRegistry.Descriptors())

	var items []ToolCallItem
	var textContent string

	firstAttempt := true
	savedMaxTokens := a.llmClient.MaxTokens()
	thinkingDisabled := false
	err := withRetry(a.getCtx(), func() error {
		// Proactive auto-compact before every LLM call.
		a.ctxMgr.AutoCompactIfNeeded(a.ctxMgr.GetAutoCompactConfig(), nil)
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
		estPrompt := ctxChars / 4
		estCompl := 0
		if firstAttempt {
			a.AddTokens(estPrompt, 0)
			firstAttempt = false
		}

		finishReason := ""
		phaseThink := true
		firstReasoning := true
		for token := range tokenCh {
			if firstReasoning && token.ReasoningContent != "" {
				firstReasoning = false
				if a.phaseFn != nil {
					a.phaseFn(tools.PhaseThinking)
				}
			}
			if phaseThink && token.Content != "" {
				phaseThink = false
				if a.phaseFn != nil {
					a.phaseFn(tools.PhaseReasoning)
				}
			}
			if token.Content != "" {
				textBuf.WriteString(token.Content)
				if a.streamFn != nil {
					a.streamFn(token.Content, false)
				}
				estCompl++
				a.AddTokens(0, 1)
			}
			if token.Usage != nil && (token.Usage.PromptTokens > 0 || token.Usage.CompletionTokens > 0) {
				a.AddTokens(token.Usage.PromptTokens-estPrompt, token.Usage.CompletionTokens-estCompl)
				a.ctxMgr.RecordUsage(token.Usage.PromptTokens, token.Usage.CompletionTokens)
			}
			if token.FinishReason != "" {
				finishReason = token.FinishReason
			}
			if token.ReasoningContent != "" {
				reasoningBuf.WriteString(token.ReasoningContent)
				if a.reasoningFn != nil {
					a.reasoningFn(token.ReasoningContent)
				}
				estCompl++
				a.AddTokens(0, 1)
				writeAgentLog("ReasoningContent[%d]: %q", len(token.ReasoningContent), token.ReasoningContent)
			}
			if token.ToolCallDelta != nil {
				// Switch to Reasoning on first tool call (text-less
				// streams like write-only would otherwise stay stuck).
				if phaseThink {
					phaseThink = false
					if a.phaseFn != nil {
						a.phaseFn(tools.PhaseReasoning)
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
				// Tool call arguments are completion tokens.
				estCompl++
				a.AddTokens(0, 1)
			}
		}

		// Two-tier escalation for finish_reason=length:
		//   Tier 1: double max_tokens to maxTokensCeiling and retry.
		//   Tier 2: if already at maxTokensCeiling, disable thinking to stop
		//            reasoning from eating the output budget.
		//   After both tiers exhausted, return partial text + error
		//   so callers can surface a non-garbled summary to the user.
		if finishReason == "length" && len(tcAccum) == 0 {
			if a.llmClient.MaxTokens() < maxTokensCeiling {
				writeAgentLog("callLLM: finish_reason=length, escalating max_tokens %d→%d", a.llmClient.MaxTokens(), maxTokensCeiling)
				a.llmClient.SetMaxTokens(maxTokensCeiling)
				return fmt.Errorf("output token limit hit, retrying with higher limit")
			}
			writeAgentLog("callLLM: finish_reason=length at max_tokens=%d, disabling thinking", maxTokensCeiling)
			a.llmClient.SetDisableThinking(true)
			thinkingDisabled = true
			return fmt.Errorf("output limit still hit at %d, retrying with thinking disabled", maxTokensCeiling)
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
				return fmt.Errorf("failed to parse tool arguments: %v", err)
			}
			items = append(items, ToolCallItem{
				ID: tc.ID, Name: tc.Function.Name, Args: args,
			})
		}

		a.ctxMgr.AddAssistantToolCall(textContent, a.lastReasoningContent, toolCalls)
		return nil
	})
	// Restore original state — escalations are per-call tactics, not permanent.
	a.llmClient.SetMaxTokens(savedMaxTokens)
	if thinkingDisabled {
		a.llmClient.SetDisableThinking(false)
	}
	if err != nil {
		return nil, textContent, err
	}
	return items, textContent, nil
}

// isGarbledToolCall detects raw XML/JSON tool-call fragments that leak
// into text output when the model's function-calling output was truncated.
func isGarbledToolCall(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	// Raw XML invoke/tool_call pattern leaked into text output.
	if strings.Contains(t, "<invoke") || strings.Contains(t, "</invoke") ||
		strings.Contains(t, "<parameter") || strings.Contains(t, "</parameter") ||
		strings.Contains(t, "<tool_call") || strings.Contains(t, "</tool_call") {
		return true
	}
	// Raw JSON tool_call pattern.
	if strings.Contains(t, `"tool_calls"`) || strings.Contains(t, `"tool_use"`) {
		return true
	}
	return false
}

type toolAccum struct {
	id   string
	name string
	args strings.Builder
}

func (a *Agent) forceSynthesize() string {
	text := a.trySynthesize()
	if text != "" {
		return text
	}
	// Emergency fallback: all retries exhausted with no output.
	// Try one last time with aggressively compacted context.
	writeAgentLog("forceSynthesize: primary path failed, attempting emergency fallback")
	if fb := a.emergencySynthesize(); fb != "" {
		return fb
	}
	return "Unable to generate summary"
}

// trySynthesize attempts to get a final summary from the LLM with full context.
// Returns empty string on failure after all retries.
func (a *Agent) trySynthesize() string {
	var text string
	firstAttempt := true
	savedMaxTokens := a.llmClient.MaxTokens()
	thinkingDisabled := false
	err := withRetry(a.getCtx(), func() error {
		a.ctxMgr.AutoCompactIfNeeded(a.ctxMgr.GetAutoCompactConfig(), nil)
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
		estPrompt := ctxChars / 4
		estCompl := 0
		if firstAttempt {
			a.AddTokens(estPrompt, 0)
			firstAttempt = false
		}

		finishReason := ""
		for token := range tokenCh {
			if token.Content != "" {
				textBuf.WriteString(token.Content)
				if a.streamFn != nil {
					a.streamFn(token.Content, false)
				}
				estCompl++
				a.AddTokens(0, 1)
			}
			if token.Usage != nil && (token.Usage.PromptTokens > 0 || token.Usage.CompletionTokens > 0) {
				a.AddTokens(token.Usage.PromptTokens-estPrompt, token.Usage.CompletionTokens-estCompl)
				a.ctxMgr.RecordUsage(token.Usage.PromptTokens, token.Usage.CompletionTokens)
			}
			if token.FinishReason != "" {
				finishReason = token.FinishReason
			}
		}

		// Two-tier escalation for finish_reason=length.
		if finishReason == "length" {
			if a.llmClient.MaxTokens() < maxTokensCeiling {
				writeAgentLog("forceSynthesize: finish_reason=length, escalating max_tokens %d→%d", a.llmClient.MaxTokens(), maxTokensCeiling)
				a.llmClient.SetMaxTokens(maxTokensCeiling)
				return fmt.Errorf("output token limit hit, retrying with higher limit")
			}
			writeAgentLog("forceSynthesize: finish_reason=length at max_tokens=%d, disabling thinking", maxTokensCeiling)
			a.llmClient.SetDisableThinking(true)
			thinkingDisabled = true
			return fmt.Errorf("output limit still hit at %d, retrying with thinking disabled", maxTokensCeiling)
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
	a.llmClient.SetMaxTokens(savedMaxTokens)
	if thinkingDisabled {
		a.llmClient.SetDisableThinking(false)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return ""
		}
		if text != "" && !isGarbledToolCall(text) {
			return text
		}
		return ""
	}
	return text
}

// emergencySynthesize is a last-resort attempt with minimal context.
// No retries — if this fails, we surface the error to the user.
func (a *Agent) emergencySynthesize() string {
	// Aggressively compact: clear ALL old tool results, force summarize.
	a.ctxMgr.MicroCompactIfNeeded()
	a.ctxMgr.ForceCompact()
	// Build minimal context: system prompt + anchor + last 3 user turns + prompt.
	msgs := a.ctxMgr.BuildMinimal()
	msgs = append(msgs, llm.Message{
		Role: "user", Content: a.synthesizePrompt,
	})

	ctx, cancel := context.WithTimeout(a.getCtx(), 30*time.Second)
	defer cancel()

	tokenCh, errCh := a.llmClient.ChatStream(ctx, msgs, nil)
	if tokenCh == nil {
		return ""
	}

	var textBuf strings.Builder
	for token := range tokenCh {
		if token.Content != "" {
			textBuf.WriteString(token.Content)
			if a.streamFn != nil {
				a.streamFn(token.Content, false)
			}
		}
	}
	select {
	case <-errCh:
	default:
	}
	text := tools.StripAnsi(textBuf.String())
	if isGarbledToolCall(text) {
		return ""
	}
	return text
}
