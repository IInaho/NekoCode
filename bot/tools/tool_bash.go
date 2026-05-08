// BashTool — execute shell commands. DangerLevel auto-classified by command keywords:
// forbidden (sudo/eval/ssh) -> reject, destructive (rm/kill/shutdown) -> confirm,
// write (mkdir/cp/git commit) -> confirm, rest -> auto-approve.
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type BashTool struct{}

func (t *BashTool) Name() string                                       { return "bash" }
func (t *BashTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }

func (t *BashTool) Description() string {
	return `Execute shell commands.

IMPORTANT: Avoid using this tool for find, grep, cat, head, tail, sed, awk, echo — use dedicated tools instead:
- File search: Glob (not find/ls)
- Content search: Grep (not grep/rg)
- Read files: Read (not cat/head/tail)
- Edit files: Edit (not sed/awk)
- Write files: Write (not echo >/cat <<EOF)

Rules:
- Verify parent directory exists before creating files/dirs. Use absolute paths, avoid cd
- Chain dependent commands with &&. NEVER use newlines to separate commands
- Git: NEVER update git config, NEVER skip hooks (--no-verify), NEVER force push to main/master
- CRITICAL: Always create NEW commits, never amend`
}

func (t *BashTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "command", Type: "string", Required: true, Description: "The command to execute"},
	}
}

func (t *BashTool) DangerLevel(args map[string]interface{}) DangerLevel {
	cmd, _ := args["command"].(string)
	cmd = strings.TrimSpace(cmd)

	if matchAny(cmd, []string{
		"sudo", "eval", "nc ", "ncat",
		"telnet", "ssh ", "scp ", "nohup", "disown",
		"> /dev/", "mkfs", "dd ", "chown", "chmod 777",
		"| bash", "| sh", "|bash", "|sh",
	}) {
		return LevelForbidden
	}

	if matchAny(cmd, []string{
		"curl", "wget", "rm ", "rmdir", "chmod ", "chown ", "kill", "pkill",
		"shutdown", "reboot", "mv ", "git push", "git reset --hard",
		"docker rm", "docker rmi",
	}) {
		return LevelDestructive
	}

	if matchAny(cmd, []string{
		"mkdir", "touch ", "cp ", "tar ", "zip ",
		"gzip ", "git commit", "git add", "pip install", "npm install",
		"go install", "cargo install", "make ", "docker build",
	}) {
		return LevelWrite
	}

	return LevelWrite
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

	cleaned := StripAnsi(string(output))

	if err != nil {
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, cleaned)
	}

	return cleaned, nil
}
