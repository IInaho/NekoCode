// block_render.go — RenderBlock 分发器 + RenderBlocks 卡片包裹。
package block

import (
	"fmt"
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

func RenderBlock(b ContentBlock, width int, sty *styles.Styles) string {
	switch b.Type {
	case BlockTool:
		return renderToolLine(b, width, sty)
	case BlockThought:
		return renderBlockThought(b, sty)
	case BlockReason:
		return renderBlockReason(b, sty)
	default:
		return b.Content
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
			lines = append(lines, "  "+toolAccent.Render(fmt.Sprintf("⚡ %d tools in parallel", b.BatchTotal)))
		}
		if b.Type == BlockThought && prevWasTool && len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, RenderBlock(b, cardW, sty))
		prevWasTool = b.Type == BlockTool
		if b.BatchTotal > 1 && b.BatchIdx == b.BatchTotal && i < len(blocks)-1 {
			lines = append(lines, "")
		}
	}

	body := strings.Join(lines, "\n")
	card := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(styles.Yellow)).
		PaddingLeft(1).PaddingRight(1).
		Width(cardW).MaxWidth(cardW).
		Render(body)

	return card
}
