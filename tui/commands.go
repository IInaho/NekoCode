package tui

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"primusbot/tui/components"
	"primusbot/tui/styles"

	tea "charm.land/bubbletea/v2"
)

func logPanic(r any) {
	stack := debug.Stack()
	path := fmt.Sprintf("primusbot-panic-%d.log", time.Now().Unix())
	msg := fmt.Sprintf("PANIC: %v\n\nStack:\n%s", r, string(stack))
	_ = os.WriteFile(path, []byte(msg), 0644)
}

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
			defer func() {
				if r := recover(); r != nil {
					logPanic(r)
				}
			}()

			var finalResponse string
			var toolNames []string
			var diffs []string
			cw := components.CappedWidth(m.Messages.Width())

			_, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string) {
				if action == "chat" {
					finalResponse = output
					return
				}

				if toolName != "" {
					toolNames = append(toolNames, toolName)
					if toolName == "edit" && output != "" {
						diffs = append(diffs, output)
					}
				}

				// Stream tool call.
				line := fmt.Sprintf("> %s", thought)
				if toolName != "" {
					line += fmt.Sprintf(" `%s(%s)`", toolName, truncate(toolArgs, 80))
				}
				m.Stream.Append(line+"\n", "")

				if output != "" {
					out := truncate(output, 600)
					m.Stream.Append(out+"\n", "")
				}

				text, _ := m.Stream.Snapshot()
				m.Messages.SetStreamContentWidth(cw)
				m.Messages.SetStreamText(styles.RenderMarkdownWithWidth(text, cw))
				if m.Messages.Follow {
					m.Messages.GotoBottom()
				}
			})

			if finalResponse == "" {
				finalResponse = "抱歉，无法完成这个任务。"
			}

			return doneMsg{
				content:          finalResponse,
				reasoningContent: toolSummary(toolNames),
				diffBlocks:       strings.Join(diffs, "\n"),
				err:              err,
			}
		},
	)
}

func toolSummary(names []string) string {
	if len(names) == 0 {
		return ""
	}
	seen := make(map[string]bool)
	var unique []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}
	return fmt.Sprintf("%d tool%s · %s", len(names), s(len(names)), strings.Join(unique, " → "))
}

func s(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
