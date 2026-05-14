// BashTool — execute shell commands. DangerLevel auto-classified by command keywords:
// forbidden (sudo/eval/ssh) -> reject, destructive (rm/kill/shutdown) -> confirm,
// write (mkdir/cp/git commit) -> confirm, rest -> auto-approve.
package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"nekocode/bot/tools"
)

type BashTool struct{}

func (t *BashTool) Name() string                                       { return "bash" }
func (t *BashTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeSequential }

func (t *BashTool) Description() string {
	return `Execute shell commands. 120s timeout. Working directory persists but shell state (env vars, cd) does NOT.

DEDICATED TOOLS FIRST — using Bash for these will be rejected or produce worse results:
- Read files → Read (never cat/head/tail)
- Edit files → Edit (never sed/awk)
- Write files → Write (never echo >/cat <<EOF)
- Search content → Grep (never grep/rg)
- Find files → Glob (never find/ls)

MULTI-COMMAND STRATEGY:
- Independent commands → parallel Bash calls in a single message
- Dependent commands → chain with && (not newlines — shell state is lost between calls)
- Independent but sequential → use ; or separate calls

RULES:
- Use absolute paths. Quote paths containing spaces.
- Verify parent directories exist before creating files.
- Git: NEVER update git config. NEVER skip hooks (--no-verify). NEVER force push to main/master. Always create NEW commits, never amend.
- Destructive commands (rm, kill, shutdown) require confirmation.

SLEEP AVOIDANCE:
- Do NOT sleep between commands that can run immediately — just run them.
- Do NOT retry failing commands in a sleep loop — diagnose the root cause.
- If waiting for a background task, wait for the completion notification — do NOT poll.
- If you must sleep, keep the duration short.

ANTI-PATTERNS THAT CAUSE FAILURES:
- "cd some_dir" then next command — shell state is lost between calls. Use absolute paths or &&.
- "git push --force" — forbidden on main/master.
- "npm install" without checking if package.json exists.
- Piped commands that hide errors — prefer && chains for critical steps.
- Long-running servers/processes — the tool has a 120s timeout.`
}

func (t *BashTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{Name: "command", Type: "string", Required: true, Description: "The command to execute"},
	}
}

func (t *BashTool) DangerLevel(args map[string]interface{}) tools.DangerLevel {
	cmd, _ := args["command"].(string)
	cmd = strings.TrimSpace(cmd)

	if matchAny(cmd, []string{
		"sudo", "eval", "nc ", "ncat",
		"telnet", "ssh ", "scp ", "nohup", "disown",
		"> /dev/", "mkfs", "dd ", "chown", "chmod 777",
		"| bash", "| sh", "|bash", "|sh",
	}) {
		return tools.LevelForbidden
	}

	if matchAny(cmd, []string{
		"curl", "wget", "rm ", "rmdir", "chmod ", "chown ", "kill", "pkill",
		"shutdown", "reboot", "mv ", "git push", "git reset --hard",
		"docker rm", "docker rmi",
	}) {
		return tools.LevelDestructive
	}

	if matchAny(cmd, []string{
		"mkdir", "touch ", "cp ", "tar ", "zip ",
		"gzip ", "git commit", "git add", "pip install", "npm install",
		"go install", "cargo install", "make ", "docker build",
	}) {
		return tools.LevelWrite
	}

	// Commands that only produce output — no file system changes.
	if isReadOnly(cmd) {
		return tools.LevelSafe
	}

	return tools.LevelWrite
}

var readOnlyPrefixes = []string{
	"go version", "go env", "go doc", "go vet", "go fmt",
	"git status", "git log", "git diff", "git branch", "git show",
	"git blame", "git tag", "git remote", "git config",
	"ls", "pwd", "whoami", "date", "env", "printenv",
	"which", "type ", "uname", "hostname", "id ", "wc ",
	"cat ", "head ", "tail ", "less ", "more ",
	"du ", "df ", "free ", "uptime", "ps ", "pgrep",
	"man ", "info ", "file ", "stat ",
}

func isReadOnly(cmd string) bool {
	lower := strings.ToLower(cmd)
	for _, p := range readOnlyPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
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

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir, _ = os.Getwd()

	// Kill the entire process group on context cancellation.
	// exec.CommandContext only kills the direct child (bash), not grandchildren.
	stop := context.AfterFunc(ctx, func() {
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	defer stop()

	output, err := cmd.CombinedOutput()
	cleaned := tools.StripAnsi(string(output))

	if err != nil {
		return "", fmt.Errorf("command failed: %v\nOutput: %s", err, cleaned)
	}

	return cleaned, nil
}
