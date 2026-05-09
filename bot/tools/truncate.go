package tools

import (
	"fmt"
	"strings"
)

const (
	maxOutputLines = 2000
	maxOutputBytes = 50 * 1024 // 50KB
)

// TruncateOutput applies size limits to a single tool result.
// Keeps the beginning of output (most relevant) and appends a truncation hint.
func TruncateOutput(output string) string {
	lines := strings.Split(output, "\n")
	lineTruncated := false
	if len(lines) > maxOutputLines {
		output = strings.Join(lines[:maxOutputLines], "\n")
		output += fmt.Sprintf("\n\n[Truncated: %d lines total, showing first %d. Use read/grep with offset for more.]",
			len(lines), maxOutputLines)
		lineTruncated = true
	}

	if len(output) > maxOutputBytes {
		cut := maxOutputBytes
		if lineTruncated {
			// The truncation message at the end is useful, try to keep it.
			cut = maxOutputBytes - 200
			if cut < 1024 {
				cut = 1024
			}
		}
		output = output[:cut]
		output += fmt.Sprintf("\n[Truncated at %d bytes. Use read with offset for full content.]", maxOutputBytes)
	}

	return output
}
