package agent

// ActionResult is returned by Feedback to describe the outcome of a tool execution step.
type ActionResult struct {
	Thought string
	Action  ActionType
	Output  string
	Error   string
	IsFinal bool
}
