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
		return t.readText(path, args)
	}
}

func (t *ReadTool) readText(path string, args map[string]interface{}) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
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
