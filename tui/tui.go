// tui.go — package tui 入口。
package tui
import (
	"fmt"

	"nekocode/bot"
	"nekocode/tui/styles"

	tea "charm.land/bubbletea/v2"
)

func Run() {
	styles.Warmup()
	b := bot.New()
	p := tea.NewProgram(NewModel(b))
	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}
}
