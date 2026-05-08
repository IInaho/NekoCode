// Package extensions provides a minimal plugin registration interface.
package extensions

import "primusbot/bot/tools"

type Extension interface {
	Name() string
	Tools() []tools.Tool
	Commands() []Command
}

type Command struct {
	Name        string
	Description string
	Handler     func(args []string) (string, error)
}
