package tools

import (
	"context"
	"fmt"
	"os"
)

type ListTool struct{}

func (t *ListTool) Name() string                                   { return "list" }
func (t *ListTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *ListTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelSafe }
func (t *ListTool) Description() string {
	return "列出目录内容。ALWAYS 用 List，NEVER invoke ls as Bash。返回按名称排序的文件和子目录列表。"
}

func (t *ListTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "要列出内容的目录路径"},
	}
}

func (t *ListTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("缺少 path 参数")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %v", err)
	}

	var result string
	for _, e := range entries {
		if e.IsDir() {
			result += fmt.Sprintf("▸ %s/\n", e.Name())
		} else {
			info, err := e.Info()
				if err != nil {
					result += fmt.Sprintf("  %s\n", e.Name())
				} else {
			result += fmt.Sprintf("  %s  %s\n", e.Name(), humanSize(info.Size()))
				}
		}
	}
	if result == "" {
		result = "(空目录)"
	}
	return result, nil
}
