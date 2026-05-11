// util.go — 工具函数：字符串处理、路径安全、HTTP 客户端。
package tools

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
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

// ValidatePath resolves path against the current working directory.
// It resolves symlinks and returns the absolute path, but no longer rejects
// paths outside cwd — the confirmation system handles user consent.
func ValidatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %v", err)
	}
	// Resolve symlinks to prevent escape via symlink indirection.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		parent := filepath.Dir(abs)
		realParent, pErr := filepath.EvalSymlinks(parent)
		if pErr != nil {
			real = abs
		} else {
			real = filepath.Join(realParent, filepath.Base(abs))
		}
	}
	return real, nil
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
