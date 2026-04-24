package agent

import (
	"primusbot/agent/tools"
)

type ToolRegistry struct {
	registry *tools.ToolRegistry
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		registry: tools.NewToolRegistry(),
	}
}

func (r *ToolRegistry) Register(tool tools.Tool) {
	r.registry.Register(tool)
}

func (r *ToolRegistry) Get(name string) tools.Tool {
	return r.registry.Get(name)
}

func (r *ToolRegistry) List() []tools.Tool {
	return r.registry.List()
}

func (r *ToolRegistry) AvailableToolsString() string {
	return r.registry.AvailableToolsString()
}

func RegisterDefaultTools(r *ToolRegistry) {
	r.Register(&tools.FileSystemTool{})
	r.Register(&tools.BashTool{})
	r.Register(&tools.GlobTool{})
}
