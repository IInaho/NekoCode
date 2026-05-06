package components

import (
	"image/color"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"primusbot/tui/styles"
)

const (
	maxTextWidth       = 120
	messageLeftPadding = 2
	barOverhead        = 3
)

var barBorder = lipgloss.Border{Left: "▐"}

type cachedRender struct {
	rendered string
	width    int
	height   int
}

func cappedWidth(available int) int {
	return min(available-messageLeftPadding, maxTextWidth)
}

func CappedWidth(available int) int {
	return cappedWidth(available)
}

func stripLeadingSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n")
}

func thickLeftBar(content string, barColor color.Color, width int) string {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(barBorder).
		BorderForeground(barColor).
		PaddingLeft(1).PaddingRight(1).
		Width(width).MaxWidth(width).
		Render(content)
}

// --- UserMessageItem ---

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
	body := strings.TrimSpace(styles.RenderMarkdownWithWidth(strings.TrimSpace(m.content), contentW))
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

// --- AssistantMessageItem ---

type AssistantMessageItem struct {
	content         string
	renderedContent string
	footer          string
	blocks          []ContentBlock
	sty             *styles.Styles
	cache           cachedRender
	mu              sync.Mutex
}

func NewAssistantMessageItem(sty *styles.Styles, content string) *AssistantMessageItem {
	return &AssistantMessageItem{content: content, sty: sty}
}

func (m *AssistantMessageItem) SetRenderedContent(content string) {
	m.mu.Lock()
	m.renderedContent = content
	m.cache = cachedRender{}
	m.mu.Unlock()
}

func (m *AssistantMessageItem) SetBlocks(blocks []ContentBlock) {
	m.mu.Lock()
	m.blocks = blocks
	m.cache = cachedRender{}
	m.mu.Unlock()
}

func (m *AssistantMessageItem) SetFooter(footer string) {
	m.mu.Lock()
	m.footer = footer
	m.cache = cachedRender{}
	m.mu.Unlock()
}

func (m *AssistantMessageItem) Render(width int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	cw := cappedWidth(width)
	contentW := cw - barOverhead

	header := m.sty.Primary.Bold(true).Render("Assistant")
	msgParts := []string{header, ""}

	if len(m.blocks) > 0 {
		cards := RenderBlocks(m.blocks, contentW, m.sty)
		if cards != "" {
			msgParts = append(msgParts, cards)
		}
	}

	raw := m.content
	if m.renderedContent != "" {
		raw = m.renderedContent
	}
	body := strings.TrimSpace(styles.RenderMarkdownWithWidth(strings.TrimSpace(raw), contentW))
	if body != "" {
		msgParts = append(msgParts, body)
	}

	if m.footer != "" {
		msgParts = append(msgParts, "", styles.SubtleStyle.Render(m.footer))
	}

	msgBlock := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(lipgloss.JoinVertical(lipgloss.Left, msgParts...))), lipgloss.Color("#4ec9b0"), cw)

	out := msgBlock
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *AssistantMessageItem) Height(width int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cw := cappedWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	lines := strings.Count(m.content, "\n") + 1
	for range m.blocks {
		lines += 2
	}
	if m.footer != "" {
		lines += 3
	}
	return lines + 3
}

// --- SystemMessageItem ---

type SystemMessageItem struct {
	content         string
	renderedContent string
	sty             *styles.Styles
	cache           cachedRender
}

func NewSystemMessageItem(sty *styles.Styles, content string) *SystemMessageItem {
	return &SystemMessageItem{content: content, sty: sty}
}

func (m *SystemMessageItem) SetRenderedContent(content string) {
	m.renderedContent = content
	m.cache = cachedRender{}
}

func (m *SystemMessageItem) Render(width int) string {
	cw := cappedWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	contentW := cw - barOverhead
	content := m.renderedContent
	if content == "" {
		content = styles.RenderMarkdownWithWidth(strings.TrimSpace(m.content), contentW)
	}
	header := m.sty.Blue.Bold(true).Render("·")
	parts := []string{header, "", content}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	out := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(joined)), lipgloss.Color("#7a8ba0"), cw)
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *SystemMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.content, "\n") + 3
}

// --- ErrorMessageItem ---

type ErrorMessageItem struct {
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewErrorMessageItem(sty *styles.Styles, content string) *ErrorMessageItem {
	return &ErrorMessageItem{content: content, sty: sty}
}

func (m *ErrorMessageItem) Render(width int) string {
	cw := cappedWidth(width)
	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}
	contentW := cw - barOverhead
	body := strings.TrimSpace(styles.RenderMarkdownWithWidth(strings.TrimSpace(m.content), contentW))
	header := m.sty.Red.Bold(true).Render("!")
	parts := []string{header, "", body}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	out := thickLeftBar(stripLeadingSpaces(strings.TrimSpace(joined)), lipgloss.Color("#e06c75"), cw)
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return out
}

func (m *ErrorMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.height > 0 && m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.content, "\n") + 3
}
