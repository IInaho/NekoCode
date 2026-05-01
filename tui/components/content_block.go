package components

import (
	"fmt"
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type BlockType int

const (
	BlockToolCall BlockType = iota
	BlockThinking
	BlockText
)

type ContentBlock struct {
	Type      BlockType
	Content   string
	ToolName  string
	ToolArgs  string
	Collapsed bool
}

var toolAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a96e"))

func renderBlockToolCall(b ContentBlock, width int, sty *styles.Styles) string {
	arrow := "[+]"
	if !b.Collapsed {
		arrow = "[-]"
	}

	icon := toolAccent.Render("◆")
	header := fmt.Sprintf("%s %s %s  %s", icon, b.ToolName, b.ToolArgs, sty.Subtle.Render(arrow))
	sep := toolAccent.Render(strings.Repeat(styles.Horizontal, min(width-4, 40)))

	var sb strings.Builder
	sb.WriteString(header)
	if !b.Collapsed && b.Content != "" {
		sb.WriteString("\n" + sep + "\n")
		for _, line := range strings.Split(b.Content, "\n") {
			sb.WriteString(sty.Muted.Render("  "+line) + "\n")
		}
		sb.WriteString(sep)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderBlockThinking(b ContentBlock, sty *styles.Styles) string {
	if b.Collapsed {
		return sty.Subtle.Render("··· " + b.Content)
	}
	return sty.Subtle.Render(b.Content)
}

func renderBlockText(b ContentBlock, _ *styles.Styles) string {
	return b.Content
}

func RenderBlock(b ContentBlock, width int, sty *styles.Styles) string {
	switch b.Type {
	case BlockToolCall:
		return renderBlockToolCall(b, width, sty)
	case BlockThinking:
		return renderBlockThinking(b, sty)
	default:
		return renderBlockText(b, sty)
	}
}

func RenderBlocks(blocks []ContentBlock, width int, sty *styles.Styles) string {
	if len(blocks) == 0 {
		return ""
	}
	var lines []string
	for _, b := range blocks {
		rendered := RenderBlock(b, width, sty)
		if rendered != "" {
			lines = append(lines, rendered)
		}
	}
	return strings.Join(lines, "\n")
}
