package components

import (
	"fmt"
	"strings"

	"primusbot/bot/tools"
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
	Type       BlockType
	Content    string
	ToolName   string
	ToolArgs   string
	Collapsed  bool
	BatchIdx   int
	BatchTotal int
}

var toolAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9a96e"))

func renderToolLine(b ContentBlock, width int, sty *styles.Styles) string {
	arrow := "[+]"
	if !b.Collapsed {
		arrow = "[-]"
	}
	icon := "◆"
	if b.BatchTotal > 1 {
		icon = "⚡"
	}
	args := b.ToolArgs
	if b.BatchTotal > 1 {
		args = fmt.Sprintf("(%d/%d) %s", b.BatchIdx, b.BatchTotal, b.ToolArgs)
	}
	header := fmt.Sprintf("%s %s %s  %s", icon, b.ToolName, args, sty.Subtle.Render(arrow))
	accentLine := toolAccent.Render(header)

	if b.Collapsed || b.Content == "" {
		return accentLine
	}

	contentW := width - 6
	if contentW < 10 {
		contentW = 10
	}
	text := tools.TruncateByRune(strings.TrimSpace(b.Content), 1200)
	rendered := styles.RenderMarkdownWithWidth(text, contentW)
	indented := lipgloss.NewStyle().PaddingLeft(2).Render(rendered)

	return lipgloss.JoinVertical(lipgloss.Left, accentLine, indented)
}

func renderBlockThinking(b ContentBlock, sty *styles.Styles) string {
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

func renderBlockText(b ContentBlock, _ *styles.Styles) string {
	return b.Content
}

func RenderBlock(b ContentBlock, width int, sty *styles.Styles) string {
	switch b.Type {
	case BlockToolCall:
		return renderToolLine(b, width, sty)
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

	cardW := width
	if cardW < 20 {
		cardW = 20
	}

	var lines []string
	prevWasTool := false
	for i, b := range blocks {
		if b.BatchTotal > 1 && b.BatchIdx == 1 {
			lines = append(lines, toolAccent.Render(fmt.Sprintf("⚡ %d tools in parallel", b.BatchTotal)))
		}
		if b.Type == BlockThinking && prevWasTool && len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, RenderBlock(b, cardW, sty))
		prevWasTool = b.Type == BlockToolCall
		if b.BatchTotal > 1 && b.BatchIdx == b.BatchTotal && i < len(blocks)-1 {
			lines = append(lines, "")
		}
	}

	body := strings.Join(lines, "\n")
	card := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#c9a96e")).
		PaddingLeft(1).PaddingRight(1).
		Width(cardW).MaxWidth(cardW).
		Render(body)

	return card
}
