// 反馈状态机：每轮 Agent 循环结束后更新记忆、推进步骤计数、
// 构建下一步 stepState，返回 shouldRetry / shouldStop 控制循环。
package agent

import (
	"time"
)

func (a *Agent) Feedback(state *stepState, result *ActionResult) (*stepState, bool, bool) {
	a.memory.Add(MemoryItem{
		Step:      a.currentStep,
		Thought:   result.Thought,
		Action:    result.Action.String(),
		Output:    result.Output,
		Timestamp: time.Now(),
	})

	a.currentStep++

	shouldStop := result.IsFinal || a.currentStep >= a.maxIterations
	shouldRetry := result.ShouldRetry && a.currentStep < a.maxIterations

	newState := &stepState{
		input:          state.input,
		previousAction: result.Action.String(),
		previousOutput: result.Output,
		success:        result.Error == "",
	}
	if result.Error != "" && result.ShouldRetry {
		newState.retryCount = state.retryCount + 1
	}

	return newState, shouldRetry, shouldStop
}
