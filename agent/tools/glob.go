package tools

import (
	"context"
	"fmt"
	"path/filepath"
)

type GlobTool struct{}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return "根据模式查找文件，如 *.go, **/*.ts 等"
}

func (t *GlobTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "pattern", Type: "string", Required: true, Description: "文件匹配模式"},
		{Name: "path", Type: "string", Required: false, Description: "搜索目录，默认为当前目录"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("missing pattern parameter")
	}

	path := "."
	if p, ok := args["path"].(string); ok {
		path = p
	}

	matches, err := filepath.Glob(filepath.Join(path, pattern))
	if err != nil {
		return "", fmt.Errorf("glob 失败: %v", err)
	}

	if len(matches) == 0 {
		return "未找到匹配的文件", nil
	}

	var result string
	for _, m := range matches {
		result += m + "\n"
	}
	return result, nil
}
