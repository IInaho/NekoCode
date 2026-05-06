package tui

// Processing phases displayed in the status line during agent execution.
const (
	PhaseReady     = "Ready"
	PhaseThinking  = "Thinking"
	PhaseReasoning = "Reasoning"
	PhaseExecuting = "Running"
)

// setPhase is the single entry point for changing the processing phase.
func (m *Model) setPhase(p string) {
	m.processingPhase = p
}
