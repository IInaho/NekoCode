package tui

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"primusbot/tui/components"

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
	m.transitionTo(StateProcessing)

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
			var diffs []string

			result, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int) {
				if action == "think" {
					m.Stream.Append(components.ContentBlock{
						Type:    components.BlockThinking,
						Content: output,
					})
					return
				}

				if action == "chat" {
					finalResponse = output
					m.Stream.Append(components.ContentBlock{
						Type:    components.BlockThinking,
						Content: output,
					})
					return
				}

				if toolName != "" {
					if toolName == "edit" && output != "" {
						diffs = append(diffs, output)
					}
					block := components.ContentBlock{
						Type:      components.BlockToolCall,
						ToolName:  toolName,
						ToolArgs:  formatBriefArgs(toolName, toolArgs),
						Collapsed: true,
					}
					if output != "" {
						block.Content = output
					}
					m.Stream.Append(block)
				}
			})

			if finalResponse == "" {
				finalResponse = result
			}
			if finalResponse == "" {
				finalResponse = "sorry, could not complete this task."
			}

			return doneMsg{
				content:    finalResponse,
				diffBlocks: strings.Join(diffs, "\n"),
				duration:   m.Bot.Duration(),
				tokens:     tokensSummary(m.Bot),
				err:        err,
			}
		},
	)
}

func tokensSummary(b BotInterface) string {
	p, c := b.TokenUsage()
	return "↑" + fmtTokensInt(p) + " ↓" + fmtTokensInt(c)
}

func fmtTokensInt(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
