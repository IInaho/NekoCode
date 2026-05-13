package builtin

import "nekocode/bot/tools"

func RegisterAll(r *tools.Registry) {
	r.Register(&BashTool{})
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&ListTool{})
	r.Register(&GlobTool{})
	r.Register(&EditTool{})
	r.Register(&GrepTool{})
	r.Register(NewWebSearchTool())
	r.Register(NewWebFetchTool())
	r.Register(NewTodoWriteTool())
	r.Register(NewTaskTool())
}
