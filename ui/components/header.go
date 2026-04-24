package components

import (
	"strings"

	"primusbot/ui/styles"
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
	return 5
}

func (h *Header) View() string {
	w := maxInt(20, h.Width)
	line := strings.Repeat("─", w-2)

	var b strings.Builder
	b.WriteString(styles.PrimaryStyle.Render("╭"+line+"╮") + "\n")
	b.WriteString(styles.PrimaryStyle.Render("│") + styles.PrimaryStyle.Bold(true).Render("  PRIMUS  ") + styles.SubtleStyle.Render("v"+h.Version) + strings.Repeat(" ", w-20) + styles.PrimaryStyle.Render("  │") + "\n")
	b.WriteString(styles.PrimaryStyle.Render("├"+line+"┤") + "\n")
	b.WriteString(styles.MutedStyle.Render("│ Provider: ") + styles.BlueStyle.Render(h.Provider) + styles.MutedStyle.Render("/") + styles.PrimaryStyle.Render(h.Model) + strings.Repeat(" ", maxInt(0, w-14-len(h.Provider)-len(h.Model))) + styles.MutedStyle.Render("│") + "\n")
	b.WriteString(styles.PrimaryStyle.Render("├"+line+"┤") + "\n")

	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
