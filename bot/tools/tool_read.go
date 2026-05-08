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
	return "读取文件内容。支持文本、图片、PDF。ALWAYS 用 Read 读文件，NEVER invoke cat/head/tail as Bash。路径必须为绝对路径。默认最多 2000 行，超长文件用 offset/limit 按行范围读取，或用 Grep 搜索。"
}

func (t *ReadTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "path", Type: "string", Required: true, Description: "要读取的文件路径（绝对路径）"},
		{Name: "offset", Type: "integer", Required: false, Description: "起始行号（1-based），不提供则从头开始。用于大文件分段读取"},
		{Name: "limit", Type: "integer", Required: false, Description: "读取行数，默认 2000"},
	}
}

const maxReadLines = 2000

func (t *ReadTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("缺少 path 参数")
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
		return "", fmt.Errorf("读取文件失败: %v", err)
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
		return fmt.Sprintf("[文件 %s 共 %d 行，offset %d 超出范围]", filepath.Base(path), totalLines, offset), nil
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
		result += fmt.Sprintf("\n\n[文件共 %d 行，已显示 %d-%d 行。使用 offset: %d 继续读取，或用 grep 搜索指定内容]",
			totalLines, offset, end, end+1)
	}

	if result == "" {
		return "[文件为空]", nil
	}
	return result, nil
}

func (t *ReadTool) readImage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开图片失败: %v", err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("解析图片失败: %v", err)
	}
	return fmt.Sprintf("[图片] %s — %s, %dx%d", filepath.Base(path), format, cfg.Width, cfg.Height), nil
}

func (t *ReadTool) readPDF(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("读取 PDF 失败: %v", err)
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
	return fmt.Sprintf("[PDF] %s — %s，共 %d 字节。使用 head 或 bash pdftotext 提取内容。",
		filepath.Base(path), sizeStr, size), nil
}
