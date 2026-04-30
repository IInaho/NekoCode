// Agent 主循环：Reason → Execute → Feedback 三阶段循环，
// 由 stepState 在迭代间传递上下文，maxIterations 控制最大轮次。
// 循环结束将最终回复写入 ctxMgr 供后续对话引用。
package agent

import (
	"fmt"

	"primusbot/bot/tools"
)

type RunResult struct {
	FinalOutput string
	Error       error
	Steps       int
}

type RunCallback func(step int, thought, action, toolName, toolArgs, output string)

type stepState struct {
	input          string
	previousAction string
	previousOutput string
	success        bool
	retryCount     int
}

func (a *Agent) Run(input string, callback RunCallback) *RunResult {
	a.Reset()
	a.ctxMgr.Add("user", input)

	state := &stepState{input: input}

	for !a.finished && a.currentStep < a.maxIterations {
		reasoning := a.Reason(state)

		result := a.Execute(reasoning)

		// Persist tool interaction in conversation history so future turns see it.
		if reasoning.Action == ActionExecuteTool {
			a.ctxMgr.Add("user", "[工具调用: "+reasoning.ActionInput+"]")
			if result.Error != "" {
				a.ctxMgr.Add("user", "[工具错误: "+result.Error+"]")
			} else {
				a.ctxMgr.Add("user", "[工具结果]\n"+result.Output)
			}
		}

		toolName, toolArgs := "", ""
		if name, args, err := tools.ParseCall(reasoning.ActionInput); err == nil {
			toolName = name
			toolArgs = formatArgs(args)
		}
		if callback != nil {
			callback(a.currentStep, reasoning.Thought, reasoning.Action.String(), toolName, toolArgs, result.Output)
		}

		newState, shouldRetry, shouldStop := a.Feedback(state, result)

		if shouldStop {
			a.finished = true
			if result.Action == ActionChat || result.Action == ActionFinish {
				a.ctxMgr.Add("assistant", result.Output)
			}
			return &RunResult{
				FinalOutput: result.Output,
				Steps:       a.currentStep,
			}
		}

		if shouldRetry {
			state = newState
			continue
		}

		state = newState
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

func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}
	s := ""
	for k, v := range args {
		if s != "" {
			s += ","
		}
		s += k + "=" + fmt.Sprint(v)
	}
	return s
}
