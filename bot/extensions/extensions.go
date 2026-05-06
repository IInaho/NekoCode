// Package extensions 提供最小化的插件注册机制。
// Extension 可以注册自定义 Tool 和 Command，
// 由 bot.New() 在初始化时加载并注入到 tool registry 和 command parser。
package extensions

import (
	"primusbot/bot/tools"
)

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

type Registry struct {
	extensions []Extension
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(ext Extension) {
	r.extensions = append(r.extensions, ext)
}

func (r *Registry) AllTools() []tools.Tool {
	var all []tools.Tool
	for _, ext := range r.extensions {
		all = append(all, ext.Tools()...)
	}
	return all
}

func (r *Registry) AllCommands() []Command {
	var all []Command
	for _, ext := range r.extensions {
		all = append(all, ext.Commands()...)
	}
	return all
}
