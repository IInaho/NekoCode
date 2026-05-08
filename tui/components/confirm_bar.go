// confirm_bar.go — 确认弹窗栏（yes/no 操作确认）。
package components
import (
	"fmt"
	"strings"

	"primusbot/bot/tools"
	"primusbot/bot/types"
	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type ConfirmBar struct {
	req *types.ConfirmRequest
	sty *styles.Styles
}

func NewConfirmBar(sty *styles.Styles) *ConfirmBar {
	return &ConfirmBar{sty: sty}
}

func (c *ConfirmBar) SetRequest(req *types.ConfirmRequest) { c.req = req }
func (c *ConfirmBar) Clear()                                { c.req = nil }
func (c *ConfirmBar) Respond(ok bool)                         { c.req.Response <- ok; c.req = nil }

func (c *ConfirmBar) Height() int {
	if c.req == nil {
		return 0
	}
	return 4
}

func (c *ConfirmBar) View(width int) string {
	if c.req == nil {
		return ""
	}
	w := max(40, width-4)

	title := c.sty.Primary.Bold(true).Render("Confirm")
	topBorder := c.sty.Border.Render(styles.Horizontal)
	titleBar := topBorder + " " + title + " " + strings.Repeat(styles.Horizontal, max(0, w-lipgloss.Width(title)-2))

	path := ""
	if p, ok := c.req.Args["path"].(string); ok {
		path = "  " + p
	}

	level := c.sty.Yellow.Render(c.req.Level.String())
	if c.req.Level == tools.LevelForbidden {
		level = c.sty.Red.Render(c.req.Level.String())
	}

	info := fmt.Sprintf("  %s%s  [%s]", c.req.ToolName, path, level)
	infoW := lipgloss.Width(info)
	infoLine := c.sty.Base.Render(info)

	hint := c.sty.Primary.Bold(true).Render("[enter] yes") + "  " + c.sty.Muted.Render("[esc] no")
	prompt := c.sty.Base.Render("  Proceed?  ") + hint
	promptW := lipgloss.Width(prompt)

	bottomBorder := c.sty.Border.Render(strings.Repeat(styles.Horizontal, w))

	var b strings.Builder
	b.WriteString(titleBar + "\n")
	b.WriteString(infoLine + strings.Repeat(" ", max(0, w-infoW)) + "\n")
	b.WriteString(prompt + strings.Repeat(" ", max(0, w-promptW)) + "\n")
	b.WriteString(bottomBorder)

	return b.String()
}
