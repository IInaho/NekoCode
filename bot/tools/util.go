// util.go — 工具函数：字符串处理、路径安全、HTTP 客户端。
package tools

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// StripAnsi removes ANSI escape sequences from a string.
func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TruncateByRune(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// validatePath resolves path against the current working directory and rejects
// paths that escape via ".." traversal or symlinks.
func validatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %v", err)
	}
	// Resolve symlinks to prevent escape via symlink indirection.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// If EvalSymlinks fails (e.g., file doesn't exist yet), validate the
		// parent directory instead so we still catch traversal through existing
		// symlinks in the path chain.
		parent := filepath.Dir(abs)
		realParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			// Can't resolve parent either — fall back to the unresolved path
			// and rely on relative-path check as a best-effort guard.
			real = abs
		} else {
			real = filepath.Join(realParent, filepath.Base(abs))
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return abs, nil // can't validate, trust the path
	}
	rel, err := filepath.Rel(cwd, real)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path is outside the working directory: %s", path)
	}
	return abs, nil
}

var toolTransport = &http.Transport{
	MaxIdleConns:    10,
	IdleConnTimeout: 60 * time.Second,
}

func NewToolHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: toolTransport,
		Timeout:   timeout,
	}
}

// SplitPairs splits on commas that are not inside double-quoted segments.
func SplitPairs(s string) []string {
	var pairs []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '\\':
			if inQuote && i+1 < len(s) {
				i++ // skip escaped char
			}
		case ',':
			if !inQuote {
				pairs = append(pairs, s[start:i])
				start = i + 1
			}
		}
	}
	pairs = append(pairs, s[start:])
	return pairs
}
