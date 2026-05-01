// BashTool 执行 Shell 命令。DangerLevel 按命令关键词自动分级：
// forbidden（sudo/eval/ssh）→ 拒绝，destructive（rm/kill/shutdown）→ 确认，
// write（mkdir/cp/git commit）→ 确认，其余 → 自动放行。
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
	return "执行 shell 命令"
}

func (t *BashTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "command", Type: "string", Required: true, Description: "要执行的命令"},
	}
}

func (t *BashTool) DangerLevel(args map[string]interface{}) DangerLevel {
	cmd, _ := args["command"].(string)
	cmd = strings.TrimSpace(cmd)

	if matchAny(cmd, []string{
		"sudo", "eval", "curl", "wget", "nc ", "ncat",
		"telnet", "ssh ", "scp ", "nohup", "disown",
		"> /dev/", "mkfs", "dd ", "chown", "chmod 777",
		"| bash", "| sh", "|bash", "|sh",
	}) {
		return LevelForbidden
	}

	if matchAny(cmd, []string{
		"rm ", "rmdir", "chmod ", "chown ", "kill", "pkill",
		"shutdown", "reboot", "mv ", "git push", "git reset --hard",
		"docker rm", "docker rmi",
	}) {
		return LevelDestructive
	}

	if matchAny(cmd, []string{
		"mkdir", "touch ", "cp ", "mv ", "tar ", "zip ",
		"gzip ", "git commit", "git add", "pip install", "npm install",
		"go install", "cargo install", "make ", "docker build",
	}) {
		return LevelWrite
	}

	return LevelSafe
}

func matchAny(cmd string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(cmd, p) || strings.HasPrefix(cmd, p) {
			return true
		}
	}
	return false
}

func (t *BashTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	cmdStr, ok := args["command"].(string)
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("missing command parameter")
	}

	cmdStr = strings.TrimSpace(cmdStr)

	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	cmd.Dir, _ = os.Getwd()
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("命令执行失败: %v\n输出: %s", err, string(output))
	}

	return string(output), nil
}
