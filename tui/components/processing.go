package components

import (
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type ProcessingItem struct {
	sty          *styles.Styles
	spinnerView  string
	streamText   string
	thinkingText string
	contentWidth int
}

func NewProcessingItem(sty *styles.Styles) *ProcessingItem {
	return &ProcessingItem{sty: sty}
}

func (p *ProcessingItem) ID() string { return "__processing__" }

func (p *ProcessingItem) SetSpinnerView(view string)  { p.spinnerView = view }
func (p *ProcessingItem) SetStreamText(text string)    { p.streamText = text }
func (p *ProcessingItem) SetThinkingText(text string)  { p.thinkingText = text }
func (p *ProcessingItem) SetContentWidth(width int)    { p.contentWidth = width }

func (p *ProcessingItem) Render(width int) string {
	spinnerDisplay := p.spinnerView
	if spinnerDisplay == "" {
		spinnerDisplay = "⋯"
	}

	dimGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a6a5a"))
	cw := p.contentWidth
	if cw <= 0 {
		cw = width - 4
	}

	var s strings.Builder
	s.WriteString(p.sty.Green.Render(styles.Vertical+" ") + p.sty.Green.Render(styles.Fisheye) + " " + spinnerDisplay + " " + p.sty.Subtle.Render("Thinking"))

	if p.thinkingText != "" {
		thinkingRendered := styles.RenderMarkdownWithWidth(p.thinkingText, cw)
		for _, line := range strings.Split(thinkingRendered, "\n") {
			s.WriteString("\n")
			s.WriteString(dimGreen.Render(styles.Vertical+" ") + p.sty.Subtle.Render(line))
		}
	}

	if p.streamText != "" {
		rendered := styles.RenderMarkdownWithWidth(p.streamText, cw)
		for _, line := range strings.Split(rendered, "\n") {
			s.WriteString("\n")
			s.WriteString(p.sty.Green.Render(styles.Vertical+" ") + line)
		}
	}
	return s.String()
}

func (p *ProcessingItem) Height(width int) int {
	return strings.Count(p.Render(width), "\n") + 1
}
