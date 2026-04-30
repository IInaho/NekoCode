// 注册所有内建工具（bash、filesystem、glob）到 Registry。
package tools

func RegisterDefaults(r *Registry) {
	r.Register(&BashTool{})
	r.Register(&FileSystemTool{})
	r.Register(&GlobTool{})
}
