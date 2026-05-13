// result.go — Structured subagent result type with XML serialization.
package subagent

import (
	"fmt"
	"strings"
)

// Status captures the terminal state of a subagent run.
type Status int

const (
	StatusCompleted Status = iota // ran to completion
	StatusFailed                  // errored before producing output
	StatusPartial                 // killed/interrupted but has partial results
)

// classification is the result of safety review on subagent transcript.
type classification int

const (
	classPass classification = iota // safe, no warnings
	classWarn                       // potentially dangerous, inject warning
	classUnavailable                // classifier unavailable, main agent should self-verify
)

// Result is the structured output of a subagent run.
type Result struct {
	Status         Status         // terminal state
	Content        string         // main output (the Result: field from structured output)
	KeyFiles       []string       // files examined
	Issues         []string       // problems or concerns flagged by the agent
	classification classification // safety review result
}

// parseStructuredOutput extracts structured fields from a subagent's final text.
//
// Expected format (from Claude Code's fork child directive):
//
//	Result: <key findings>
//	Key files: <comma-separated paths>
//	Files changed: <comma-separated paths>
//	Issues: <comma-separated issues>
//
// Fields are optional — missing fields are left empty rather than causing errors.
func parseStructuredOutput(raw string) (content string, keyFiles, filesChanged, issues []string) {
	content = extractField(raw, "Result:")
	keyFiles = extractListField(raw, "Key files:")
	filesChanged = extractListField(raw, "Files changed:")
	issues = extractListField(raw, "Issues:")
	return
}

// extractField extracts the value of a labeled field from text.
func extractField(text, label string) string {
	idx := findLabel(text, label)
	if idx < 0 {
		return ""
	}
	start := idx + len(label)
	rest := text[start:]

	rest = strings.TrimLeft(rest, " \t")

	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	return strings.TrimSpace(rest)
}

// extractListField extracts a comma-separated or newline-separated list field.
func extractListField(text, label string) []string {
	idx := findLabel(text, label)
	if idx < 0 {
		return nil
	}
	start := idx + len(label)

	var lines []string
	remaining := text[start:]
	for remaining != "" {
		trimmed := strings.TrimLeft(remaining, " \t")
		if trimmed == "" {
			break
		}
		if isLabelLine(trimmed) {
			break
		}
		nl := strings.IndexByte(trimmed, '\n')
		line := trimmed
		if nl >= 0 {
			line = trimmed[:nl]
			remaining = trimmed[nl+1:]
		} else {
			remaining = ""
		}
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		lower := strings.ToLower(line)
		if line != "" && lower != "n/a" && lower != "none" && lower != "nil" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 1 && strings.Contains(lines[0], ",") {
		parts := strings.Split(lines[0], ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}

	var result []string
	for _, l := range lines {
		if strings.Contains(l, ",") {
			for _, p := range strings.Split(l, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					result = append(result, p)
				}
			}
		} else {
			result = append(result, l)
		}
	}
	return result
}

func findLabel(text, label string) int {
	lower := strings.ToLower(text)
	ll := strings.ToLower(label)
	idx := strings.Index(lower, ll)
	if idx < 0 {
		return -1
	}
	before := text[:idx]
	if before != "" && before[len(before)-1] != '\n' {
		for i := len(before) - 1; i >= 0; i-- {
			if before[i] == '\n' {
				return idx
			}
			if before[i] != ' ' && before[i] != '\t' {
				return -1
			}
		}
	}
	return idx
}

func isLabelLine(line string) bool {
	labels := []string{"scope:", "result:", "key files:", "files changed:", "issues:"}
	lower := strings.ToLower(strings.TrimSpace(line))
	for _, l := range labels {
		if strings.HasPrefix(lower, l) {
			return true
		}
	}
	return false
}

// ToXML serializes a Result as a compact subagent output block.
func ToXML(r *Result) string {
	var b strings.Builder
	b.WriteString("<subagent-result>\n")

	if r.Content != "" {
		content := r.Content
		if len(content) > 200 {
			content = content[:200]
		}
		fmt.Fprintf(&b, "  <result>%s</result>\n", xmlEscape(content))
	}
	if len(r.KeyFiles) > 0 {
		fmt.Fprintf(&b, "  <key-files>%s</key-files>\n", xmlEscape(strings.Join(r.KeyFiles, ", ")))
	}
	if len(r.Issues) > 0 {
		fmt.Fprintf(&b, "  <issues>%s</issues>\n", xmlEscape(strings.Join(r.Issues, "; ")))
	}

	b.WriteString("</subagent-result>")

	if r.classification == classWarn {
		return "SECURITY WARNING: This sub-agent performed actions that may violate security policy.\n\n" + b.String()
	}
	return b.String()
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
