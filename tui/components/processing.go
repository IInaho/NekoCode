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
	tokenPrompt  int
	tokenCompl   int
	cachedTool   string
	cachedToolN  int
	cachedWidth  int
}

func NewProcessingItem(sty *styles.Styles) *ProcessingItem {
	return &ProcessingItem{sty: sty}
}

func (p *ProcessingItem) SetSpinnerView(view string)     { p.spinnerView = view }
func (p *ProcessingItem) SetStatusText(text string)       { p.statusText = text }
func (p *ProcessingItem) SetContentWidth(width int)       { p.contentWidth = width }
func (p *ProcessingItem) SetBlocks(blocks []ContentBlock) { p.blocks = blocks }
func (p *ProcessingItem) SetTokens(prompt, completion int) {
	p.tokenPrompt = prompt
	p.tokenCompl = completion
}

func (p *ProcessingItem) Render(width int) string {
	spinnerDisplay := p.spinnerView
	if spinnerDisplay == "" {
		spinnerDisplay = "..."
	}
	cw := p.contentWidth
	if cw <= 0 {
		cw = width - 4
	}

	toolN := len(p.blocks)
	streamText := ""
	if toolN > 0 && p.blocks[toolN-1].Type == BlockText {
		streamText = p.blocks[toolN-1].Content
		toolN--
	}

	if toolN != p.cachedToolN || cw != p.cachedWidth {
		p.cachedTool = ""
		if toolN > 0 {
			green := p.sty.Green.Render(styles.Vertical)
			toolStr := RenderBlocks(p.blocks[:toolN], cw, p.sty)
			var sb strings.Builder
			for _, line := range strings.Split(toolStr, "\n") {
				sb.WriteString("\n")
				sb.WriteString(green + " " + line)
			}
			p.cachedTool = sb.String()
		}
		p.cachedToolN = toolN
		p.cachedWidth = cw
	}

	streamOut := ""
	if streamText != "" {
		green := p.sty.Green.Render(styles.Vertical)
		wrapped := wrapPlain(streamText, cw)
		for _, line := range strings.Split(wrapped, "\n") {
			streamOut += "\n" + green + " " + line
		}
	}

	label := p.statusText
	if label == "" {
		label = "Thinking"
	}
	tokenPart := ""
	if p.tokenPrompt > 0 || p.tokenCompl > 0 {
		tokenPart = " " + p.sty.Subtle.Render("↑"+fmtTokens(p.tokenPrompt)) + " " + p.sty.Green.Render("↓"+fmtTokens(p.tokenCompl))
	}

	return p.sty.Green.Render(styles.Vertical+" ") +
		p.sty.Green.Render(styles.Fisheye) + " " +
		spinnerDisplay +
		p.sty.Subtle.Render(label) +
		tokenPart +
		p.cachedTool +
		streamOut
}

func (p *ProcessingItem) Height(width int) int {
	return strings.Count(p.Render(width), "\n") + 1
}

func wrapPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	var lines []string
	for i := 0; i < len(runes); i += width {
		end := i + width
		if end > len(runes) {
			end = len(runes)
		}
		lines = append(lines, string(runes[i:end]))
	}
	return strings.Join(lines, "\n")
}
