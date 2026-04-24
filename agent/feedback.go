package agent

import (
	"time"
)

type FeedbackResult struct {
	UpdatedState *PerceptionResult
	UIUpdate     UIUpdate
	ShouldRetry  bool
	ShouldStop   bool
}

type UIUpdate struct {
	MessageType string
	Content     string
	Stream      bool
}

func (a *Agent) Feedback(state *PerceptionResult, result *ActionResult) *FeedbackResult {
	a.memory.Add(MemoryItem{
		Step:      a.currentStep,
		Thought:   result.Thought,
		Action:    result.Action.String(),
		Output:    result.Output,
		Timestamp: time.Now(),
	})

	a.currentStep++

	context := state.Context
	if context == nil {
		context = make(map[string]interface{})
	}

	newState := &PerceptionResult{
		InputType: InputTypeText,
		Intent:    "observation",
		Entities: map[string]interface{}{
			"previous_action": result.Action.String(),
			"previous_output": result.Output,
			"success":         result.Error == "",
		},
		Context: context,
	}

	shouldStop := result.IsFinal || a.currentStep >= a.maxIterations
	if result.Error != "" && result.ShouldRetry {
		retryCount := 0
		if rc, ok := newState.Context["retry_count"].(int); ok {
			retryCount = rc
		}
		newState.Context["retry_count"] = retryCount + 1
	}

	shouldRetry := result.ShouldRetry && a.currentStep < a.maxIterations

	return &FeedbackResult{
		UpdatedState: newState,
		UIUpdate: UIUpdate{
			MessageType: "system",
			Content:     result.Thought,
			Stream:      false,
		},
		ShouldRetry: shouldRetry,
		ShouldStop:  shouldStop,
	}
}
