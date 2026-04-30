// FileSystemTool 文件读写列表。read/list → LevelSafe 自动放行，
// write → LevelWrite 需确认。
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type FileSystemTool struct{}

func (t *FileSystemTool) Name() string { return "filesystem" }

func (t *FileSystemTool) Description() string {
	return "文件系统操作：读取、写入、列出目录"
}

func (t *FileSystemTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "operation", Type: "string", Required: true, Description: "操作类型: read, write, list"},
		{Name: "path", Type: "string", Required: true, Description: "文件路径"},
		{Name: "content", Type: "string", Required: false, Description: "写入内容(仅write时需要)"},
	}
}

func (t *FileSystemTool) DangerLevel(args map[string]interface{}) DangerLevel {
	if op, _ := args["operation"].(string); op == "write" {
		return LevelWrite
	}
	return LevelSafe
}

func (t *FileSystemTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("missing operation parameter")
	}
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing path parameter")
	}

	switch operation {
	case "read":
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %v", err)
		}
		return string(content), nil

	case "write":
		content, ok := args["content"].(string)
		if !ok {
			return "", fmt.Errorf("missing content parameter for write operation")
		}
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("写入文件失败: %v", err)
		}
		return fmt.Sprintf("已写入文件: %s", path), nil

	case "list":
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("读取目录失败: %v", err)
		}
		var result string
		for _, e := range entries {
			if e.IsDir() {
				result += fmt.Sprintf("d %s/\n", e.Name())
			} else {
				info, _ := e.Info()
				result += fmt.Sprintf("- %s (%d bytes)\n", e.Name(), info.Size())
			}
		}
		return result, nil

	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}
