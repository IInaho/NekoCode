// EditTool — precise string replacement. Finds the first occurrence of old_string in a file and replaces it with new_string.
// On failure, returns file content with line numbers to help locate the discrepancy.
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EditTool struct{}

func (t *EditTool) Name() string                                       { return "edit" }
func (t *EditTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }

func (t *EditTool) Description() string {
	return "Precise string replacement. ALWAYS prefer Edit over Write for modifications. MUST Read file first. old_string must match character-for-character (including indentation/newlines) and be unique. Use replace_all=true to replace all occurrences."
}

func (t *EditTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "File path to edit"},
		{Name: "old_string", Type: "string", Required: true, Description: "The original string to replace (must match exactly)"},
		{Name: "new_string", Type: "string", Required: true, Description: "The replacement string"},
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

	safePath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	oldStr, ok := args["old_string"].(string)
	if !ok || oldStr == "" {
		return "", fmt.Errorf("missing old_string parameter")
	}
	newStr, _ := args["new_string"].(string)

	content, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	idx := strings.Index(string(content), oldStr)
	if idx == -1 {
		return "", fmt.Errorf("string not found in file. File contents (%s):\n%s\nHint: use Read to re-read the file, confirm the exact content before retrying.", filepath.Base(safePath), withLineNumbers(StripAnsi(string(content))))
	}

	replaced := string(content)[:idx] + newStr + string(content)[idx+len(oldStr):]

	dir := filepath.Dir(safePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(safePath, []byte(replaced), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	var diffLines []string
	for _, l := range strings.Split(oldStr, "\n") {
		diffLines = append(diffLines, "- "+l)
	}
	for _, l := range strings.Split(newStr, "\n") {
		diffLines = append(diffLines, "+ "+l)
	}
	diff := strings.Join(diffLines, "\n")

	return fmt.Sprintf("Replaced %s:\n%s", filepath.Base(safePath), diff), nil
}

func withLineNumbers(content string) string {
	content = StripAnsi(content)
	lines := strings.Split(content, "\n")
	var b strings.Builder
	maxShow := 40
	for i, line := range lines {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, line)
		if i >= maxShow {
			fmt.Fprintf(&b, "... [%d lines total, showing first %d]", len(lines), maxShow+1)
			break
		}
	}
	return b.String()
}
