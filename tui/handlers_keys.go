// handlers_keys.go — 按键处理 + 确认键 + 调试日志 + suggestion 辅助。
package tui

import (
	"fmt"
	"os"
	"time"

	"nekocode/tui/components/message"

	tea "charm.land/bubbletea/v2"
)

const (
	debugLogPath   = "/tmp/nekocode-debug.log"
	contentMarginV = 2
)

func tuiLog(format string, args ...interface{}) {
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "TUI: "+format+"\n", args...)
}

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.ConfirmBar.Respond(true)
	case "esc", "n", "N", "ctrl+c":
		m.ConfirmBar.Respond(false)
	default:
		return m, nil
	}
	m.state = stateProcessing
	m.resizeMessages()
	return m, tea.Batch(listenConfirm(m.confirmCh), spinnerTick())
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit

	case "ctrl+e":
		if m.state != stateProcessing {
			m.Messages.ToggleLastAssistant()
		}
		return nil

	case "up":
		m.Input.HistoryUp()
		return nil
	case "down":
		m.Input.HistoryDown()
		return nil

	case "pgup", "pgdown":
		m.Messages.Update(msg)
		m.Input.SetFollow(m.Messages.Follow)
		return nil
	}

	if m.state == stateProcessing {
		return m.handleProcessingKey(msg)
	}

	return m.handleIdleKey(msg)
}

func (m *Model) handleProcessingKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		value := m.Input.Value()
		tuiLog("BTW Enter: value=%q len=%d phase=%q", value, len(value), m.processingPhase)
		if value != "" {
			m.Suggestions.Hide()
			m.Input.AddHistory(value)
			m.Input.Reset()
			m.Messages.AddMessage(message.ChatMessage{Role: "user", Content: value})
			m.Messages.ClearProcessing()
			m.Messages.SetBlocks(nil)
			m.processingStart = time.Now()
			m.processingPhase = phaseSteer
			m.Messages.SetProcessingStatus(phaseSteer)
			m.Bot.Steer(value)
			tuiLog("BTW Enter: done, phase=%q", m.processingPhase)
		} else {
			tuiLog("BTW Enter: value empty, skipped")
		}
	case "esc":
		m.Bot.Abort()
		m.Messages.SetProcessingStatus("Aborted")
	case "pgup", "pgdown", "up", "down":
		m.Messages.Update(msg)
		m.Input.SetFollow(false)
	default:
		input, cmd := m.Input.Update(msg)
		m.Input = input
		return cmd
	}
	return nil
}

func (m *Model) handleIdleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "end":
		m.Messages.GotoBottom()
		m.Input.SetFollow(true)
	case "tab":
		m.cycleSuggestion(1)
		return nil
	case "shift+tab":
		m.cycleSuggestion(-1)
		return nil
	case "esc":
	case "enter":
		if m.Suggestions.Visible() {
			m.acceptSuggestion()
			return nil
		}
		value := m.Input.Value()
		if value == "" {
			return nil
		}
		m.Suggestions.Hide()
		m.Input.AddHistory(value)
		m.Input.Reset()
		return m.startChat(value)
	default:
		input, cmd := m.Input.Update(msg)
		m.Input = input
		m.refreshSuggestions()
		return cmd
	}
	return nil
}

func (m *Model) refreshSuggestions() {
	m.Suggestions.Refresh(m.Input.Value(), m.Bot.CommandNames())
}

func (m *Model) acceptSuggestion() {
	if val := m.Suggestions.Accept(); val != "" {
		m.Input.SetValue(val)
		m.Input.SetCursorEnd()
	}
}

func (m *Model) cycleSuggestion(delta int) {
	m.Suggestions.Cycle(delta)
}
