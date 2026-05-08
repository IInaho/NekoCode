// message_assistant.go — AssistantMessageItem：助手消息渲染（teal 左侧竖条 + 工具卡片）。
package message

import (
	"strings"
	"sync"

	"primusbot/tui/components/block"
	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type AssistantMessageItem struct {
	content         string
	renderedContent string
	footer          string
	blocks          []block.ContentBlock
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

func (m *AssistantMessageItem) SetBlocks(blocks []block.ContentBlock) {
	m.mu.Lock()
	m.blocks = blocks
	m.cache = cachedRender{}
	m.mu.Unlock()
}

func (m *AssistantMessageItem) Blocks() []block.ContentBlock {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.blocks
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
		cards := block.RenderBlocks(m.blocks, contentW, m.sty)
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
