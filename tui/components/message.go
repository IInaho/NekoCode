package components

import "primusbot/tui/styles"

type ChatMessage struct {
	Role            string
	Content         string
	RenderedContent string
	Blocks          []ContentBlock
	Footer          string // TUI metadata: duration, tokens — rendered separately from LLM content
}

func (m ChatMessage) ToMessageItem(sty *styles.Styles) Item {
	switch m.Role {
	case "user":
		return NewUserMessageItem(sty, m.Content)
	case "assistant":
		item := NewAssistantMessageItem(sty, m.Content)
		if m.RenderedContent != "" {
			item.SetRenderedContent(m.RenderedContent)
		}
		item.SetBlocks(m.Blocks)
		if m.Footer != "" {
			item.SetFooter(m.Footer)
		}
		return item
	case "system":
		item := NewSystemMessageItem(sty, m.Content)
		if m.RenderedContent != "" {
			item.SetRenderedContent(m.RenderedContent)
		}
		return item
	case "error":
		return NewErrorMessageItem(sty, m.Content)
	default:
		return NewUserMessageItem(sty, m.Content)
	}
}
