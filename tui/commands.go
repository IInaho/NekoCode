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
	m.transitionTo(StateProcessing)
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
			var diffs []string

			_, err := m.Bot.RunAgent(value, func(step int, thought, action, toolName, toolArgs, output string) {
				if action == "chat" {
					finalResponse = output
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
						block.Content = styles.RenderMarkdownWithWidth(truncate(output, 600), components.CappedWidth(m.Messages.Width()))
					}
					m.Stream.Append(block)
				}
			})

			if finalResponse == "" {
				finalResponse = "sorry, could not complete this task."
			}

			return doneMsg{
				content:    finalResponse,
				diffBlocks: strings.Join(diffs, "\n"),
				err:        err,
			}
		},
	)
}

// formatBriefArgs extracts key identifying args for a clean one-line tool display.
func formatBriefArgs(toolName, toolArgs string) string {
	// Parse flat key=value pairs (may contain quoted values).
	parse := func(s string) map[string]string {
		m := make(map[string]string)
		for _, pair := range splitArgPairs(s) {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
		return m
	}
	args := parse(toolArgs)

	switch toolName {
	case "filesystem":
		op := args["operation"]
		path := args["path"]
		if op != "" && path != "" {
			return op + " " + path
		}
		return path
	case "edit":
		return args["path"]
	case "bash":
		cmd := args["command"]
		if len(cmd) > 50 {
			cmd = cmd[:47] + "..."
		}
		return cmd
	case "glob":
		return args["pattern"]
	case "grep":
		pat := args["pattern"]
		p := args["path"]
		if p != "" {
			return pat + " " + p
		}
		return pat
	default:
		// Show first non-empty value.
		for _, v := range args {
			if len(v) > 30 {
				v = v[:27] + "..."
			}
			return v
		}
		return ""
	}
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func splitArgPairs(s string) []string {
	var pairs []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '\\':
			if inQuote && i+1 < len(s) {
				i++
			}
		case ',':
			if !inQuote {
				pairs = append(pairs, s[start:i])
				start = i + 1
			}
		}
	}
	pairs = append(pairs, s[start:])
	return pairs
}
