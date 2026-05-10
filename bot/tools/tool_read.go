package tools

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

type ReadTool struct{}

func (t *ReadTool) Name() string                                   { return "read" }
func (t *ReadTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *ReadTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelSafe }
func (t *ReadTool) Description() string {
	return "Read file contents. Supports text, images, PDF. ALWAYS use Read — NEVER invoke cat/head/tail as Bash. Path must be absolute. Default max 2000 lines; use offset/limit for large files, or Grep for search."
}

func (t *ReadTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "File path (absolute)"},
		{Name: "offset", Type: "integer", Required: false, Description: "Starting line number (1-based). Use for chunked reads on large files"},
		{Name: "limit", Type: "integer", Required: false, Description: "Number of lines to read, default 2000"},
	}
}

const maxReadLines = 2000

func (t *ReadTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("missing path parameter")
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
		return t.readImage(path)
	case ".pdf":
		return t.readPDF(path)
	default:
		return t.readTextCached(path, args)
	}
}

// readTextCached checks the FileStateCache before reading. If the file hasn't
// changed since last read (same mtime+size) and the requested range is covered,
// returns a stub instead of re-sending full content.
func (t *ReadTool) readTextCached(path string, args map[string]interface{}) (string, error) {
	offset := 1
	if v, ok := args["offset"].(float64); ok && v > 0 {
		offset = int(v)
	}
	limit := maxReadLines
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	if GlobalFileCache != nil {
		if _, hit := GlobalFileCache.Get(path, offset, limit); hit {
			return fmt.Sprintf("[File unchanged: %s — content matches previous read at offset=%d limit=%d. Use different offset/limit to re-read.]",
				filepath.Base(path), offset, limit), nil
		}
	}

	result, err := t.readText(path, args)
	if err != nil {
		return result, err
	}

	// Cache non-error results.
	// isPartial is true when the read didn't cover the entire file from
	// the beginning, or the file was truncated. Detected via the trailer
	// readText appends ("\n\n[File has N lines, showing...]"), which is
	// distinctive enough not to be confused with file content.
	if GlobalFileCache != nil && !strings.HasPrefix(result, "[Binary") && !strings.HasPrefix(result, "[file is empty") {
		isPartial := offset > 1 || strings.Contains(result, "\n\n[File has ")
		GlobalFileCache.Put(path, result, offset, limit, isPartial)
	}

	return result, nil
}

func (t *ReadTool) readText(path string, args map[string]interface{}) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			msg := fmt.Sprintf("File not found: %s", filepath.Base(path))
			if suggestions := suggestSimilar(path); len(suggestions) > 0 {
				msg += "\nDid you mean one of these?"
				for _, s := range suggestions {
					msg += "\n  - " + s
				}
			}
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	if isBinary(content) {
		return fmt.Sprintf("[Binary file: %s — cannot display. Use bash file or hexdump to inspect.]",
			filepath.Base(path)), nil
	}

	text := StripAnsi(string(content))
	lines := strings.Split(text, "\n")
	totalLines := len(lines)

	offset := 1
	if v, ok := args["offset"].(float64); ok && v > 0 {
		offset = int(v)
	}
	limit := maxReadLines
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	if offset > totalLines {
		return fmt.Sprintf("[File %s has %d lines, offset %d out of range]", filepath.Base(path), totalLines, offset), nil
	}

	start := offset - 1 // 1-based to 0-based
	end := start + limit
	if end > totalLines {
		end = totalLines
	}

	var out strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&out, "%d\t%s\n", i+1, lines[i])
	}

	result := strings.TrimRight(out.String(), "\n")

	if end < totalLines {
		result += fmt.Sprintf("\n\n[File has %d lines, showing %d-%d. Use offset: %d to continue, or grep to search]",
			totalLines, offset, end, end+1)
	}

	if result == "" {
		return "[file is empty]", nil
	}
	return result, nil
}

// isBinary detects binary files by checking for null bytes and non-printable
// character ratio in the first 4096 bytes.
func isBinary(data []byte) bool {
	checkLen := 4096
	if len(data) < checkLen {
		checkLen = len(data)
	}
	if checkLen == 0 {
		return false
	}
	for _, b := range data[:checkLen] {
		if b == 0 {
			return true
		}
	}
	nonPrintable := 0
	for _, b := range data[:checkLen] {
		if b != '\n' && b != '\r' && b != '\t' && (b < 32 || b > 126) {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(checkLen) > 0.3
}

func (t *ReadTool) readImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %v", err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}
	return fmt.Sprintf("[Image] %s — %s, %dx%d", filepath.Base(path), format, cfg.Width, cfg.Height), nil
}

func (t *ReadTool) readPDF(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF: %v", err)
	}
	size := st.Size()
	sizeStr := ""
	switch {
	case size >= 1<<20:
		sizeStr = fmt.Sprintf("%.1fMB", float64(size)/(1<<20))
	case size >= 1<<10:
		sizeStr = fmt.Sprintf("%.1fKB", float64(size)/(1<<10))
	default:
		sizeStr = fmt.Sprintf("%dB", size)
	}
	return fmt.Sprintf("[PDF] %s — %s, %d bytes. Use head or bash pdftotext to extract content.",
		filepath.Base(path), sizeStr, size), nil
}

// suggestSimilar returns up to 3 files in the same directory with names
// similar to the requested path.
func suggestSimilar(path string) []string {
	dir := filepath.Dir(path)
	base := strings.ToLower(filepath.Base(path))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var scored []struct {
		path  string
		score int
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		score := 0
		if name == base {
			continue // exact match shouldn't happen for not-found
		}
		if strings.Contains(name, base) || strings.Contains(base, name) {
			score = 10
		} else if strings.HasPrefix(name, base[:min(3, len(base))]) {
			score = 5
		}
		d := levenshteinDist(name, base)
		if d <= 3 {
			score += 8 - d
		}
		if score > 0 {
			scored = append(scored, struct {
				path  string
				score int
			}{filepath.Join(dir, e.Name()), score})
		}
	}
	// Sort by score descending, stable.
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if len(scored) > 3 {
		scored = scored[:3]
	}
	out := make([]string, len(scored))
	for i, s := range scored {
		out[i] = s.path
	}
	return out
}

func levenshteinDist(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, min(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[len(a)][len(b)]
}
