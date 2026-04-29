package tui

import (
	"fmt"

	"primusbot/tui/components"
	"primusbot/tui/styles"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true

		m.Header.SetWidth(msg.Width)
		m.Input.SetWidth(msg.Width)
		m.Splash.SetSize(msg.Width, msg.Height)

		contentHeight := msg.Height - m.Header.Height() - m.Input.Height() - 2
		m.Messages.SetSize(msg.Width, contentHeight)
		return m, nil

	case spinner.TickMsg:
		return m, m.handleSpinnerTick(msg)

	case doneMsg:
		return m, m.handleDone(msg)

	case tea.KeyPressMsg:
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
	m.Messages.SetSpinnerView(m.Spinner.View())

	if m.Stream.Active() {
		text, reasoning := m.Stream.Snapshot()
		if m.Stream.HasNew() {
			cw := components.CappedWidth(m.Messages.Width())
			m.Messages.SetStreamContentWidth(cw)
			m.Messages.SetStreamText(styles.RenderMarkdownWithWidth(text, cw))
			m.Messages.SetReasoningText(reasoning)
			m.Stream.MarkSeen()
			if m.Messages.Follow {
				m.Messages.GotoBottom()
			}
		}
	}

	if m.Stream.Active() || m.Messages.Processing {
		return tea.Batch(cmd, m.Spinner.Tick)
	}
	return nil
}

func (m *Model) handleDone(msg doneMsg) tea.Cmd {
	m.Stream.Stop()
	m.Messages.SetProcessing(false)
	m.Messages.SetStreamText("")
	m.Messages.SetReasoningText("")
	m.Input.SetSending(false)

	if msg.err != nil {
		m.Messages.AddMessage(components.ChatMessage{
			Role:    "error",
			Content: fmt.Sprintf("Error: %v", msg.err),
		})
	} else {
		cw := components.CappedWidth(m.Messages.Width())
		renderedContent := styles.RenderMarkdownWithWidth(msg.content, cw)
		m.Messages.AddMessage(components.ChatMessage{
			Role:             "assistant",
			Content:          msg.content,
			ReasoningContent: msg.reasoningContent,
			RenderedContent:  renderedContent,
		})
	}
	m.Messages.GotoBottom()
	return nil
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit

	case "up", "down":
		if m.Input.IsEmpty() {
			if msg.String() == "up" {
				m.Input.HistoryUp()
			} else {
				m.Input.HistoryDown()
			}
			return nil
		}
		m.Messages.Update(msg)
		m.Input.SetFollow(m.Messages.Follow)
		return nil

	case "pgup", "pgdown":
		m.Messages.Update(msg)
		m.Input.SetFollow(m.Messages.Follow)
		return nil
	}

	if m.Stream.Active() {
		if msg.String() == "esc" {
			m.Bot.CancelStream()
		}
		return nil
	}

	switch msg.String() {
	case "end":
		m.Messages.GotoBottom()
		m.Input.SetFollow(true)
	case "tab":
		m.handleTabCompletion()
		return nil
	case "enter":
		value := m.Input.Value()
		if value == "" {
			return nil
		}
		m.completions = nil
		m.Input.AddHistory(value)
		m.Input.Reset()
		return m.startChat(value)
	default:
		m.completions = nil
		input, cmd := m.Input.Update(msg)
		m.Input = input
		return cmd
	}

	return nil
}
