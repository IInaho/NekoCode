package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type BashTool struct{}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return "在当前 shell 执行读类命令，如 ls, cat, pwd, grep, find 等"
}

func (t *BashTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "command", Type: "string", Required: true, Description: "要执行的命令"},
	}
}

func (t *BashTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("missing command parameter")
	}

	cmdStr = strings.TrimSpace(cmdStr)
	allowedPrefixes := []string{"ls", "cat", "pwd", "grep", "find", "head", "tail", "wc", "which", "file", "stat", "readlink", "realpath", "dirname", "basename", "tree", "less", "more"}

	isAllowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cmdStr, prefix) {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return "", fmt.Errorf("命令不在允许列表中: %s", cmdStr)
	}

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir, _ = os.Getwd()
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("命令执行失败: %v\n输出: %s", err, string(output))
	}

	return string(output), nil
}
