// ChatMessage 消息模型 + 类型分发：根据 Role 生成对应的 MessageItem（User/Assistant/System/Error）。
package components

import (
	"primusbot/tui/styles"
)

type ChatMessage struct {
	Role             string
	Content          string
	ReasoningContent string
	RenderedContent  string
}

func (m ChatMessage) ToMessageItem(sty *styles.Styles, id string) Item {
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
		item := NewSystemMessageItem(sty, id, m.Content)
		if m.RenderedContent != "" {
			item.SetRenderedContent(m.RenderedContent)
		}
		return item
	case "error":
		return NewErrorMessageItem(sty, id, m.Content)
	default:
		return NewUserMessageItem(sty, id, m.Content)
	}
}
