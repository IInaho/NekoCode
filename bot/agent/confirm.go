// 确认机制：Agent 执行危险操作前，通过 ConfirmFunc 回调征求用户同意。
// ConfirmRequest 携带工具名、参数、危险等级和应答 channel，
// TUI 监听该 channel 渲染确认栏，用户 enter/esc 后返回结果。
package agent

import (
	"primusbot/bot/tools"
)

type ConfirmRequest struct {
	ToolName string
	Args     map[string]interface{}
	Level    tools.DangerLevel
	Response chan bool
}

type ConfirmFunc func(req ConfirmRequest) bool
