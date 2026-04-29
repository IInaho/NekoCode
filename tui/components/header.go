package components

import (
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type Header struct {
	Width    int
	Provider string
	Model    string
	Version  string
}

func NewHeader(width int, provider, model, version string) *Header {
	return &Header{
		Width:    width,
		Provider: provider,
		Model:    model,
		Version:  version,
	}
}

func (h *Header) SetWidth(width int) {
	h.Width = width
}

func (h *Header) SetInfo(provider, model string) {
	h.Provider = provider
	h.Model = model
}

func (h *Header) Height() int {
	return 2
}

func (h *Header) View() string {
	w := maxInt(20, h.Width)

	catIcon := styles.CatBodyStyle.Render("(=") + styles.CatEyeStyle.Render("^.^") + styles.CatBodyStyle.Render("=)")
	left := catIcon + " " + styles.PrimaryStyle.Bold(true).Render("PRIMUS") + " " + styles.SubtleStyle.Render("v"+h.Version)
	right := styles.MutedStyle.Render(h.Provider + "/" + h.Model)
	dot := styles.BorderStyle.Render(" · ")
	content := left + dot + right
	contentW := lipgloss.Width(content)
	pad := maxInt(0, w-contentW)

	line := strings.Repeat(styles.Horizontal, w)

	var b strings.Builder
	b.WriteString(content + strings.Repeat(" ", pad) + "\n")
	b.WriteString(styles.BorderStyle.Render(line) + "\n")

	return b.String()
}

