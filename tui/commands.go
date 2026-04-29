package tui

import (
	"fmt"
	"strings"

	"primusbot/tui/components"
	"primusbot/tui/styles"

	tea "charm.land/bubbletea/v2"
)

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
	m.Input.SetFollow(true)
	m.Input.SetSending(true)
	m.Messages.SetProcessing(true)
	m.Stream.Start()

	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			err := m.Bot.Chat(value,
				func(content, reasoning string) { m.Stream.Append(content, reasoning) },
				func() {},
			)
			content, reasoning := m.Stream.Snapshot()
			return doneMsg{content: content, reasoningContent: reasoning, err: err}
		},
	)
}

func (m *Model) startAgent(value string) tea.Cmd {
	m.Messages.AddMessage(components.ChatMessage{Role: "user", Content: value})
	m.Messages.GotoBottom()
	m.Input.SetFollow(true)
	m.Input.SetSending(true)
	m.Messages.SetProcessing(true)
	m.Stream.Start()

	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			var lastOutput string
			cw := components.CappedWidth(m.Messages.Width())
			_, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string) {
				v := styles.Vertical
				stepInfo := fmt.Sprintf("\n%s Step %d: %s", v, step+1, thought)
				stepInfo += fmt.Sprintf("\n%s   Action: %s", v, action)
				if toolName != "" {
					stepInfo += fmt.Sprintf("\n%s   Tool: %s(%s)", v, toolName, toolArgs)
				}
				stepInfo += fmt.Sprintf("\n%s   Output: %s\n", v, truncate(output, 200))
				m.Stream.Append(stepInfo, "")
				lastOutput = output

				text, _ := m.Stream.Snapshot()
				m.Messages.SetStreamContentWidth(cw)
				m.Messages.SetStreamText(styles.RenderMarkdownWithWidth(text, cw))
				if m.Messages.Follow {
					m.Messages.GotoBottom()
				}
			})

			content, _ := m.Stream.Snapshot()
			return doneMsg{content: content, reasoningContent: lastOutput, err: err}
		},
	)
}

func (m *Model) handleTabCompletion() {
	input := m.Input.Value()

	if m.completions == nil {
		m.completionIdx = 0
		if strings.HasPrefix(input, "/") {
			prefix := strings.TrimPrefix(input, "/")
			for _, name := range m.Bot.CommandNames() {
				if strings.HasPrefix(name, prefix) {
					m.completions = append(m.completions, "/"+name)
				}
			}
		} else if strings.HasPrefix(input, "@") {
			if strings.HasPrefix("agent", strings.TrimPrefix(input, "@")) {
				m.completions = []string{"@agent"}
			}
		}
	} else {
		m.completionIdx = (m.completionIdx + 1) % len(m.completions)
	}

	if len(m.completions) > 0 {
		m.Input.SetValue(m.completions[m.completionIdx])
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
