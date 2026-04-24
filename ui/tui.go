package ui

import (
	"fmt"
	"strings"
	"sync"

	"primusbot/bot"
	"primusbot/ui/components"
	"primusbot/ui/styles"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type doneMsg struct {
	content          string
	reasoningContent string
	err              error
}

type Model struct {
	Bot      *bot.Bot
	Header   *components.Header
	Messages *components.Messages
	Input    *components.Input
	Footer   *components.Footer
	Spinner  spinner.Model
	Width    int
	Height   int
	Ready    bool

	streamMu    sync.Mutex
	streamText  strings.Builder
	reasoning   strings.Builder
	streaming   bool
	lastContent int
}

func NewModel() *Model {
	b := bot.New()
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		Bot:       b,
		Header:    components.NewHeader(80, b.Cfg.Provider, b.Cfg.Model, Version),
		Messages:  components.NewMessages(80, 14),
		Input:     components.NewInput(80),
		Footer:    components.NewFooter(80),
		Spinner:   sp,
		Width:     80,
		Height:    24,
		streaming: false,
	}
}

func (m *Model) Init() tea.Cmd {
	return m.Input.Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Ready = true

		m.Header.SetWidth(msg.Width)
		m.Footer.SetWidth(msg.Width)
		m.Input.SetWidth(msg.Width)

		contentHeight := msg.Height - m.Header.Height() - m.Input.Height() - m.Footer.Height() - 2
		m.Messages.SetSize(msg.Width, contentHeight)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		m.Messages.SetSpinnerView(m.Spinner.View())

		if m.streaming {
			m.streamMu.Lock()
			text := m.streamText.String()
			reasoning := m.reasoning.String()
			m.streamMu.Unlock()

			if len(text) > m.lastContent || len(reasoning) > 0 {
				m.Messages.SetStreamText(styles.RenderMarkdown(text))
				m.Messages.SetReasoningText(reasoning)
				m.lastContent = len(text)
				if m.Messages.Follow {
					m.Messages.GotoBottom()
				}
			}
		}

		if m.streaming || m.Messages.Processing {
			return m, tea.Batch(cmd, m.Spinner.Tick)
		}
		return m, nil

	case doneMsg:
		m.streaming = false
		m.Messages.SetProcessing(false)
		m.Messages.SetStreamText("")
		m.Messages.SetReasoningText("")

		m.streamMu.Lock()
		m.streamText.Reset()
		m.reasoning.Reset()
		m.lastContent = 0
		m.streamMu.Unlock()

		if msg.err != nil {
			m.Messages.AddMessage(components.ChatMessage{
				Role:    "error",
				Content: fmt.Sprintf("Error: %v", msg.err),
			})
		} else {
			renderedContent := styles.RenderMarkdown(msg.content)
			m.Messages.AddMessage(components.ChatMessage{
				Role:             "assistant",
				Content:          msg.content,
				ReasoningContent: msg.reasoningContent,
				RenderedContent:  renderedContent,
			})
		}
		m.Messages.GotoBottom()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up", "down", "pgup", "pgdown":
			m.Messages.Update(msg)
			m.Footer.SetFollow(m.Messages.Follow)
			return m, nil
		}

		if m.streaming {
			return m, nil
		}

		switch msg.String() {
		case "end":
			m.Messages.GotoBottom()
			m.Footer.SetFollow(true)
		case "enter":
			value := m.Input.Value()
			if value == "" {
				return m, nil
			}
			m.Input.Reset()
			return m, m.startChat(value)
		default:
			var cmd tea.Cmd
			m.Input, cmd = m.Input.Update(msg)
			return m, cmd
		}

	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		m.Messages.Update(msg)
		m.Footer.SetFollow(m.Messages.Follow)
	}

	return m, nil
}

func (m *Model) startChat(value string) tea.Cmd {
	resp, ok := m.Bot.ExecuteCommand(value)
	if ok && resp != "" {
		m.Messages.AddMessage(components.ChatMessage{Role: "system", Content: resp})
		return nil
	}

	if strings.HasPrefix(value, "@agent") {
		return m.startAgent(value)
	}

	m.Messages.AddMessage(components.ChatMessage{Role: "user", Content: value})
	m.Messages.GotoBottom()
	m.Footer.SetFollow(true)
	m.Messages.SetProcessing(true)
	m.streaming = true

	m.streamMu.Lock()
	m.streamText.Reset()
	m.reasoning.Reset()
	m.lastContent = 0
	m.streamMu.Unlock()

	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			err := m.Bot.Chat(value,
				func(content, reasoning string) {
					m.streamMu.Lock()
					if content != "" {
						m.streamText.WriteString(content)
					}
					if reasoning != "" {
						m.reasoning.WriteString(reasoning)
					}
					m.streamMu.Unlock()
				},
				func() {},
			)

			m.streamMu.Lock()
			content := m.streamText.String()
			reasoning := m.reasoning.String()
			m.streamMu.Unlock()

			return doneMsg{content: content, reasoningContent: reasoning, err: err}
		},
	)
}

func (m *Model) startAgent(value string) tea.Cmd {
	m.Messages.AddMessage(components.ChatMessage{Role: "user", Content: value})
	m.Messages.GotoBottom()
	m.Footer.SetFollow(true)
	m.Messages.SetProcessing(true)

	m.streamMu.Lock()
	m.streamText.Reset()
	m.reasoning.Reset()
	m.lastContent = 0
	m.streamMu.Unlock()

	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			var lastOutput string
			_, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string) {
				m.streamMu.Lock()
				stepInfo := fmt.Sprintf("\n[Step %d] %s\n  Action: %s", step+1, thought, action)
				if toolName != "" {
					stepInfo += fmt.Sprintf("\n  Tool: %s(%s)", toolName, toolArgs)
				}
				stepInfo += fmt.Sprintf("\n  Output: %s\n", truncate(output, 200))
				m.streamText.WriteString(stepInfo)
				lastOutput = output
				m.streamMu.Unlock()

				m.Messages.SetStreamText(styles.RenderMarkdown(m.streamText.String()))
				if m.Messages.Follow {
					m.Messages.GotoBottom()
				}
			})

			m.streamMu.Lock()
			content := m.streamText.String()
			m.streamMu.Unlock()

			return doneMsg{content: content, reasoningContent: lastOutput, err: err}
		},
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (m *Model) View() tea.View {
	if !m.Ready {
		return tea.NewView("Loading...")
	}

	var b strings.Builder
	b.WriteString(m.Header.View())
	b.WriteString(m.Messages.View())
	b.WriteString("\n")
	b.WriteString(m.Input.View())
	b.WriteString(m.Footer.View())

	v := tea.NewView(b.String())
	v.AltScreen = true

	c := m.Input.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(m.Messages.View()) + m.Header.Height()
	}
	v.Cursor = c

	return v
}

func Run() {
	p := tea.NewProgram(NewModel())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
