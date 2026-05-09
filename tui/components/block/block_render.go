// block_render.go — RenderBlock 分发器 + RenderBlocks 卡片包裹（含同名单行收折）。
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

// ToolGroupInfo describes a group of consecutive same-name tool blocks.
type ToolGroupInfo struct {
	Name  string
	Count int
	First ContentBlock
	Rest  []ContentBlock
}

// BuildToolGroups groups consecutive same-name tool blocks.
func BuildToolGroups(blocks []ContentBlock) []ToolGroupInfo {
	var groups []ToolGroupInfo
	for _, b := range blocks {
		if b.Type != BlockTool {
			continue
		}
		if len(groups) > 0 && groups[len(groups)-1].Name == b.ToolName {
			g := &groups[len(groups)-1]
			g.Count++
			g.Rest = append(g.Rest, b)
		} else {
			groups = append(groups, ToolGroupInfo{Name: b.ToolName, Count: 1, First: b})
		}
	}
	return groups
}

func RenderBlocks(blocks []ContentBlock, width int, sty *styles.Styles) string {
	if len(blocks) == 0 {
		return ""
	}

	cardW := width
	if cardW < 20 {
		cardW = 20
	}

	groups := BuildToolGroups(blocks)
	var lines []string
	for _, g := range groups {
		if g.Count <= 1 {
			lines = append(lines, RenderBlock(g.First, cardW, sty))
		} else {
			lines = append(lines, renderGroupLine(g, cardW, sty))
		}
	}

	if len(lines) == 0 {
		return ""
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

func renderGroupLine(g ToolGroupInfo, cardW int, sty *styles.Styles) string {
	header := fmt.Sprintf("◆ %s ×%d", g.Name, g.Count)
	arrow := ""
	if g.First.Collapsed {
		arrow = " " + sty.Subtle.Render("[+] 展开")
	} else {
		arrow = " " + sty.Subtle.Render("[-] 收起")
	}
	accentLine := "  " + toolAccent.Render(header+arrow)

	if g.First.Collapsed {
		return accentLine
	}

	// For edit groups, expand diffs inline — one level, no nested toggles.
	if g.Name == "edit" {
		return RenderEditGroupExpanded(g, cardW, sty, accentLine)
	}

	indent := "  "
	var sb strings.Builder
	sb.WriteString(accentLine)
	all := append([]ContentBlock{g.First}, g.Rest...)
	for _, b := range all {
		line := RenderBlock(b, cardW, sty)
		for _, l := range strings.Split(line, "\n") {
			if l != "" {
				sb.WriteString("\n" + indent + l)
			}
		}
	}
	return sb.String()
}

// RenderEditGroupExpanded renders an expanded edit group with each file's
// diff shown inline under a ▍ path header. Single toggle, all diffs visible.
func RenderEditGroupExpanded(g ToolGroupInfo, cardW int, sty *styles.Styles, accentLine string) string {
	var sb strings.Builder
	sb.WriteString(accentLine)

	diffW := cardW - 6
	if diffW < 10 {
		diffW = 10
	}

	all := append([]ContentBlock{g.First}, g.Rest...)
	for _, b := range all {
		// File path header
		fileHeader := sty.Teal.Render("▍ ") + sty.Subtle.Render(b.ToolArgs)
		sb.WriteString("\n  " + fileHeader)

		if b.Content == "" {
			continue
		}
		// Diff content, indented 4 spaces under the file header
		diff := renderBlockDiff(ContentBlock{Content: b.Content}, sty)
		for _, l := range strings.Split(diff, "\n") {
			if l != "" {
				sb.WriteString("\n    " + l)
			}
		}
	}
	return sb.String()
}
