package components

import (
	"strings"
	"sync"

	"primusbot/tui/styles"
)

const (
	maxTextWidth       = 120
	messageLeftPadding = 2
)

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

	content := strings.TrimSpace(m.content)
	content = styles.RenderMarkdownWithWidth(content, cw)

	var s strings.Builder
	s.WriteString(m.sty.Yellow.Render(styles.Vertical+" ") + m.sty.Yellow.Bold(true).Render("You") + "\n")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		s.WriteString(m.sty.Yellow.Render(styles.Vertical+" ") + line + "\n")
	}

	out := strings.TrimRight(s.String(), "\n")
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return m.cache.rendered
}

func (m *UserMessageItem) Height(width int) int {
	return strings.Count(m.Render(width), "\n") + 1
}

// --- AssistantMessageItem ---

type AssistantMessageItem struct {
	content         string
	renderedContent string
	blocks           []ContentBlock
	sty              *styles.Styles
	cache            cachedRender
	mu               sync.Mutex
}

func NewAssistantMessageItem(sty *styles.Styles, content string) *AssistantMessageItem {
	return &AssistantMessageItem{content: content, sty: sty}
}

func (m *AssistantMessageItem) SetRenderedContent(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renderedContent = content
	m.cache = cachedRender{}
}

func (m *AssistantMessageItem) SetBlocks(blocks []ContentBlock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks = blocks
	m.cache = cachedRender{}
}


func (m *AssistantMessageItem) Render(width int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	cw := cappedWidth(width)

	green := m.sty.Green.Render(styles.Vertical)
	var sb strings.Builder
	sb.WriteString(green + " " + m.sty.Primary.Bold(true).Render("Assistant") + "\n")

	// Render content blocks (tool calls, thinking, etc.)
	for _, b := range m.blocks {
		rendered := RenderBlock(b, cw, m.sty)
		for _, line := range strings.Split(rendered, "\n") {
			sb.WriteString(green + " " + line + "\n")
		}
	}

	// Render final text response
	sb.WriteString(green + "\n")
	content := m.content
	if m.renderedContent != "" {
		content = m.renderedContent
	}
	content = strings.TrimSpace(content)
	content = styles.RenderMarkdownWithWidth(content, cw)
	for _, line := range strings.Split(content, "\n") {
		sb.WriteString(green + " " + line + "\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (m *AssistantMessageItem) Height(width int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.Count(m.Render(width), "\n") + 1
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

	content := m.renderedContent
	if content == "" {
		content = strings.TrimSpace(m.content)
	}

	var s strings.Builder
	s.WriteString(m.sty.Blue.Render(styles.Vertical+" ") + m.sty.Blue.Bold(true).Render(".") + " " + content)

	out := s.String()
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return m.cache.rendered
}

func (m *SystemMessageItem) Height(width int) int {
	return strings.Count(m.Render(width), "\n") + 1
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

	content := strings.TrimSpace(m.content)

	var s strings.Builder
	s.WriteString(m.sty.Red.Render(styles.Vertical+" ") + m.sty.Red.Bold(true).Render("!") + " " + content)

	out := s.String()
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return m.cache.rendered
}

func (m *ErrorMessageItem) Height(width int) int {
	return strings.Count(m.Render(width), "\n") + 1
}
