// 程序入口：创建 Bubble Tea Program，运行主循环。
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	p := tea.NewProgram(NewModel())
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
