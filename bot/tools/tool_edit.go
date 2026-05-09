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
	text := string(content)

	idx, pass := findWithFuzzy(text, oldStr)
	if idx == -1 {
		return "", fmt.Errorf("string not found in file. File contents (%s):\n%s\nHint: use Read to re-read the file, confirm the exact content before retrying.", filepath.Base(safePath), withLineNumbers(StripAnsi(text)))
	}

	var replaced string
	switch pass {
	case 1: // exact match — use original positions.
		replaced = text[:idx] + newStr + text[idx+len(oldStr):]
	default: // fuzzy match — replace the matched segment in-place.
		replaced = text[:idx] + newStr + text[idx+len(oldStr):]
	}

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
// Returns the byte offset in the original text and the pass number.
func findWithFuzzy(text, oldStr string) (int, int) {
	// Pass 1: exact match.
	if idx := strings.Index(text, oldStr); idx != -1 {
		return idx, 1
	}

	// Pass 2: rstrip each line (tolerate trailing whitespace differences).
	if idx := fuzzyIndex(text, oldStr, rstripLines); idx != -1 {
		return idx, 2
	}

	// Pass 3: strip each line (tolerate leading+trailing whitespace differences).
	if idx := fuzzyIndex(text, oldStr, stripLines); idx != -1 {
		return idx, 3
	}

	return -1, 0
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
// Returns the byte offset in the original text.
func fuzzyIndex(text, oldStr string, fn lineNorm) int {
	normOld := fn(oldStr)
	normText := fn(text)

	idx := strings.Index(normText, normOld)
	if idx == -1 {
		return -1
	}

	// Map the normalized position back to the original text.
	// Walk both original and normalized, tracking byte offsets.
	origPos := 0
	normPos := 0
	for normPos < idx && origPos < len(text) && normPos < len(normText) {
		origLine := lineAt(text, origPos)
		normLine := fn(origLine)
		if normPos+len(normLine)+1 <= idx {
			origPos += len(origLine) + 1 // +1 for newline
			normPos += len(normLine) + 1
		} else {
			// Inside the target line.
			offset := idx - normPos
			// Scan character-by-character within this line.
			o, n := 0, 0
			for n < offset && o < len(origLine) {
				if fn(string(origLine[o])) == string(normLine[n]) {
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
