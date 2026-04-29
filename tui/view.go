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

	contentStr := strings.TrimRight(content.String(), "\n")
	inputStr := m.Input.View()
	fullView := contentStr + "\n\n" + inputStr

	v := tea.NewView(fullView)
	v.AltScreen = true

	c := m.Input.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(contentStr) + 2
	}
	v.Cursor = c

	return v
}
