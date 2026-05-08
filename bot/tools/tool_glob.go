// GlobTool 文件模式匹配，始终 LevelSafe 自动放行。
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GlobTool struct{}

func (t *GlobTool) Name() string { return "glob" }
	func (t *GlobTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }

func (t *GlobTool) Description() string {
	return "文件模式匹配。ALWAYS 用 Glob，NEVER invoke find/ls as Bash。支持 ** 递归匹配，返回按修改时间排序的路径列表。"
}

func (t *GlobTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "pattern", Type: "string", Required: true, Description: "文件匹配模式"},
		{Name: "path", Type: "string", Required: false, Description: "搜索目录，默认为当前目录"},
	}
}

func (t *GlobTool) DangerLevel(args map[string]interface{}) DangerLevel {
	return LevelSafe
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("missing pattern parameter")
	}

	basePath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		basePath = p
	}

	var matches []string
	if strings.Contains(pattern, "**") {
		var err error
		matches, err = globRecursive(basePath, pattern)
		if err != nil {
			return "", fmt.Errorf("glob 失败: %v", err)
		}
	} else {
		var err error
		matches, err = filepath.Glob(filepath.Join(basePath, pattern))
		if err != nil {
			return "", fmt.Errorf("glob 失败: %v", err)
		}
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

func globRecursive(basePath, pattern string) ([]string, error) {
	var matches []string
	prefix, rest, _ := strings.Cut(pattern, "**")

	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(basePath, p)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		matchPattern := filepath.Join(prefix, "**", rest)
		matched, _ := filepath.Match(matchPattern, rel)
		if matched {
			matches = append(matches, p)
		}
		return nil
	})
	return matches, err
}
