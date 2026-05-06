package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *Model) View() tea.View {
	if !m.Ready {
		return tea.NewView("Loading...")
	}

	var parts []string

	if m.Messages.Len() == 0 {
		parts = append(parts, m.Splash.View())
	} else {
		parts = append(parts, m.Header.View())

		// Update scrollbar state from Messages list.
		m.Scrollbar.Update(
			m.Messages.TotalContentHeight(),
			m.Messages.Height(),
			m.Messages.ScrollPercent(),
		)

		// Messages + Scrollbar as independent horizontal pair.
		msgView := lipgloss.NewStyle().Width(m.Width - 1).Render(m.Messages.View())
		barView := m.Scrollbar.View()
		row := msgView
		if barView != "" {
			row = lipgloss.JoinHorizontal(lipgloss.Top, msgView, barView)
		}
		parts = append(parts, row)
	}

	if m.state == StateConfirming {
		if bar := m.ConfirmBar.View(m.Width); bar != "" {
			parts = append(parts, bar)
		}
	}

	if sug := m.Suggestions.View(m.Width); sug != "" {
		parts = append(parts, sug)
	}

	parts = append(parts, "", m.Input.View())

	view := lipgloss.JoinVertical(lipgloss.Left, parts...)

	v := tea.NewView(view)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	c := m.Input.Cursor()
	if c != nil {
		above := lipgloss.JoinVertical(lipgloss.Left, parts[:len(parts)-2]...)
		c.Y += lipgloss.Height(above) + 2
	}
	v.Cursor = c

	return v
}
