package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type WriteTool struct{}

func (t *WriteTool) Name() string                                   { return "write" }
func (t *WriteTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }
func (t *WriteTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelWrite }
func (t *WriteTool) Description() string {
	return "创建或覆盖文件。MUST Read existing file first（否则失败）。Prefer Edit for modifications。NEVER create documentation files (*.md) or README。自动创建父目录。"
}

func (t *WriteTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "文件路径"},
		{Name: "content", Type: "string", Required: true, Description: "要写入的内容"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("缺少 path 参数")
	}

	safePath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}
	return fmt.Sprintf("已写入: %s (%d 字符)", safePath, len(content)), nil
}
