// report.go — context usage report for /context command.
package ctxmgr

import (
	"fmt"
	"strings"
)

// ContextReport holds a breakdown of context window usage.
type ContextReport struct {
	Budget         int
	SystemPrompt   int
	Anchor         int
	TodoText       int
	SkillList      int
	ToolDefTokens  int
	SkillTokens    []SkillToken
	Summary        int
	Messages       int
	Archived       int
	ClearedMarkers int
	CompactCount   int
	TrimCount      int
	ToolDefCount   int
	UserMessages   int
	SysInjections  int
	AssistantMsgs  int
	ToolResults    int
}

type SkillToken struct {
	Name   string
	Tokens int
}

// Report returns a context usage report.
func (m *Manager) Report() ContextReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r := ContextReport{}
	r.SystemPrompt = estimateString(m.systemPrompt)
	if m.anchor != nil {
		r.Anchor = estimateString(m.anchor.estimate())
	}
	r.TodoText = estimateString(m.todoText)
	r.SkillList = estimateString(m.skillList)
	r.SkillTokens = estimateSkills(m.skillList)
	r.Summary = estimateString(m.summary)

	for i := m.compactBoundary; i < len(m.messages); i++ {
		msg := m.messages[i]
		if msg.Content == clearedMarker {
			r.ClearedMarkers++
			continue
		}
		switch msg.Role {
		case "user":
			if strings.HasPrefix(msg.Content, "[System]") {
				r.SysInjections++
			} else {
				r.UserMessages++
			}
		case "assistant":
			r.AssistantMsgs++
		case "tool":
			r.ToolResults++
		}
	}
	r.Messages = estimateTokens(m.messages[m.compactBoundary:])
	r.Archived = m.compactBoundary
	r.CompactCount = m.compactCount
	r.TrimCount = m.trimCount
	r.Budget = m.tokenBudget
	return r
}

func estimateSkills(skillList string) []SkillToken {
	if skillList == "" {
		return nil
	}
	var skills []SkillToken
	for _, line := range strings.Split(skillList, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		content := strings.TrimPrefix(line, "- ")
		content = strings.TrimPrefix(content, "**")
		if idx := strings.Index(content, "**:"); idx > 0 {
			content = content[:idx] + content[idx+2:]
		}
		if idx := strings.Index(content, ":"); idx > 0 {
			name := content[:idx]
			tokens := estimateString(line)
			skills = append(skills, SkillToken{Name: strings.TrimSpace(name), Tokens: tokens})
		}
	}
	return skills
}

// FormatContextReport renders a context report as a styled string.
// Uses unicode box-drawing characters for visual hierarchy.
// nbsp is a non-breaking space used for indentation — stripLeadingSpaces
// in the TUI would otherwise remove regular spaces.
const nbsp = "\u00a0"

func indent(n int) string { return strings.Repeat(nbsp, n) }

func FormatContextReport(r ContextReport) string {
	used := r.SystemPrompt + r.Anchor + r.ToolDefTokens + r.TodoText + r.SkillList + r.Summary + r.Messages
	free := r.Budget - used
	if free < 0 {
		free = 0
	}
	pct := func(n int) string {
		if r.Budget == 0 {
			return "—"
		}
		return fmt.Sprintf("%.1f%%", float64(n)/float64(r.Budget)*100)
	}

	var b strings.Builder

	// Header with horizontal bar
	b.WriteString("Context Report\n")
	bar := buildBar(r.Budget, []barSegment{
		{size: r.SystemPrompt, label: "", kind: "sys"},
		{size: r.ToolDefTokens, label: "", kind: "tools"},
		{size: r.SkillList, label: "", kind: "skills"},
		{size: r.Summary, label: "", kind: "summary"},
		{size: r.Messages, label: "", kind: "msgs"},
		{size: free, label: "", kind: "free"},
	}, 40)
	b.WriteString(bar + "\n")
	fmt.Fprintf(&b, "%s / %s (%s)\n\n", formatTokens(used), formatTokens(r.Budget), pct(used))

	// System section
	b.WriteString(indent(0) + "▸ System\n")
	fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Prompt", formatTokens(r.SystemPrompt), pct(r.SystemPrompt))
	fmt.Fprintf(&b, indent(2) + "%-20s %s (%s) · %d tools\n", "Tool definitions", formatTokens(r.ToolDefTokens), pct(r.ToolDefTokens), r.ToolDefCount)
	if r.Anchor > 0 {
		fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Anchor", formatTokens(r.Anchor), pct(r.Anchor))
	}
	if r.TodoText > 0 {
		fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Todo", formatTokens(r.TodoText), pct(r.TodoText))
	}

	// Skills
	if len(r.SkillTokens) > 0 {
		fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Skills", formatTokens(r.SkillList), pct(r.SkillList))
		for i, s := range r.SkillTokens {
			prefix := "├"
			if i == len(r.SkillTokens)-1 {
				prefix = "└"
			}
			fmt.Fprintf(&b, indent(4)+"%s %-30s %s\n", prefix, s.Name, formatTokens(s.Tokens))
		}
	}
	if r.Summary > 0 {
		fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Summary", formatTokens(r.Summary), pct(r.Summary))
	}

	// Messages
	total := r.UserMessages + r.AssistantMsgs + r.ToolResults + r.SysInjections
	if total > 0 || r.Archived > 0 || r.ClearedMarkers > 0 {
		b.WriteString("\n" + indent(0) + "▸ Messages\n")
		fmt.Fprintf(&b, indent(2) + "%-20s %s (%s)\n", "Total", formatTokens(r.Messages), pct(r.Messages))
		fmt.Fprintf(&b, indent(2) + "%-20s %d\n", "User messages", r.UserMessages)
		fmt.Fprintf(&b, indent(2) + "%-20s %d\n", "Assistant", r.AssistantMsgs)
		fmt.Fprintf(&b, indent(2) + "%-20s %d\n", "Tool results", r.ToolResults)
		if r.SysInjections > 0 {
			fmt.Fprintf(&b, indent(2) + "%-20s %d\n", "[System] hints", r.SysInjections)
		}
		if r.Archived > 0 {
			fmt.Fprintf(&b, indent(2) + "%-20s %d messages\n", "Archived", r.Archived)
			if r.TrimCount > 0 {
				fmt.Fprintf(&b, indent(2) + "%-20s %d messages\n", "Trimmed", r.TrimCount)
			}
		}
		if r.ClearedMarkers > 0 {
			fmt.Fprintf(&b, indent(2) + "%-20s %d (total %d)\n", "Compacted", r.ClearedMarkers, r.CompactCount)
		}
	}

	return b.String()
}

type barSegment struct {
	size  int
	label string
	kind  string // sys, tools, skills, summary, msgs, free
}

func buildBar(total int, segments []barSegment, width int) string {
	if total <= 0 {
		return ""
	}
	// Allocate width proportionally, minimum 1 char per non-empty segment.
	allocated := make([]int, len(segments))
	remaining := width
	for i, s := range segments {
		if s.size > 0 {
			w := s.size * width / total
			if w < 1 {
				w = 1
			}
			allocated[i] = w
			remaining -= w
		}
	}
	// Distribute any remaining width to the last non-empty segment.
	for i := len(segments) - 1; i >= 0 && remaining > 0; i-- {
		if segments[i].size > 0 {
			allocated[i] += remaining
			break
		}
	}

	chars := map[string]string{
		"sys":     "▨",
		"tools":   "▩",
		"skills":  "◆",
		"summary": "◇",
		"msgs":    "▣",
		"free":    "·",
	}

	var b strings.Builder
	for i, s := range segments {
		if allocated[i] <= 0 {
			continue
		}
		ch := chars[s.kind]
		if ch == "" {
			ch = " "
		}
		b.WriteString(strings.Repeat(ch, allocated[i]))
	}
	return b.String()
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
