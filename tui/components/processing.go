package components

import (
	"strings"

	"primusbot/tui/styles"
)

type ProcessingItem struct {
	sty          *styles.Styles
	spinnerView  string
	statusText   string
	contentWidth int
	blocks       []ContentBlock
}

func NewProcessingItem(sty *styles.Styles) *ProcessingItem {
	return &ProcessingItem{sty: sty}
}

func (p *ProcessingItem) SetSpinnerView(view string)  { p.spinnerView = view }
func (p *ProcessingItem) SetStatusText(text string)    { p.statusText = text }
func (p *ProcessingItem) SetContentWidth(width int)    { p.contentWidth = width }
func (p *ProcessingItem) SetBlocks(blocks []ContentBlock) { p.blocks = blocks }

func (p *ProcessingItem) Render(width int) string {
	spinnerDisplay := p.spinnerView
	if spinnerDisplay == "" {
		spinnerDisplay = "..."
	}

	cw := p.contentWidth
	if cw <= 0 {
		cw = width - 4
	}

	var sb strings.Builder
	label := p.statusText
	if label == "" {
		label = "Thinking"
	}
	sb.WriteString(p.sty.Green.Render(styles.Vertical+" ") + p.sty.Green.Render(styles.Fisheye) + " " + spinnerDisplay + " " + p.sty.Subtle.Render(label))

	// Render completed blocks during streaming.
	if len(p.blocks) > 0 {
		green := p.sty.Green.Render(styles.Vertical)
		blocksStr := RenderBlocks(p.blocks, cw, p.sty)
		for _, line := range strings.Split(blocksStr, "\n") {
			sb.WriteString("\n")
			sb.WriteString(green + " " + line)
		}
	}

	return sb.String()
}

func (p *ProcessingItem) Height(width int) int {
	return strings.Count(p.Render(width), "\n") + 1
}
