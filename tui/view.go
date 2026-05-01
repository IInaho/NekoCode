package tui

import (
	"strings"

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

	if m.state == StateConfirming {
		if bar := m.ConfirmBar.View(m.Width); bar != "" {
			content.WriteString("\n")
			content.WriteString(bar)
		}
	}

	contentStr := strings.TrimRight(content.String(), "\n")

	if sug := m.Suggestions.View(m.Width); sug != "" {
		contentStr += "\n" + sug
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
