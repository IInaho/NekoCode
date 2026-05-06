package agent

import (
	"fmt"
	"strings"

	"primusbot/bot/tools"
	"primusbot/llm"
)

type RunResult struct {
	FinalOutput string
	Error       error
	Steps       int
}

type RunCallback func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)

type stepState struct {
	Input          string
	PreviousAction string
	PreviousOutput string
	Success        bool
	RetryCount     int
	SearchCount    int
	FetchCount     int
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	a.Reset()
	a.ctxMgr.Add("user", input)

	state := &stepState{Input: input}

	for !a.finished && a.currentStep < a.maxIterations {
		if a.currentStep > 0 && a.shouldStop != nil && a.shouldStop(StopInfo{
			Step: a.currentStep, State: state,
		}) {
			output := a.forceSynthesize()
			a.ctxMgr.AddAssistantResponse(output, "")
			if callback != nil {
				callback(a.currentStep, "done", "chat", "", "", output, 0, 0)
			}
			return &RunResult{FinalOutput: output, Steps: a.currentStep}
		}

		a.drainSteering()
		if a.ctx.Err() != nil {
			if callback != nil {
				callback(a.currentStep, "done", "chat", "", "", "已中断", 0, 0)
			}
			return &RunResult{FinalOutput: "已中断", Steps: a.currentStep}
		}

		reasoning := a.Reason(state)

		calls := a.collectCalls(reasoning)
		if len(calls) > 0 {
			state = a.executeAndFeedback(calls, reasoning, state, callback)
			continue
		}

		a.finished = true
		if reasoning.Action == ActionChat || reasoning.Action == ActionFinish {
			a.ctxMgr.AddAssistantResponse(reasoning.ActionInput, a.lastReasoningContent)
		}
		if callback != nil {
			callback(a.currentStep, reasoning.Thought, reasoning.Action.String(), "", "", reasoning.ActionInput, 0, 0)
		}
		return &RunResult{FinalOutput: reasoning.ActionInput, Steps: a.currentStep}
	}

	// maxIterations reached — force synthesize.
	output := a.forceSynthesize()
	a.ctxMgr.AddAssistantResponse(output, "")
	if callback != nil {
		callback(a.currentStep, "done", "chat", "", "", output, 0, 0)
	}
	return &RunResult{FinalOutput: output, Steps: a.currentStep}
}

func (a *Agent) collectCalls(reasoning *ReasoningResult) []ToolCallItem {
	if len(reasoning.ToolCalls) > 0 {
		return reasoning.ToolCalls
	}
	if reasoning.Action == ActionExecuteTool && reasoning.ActionInput != "" {
		name, args, err := tools.ParseCall(reasoning.ActionInput)
		if err != nil {
			return nil
		}
		return []ToolCallItem{{ID: reasoning.ToolCallID, Name: name, Args: args}}
	}
	return nil
}

func (a *Agent) executeAndFeedback(calls []ToolCallItem, reasoning *ReasoningResult, state *stepState, callback RunCallback) *stepState {
	// Surface LLM thinking text between tool calls.
	if reasoning.TextContent != "" && callback != nil {
		callback(a.currentStep, reasoning.Thought, "think", "", "", reasoning.TextContent, 0, 0)
	}

	results := a.executor.ExecuteBatch(a.ctx, calls)

	msgs := make([]llm.Message, 0, len(results))
	for i, r := range results {
		content := r.Output
		if r.Error != "" {
			content = r.Error
		}
		msgs = append(msgs, llm.Message{Content: content, ToolCallID: r.ID})
		if callback != nil {
			toolArgs := ""
			if i < len(calls) {
				toolArgs = formatArgs(calls[i].Args)
			}
			callback(a.currentStep, reasoning.Thought, "execute_tool", r.Name, toolArgs, content, i+1, len(results))
		}
	}
	if len(calls) == 1 && calls[0].ID != "" {
		a.ctxMgr.AddToolResult(calls[0].ID, msgs[0].Content)
	} else {
		a.ctxMgr.AddToolResultsBatch(msgs)
	}

	result := &ActionResult{Thought: reasoning.Thought, Action: ActionExecuteTool, IsFinal: false}
	newState, _, _ := a.Feedback(state, result)

	for _, tc := range calls {
		switch tc.Name {
		case "web_search":
			newState.SearchCount = state.SearchCount + 1
		case "web_fetch":
			newState.FetchCount = state.FetchCount + 1
		}
	}
	return newState
}

func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range args {
		val := fmt.Sprint(v)
		if strings.ContainsAny(val, ",=\"") {
			val = `"` + strings.ReplaceAll(strings.ReplaceAll(val, "\\", "\\\\"), "\"", "\\\"") + `"`
		}
		pairs = append(pairs, k+"="+val)
	}
	return strings.Join(pairs, ",")
}

func (a *Agent) drainSteering() {
	for {
		select {
		case msg := <-a.steeringCh:
			a.ctxMgr.Add("user", msg)
		default:
			return
		}
	}
}

func (a *Agent) Feedback(state *stepState, result *ActionResult) (*stepState, bool, bool) {
	a.currentStep++
	shouldStop := result.IsFinal || a.currentStep >= a.maxIterations
	shouldRetry := result.ShouldRetry && a.currentStep < a.maxIterations

	newState := &stepState{
		Input:          state.Input,
		PreviousAction: result.Action.String(),
		PreviousOutput: result.Output,
		Success:        result.Error == "",
		SearchCount:    state.SearchCount,
		FetchCount:     state.FetchCount,
	}
	if result.Error != "" && result.ShouldRetry {
		newState.RetryCount = state.RetryCount + 1
	}
	return newState, shouldRetry, shouldStop
}
