// Package types holds shared type definitions used across bot, agent, and TUI.
package types

import "primusbot/bot/tools"

type ConfirmRequest struct {
	ToolName string
	Args     map[string]interface{}
	Level    tools.DangerLevel
	Response chan bool
}

type ConfirmFunc func(req ConfirmRequest) bool

type PhaseFunc func(phase string)
