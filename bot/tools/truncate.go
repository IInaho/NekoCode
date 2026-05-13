package tools

import (
	"fmt"
	"strings"
)

const (
	maxOutputLines = 2000
	maxOutputBytes = 50 * 1024 // 50KB
	headLines      = 40        // keep first N lines
	tailLines      = 20        // keep last N lines for error context
	errorTailLines = 60        // keep more tail lines when errors detected
	minUniqueLines = 8         // collapse repeated line when count >= this
)

// TruncateOutput applies size limits to a single tool result.
// Smart truncation: keeps head (context) + tail (errors/results),
// compresses repetitive middle, prioritizes error output.
func TruncateOutput(output string) string {
	lines := strings.Split(output, "\n")

	// Check if we even need to truncate.
	if len(lines) <= maxOutputLines && len(output) <= maxOutputBytes {
		return output
	}

	// Detect error content in the tail — keep more context when errors present.
	hasErrors := false
	errorStart := len(lines)
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-errorTailLines; i-- {
		if isErrorLine(lines[i]) {
			hasErrors = true
			if i < errorStart {
				errorStart = i
			}
		}
	}

	keepTail := tailLines
	if hasErrors {
		keepTail = errorTailLines
		// Extend to include error context (surrounding lines around first error from bottom).
		if errorStart > len(lines)-keepTail {
			keepTail = len(lines) - errorStart + 5 // +5 lines of context before error
			if keepTail > errorTailLines+20 {
				keepTail = errorTailLines + 20
			}
		}
	}
	// Safety: never keep more lines than we have.
	if keepTail > len(lines) {
		keepTail = len(lines)
	}

	// Build truncated output: head + summary + tail.
	var result strings.Builder
	totalTruncated := 0

	// Write head.
	headEnd := headLines
	if headEnd < 0 {
		headEnd = 0
	}
	tailStart := len(lines) - keepTail
	if tailStart < 0 {
		tailStart = 0
	}
	if headEnd > tailStart {
		headEnd = tailStart
	}
	for i := 0; i < headEnd; i++ {
		result.WriteString(lines[i])
		result.WriteByte('\n')
	}

	// Middle: collapse repetitive lines then summarize.
	if headEnd < tailStart {
		middle := lines[headEnd:tailStart]
		collapsed, skipped := collapseRepetitive(middle)
		for _, l := range collapsed {
			result.WriteString(l)
			result.WriteByte('\n')
		}
		totalTruncated = skipped
		if totalTruncated > 0 {
			result.WriteString(fmt.Sprintf("\n[... %d repetitive lines collapsed ...]\n\n", totalTruncated))
		} else {
			result.WriteString(fmt.Sprintf("\n[... %d lines truncated ...]\n\n", len(middle)))
		}
	}

	// Write tail.
	for i := tailStart; i < len(lines); i++ {
		result.WriteString(lines[i])
		result.WriteByte('\n')
	}

	result.WriteString(fmt.Sprintf("\n[Truncated: %d lines total. Use Read with offset for full content.]", len(lines)))

	// Byte-level safety net.
	final := result.String()
	if len(final) > maxOutputBytes {
		cut := maxOutputBytes - 200
		if cut < 1024 {
			cut = 1024
		}
		final = final[:cut]
		final += fmt.Sprintf("\n[Truncated at %d bytes. Use Read with offset for full content.]", maxOutputBytes)
	}

	return final
}

// isErrorLine checks if a line looks like an error message.
func isErrorLine(line string) bool {
	lower := strings.ToLower(line)
	indicators := []string{
		"error:", "fail", "panic:", "fatal:", "cannot ",
		"undefined:", "not found", "permission denied",
		"no such file", "command not found", "signal:",
		"stack trace", "goroutine", "traceback",
		// Build/test failure markers ("fail" above already catches most cases).
		"--- fail", "--- fail:",
		"tests failed", "build failed", "compilation failed",
		"exit status", "exit code",
	}
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

// collapseRepetitive compresses sequences of identical or near-identical lines.
// Returns the collapsed lines and the count of lines removed.
func collapseRepetitive(lines []string) ([]string, int) {
	if len(lines) == 0 {
		return nil, 0
	}

	type run struct {
		line  string
		count int
	}

	var runs []run
	current := run{line: lines[0], count: 1}

	for i := 1; i < len(lines); i++ {
		if lines[i] == current.line {
			current.count++
		} else {
			runs = append(runs, current)
			current = run{line: lines[i], count: 1}
		}
	}
	runs = append(runs, current)

	var result []string
	skipped := 0
	for _, r := range runs {
		if r.count >= minUniqueLines {
			result = append(result, r.line)
			result = append(result, fmt.Sprintf("  [repeated %d times]", r.count))
			skipped += r.count - 2 // -2 because we keep the line + the count note
		} else {
			for j := 0; j < r.count; j++ {
				result = append(result, r.line)
			}
		}
	}
	return result, skipped
}
