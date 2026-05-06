package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FileSystemTool struct {
	mu       sync.RWMutex
	readHash map[string]string // path → sha256[:16]
}

func (t *FileSystemTool) Name() string        { return "filesystem" }
func (t *FileSystemTool) Description() string { return "文件系统操作：读取、写入、列出目录" }

func (t *FileSystemTool) ExecutionMode(args map[string]interface{}) ExecutionMode {
	if op, ok := args["operation"].(string); ok && (op == "read" || op == "list") {
		return ModeParallel
	}
	return ModeSequential
}

func (t *FileSystemTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "operation", Type: "string", Required: true, Description: "操作类型: read, write, list"},
		{Name: "path", Type: "string", Required: true, Description: "文件路径"},
		{Name: "content", Type: "string", Required: false, Description: "写入内容(仅write时需要)"},
	}
}

func (t *FileSystemTool) DangerLevel(args map[string]interface{}) DangerLevel {
	if op, _ := args["operation"].(string); op == "write" {
		return LevelWrite
	}
	return LevelSafe
}

func (t *FileSystemTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("missing operation parameter")
	}
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing path parameter")
	}

	switch operation {
	case "read":
		return t.readFile(path)

	case "write":
		content, ok := args["content"].(string)
		if !ok {
			return "", fmt.Errorf("missing content parameter for write operation")
		}
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("写入文件失败: %v", err)
		}
		// Invalidate read cache on write.
		t.mu.Lock()
		delete(t.readHash, path)
		t.mu.Unlock()
		return fmt.Sprintf("已写入文件: %s", path), nil

	case "list":
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("读取目录失败: %v", err)
		}
		var result string
		for _, e := range entries {
			if e.IsDir() {
				result += fmt.Sprintf("▸ %s/\n", e.Name())
			} else {
				info, _ := e.Info()
				result += fmt.Sprintf("  %s  %s\n", e.Name(), humanSize(info.Size()))
			}
		}
		return result, nil

	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

func (t *FileSystemTool) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	t.mu.Lock()
	if t.readHash == nil {
		t.readHash = make(map[string]string)
	}
	t.mu.Unlock()

	// Deduplicate: if file unchanged since last read, return a short reference.
	hash := fmt.Sprintf("%x", sha256.Sum256(content))[:16]
	t.mu.RLock()
	prevHash, exists := t.readHash[path]
	t.mu.RUnlock()
	if exists && prevHash == hash {
		return fmt.Sprintf("[未变更] %s — 内容与上次读取相同，无需重复读取", filepath.Base(path)), nil
	}
	t.mu.Lock()
	t.readHash[path] = hash
	t.mu.Unlock()

	text := StripAnsi(string(content))
	totalLines := strings.Count(text, "\n") + 1

	// Smart truncation: keep at most ~2500 chars, which is enough for
	// imports + top-level declarations + a few functions.
	const maxChars = 2500
	if len([]rune(text)) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars])
		text += fmt.Sprintf("\n\n[文件共 %d 行，已截断。使用 grep 搜索 %s 中的具体内容]",
			totalLines, filepath.Base(path))
	}

	return text, nil
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
