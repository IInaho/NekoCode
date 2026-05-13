package agent

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"nekocode/bot/tools"
	"nekocode/llm"
)

const (
	doomLoopWindow   = 4 // consecutive identical batches to trigger doom loop
	readOnlyMaxTurns = 6 // consecutive exploration-only turns before synthesis injection
)

// explorationTools are tools that only gather information without taking action.
var explorationTools = map[string]bool{
	"read": true, "grep": true, "glob": true, "list": true,
	"web_search": true, "web_fetch": true,
}

type RunResult struct {
	FinalOutput string
	Error       error
	Steps       int
}

type RunCallback func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)

type stepState struct {
	Input          string
	SearchCount    int
	FetchCount     int
	readOnlyTurns   int  // consecutive turns with only exploration tools
	filesModified   bool // write/edit used this turn
	didReproduce    bool // bash command produced an actual failure
	verifyInjected  bool // already asked model to verify, don't ask again
	needsVerification bool // non-trivial changes made, verification required before reporting done
}

// synthesizeAndReturn force-synthesizes a final response, logs it to context,
// fires the callback, and returns a RunResult. Used by all stop-condition paths.
func (a *Agent) synthesizeAndReturn(callback RunCallback) *RunResult {
	output := a.forceSynthesize()
	a.ctxMgr.AddAssistantResponse(output, "")
	if callback != nil {
		callback(a.currentStep, "done", "chat", "", "", output, 0, 0)
	}
	return &RunResult{FinalOutput: output, Steps: a.currentStep}
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	a.Reset()
	a.ctxMgr.Add("user", input)

	state := &stepState{Input: input}

	// Main loop: runs until a stop condition triggers (doom loop, diminishing
	// returns, budget pressure, user abort, shouldStop hook).
	for !a.finished {
		// --- Stop condition: configurable shouldStop hook ---
		if a.currentStep > 0 && a.shouldStop != nil {
			ctxTokens, tokenBudget := a.ctxMgr.TokenUsage()
			budgetPressure := tokenBudget > 0 && float64(ctxTokens) >= float64(tokenBudget)*budgetPressureRatio

			if a.shouldStop(StopInfo{
				Step:             a.currentStep,
				State:            state,
				TokensUsed:       ctxTokens,
				TokenBudget:      tokenBudget,
				BudgetPressure:   budgetPressure,
				ConsecutiveTurns: a.diminishingStreak,
			}) {
				a.stopReason = StopHookPrevented
				return a.synthesizeAndReturn(callback)
			}
		}

		// --- Stop condition: user abort / context canceled ---
		a.drainSteering()
		if a.getCtx().Err() != nil {
			a.stopReason = StopInterrupted
			if callback != nil {
				callback(a.currentStep, "done", "chat", "", "", "Interrupted", 0, 0)
			}
			return &RunResult{FinalOutput: "Interrupted", Steps: a.currentStep}
		}

		// --- Budget pressure injection (Claude Code pattern) ---
		// When context exceeds 80% of the token budget, inject a meta-message
		// urging the model to synthesize rather than continue exploring.
		// Only inject once per run to avoid spamming.
		if !a.budgetPressureInjected {
			ctxTokens, tokBudget := a.ctxMgr.TokenUsage()
			if tokBudget > 0 && float64(ctxTokens) >= float64(tokBudget)*budgetPressureRatio {
				a.budgetPressureInjected = true
				writeAgentLog("budgetPressure: %d/%d tokens (%.0f%%) — injecting synthesis prompt",
					ctxTokens, tokBudget, float64(ctxTokens)/float64(tokBudget)*100)
				a.ctxMgr.Add("user", "[System] Context is filling up. Prioritize finishing your current task — if you have enough information, synthesize your findings. Avoid starting new exploration threads.")
			}
		}

		// --- LLM call ---
		reasoning := a.Reason(state)

		if reasoning.Interrupted {
			if a.finished {
				a.stopReason = StopInterrupted
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

		// --- Tool execution path ---
		calls := a.collectCalls(reasoning)
		if len(calls) > 0 {
			var stopReason StopReason
			var shouldStop bool
			state, shouldStop, stopReason = a.executeAndFeedback(calls, reasoning, state, callback)
			if shouldStop {
				a.finished = true
				a.stopReason = stopReason
				return a.synthesizeAndReturn(callback)
			}
			continue
		}

		// --- Text response path (no tools) ---
		// Verification contract (research §3 layer 5 + §7.3):
		// After non-trivial changes, independent adversarial verification
		// must happen before reporting completion. The model's own checks
		// and a subagent's self-checks do NOT substitute.
		if state.needsVerification && !state.verifyInjected {
			state.verifyInjected = true
			if !state.didReproduce {
				a.ctxMgr.Add("user",
					"[System] You modified files. Did you reproduce the bug first? If not, re-run the failing command. Then: build + test. After that, spawn task(verify) for independent adversarial verification before reporting done.")
			} else {
				a.ctxMgr.Add("user",
					"[System] You modified files. Before reporting done: build + test, then spawn task(verify) for independent adversarial verification. Your own checks are not sufficient — a separate verifier must confirm.")
			}
			a.diminishingStreak = 0
			continue
		}

		// Modified files but verification already handled, or trivial changes.
		if state.filesModified && !state.needsVerification && !state.verifyInjected {
			state.verifyInjected = true
			a.ctxMgr.Add("user",
				"[System] You modified files. Build + test to confirm, then report.")
			a.diminishingStreak = 0
			continue
		}

		// Diminishing returns only applies when the model has used tools —
		// pure chat with naturally short replies should not be penalized.
		if a.currentStep == 0 {
			// First turn, no tools used yet — accept short replies as normal.
			a.finished = true
			a.stopReason = StopCompleted
			a.ctxMgr.AddAssistantResponse(reasoning.ActionInput, a.lastReasoningContent)
			if callback != nil {
				callback(a.currentStep, reasoning.Thought, reasoning.Action.String(), "", "", reasoning.ActionInput, 0, 0)
			}
			return &RunResult{FinalOutput: reasoning.ActionInput, Steps: a.currentStep}
		}
		turnTokens := a.tokenCompletion.Load() - a.lastTurnTokens
		a.lastTurnTokens = a.tokenCompletion.Load()
		if turnTokens < minCompletionTokens {
			a.diminishingStreak++
			if a.diminishingStreak >= diminishingThreshold {
				writeAgentLog("diminishingReturns: %d consecutive low-token text-only turns (last: %d tokens) — stopping",
					a.diminishingStreak, turnTokens)
				a.stopReason = StopDiminishingReturns
				return a.synthesizeAndReturn(callback)
			}
			continue
		}
		a.diminishingStreak = 0

		// Normal completion: model returned substantial text without tool calls.
		a.finished = true
		a.stopReason = StopCompleted
		if reasoning.Action == ActionChat {
			a.ctxMgr.AddAssistantResponse(reasoning.ActionInput, a.lastReasoningContent)
		}
		if callback != nil {
			callback(a.currentStep, reasoning.Thought, reasoning.Action.String(), "", "", reasoning.ActionInput, 0, 0)
		}
		return &RunResult{FinalOutput: reasoning.ActionInput, Steps: a.currentStep}
	}

	// Loop exited without finishing normally — force synthesize.
	if a.getCtx().Err() != nil {
		a.stopReason = StopInterrupted
		return &RunResult{FinalOutput: "Interrupted", Steps: a.currentStep}
	}
	a.stopReason = StopDiminishingReturns
	return a.synthesizeAndReturn(callback)
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

func (a *Agent) executeAndFeedback(calls []tools.ToolCallItem, reasoning *ReasoningResult, state *stepState, callback RunCallback) (*stepState, bool, StopReason) {
	if reasoning.TextContent != "" && callback != nil {
		callback(a.currentStep, reasoning.Thought, "think", "", "", reasoning.TextContent, 0, 0)
	}

	if callback != nil {
		for i, c := range calls {
			callback(a.currentStep, reasoning.Thought, "tool_start", c.Name, formatArgs(c.Args), "", i+1, len(calls))
		}
	}
	results := a.executor.ExecuteBatch(a.getCtx(), calls)

	// Track file modifications only on success — failed attempts
	// (read-before-write errors, permission issues) don't count.
	for i, r := range results {
		if r.Error == "" && i < len(calls) {
			switch calls[i].Name {
			case "write", "edit":
				state.filesModified = true
				state.needsVerification = true
			}
		}
	}

	// --- Stop condition: doom loop (identical tool call batches) ---
	if a.doomLoopCheck(calls) {
		return state, true, StopDoomLoop
	}

	msgs := make([]llm.Message, 0, len(results))
	for i, r := range results {
		content := r.Output
		if r.Error != "" {
			content = r.Error
		}

		if i < len(calls) && calls[i].Name == "bash" && hasFailedOutput(r.Output, r.Error) {
			state.didReproduce = true
		}

		msgs = append(msgs, llm.Message{Content: content, ToolCallID: r.ID})
		if callback != nil {
			callback(a.currentStep, reasoning.Thought, "execute_tool", r.Name, formatArgs(calls[i].Args), content, i+1, len(results))
		}
	}
	if len(calls) == 1 && calls[0].ID != "" {
		a.ctxMgr.AddToolResult(calls[0].ID, msgs[0].Content)
	} else {
		a.ctxMgr.AddToolResultsBatch(msgs)
	}

		// Using tools IS progress — reset the diminishing streak regardless of
		// how short the accompanying text was.
		a.diminishingStreak = 0
		a.lastTurnTokens = a.tokenCompletion.Load()

		// --- Read-only spiral detection ---
	if allExploration(calls) {
		state.readOnlyTurns++
	} else {
		state.readOnlyTurns = 0
	}

	if state.readOnlyTurns >= readOnlyMaxTurns {
		writeAgentLog("readOnlySpiral: %d exploration turns — injecting synthesis prompt", state.readOnlyTurns)
		a.ctxMgr.Add("user", "[System] You've been exploring for "+strconv.Itoa(state.readOnlyTurns)+" turns without taking action. Summarize your findings and report. If you need more files, explain which and why — but make progress.")
		state.readOnlyTurns = 0
	}

	a.currentStep++
	newState := &stepState{
		Input:            state.Input,
		SearchCount:      state.SearchCount,
		FetchCount:       state.FetchCount,
		readOnlyTurns:    state.readOnlyTurns,
		filesModified:    state.filesModified,
		didReproduce:     state.didReproduce,
		verifyInjected:   state.verifyInjected,
		needsVerification: state.needsVerification,
	}

	for _, tc := range calls {
		switch tc.Name {
		case "web_search":
			newState.SearchCount++
		case "web_fetch":
			newState.FetchCount++
		}
	}
	return newState, false, StopCompleted
}

// allExploration returns true if every call in the batch is a read-only exploration tool.
func allExploration(calls []tools.ToolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		if !explorationTools[c.Name] {
			return false
		}
	}
	return true
}

func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		val := fmt.Sprint(args[k])
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

func (a *Agent) doomLoopCheck(calls []tools.ToolCallItem) bool {
	if len(calls) == 0 {
		return false
	}
	parts := make([]string, len(calls))
	for i, c := range calls {
		parts[i] = classifyCall(c)
	}
	sort.Strings(parts)
	key := strings.Join(parts, ";")
	a.doomLoopHistory = append(a.doomLoopHistory, key)
	if len(a.doomLoopHistory) > doomLoopWindow {
		a.doomLoopHistory = a.doomLoopHistory[1:]
	}
	if len(a.doomLoopHistory) == doomLoopWindow {
		for i := 1; i < doomLoopWindow; i++ {
			if a.doomLoopHistory[i] != a.doomLoopHistory[0] {
				return false
			}
		}
		writeAgentLog("doomLoop: %d identical call batches — forcing synthesis", doomLoopWindow)
		return true
	}
	return false
}

// classifyCall produces a category label for doom loop detection.
// Includes path/command specificity for read/write/edit so different files
// don't get lumped together, but groups by directory to catch true repeats.
func classifyCall(tc tools.ToolCallItem) string {
	switch tc.Name {
	case "bash":
		cmd, _ := tc.Args["command"].(string)
		for _, prefix := range []string{
			"go build", "go test", "go vet", "go run", "go mod", "go fmt",
			"npm ", "npx ", "python", "cargo ", "make ", "just ",
			"git ", "ls ", "cat ", "ps ", "du ", "file ",
		} {
			if strings.HasPrefix(cmd, prefix) {
				return "bash:" + strings.TrimSpace(strings.SplitN(prefix, " ", 2)[0])
			}
		}
		if strings.HasPrefix(cmd, "go ") {
			return "bash:go-other"
		}
		return "bash:other"
	case "read", "write", "edit":
		path, _ := tc.Args["path"].(string)
		// Use parent directory + basename first char to distinguish files
		// without being too granular on minor path variations.
		dir, file := filepath.Split(path)
		parent := filepath.Base(filepath.Clean(dir))
		prefix := ""
		if len(file) > 0 {
			prefix = string(file[0])
		}
		return tc.Name + ":" + parent + "/" + prefix
	case "grep":
		pattern, _ := tc.Args["pattern"].(string)
		// Truncate pattern to avoid too much variance.
		if len(pattern) > 8 {
			pattern = pattern[:8]
		}
		return "grep:" + pattern
	case "glob":
		pattern, _ := tc.Args["pattern"].(string)
		if len(pattern) > 8 {
			pattern = pattern[:8]
		}
		return "glob:" + pattern
	case "list":
		return "list:dir"
	case "web_search":
		return "web:search"
	case "web_fetch":
		return "web:fetch"
	case "task":
		agentType, _ := tc.Args["agent_type"].(string)
		return "task:" + agentType
	case "todo_write":
		return "todo:update"
	default:
		return tc.Name
	}
}

// hasFailedOutput checks whether a bash command result indicates an actual failure.
func hasFailedOutput(output, errStr string) bool {
	if errStr != "" {
		return true
	}
	lower := strings.ToLower(output)
	markers := []string{"panic:", "error:", "fatal:", "signal:", "segmentation fault",
		"stack trace", "goroutine", "traceback", "exit status 1", "exit code"}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
