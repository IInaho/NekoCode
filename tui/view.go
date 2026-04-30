// View 渲染：Splash / Header / Messages / Suggestions / ConfirmBar / Input 布局。
// renderSuggestions 命令提示 popup，renderConfirmBar 危险操作确认栏。
package tui

import (
	"fmt"
	"strings"

	"primusbot/bot/agent"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *Model) View() tea.View {
	if !m.Ready {
		return tea.NewView("Loading...")
	}

	var content strings.Builder
	if m.Messages.Len() == 0 {
		content.WriteString(m.Splash.View())
	} else {
		content.WriteString(m.Header.View())
		content.WriteString(m.Messages.View())
	}

	if m.PendingConfirm != nil {
		content.WriteString("\n")
		content.WriteString(renderConfirmBar(m.PendingConfirm, m.Width))
	}

	contentStr := strings.TrimRight(content.String(), "\n")

	if m.suggestionsVisible && len(m.suggestions) > 0 {
		contentStr += "\n" + renderSuggestions(m.suggestions, m.suggestionIdx, m.Width)
	}

	inputStr := m.Input.View()
	fullView := contentStr + "\n\n" + inputStr

	v := tea.NewView(fullView)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	c := m.Input.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(contentStr) + 2
	}
	v.Cursor = c

	return v
}

func renderSuggestions(items []string, idx, width int) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	highlight := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0")).Bold(true)
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("#a0a0a0"))

	var b strings.Builder
	b.WriteString(dim.Render("── suggestions ──") + "\n")
	for i, item := range items {
		if i == idx {
			b.WriteString(highlight.Render("> "+item) + "\n")
		} else {
			b.WriteString(normal.Render("  "+item) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderConfirmBar(req *agent.ConfirmRequest, width int) string {
	levelStyle := lipgloss.NewStyle()
	switch req.Level {
	case 1:
		levelStyle = levelStyle.Foreground(lipgloss.Color("#f0c040"))
	case 2:
		levelStyle = levelStyle.Foreground(lipgloss.Color("#f06040"))
	}

	argsStr := ""
	for k, v := range req.Args {
		argsStr += fmt.Sprintf("%s=%v ", k, v)
	}
	argsStr = strings.TrimSpace(argsStr)
	if len(argsStr) > width-30 {
		argsStr = argsStr[:width-33] + "..."
	}

	label := levelStyle.Render(fmt.Sprintf("[%s]", req.Level.String()))
	detail := fmt.Sprintf(" %s %s", req.ToolName, argsStr)
	prompt := " [enter] confirm  [esc] cancel"

	bar := label + detail
	pad := width - lipgloss.Width(bar) - lipgloss.Width(prompt)
	if pad < 0 {
		pad = 0
	}
	return bar + strings.Repeat(" ", pad) + prompt
}
