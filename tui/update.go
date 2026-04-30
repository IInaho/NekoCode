// Update 消息循环：处理 WindowSize、SpinnerTick、doneMsg、confirmMsg、KeyPress。
// 确认键分流、历史翻阅、命令提示选择、消息发送均在此路由。
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
	defer func() {
		if r := recover(); r != nil {
			logPanic(r)
			panic(r) // re-panic so Bubble Tea can clean up the terminal
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

		contentHeight := msg.Height - m.Header.Height() - m.Input.Height() - 2
		m.Messages.SetSize(msg.Width, contentHeight)
		return m, nil

	case spinner.TickMsg:
		return m, m.handleSpinnerTick(msg)

	case doneMsg:
		return m, m.handleDone(msg)

	case confirmMsg:
		m.PendingConfirm = &msg.req
		return m, nil

	case tea.KeyPressMsg:
		if m.PendingConfirm != nil {
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
	if m.PendingConfirm != nil {
		m.Messages.SetSpinnerView("")
		return nil
	}
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

	if m.PendingConfirm != nil {
		return nil
	}
	if m.Stream.Active() || m.Messages.Processing {
		return tea.Batch(cmd, m.Spinner.Tick)
	}
	return nil
}

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.PendingConfirm.Response <- true
	case "esc", "n", "N", "ctrl+c":
		m.PendingConfirm.Response <- false
	default:
		return m, nil
	}
	m.PendingConfirm = nil
	return m, listenConfirm(m.confirmCh)
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
		// Prepend diffs to final content so they render with colors.
		content := msg.content
		if msg.diffBlocks != "" {
			content = msg.diffBlocks + "\n" + content
		}

		cw := components.CappedWidth(m.Messages.Width())
		renderedContent := styles.RenderMarkdownWithWidth(content, cw)
		m.Messages.AddMessage(components.ChatMessage{
			Role:             "assistant",
			Content:          content,
			ReasoningContent: msg.reasoningContent,
			RenderedContent:  renderedContent,
		})
	}
	m.PendingConfirm = nil
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

	// up/down always navigate input history; scroll wheel handles messages
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

	if m.Stream.Active() {
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
		if m.suggestionsVisible {
			m.acceptSuggestion()
			return nil
		}
		value := m.Input.Value()
		if value == "" {
			return nil
		}
		m.suggestionsVisible = false
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
