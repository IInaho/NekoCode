package tui

import (
	"fmt"
	"time"

	"primusbot/tui/components"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const contentMarginV = 2

// --- spinner tick ---

func (m *Model) handleSpinnerTick(msg spinner.TickMsg) tea.Cmd {
	var cmd tea.Cmd
	m.Spinner, cmd = m.Spinner.Update(msg)

	if m.state == StateConfirming {
		m.Messages.SetSpinnerView("")
		return nil
	}

	m.Messages.SetSpinnerView(m.Spinner.View())

	if m.state == StateProcessing {
		elapsed := time.Since(m.processingStart)
		m.Messages.SetProcessingStatus(fmt.Sprintf("%s (%.1fs)", m.processingPhase, elapsed.Seconds()))
		prompt, compl := m.Bot.TokenUsage()
		if prompt == 0 {
			prompt = m.Bot.ContextTokens()
		}
		m.Messages.SetProcessingTokens(prompt, compl)

		if m.Stream.Dirty() && time.Since(m.lastStreamRender) > 100*time.Millisecond {
			blocks := m.Stream.Snapshot()
			m.Messages.SetBlocks(blocks)
			m.Stream.MarkSeen()
			m.lastStreamRender = time.Now()
			if m.Messages.Follow {
				m.Messages.GotoBottom()
			}
		}
		return tea.Batch(cmd, m.Spinner.Tick)
	}

	return nil
}

// --- confirm key ---

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.ConfirmBar.Respond(true)
	case "esc", "n", "N", "ctrl+c":
		m.ConfirmBar.Respond(false)
	default:
		return m, nil
	}
	m.state = StateProcessing
	return m, tea.Batch(listenConfirm(m.confirmCh), m.Spinner.Tick)
}

// --- agent done ---

func (m *Model) handleDone(msg doneMsg) tea.Cmd {
	_ = msg.tokens
	m.transitionTo(StateReady)

	if msg.err != nil {
		m.Messages.AddMessage(components.ChatMessage{
			Role:    "error",
			Content: fmt.Sprintf("Error: %v", msg.err),
		})
	} else {
		content := msg.content
		if msg.diffBlocks != "" {
			content = msg.diffBlocks + "\n" + content
		}

		footer := ""
		if msg.duration != "" || msg.tokens != "" {
			footer = "Duration: " + msg.duration
			if msg.tokens != "" {
				footer += "  " + msg.tokens
			}
		}
		blocks := m.Stream.Finalize()
		m.Messages.AddMessage(components.ChatMessage{
			Role:    "assistant",
			Content: content,
			Footer:  footer,
			Blocks:  blocks,
		})
	}

	prompt, compl := m.Bot.TokenUsage()
	m.Header.SetTokens(prompt + compl)
	m.Messages.GotoBottom()
	return nil
}

func (m *Model) updateTokens() {
	prompt, compl := m.Bot.TokenUsage()
	m.Header.SetTokens(prompt + compl)
}

// --- key press ---

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit

	case "ctrl+e":
		if m.state != StateProcessing {
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

	if m.state == StateProcessing {
		switch msg.String() {
		case "enter":
			value := m.Input.Value()
			if value != "" {
				m.Suggestions.Hide()
				m.Input.AddHistory(value)
				m.Input.Reset()
				m.Messages.AddMessage(components.ChatMessage{Role: "user", Content: value})
				m.Bot.Steer(value)
			}
		case "esc":
			m.Bot.Abort()
			m.Messages.SetProcessingStatus("Aborted")
		case "pgup", "pgdown", "up", "down":
			m.Messages.Update(msg)
			m.Input.SetFollow(false)
		}
		return nil
	}

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
