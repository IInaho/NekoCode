package tui

import (
	"fmt"

	"primusbot/bot"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	b := bot.New()
	p := tea.NewProgram(NewModel(b))
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
