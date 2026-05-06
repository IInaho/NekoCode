// GrepTool 基于 ripgrep 的内容搜索，返回匹配行及行号，支持正则和上下文。
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type GrepTool struct{}

func (t *GrepTool) Name() string { return "grep" }
	func (t *GrepTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }

func (t *GrepTool) Description() string {
	return "基于 ripgrep 的内容搜索。在文件中搜索正则模式，返回匹配行及行号。支持文件过滤和上下文行数。"
}

func (t *GrepTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "pattern", Type: "string", Required: true, Description: "搜索模式（正则表达式）"},
		{Name: "path", Type: "string", Required: false, Description: "搜索目录，默认为当前目录"},
		{Name: "glob", Type: "string", Required: false, Description: "文件过滤模式，如 *.go, *.py"},
		{Name: "context_lines", Type: "string", Required: false, Description: "上下文行数，如 -A3, -B2, -C5 或数字(默认-C)"},
	}
}

func (t *GrepTool) DangerLevel(args map[string]interface{}) DangerLevel {
	return LevelSafe
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return "", fmt.Errorf("missing pattern parameter")
	}

	cmdArgs := []string{"--line-number", "--no-heading", "--color=never", "--smart-case"}

	glob, ok := args["glob"].(string)
	if ok && glob != "" {
		cmdArgs = append(cmdArgs, "--glob", glob)
	}

	ctxLines, ok := args["context_lines"].(string)
	if ok && ctxLines != "" {
		ctxLines = strings.TrimSpace(ctxLines)
		if strings.HasPrefix(ctxLines, "-") {
			cmdArgs = append(cmdArgs, strings.Fields(ctxLines)...)
		} else if ctxLines == "0" || ctxLines == "all" {
			// nothing
		} else {
			cmdArgs = append(cmdArgs, "-C", ctxLines)
		}
	}

	searchPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		searchPath = p
	}

	cmdArgs = append(cmdArgs, "--", pattern, searchPath)

	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "未找到匹配的内容", nil
			}
		}
		return "", fmt.Errorf("grep 执行失败: %v\n输出: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
