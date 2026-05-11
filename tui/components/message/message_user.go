// message_user.go — UserMessageItem：用户消息渲染（金色左侧竖条）。
package message

import (
	"strings"

	"nekocode/tui/styles"

	"charm.land/lipgloss/v2"
)

type UserMessageItem struct {

	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewUserMessageItem(sty *styles.Styles, content string) *UserMessageItem {
	return &UserMessageItem{content: content, sty: sty}
}

func (m *UserMessageItem) Render(width int) string {
	cw := cappedWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	contentW := cw - barOverhead
	header := m.sty.Yellow.Bold(true).Render("You")
	body := strings.TrimSpace(RenderMarkdown(strings.TrimSpace(m.content), contentW))
	parts := []string{header, "", body}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	out := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(joined)), lipgloss.Color("#c9a96e"), cw)
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *UserMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	lines := strings.Count(m.content, "\n") + 1
	return lines + 3
}
