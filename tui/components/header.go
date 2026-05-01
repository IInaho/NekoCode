// Header 顶部状态栏：猫脸图标 + 应用名 + 版本 + token 用量 + provider/model。
// token 用量按消耗比例着色（灰/黄/红），每次消息往返后刷新。
package components

import (
	"fmt"
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type Header struct {
	Width       int
	Provider    string
	Model       string
	Version     string
	Tokens      int
	TokenBudget int
}

func NewHeader(width int, provider, model, version string) *Header {
	return &Header{
		Width:       width,
		Provider:    provider,
		Model:       model,
		Version:     version,
		TokenBudget: 8000,
	}
}

func (h *Header) SetWidth(width int) { h.Width = width }
func (h *Header) SetTokens(used, budget int) {
	h.Tokens = used
	h.TokenBudget = budget
}

func (h *Header) Height() int {
	return 2
}

func (h *Header) View() string {
	w := max(20, h.Width)

	catIcon := styles.CatBodyStyle.Render("(=") + styles.CatEyeStyle.Render("^.^") + styles.CatBodyStyle.Render("=)")
	left := catIcon + " " + styles.PrimaryStyle.Bold(true).Render("PRIMUS") + " " + styles.SubtleStyle.Render("v"+h.Version)
	right := styles.MutedStyle.Render(h.Provider + "/" + h.Model)
	dot := styles.BorderStyle.Render(" · ")

	var tokenStr string
	if h.Tokens > 0 {
		pct := h.Tokens * 100 / h.TokenBudget
		tk := fmtTokens(h.Tokens)
		bk := fmtTokens(h.TokenBudget)
		switch {
		case pct >= 90:
			tokenStr = styles.RedStyle.Render(tk + "/" + bk)
		case pct >= 60:
			tokenStr = styles.YellowStyle.Render(tk + "/" + bk)
		default:
			tokenStr = styles.SubtleStyle.Render(tk + "/" + bk)
		}
		right = tokenStr + dot + right
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

