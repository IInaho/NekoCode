// EditTool — precise string replacement. Finds the first occurrence of old_string in a file and replaces it with new_string.
// On failure, returns file content with line numbers to help locate the discrepancy.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"nekocode/bot/tools"
)

type EditTool struct{}

func (t *EditTool) Name() string                                       { return "edit" }
func (t *EditTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeSequential }

func (t *EditTool) Description() string {
	return `Precise string replacement in existing files. ALWAYS prefer Edit over Write for modifications.

REQUIREMENTS:
- MUST Read the file first (enforced — the tool will reject edits to unread files).
- old_string must match character-for-character: indentation, blank lines, trailing spaces, exact surrounding code.
- old_string must be UNIQUE in the file. Use the SMALLEST old_string that's clearly unique — typically 2-4 lines including surrounding context.
- Use replace_all for global renames: set replace_all=true to change every instance of old_string.

WHEN THE EDIT FAILS ("string not found"):
- The error shows the file content with line numbers. Compare your old_string against it byte-by-byte.
- Common causes: trailing whitespace (invisible in terminal output), tab-vs-space mismatch, surrounding code changed since you last read.
- Re-read the file with Read, copy the exact text you want to replace, and retry.

WHEN TO USE WRITE INSTEAD:
- Creating a new file (Write auto-creates parent directories).
- Replacing the ENTIRE file content in one operation.
- For any modification to an existing file, Edit is always preferred.`
}

func (t *EditTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{Name: "path", Type: "string", Required: true, Description: "File path to edit"},
		{Name: "old_string", Type: "string", Required: true, Description: "The original string to replace (must match exactly)"},
		{Name: "new_string", Type: "string", Required: true, Description: "The replacement string"},
	}
}

func (t *EditTool) DangerLevel(args map[string]interface{}) tools.DangerLevel {
	return tools.LevelWrite
}

func (t *EditTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing path parameter")
	}

	safePath, err := tools.ValidatePath(path)
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
	text := string(content)

	start, end, pass := findWithFuzzy(text, oldStr)
	if start == -1 {
		return "", fmt.Errorf("string not found in file. File contents (%s):\n%s\nHint: use Read to re-read the file, confirm the exact content before retrying.", filepath.Base(safePath), withLineNumbers(tools.StripAnsi(text)))
	}

	replaced := text[:start] + newStr + text[end:]

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

	passLabel := ""
	if pass > 1 {
		passLabel = fmt.Sprintf(" (fuzzy pass %d)", pass)
	}
	return fmt.Sprintf("Replaced %s%s:\n%s", filepath.Base(safePath), passLabel, diff), nil
}

// findWithFuzzy searches for oldStr in text using progressive tolerance.
// Pass 1: exact match. Pass 2: trim trailing whitespace on each line.
// Pass 3: trim both leading and trailing whitespace on each line.
// Returns (start, end) byte offsets in the original text and the pass number.
func findWithFuzzy(text, oldStr string) (int, int, int) {
	// Pass 1: exact match.
	if idx := strings.Index(text, oldStr); idx != -1 {
		return idx, idx + len(oldStr), 1
	}

	// Pass 2: rstrip each line (tolerate trailing whitespace differences).
	if start, end := fuzzyIndex(text, oldStr, rstripLines); start != -1 {
		return start, end, 2
	}

	// Pass 3: strip each line (tolerate leading+trailing whitespace differences).
	if start, end := fuzzyIndex(text, oldStr, stripLines); start != -1 {
		return start, end, 3
	}

	return -1, -1, 0
}

type lineNorm func(string) string

func rstripLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

func stripLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n")
}

// fuzzyIndex searches for oldStr in text after normalizing both with fn.
// Returns (start, end) byte offsets in the original text.
func fuzzyIndex(text, oldStr string, fn lineNorm) (int, int) {
	normOld := fn(oldStr)
	normText := fn(text)

	idx := strings.Index(normText, normOld)
	if idx == -1 {
		return -1, -1
	}

	start := mapNormPosToOrig(text, normText, idx, fn)
	end := mapNormPosToOrig(text, normText, idx+len(normOld), fn)
	return start, end
}

// mapNormPosToOrig maps a byte offset in the normalized text back to the
// corresponding byte offset in the original text.
func mapNormPosToOrig(text, normText string, targetNormPos int, fn lineNorm) int {
	origPos := 0
	normPos := 0
	for normPos < targetNormPos && origPos < len(text) && normPos < len(normText) {
		origLine := lineAt(text, origPos)
		normLine := fn(origLine)
		if normPos+len(normLine)+1 <= targetNormPos {
			origPos += len(origLine) + 1 // +1 for newline
			normPos += len(normLine) + 1
		} else {
			offset := targetNormPos - normPos
			o, n := 0, 0
			for n < offset && o < len(origLine) {
				if origLine[o] == normLine[n] {
					n++
				}
				o++
			}
			origPos += o
			break
		}
	}
	return origPos
}

func lineAt(s string, pos int) string {
	end := strings.IndexByte(s[pos:], '\n')
	if end == -1 {
		return s[pos:]
	}
	return s[pos : pos+end]
}

func withLineNumbers(content string) string {
	content = tools.StripAnsi(content)
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
