// confirm.go — 用户确认、阶段回调、todo 和 snip 类型和常量。
package tools

type ConfirmRequest struct {
	ToolName string
	Args     map[string]interface{}
	Level    DangerLevel
	Response chan bool
}

type ConfirmFunc func(req ConfirmRequest) bool
type PhaseFunc func(phase string)

// TodoItem represents a single task in the agent's todo list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoFunc is called whenever the todo list is updated.
type TodoFunc func(items []TodoItem)

// SnipFunc is called to remove messages from context.
type SnipFunc func(startIdx, endIdx int) string

// Phase constants — emitted by agent, displayed by TUI status line.
const (
	PhaseReady     = "Ready"
	PhaseWaiting   = "Waiting"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseRunning   = "Running"
)
