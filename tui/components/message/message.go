// message.go — ChatMessage 类型：用户/助手/系统对话消息的数据载体。
package message

import (
	"primusbot/tui/components/block"
)

type ChatMessage struct {
	Role            string
	Content         string
	RenderedContent string
	Blocks          []block.ContentBlock
	Footer          string
}
