// handlers_spin.go — spinner tick、流式消息处理回调。
package tui

import (
	"fmt"
	"time"

	"primusbot/tui/components/processing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m *Model) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	m.Spinner, _ = m.Spinner.Update(msg)

	if m.state == stateConfirming {
		m.Messages.SetSpinnerView("")
		return nil
	}

	if m.state == stateProcessing {
		elapsed := time.Since(m.processingStart)
		statusText := fmt.Sprintf("%s (%.1fs)", m.processingPhase, elapsed.Seconds())
		prompt, compl := m.Bot.TokenUsage()
		if prompt == 0 {
			prompt = m.Bot.ContextTokens()
		}
		spinnerView := m.Spinner.View()
		m.Messages.UpdateProcessing(func(p *processing.ProcessingItem) {
			p.SetSpinnerView(spinnerView)
			p.SetStatusText(statusText)
			p.SetTokens(prompt, compl)
			p.SetCompactCount(m.Bot.CompactCount())
		})

		return spinnerTick()
	}

	return nil
}

func spinnerTick() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return spinner.TickMsg{}
	}
}


