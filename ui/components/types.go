package components

import (
	"strings"
	"sync"

	"primusbot/ui/components/list"
	"primusbot/ui/styles"
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
	lineWidth := max(10, width-2)
	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	content := strings.TrimSpace(m.content)
	content = styles.RenderMarkdownWithWidth(content, cw)

	var s strings.Builder
	s.WriteString(m.sty.Muted.Render("╭─ ") + m.sty.Yellow.Render("You") + "\n")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		s.WriteString(m.sty.Muted.Render("│ ") + line + "\n")
	}
	s.WriteString(m.sty.Muted.Render("╰" + strings.Repeat("─", lineWidth)))

	m.cache.rendered = s.String()
	m.cache.width = cw
	m.cache.height = strings.Count(m.cache.rendered, "\n") + 1
	return m.cache.rendered
}

func (m *UserMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

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

	lineWidth := max(10, width-2)
	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	var s strings.Builder
	s.WriteString(m.sty.Green.Render("╭─ ") + m.sty.Primary.Render("Assistant") + "\n")

	if m.reasoningContent != "" {
		thinkingRendered := styles.RenderMarkdownWithWidth(m.reasoningContent, cw)
		lines := strings.Split(thinkingRendered, "\n")
		for _, line := range lines {
			s.WriteString(m.sty.Muted.Render("│ ") + m.sty.Subtle.Render(line) + "\n")
		}
		s.WriteString(m.sty.Muted.Render("├"+strings.Repeat("─", lineWidth-1)) + "\n")
	}

	content := m.content
	if m.renderedContent != "" {
		content = m.renderedContent
	}
	content = strings.TrimSpace(content)
	content = styles.RenderMarkdownWithWidth(content, cw)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		s.WriteString(m.sty.Muted.Render("│ ") + line + "\n")
	}
	s.WriteString(m.sty.Muted.Render("╰" + strings.Repeat("─", lineWidth)))

	m.cache.rendered = s.String()
	m.cache.width = cw
	m.cache.height = strings.Count(m.cache.rendered, "\n") + 1
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
	lineWidth := max(10, width-2)
	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	content := strings.TrimSpace(m.content)

	var s strings.Builder
	s.WriteString(m.sty.Muted.Render("╭─ ") + m.sty.Blue.Render("System") + "\n")
	s.WriteString(m.sty.Muted.Render("│ ") + content + "\n")
	s.WriteString(m.sty.Muted.Render("╰" + strings.Repeat("─", lineWidth)))

	m.cache.rendered = s.String()
	m.cache.width = cw
	m.cache.height = strings.Count(m.cache.rendered, "\n") + 1
	return m.cache.rendered
}

func (m *SystemMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

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
	lineWidth := max(10, width-2)
	cw := cappedWidth(width)

	if m.cache.width == cw && m.cache.rendered != "" {
		return m.cache.rendered
	}

	content := strings.TrimSpace(m.content)

	var s strings.Builder
	s.WriteString(m.sty.Muted.Render("╭─ ") + m.sty.Red.Render("Error") + "\n")
	s.WriteString(m.sty.Muted.Render("│ ") + content + "\n")
	s.WriteString(m.sty.Muted.Render("╰" + strings.Repeat("─", lineWidth)))

	m.cache.rendered = s.String()
	m.cache.width = cw
	m.cache.height = strings.Count(m.cache.rendered, "\n") + 1
	return m.cache.rendered
}

func (m *ErrorMessageItem) Height(width int) int {
	cw := cappedWidth(width)
	if m.cache.width == cw {
		return m.cache.height
	}
	return strings.Count(m.Render(width), "\n") + 1
}

type ProcessingItem struct {
	sty             *styles.Styles
	spinnerView     string
	streamText      string
	thinkingText    string
	streamTextCache string
	thinkingCache   string
	renderedWidth   int
}

func NewProcessingItem(sty *styles.Styles) *ProcessingItem {
	return &ProcessingItem{sty: sty}
}

func (p *ProcessingItem) ID() string { return "__processing__" }

func (p *ProcessingItem) SetSpinnerView(view string) {
	p.spinnerView = view
}

func (p *ProcessingItem) SetStreamText(text string) {
	if p.streamText != text {
		p.streamText = text
		p.streamTextCache = ""
	}
}

func (p *ProcessingItem) SetThinkingText(text string) {
	if p.thinkingText != text {
		p.thinkingText = text
		p.thinkingCache = ""
	}
}

func (p *ProcessingItem) Render(width int) string {
	spinnerDisplay := p.spinnerView
	if spinnerDisplay == "" {
		spinnerDisplay = "⋯"
	}

	var s strings.Builder
	s.WriteString(p.sty.Green.Render("◉ ") + spinnerDisplay + " " + p.sty.Muted.Render("Thinking..."))

	if p.thinkingText != "" {
		s.WriteString("\n")
		if p.renderedWidth == width && p.thinkingCache != "" {
			s.WriteString(p.thinkingCache)
		} else {
			thinkingRendered := styles.RenderMarkdownWithWidth(p.thinkingText, width-4)
			p.thinkingCache = thinkingRendered
			p.renderedWidth = width
			s.WriteString(p.sty.Subtle.Render(thinkingRendered))
		}
	}

	if p.streamText != "" {
		s.WriteString("\n")
		if p.renderedWidth == width && p.streamTextCache != "" {
			s.WriteString(p.streamTextCache)
		} else {
			rendered := styles.RenderMarkdownWithWidth(p.streamText, width-4)
			p.streamTextCache = rendered
			p.renderedWidth = width
			s.WriteString(rendered)
		}
	}
	return s.String()
}

func (p *ProcessingItem) Height(width int) int {
	count := strings.Count(p.Render(width), "\n") + 1
	return count
}

type ChatMessage struct {
	Role             string
	Content          string
	ReasoningContent string
	RenderedContent  string
}

func (m ChatMessage) ToMessageItem(sty *styles.Styles, id string) list.Item {
	switch m.Role {
	case "user":
		return NewUserMessageItem(sty, id, m.Content)
	case "assistant":
		item := NewAssistantMessageItem(sty, id, m.Content)
		if m.RenderedContent != "" {
			item.SetRenderedContent(m.RenderedContent)
		}
		if m.ReasoningContent != "" {
			item.SetReasoningContent(m.ReasoningContent)
		}
		return item
	case "system":
		return NewSystemMessageItem(sty, id, m.Content)
	case "error":
		return NewErrorMessageItem(sty, id, m.Content)
	default:
		return NewUserMessageItem(sty, id, m.Content)
	}
}
