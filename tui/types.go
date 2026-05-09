// types.go — TUI 类型定义：BotInterface、状态枚举、消息类型。
package tui

import (
	"nekocode/bot/tools"
)

// BotInterface is the contract any bot implementation must satisfy for the TUI.
type BotInterface interface {
	RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)) (string, error)
	ExecuteCommand(input string) (string, bool)
	TokenUsage() (prompt, completion int)
	ContextTokens() int
	CompactCount() int
	Duration() string
	CommandNames() []string
	SetConfirmFn(tools.ConfirmFunc)
	SetPhaseFn(tools.PhaseFunc)
	Steer(msg string)
	Abort()
	SetStreamFn(fn func(delta string))
	SetReasoningStreamFn(fn func(delta string))
	WireTodoWrite(fn tools.TodoFunc)
	SetCtxTodos(text string)
	Provider() string
	Model() string
}

type chatState int

const (
	stateReady chatState = iota
	stateProcessing
	stateConfirming
)

type doneMsg struct {
	content  string
	duration string
	tokens   string
	err      error
}

type confirmMsg struct {
	req tools.ConfirmRequest
}
