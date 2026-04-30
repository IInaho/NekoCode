// 用户交互调度：startChat（命令 vs Agent 分流）、startAgent（Agent 执行 + 流式回调）、
// refreshSuggestions / acceptSuggestion / cycleSuggestion（命令提示逻辑）、handleTabCompletion。
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

	return m.startAgent(value)
}

func (m *Model) startAgent(value string) tea.Cmd {
	m.Messages.AddMessage(components.ChatMessage{Role: "user", Content: value})
	m.Messages.GotoBottom()
	m.Input.SetFollow(true)
	m.Input.SetSending(true)
	m.Messages.SetProcessing(true)
	m.Stream.Start()
	m.updateTokens()

	return tea.Batch(
		m.Spinner.Tick,
		listenConfirm(m.confirmCh),
		func() tea.Msg {
			var finalResponse string
			var toolSteps []string
			cw := components.CappedWidth(m.Messages.Width())

			_, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string) {
				if action == "chat" {
					finalResponse = output
					return
				}

				line := fmt.Sprintf("> %s", thought)
				if toolName != "" {
					line += fmt.Sprintf(" `%s(%s)`", toolName, truncate(toolArgs, 80))
				}
				toolSteps = append(toolSteps, line)

				m.Stream.Append(line+"\n", "")
				text, _ := m.Stream.Snapshot()
				m.Messages.SetStreamContentWidth(cw)
				m.Messages.SetStreamText(styles.RenderMarkdownWithWidth(text, cw))
				if m.Messages.Follow {
					m.Messages.GotoBottom()
				}
			})

			if finalResponse == "" {
				finalResponse = strings.Join(toolSteps, "\n")
			}

			reasoning := strings.Join(toolSteps, "\n")
			return doneMsg{content: finalResponse, reasoningContent: reasoning, err: err}
		},
	)
}

func (m *Model) refreshSuggestions() {
	m.suggestions = nil
	m.suggestionIdx = 0
	m.suggestionsVisible = false

	input := m.Input.Value()
	if !strings.HasPrefix(input, "/") {
		return
	}

	prefix := strings.TrimPrefix(input, "/")
	for _, name := range m.Bot.CommandNames() {
		if strings.HasPrefix(name, prefix) {
			m.suggestions = append(m.suggestions, "/"+name)
		}
	}
	if len(m.suggestions) == 1 && m.suggestions[0] == input {
		return
	}
	if len(m.suggestions) > 0 {
		m.suggestionsVisible = true
	}
}

func (m *Model) acceptSuggestion() {
	if !m.suggestionsVisible || len(m.suggestions) == 0 {
		return
	}
	m.Input.SetValue(m.suggestions[m.suggestionIdx])
	m.Input.SetCursorEnd()
	m.suggestionsVisible = false
}

func (m *Model) cycleSuggestion(delta int) {
	if !m.suggestionsVisible || len(m.suggestions) == 0 {
		return
	}
	m.suggestionIdx = (m.suggestionIdx + delta + len(m.suggestions)) % len(m.suggestions)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
