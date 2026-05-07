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
	// Don't let the agent override BTW feedback with "Thinking".
	// "Processing new input..." persists until agent transitions to Reasoning or Running.
	if m.processingPhase == "Processing new input..." && p == PhaseThinking {
		return
	}
	m.processingPhase = p
}
