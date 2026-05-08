// block.go — BlockType 枚举、ContentBlock 结构体、共享样式变量。
package block

import (
	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type BlockType int

const (
	BlockTool    BlockType = iota
	BlockThought
	BlockReason
)

type ContentBlock struct {
	Type       BlockType
	Content    string
	ToolName   string
	ToolArgs   string
	Collapsed  bool
	BatchIdx   int
	BatchTotal int
}

var toolAccent = lipgloss.NewStyle().Foreground(lipgloss.Color(styles.Yellow))
