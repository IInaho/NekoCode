package tui

import (
	"fmt"
	"time"

	"primusbot/tui/components"
	"primusbot/tui/styles"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const contentMarginV = 2

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			logPanic(r)
		}
	}()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true

		m.Header.SetWidth(msg.Width)
		m.Input.SetWidth(msg.Width)
		m.Splash.SetSize(msg.Width, msg.Height)

		m.resizeMessages()
		return m, nil

	case spinner.TickMsg:
		return m, m.handleSpinnerTick(msg)

	case doneMsg:
		return m, m.handleDone(msg)

	case confirmMsg:
		m.ConfirmBar.SetRequest(&msg.req)
		m.state = StateConfirming
		m.resizeMessages()
		return m, nil

	case tea.KeyPressMsg:
		if m.state == StateConfirming {
			return m.handleConfirmKey(msg)
		}
		return m, m.handleKeyPress(msg)

	case components.TickMsg:
		if m.Messages.Len() == 0 {
			m.Splash.Blink()
			return m, components.BlinkTick()
		}
		return m, nil

	case cursor.BlinkMsg:
		input, cmd := m.Input.Update(msg)
		m.Input = input
		return m, cmd

	case tea.MouseMsg:
		m.Messages.Update(msg)
		m.Input.SetFollow(m.Messages.Follow)
	}

	return m, nil
}

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

		if m.Stream.HasNew() {
			blocks := m.Stream.Snapshot()
			m.Messages.SetBlocks(blocks)
			m.Stream.MarkSeen()
			if m.Messages.Follow {
				m.Messages.GotoBottom()
			}
		}
		return tea.Batch(cmd, m.Spinner.Tick)
	}

	return nil
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
	m.state = StateProcessing
	return m, tea.Batch(listenConfirm(m.confirmCh), m.Spinner.Tick)
}

func (m *Model) handleDone(msg doneMsg) tea.Cmd {
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

		cw := components.CappedWidth(m.Messages.Width())
		renderedContent := styles.RenderMarkdownWithWidth(content, cw)
		blocks := m.Stream.Snapshot()
		m.Messages.AddMessage(components.ChatMessage{
			Role:            "assistant",
			Content:         content,
			RenderedContent: renderedContent,
			Blocks:          blocks,
		})
	}

	m.updateTokens()
	m.Messages.GotoBottom()
	return nil
}

func (m *Model) updateTokens() {
	used, budget := m.Bot.TokenUsage()
	m.Header.SetTokens(used, budget)
}

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
