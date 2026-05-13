// classify.go — Safety classifier for subagent handoff results.
//
// Pattern taken from Claude Code's classifyHandoffIfNeeded():
//   - Reviews subagent transcript for dangerous actions
//   - Returns pass/warn/block without stopping the information flow
//   - Warn classification injects SECURITY WARNING prefix rather than blocking
//
// Design principle (from research §3 layer 3):
//   The classifier does NOT block information flow — it injects warnings
//   so the main agent stays vigilant while the system remains usable.
package subagent

import "strings"

// classifyHandoff reviews a subagent result for dangerous patterns.
// It does NOT require an LLM call — it uses pattern-matching heuristics
// on the raw output text and structured fields, catching the most common
// dangerous actions at zero cost.
func classifyHandoff(rawOutput string, filesChanged, keyFiles []string) classification {
	lower := strings.ToLower(rawOutput)

	dangerousCmds := []string{
		"rm -rf", "rm -r", "rmdir",
		"git push --force", "git push -f",
		"git reset --hard",
		"chmod 777", "chmod -r 777",
		"> /dev/", "dd if=",
		"mkfs.", "format ",
		":(){ :|:& };:", // fork bomb
	}
	for _, cmd := range dangerousCmds {
		if strings.Contains(lower, cmd) {
			return classWarn
		}
	}

	sensitiveFiles := []string{
		".env", ".env.local", ".env.production",
		"credentials", "secrets", "password",
		".git/config", ".gitconfig",
		"id_rsa", "id_ed25519", "private key",
		".claude/settings.json", ".claude/settings.local.json",
	}
	for _, f := range sensitiveFiles {
		for _, changed := range filesChanged {
			if strings.Contains(strings.ToLower(changed), f) {
				return classWarn
			}
		}
	}

	if len(filesChanged) > 0 && len(keyFiles) == 0 {
		return classWarn
	}

	return classPass
}
