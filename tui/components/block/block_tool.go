// block_tool.go — 工具调用行渲染（◆ read game.go [+]）。
package block

import (
	"fmt"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

func renderToolLine(b ContentBlock, width int, sty *styles.Styles) string {
	icon := "◆"
	if b.BatchTotal > 1 {
		icon = "⚡"
	}
	args := b.ToolArgs
	if b.BatchTotal > 1 {
		args = fmt.Sprintf("(%d/%d) %s", b.BatchIdx, b.BatchTotal, b.ToolArgs)
	}
	hasContent := b.ToolName == "edit" && b.Content != ""
	arrow := ""
	if hasContent {
		if b.Collapsed {
			arrow = " " + sty.Subtle.Render("[+]")
		} else {
			arrow = " " + sty.Subtle.Render("[-]")
		}
	}
	header := fmt.Sprintf("%s %s %s%s", icon, b.ToolName, args, arrow)
	accentLine := "  " + toolAccent.Render(header)

	if !hasContent || b.Collapsed {
		return accentLine
	}

	contentW := width - 6
	if contentW < 10 {
		contentW = 10
	}
	diff := renderBlockDiff(ContentBlock{Content: b.Content}, sty)
	indented := lipgloss.NewStyle().PaddingLeft(2).Render(diff)
	return lipgloss.JoinVertical(lipgloss.Left, accentLine, indented)
}
