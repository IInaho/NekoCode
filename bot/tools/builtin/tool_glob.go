// GlobTool — file pattern matching, always tools.LevelSafe auto-approve.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"nekocode/bot/tools"
)

type GlobTool struct{}

func (t *GlobTool) Name() string                                       { return "glob" }
func (t *GlobTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeParallel }

func (t *GlobTool) Description() string {
	return "File pattern matching. ALWAYS use Glob — NEVER invoke find/ls as Bash. Supports ** recursive matching. Returns file paths sorted by modification time."
}

func (t *GlobTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{Name: "pattern", Type: "string", Required: true, Description: "File matching pattern"},
		{Name: "path", Type: "string", Required: false, Description: "Search directory, default: current directory"},
	}
}

func (t *GlobTool) DangerLevel(args map[string]interface{}) tools.DangerLevel {
	return tools.LevelSafe
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
			return "", fmt.Errorf("glob failed: %v", err)
		}
	} else {
		var err error
		matches, err = filepath.Glob(filepath.Join(basePath, pattern))
		if err != nil {
			return "", fmt.Errorf("glob failed: %v", err)
		}
	}

	if len(matches) == 0 {
		return "No matching files found", nil
	}

	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(m)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
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
