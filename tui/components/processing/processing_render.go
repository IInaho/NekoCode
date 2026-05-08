// processing_render.go — Render 编排器 + 5 个 section 渲染方法。
package processing

import (
	"strings"

	"primusbot/tui/components/block"
	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

func (p *ProcessingItem) Render(width int) string {
	if p.cachedRenderW == width && p.cachedRender != "" {
		return p.cachedRender
	}
	cw := p.contentWidth
	if cw <= 0 {
		cw = width - 4
	}
	contentW := cw - 4

	var sections []string
	sections = append(sections, p.renderHeader())
	if s := p.renderTodos(cw); s != "" {
		sections = append(sections, s)
	}
	if s := p.renderToolSection(contentW, cw); s != "" {
		sections = append(sections, s)
	}
	if s := p.renderOutputSection(contentW); s != "" {
		sections = append(sections, s)
	}
	if s := p.renderReasoningSection(contentW); s != "" {
		sections = append(sections, s)
	}

	body := strings.Join(sections, "\n")
	card := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(styles.Primary)).
		PaddingLeft(0).PaddingRight(0).Width(cw).MaxWidth(cw).Render(body)

	p.cachedRender = card
	p.cachedRenderW = width
	p.cachedHeight = strings.Count(card, "\n") + 1
	return card
}

func (p *ProcessingItem) renderHeader() string {
	s := p.spinnerView
	if s == "" {
		s = "..."
	}
	l := p.statusText
	if l == "" {
		l = "Thinking"
	}
	tp := ""
	if p.tokenPrompt > 0 || p.tokenCompl > 0 {
		tp = "  " + p.sty.Subtle.Render("↑"+styles.FmtTokens(p.tokenPrompt)) + " " + p.sty.Teal.Render("↓"+styles.FmtTokens(p.tokenCompl))
	}
	return p.sty.Teal.Render(s) + " " + p.sty.Subtle.Render(l) + tp
}

func (p *ProcessingItem) renderTodos(cw int) string {
	if p.todos == "" {
		return ""
	}
	if p.cachedTodosW < 0 || cw != p.cachedTodosW {
		p.cachedTodos = ""
		for _, line := range strings.Split(p.todos, "\n") {
			p.cachedTodos += "\n  " + line
		}
		p.cachedTodosW = cw
	}
	return p.cachedTodos
}

func (p *ProcessingItem) renderToolSection(contentW, cw int) string {
	toolN := 0
	for _, b := range p.blocks {
		if b.Type == block.BlockTool {
			toolN++
		}
	}
	if toolN != p.cachedToolN || cw != p.cachedToolW {
		p.cachedTool = ""
		if toolN > 0 {
			var sb strings.Builder
			for _, b := range p.blocks {
				switch b.Type {
				case block.BlockTool:
					for _, l := range strings.Split(block.RenderBlock(b, contentW, p.sty), "\n") {
						sb.WriteString("\n" + l)
					}
				}
			}
			p.cachedTool = sb.String()
		}
		p.cachedToolN = toolN
		p.cachedToolW = cw
	}
	return p.cachedTool
}

func (p *ProcessingItem) renderOutputSection(contentW int) string {
	text := strings.TrimSpace(p.outputText)
	if text == "" {
		return ""
	}
	var sb strings.Builder
	if p.cachedTool != "" {
		sb.WriteString("\n")
	}
	sep := p.sty.Teal.Render("▍ output " + strings.Repeat("─", contentW-lipgloss.Width("▍ output ")))
	sb.WriteString(sep)
	sb.WriteString("\n\n")
	sb.WriteString(RenderFixed(WrapPlain(text, contentW), outputLines, false, p.sty.Subtle))
	return sb.String()
}

func (p *ProcessingItem) renderReasoningSection(contentW int) string {
	if p.reasoningText == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sep := p.sty.Blue.Render("▍ reasoning " + strings.Repeat("─", contentW-lipgloss.Width("▍ reasoning ")))
	sb.WriteString(sep)
	sb.WriteString("\n\n")
	sb.WriteString(RenderFixed(WrapPlain(p.reasoningText, contentW), reasonLines, false, p.sty.Muted))
	return sb.String()
}

