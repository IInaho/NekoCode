// confirm.go — user confirmation and phase callback types.
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
