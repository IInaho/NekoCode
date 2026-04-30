// EditTool 精确字符串替换。在文件中查找 old_string 首次出现并替换为 new_string，
// 失败时返回文件内容上下文帮助定位差异。
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EditTool struct{}

func (t *EditTool) Name() string { return "edit" }

func (t *EditTool) Description() string {
	return "精确编辑文件：查找并替换文件中首次出现的字符串。old_string 必须与文件内容精确匹配（含缩进和换行）。失败时返回文件内容帮助定位差异。"
}

func (t *EditTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "要编辑的文件路径"},
		{Name: "old_string", Type: "string", Required: true, Description: "要替换的原字符串（必须精确匹配）"},
		{Name: "new_string", Type: "string", Required: true, Description: "替换后的新字符串"},
	}
}

func (t *EditTool) DangerLevel(args map[string]interface{}) DangerLevel {
	return LevelWrite
}

func (t *EditTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing path parameter")
	}
	oldStr, ok := args["old_string"].(string)
	if !ok || oldStr == "" {
		return "", fmt.Errorf("missing old_string parameter")
	}
	newStr, _ := args["new_string"].(string)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	idx := strings.Index(string(content), oldStr)
	if idx == -1 {
		return "", fmt.Errorf("未找到匹配的字符串。文件内容 (%s):\n%s", filepath.Base(path), withLineNumbers(string(content)))
	}

	replaced := string(content)[:idx] + newStr + string(content)[idx+len(oldStr):]

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(replaced), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}

	var diffLines []string
	for _, l := range strings.Split(oldStr, "\n") {
		diffLines = append(diffLines, "- "+l)
	}
	for _, l := range strings.Split(newStr, "\n") {
		diffLines = append(diffLines, "+ "+l)
	}
	diff := strings.Join(diffLines, "\n")

	return fmt.Sprintf("已替换 %s:\n%s", filepath.Base(path), diff), nil
}

func withLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, line)
	}
	return b.String()
}
