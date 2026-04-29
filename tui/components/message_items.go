package components

import (
	"strings"
	"sync"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
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

// CappedWidth returns the maximum text content width for a given available width.
func CappedWidth(available int) int {
	return cappedWidth(available)
}

// --- UserMessageItem ---

type UserMessageItem struct {
	id      string
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewUserMessageItem(sty *styles.Styles, id, content string) *UserMessageItem {
	return &UserMessageItem{id: id, content: content, sty: sty}
}

func (m *UserMessageItem) ID() string { return m.id }

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
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

// --- AssistantMessageItem ---

type AssistantMessageItem struct {
	id               string
	content          string
	reasoningContent string
	renderedContent  string
	sty              *styles.Styles
	cache            cachedRender
	mu               sync.Mutex
}

func NewAssistantMessageItem(sty *styles.Styles, id, content string) *AssistantMessageItem {
	return &AssistantMessageItem{id: id, content: content, sty: sty}
}

func (m *AssistantMessageItem) ID() string { return m.id }

func (m *AssistantMessageItem) SetRenderedContent(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renderedContent = content
	m.cache = cachedRender{}
}

func (m *AssistantMessageItem) SetReasoningContent(reasoning string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reasoningContent = reasoning
	m.cache = cachedRender{}
}

func (m *AssistantMessageItem) Render(width int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	dimGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#3a6a5a"))

	var s strings.Builder
	s.WriteString(m.sty.Green.Render(styles.Vertical+" ") + m.sty.Primary.Bold(true).Render("Assistant") + "\n")

	if m.reasoningContent != "" {
		thinkingRendered := styles.RenderMarkdownWithWidth(m.reasoningContent, cw)
		lines := strings.Split(thinkingRendered, "\n")
		for _, line := range lines {
			s.WriteString(dimGreen.Render(styles.Vertical+" ") + m.sty.Subtle.Render(line) + "\n")
		}
		s.WriteString(m.sty.Green.Render(styles.Vertical) + "\n")
	}

	content := m.content
	if m.renderedContent != "" {
		content = m.renderedContent
	}
	content = strings.TrimSpace(content)
	content = styles.RenderMarkdownWithWidth(content, cw)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		s.WriteString(m.sty.Green.Render(styles.Vertical+" ") + line + "\n")
	}

	out := strings.TrimRight(s.String(), "\n")
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return m.cache.rendered
}

func (m *AssistantMessageItem) Height(width int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

// --- SystemMessageItem ---

type SystemMessageItem struct {
	id      string
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewSystemMessageItem(sty *styles.Styles, id, content string) *SystemMessageItem {
	return &SystemMessageItem{id: id, content: content, sty: sty}
}

func (m *SystemMessageItem) ID() string { return m.id }

func (m *SystemMessageItem) Render(width int) string {
	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	content := strings.TrimSpace(m.content)

	var s strings.Builder
	s.WriteString(m.sty.Blue.Render(styles.Vertical+" ") + m.sty.Blue.Bold(true).Render("·") + " " + content)

	out := s.String()
	m.cache.rendered = out
	m.cache.width = cw
	m.cache.height = strings.Count(out, "\n") + 1
	return m.cache.rendered
}

func (m *SystemMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

// --- ErrorMessageItem ---

type ErrorMessageItem struct {
	id      string
	content string
	sty     *styles.Styles
	cache   cachedRender
}

func NewErrorMessageItem(sty *styles.Styles, id, content string) *ErrorMessageItem {
	return &ErrorMessageItem{id: id, content: content, sty: sty}
}

func (m *ErrorMessageItem) ID() string { return m.id }

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
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}
