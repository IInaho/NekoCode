package agent

import (
	"strings"
)

type RunResult struct {
	FinalOutput string
	Error       error
	Steps       int
}

type RunCallback func(step int, thought, action, toolName, toolArgs, output string)

type ToolCallInfo struct {
	Name string
	Args map[string]interface{}
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	a.Reset()

	state := a.Perceive(input)

	for !a.finished && a.currentStep < a.maxIterations {
		reasoning := a.Reason(state)

		result := a.Execute(reasoning)

		toolName, toolArgs := parseToolCall(reasoning.ActionInput)
		if callback != nil {
			callback(a.currentStep, reasoning.Thought, reasoning.Action.String(), toolName, toolArgs, result.Output)
		}

		feedback := a.Feedback(state, result)

		if feedback.ShouldStop {
			a.finished = true
			return &RunResult{
				FinalOutput: result.Output,
				Steps:       a.currentStep,
			}
		}

		if feedback.ShouldRetry {
			state = feedback.UpdatedState
			continue
		}

		state = feedback.UpdatedState
	}

	if a.currentStep >= a.maxIterations {
		return &RunResult{
			FinalOutput: "达到最大迭代次数",
			Error:       nil,
			Steps:       a.currentStep,
		}
	}

	return &RunResult{
		FinalOutput: "未知错误",
		Steps:       a.currentStep,
	}
}

func parseToolCall(actionInput string) (string, string) {
	if actionInput == "" {
		return "", ""
	}
	parts := strings.SplitN(actionInput, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}
