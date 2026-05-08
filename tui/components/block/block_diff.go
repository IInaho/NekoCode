// block_diff.go — diff 高亮渲染（-/+ 行着色）。
package block

import (
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

func renderBlockDiff(b ContentBlock, _ *styles.Styles) string {
	lines := strings.Split(b.Content, "\n")
	red := lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Red))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color(styles.DiffGreen))
	subtle := lipgloss.NewStyle().Foreground(lipgloss.Color(styles.DiffSubtle))

	var out strings.Builder
	out.WriteString(subtle.Render("── diff ──"))
	for _, line := range lines {
		if strings.HasPrefix(line, "-") {
			out.WriteString("\n" + red.Render(line))
		} else if strings.HasPrefix(line, "+") {
			out.WriteString("\n" + green.Render(line))
		} else {
			out.WriteString("\n" + subtle.Render(line))
		}
	}
	return out.String()
}
