// confirm.go — 用户确认、阶段回调类型和常量。
package tools

type ConfirmRequest struct {
	ToolName string
	Args     map[string]interface{}
	Level    DangerLevel
	Response chan bool
}

type ConfirmFunc func(req ConfirmRequest) bool
type PhaseFunc func(phase string)

// Phase constants — emitted by agent, displayed by TUI status line.
const (
	PhaseReady     = "Ready"
	PhaseWaiting   = "Waiting"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseRunning   = "Running"
)
