// block_text.go — 文本块渲染（Thought / Reason）。
package block

import (
	"strings"

	"primusbot/tui/styles"
)

func renderBlockThought(b ContentBlock, sty *styles.Styles) string {
	text := strings.TrimSpace(b.Content)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = sty.Subtle.Render("💭 " + line)
		} else {
			lines[i] = sty.Subtle.Render("   " + line)
		}
	}
	return strings.Join(lines, "\n")
}

func renderBlockReason(b ContentBlock, sty *styles.Styles) string {
	if b.Content == "" {
		return sty.Subtle.Render("💭 ...")
	}
	text := strings.TrimSpace(b.Content)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = sty.Subtle.Render("💭 " + line)
		} else {
			lines[i] = sty.Subtle.Render("  " + line)
		}
	}
	return strings.Join(lines, "\n")
}
