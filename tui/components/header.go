package components

import (
	"fmt"
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type Header struct {
	Width    int
	Provider string
	Model    string
	Version  string
	Tokens   int
}

func NewHeader(width int, provider, model, version string) *Header {
	return &Header{
		Width:    width,
		Provider: provider,
		Model:    model,
		Version:  version,
	}
}

func (h *Header) SetWidth(width int) { h.Width = width }
func (h *Header) SetTokens(total int) { h.Tokens = total }
func (h *Header) Height() int         { return 2 }

func (h *Header) View() string {
	w := max(20, h.Width)

	catIcon := styles.CatBodyStyle.Render("(=") + styles.CatEyeStyle.Render("^.^") + styles.CatBodyStyle.Render("=)")
	left := catIcon + " " + styles.PrimaryStyle.Bold(true).Render("PRIMUS") + " " + styles.SubtleStyle.Render("v"+h.Version)
	right := styles.MutedStyle.Render(h.Provider + "/" + h.Model)
	dot := styles.BorderStyle.Render(" · ")

	if h.Tokens > 0 {
		right = fmtTokens(h.Tokens) + dot + right
	}

	content := left + dot + right
	contentW := lipgloss.Width(content)
	pad := max(0, w-contentW)

	line := strings.Repeat(styles.Horizontal, w)

	var b strings.Builder
	b.WriteString(content + strings.Repeat(" ", pad) + "\n")
	b.WriteString(styles.BorderStyle.Render(line) + "\n")

	return b.String()
}

func fmtTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
