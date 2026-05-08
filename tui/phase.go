// phase.go — 处理阶段常量 + setPhase 状态切换。
package tui

import "primusbot/bot/types"

// Processing phases displayed in the status line during agent execution.
const (
	phaseSteer = "Processing new input..."
	PhaseReady     = types.PhaseReady
	PhaseWaiting   = types.PhaseWaiting
	PhaseThinking  = types.PhaseThinking
	PhaseReasoning = types.PhaseReasoning
	PhaseRunning   = types.PhaseRunning
)

// setPhase is the single entry point for changing the processing phase.
func (m *Model) setPhase(p string) {
	if m.processingPhase == phaseSteer && p == PhaseWaiting {
		return
	}
	m.processingPhase = p
}
