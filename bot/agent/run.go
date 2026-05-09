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
	lowOutputTurns int // consecutive turns with minimal output
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
		if a.getCtx().Err() != nil {
			if callback != nil {
				callback(a.currentStep, "done", "chat", "", "", "Interrupted", 0, 0)
			}
			return &RunResult{FinalOutput: "Interrupted", Steps: a.currentStep}
		}

		reasoning := a.Reason(state)

		if reasoning.Interrupted {
			if a.finished {
				writeAgentLog("Run: interrupted + finished → abort")
				if callback != nil {
					callback(a.currentStep, "done", "chat", "", "", "Interrupted", 0, 0)
				}
				return &RunResult{FinalOutput: "Interrupted", Steps: a.currentStep}
			}
			writeAgentLog("Run: interrupted → draining steering")
			a.drainSteering()
			writeAgentLog("Run: steering drained, continuing")
			continue
		}

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

	// maxIterations reached — force synthesize, unless aborted.
	if a.getCtx().Err() != nil {
		return &RunResult{FinalOutput: "Interrupted", Steps: a.currentStep}
	}
	output := a.forceSynthesize()
	a.ctxMgr.AddAssistantResponse(output, "")
	if callback != nil {
		callback(a.currentStep, "done", "chat", "", "", output, 0, 0)
	}
	return &RunResult{FinalOutput: output, Steps: a.currentStep}
}

func (a *Agent) collectCalls(reasoning *ReasoningResult) []tools.ToolCallItem {
	if len(reasoning.ToolCalls) > 0 {
		out := make([]tools.ToolCallItem, len(reasoning.ToolCalls))
		for i, tc := range reasoning.ToolCalls {
			out[i] = tools.ToolCallItem{ID: tc.ID, Name: tc.Name, Args: tc.Args}
		}
		return out
	}
	return nil
}

func (a *Agent) executeAndFeedback(calls []tools.ToolCallItem, reasoning *ReasoningResult, state *stepState, callback RunCallback) *stepState {
	// Surface LLM thinking text between tool calls.
	if reasoning.TextContent != "" && callback != nil {
		callback(a.currentStep, reasoning.Thought, "think", "", "", reasoning.TextContent, 0, 0)
	}

	// Signal tool starts before execution so slow tools (task/subagent)
	// display immediately in the TUI rather than after they complete.
	if callback != nil {
		for i, c := range calls {
			toolArgs := ""
			if i < len(calls) {
				toolArgs = formatArgs(calls[i].Args)
			}
			callback(a.currentStep, reasoning.Thought, "tool_start", c.Name, toolArgs, "", i+1, len(calls))
		}
	}
	results := a.executor.ExecuteBatch(a.getCtx(), calls)

	if a.doomLoopCheck(state, calls) {
		output := a.forceSynthesize()
		a.ctxMgr.AddAssistantResponse(output, "")
		if callback != nil {
			callback(a.currentStep, "done", "chat", "", "", output, 0, 0)
		}
		a.finished = true
		return state
	}

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

	done := a.detectDiminishingReturns(state, results, calls)
	result := &ActionResult{Thought: reasoning.Thought, Action: ActionExecuteTool, IsFinal: done}
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
	shouldRetry := false

	newState := &stepState{
		Input:          state.Input,
		PreviousAction: result.Action.String(),
		PreviousOutput: result.Output,
		Success:        result.Error == "",
		SearchCount:    state.SearchCount,
		FetchCount:     state.FetchCount,
		lowOutputTurns: state.lowOutputTurns,
	}

	return newState, shouldRetry, shouldStop
}

// doomLoopCheck detects 3+ consecutive identical tool calls and forces stop.
func (a *Agent) doomLoopCheck(state *stepState, calls []tools.ToolCallItem) bool {
	if len(calls) != 1 {
		a.doomLoopHistory = nil
		return false
	}
	key := calls[0].Name + "|" + formatArgs(calls[0].Args)
	a.doomLoopHistory = append(a.doomLoopHistory, key)
	if len(a.doomLoopHistory) > 3 {
		a.doomLoopHistory = a.doomLoopHistory[1:]
	}
	if len(a.doomLoopHistory) == 3 {
		for i := 1; i < 3; i++ {
			if a.doomLoopHistory[i] != a.doomLoopHistory[0] {
				return false
			}
		}
		writeAgentLog("doomLoop: 3 identical calls to %s — forcing synthesis", calls[0].Name)
		return true
	}
	return false
}

// detectDiminishingReturns checks if tool output is stagnating.
// Called from executeAndFeedback after collecting results.
func (a *Agent) detectDiminishingReturns(state *stepState, results []tools.ToolCallResult, calls []tools.ToolCallItem) bool {
	// Skip coordination tools (task, todo_write) — their output is naturally short.
	isCoord := func(name string) bool { return name == "task" || name == "todo_write" }

	totalOut := 0
	hasContent := false
	for i, r := range results {
		if isCoord(calls[i].Name) {
			continue
		}
		hasContent = true
		totalOut += len(r.Output)
	}
	if !hasContent {
		return false // all tools were coordination — don't count as low output
	}

	const minOutput = 200
	if totalOut < minOutput {
		state.lowOutputTurns++
		if state.lowOutputTurns >= 3 {
			writeAgentLog("Feedback: stop — 3 consecutive low-output turns (<%d chars)", minOutput)
			return true
		}
	} else {
		state.lowOutputTurns = 0
	}
	return false
}
