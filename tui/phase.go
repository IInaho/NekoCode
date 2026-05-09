// phase.go — 处理阶段常量 + setPhase 状态切换。
package tui

import "nekocode/bot/tools"

// Processing phases displayed in the status line during agent execution.
const (
	phaseSteer     = "Processing new input..."
	PhaseReady     = tools.PhaseReady
	PhaseWaiting   = tools.PhaseWaiting
	PhaseThinking  = tools.PhaseThinking
	PhaseReasoning = tools.PhaseReasoning
	PhaseRunning   = tools.PhaseRunning
)

// setPhase is the single entry point for changing the processing phase.
func (m *Model) setPhase(p string) {
	if m.processingPhase == phaseSteer && p == PhaseWaiting {
		return
	}
	m.processingPhase = p
}
