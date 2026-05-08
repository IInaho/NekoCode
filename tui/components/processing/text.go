// text.go — 文本工具函数：固定高度渲染、折行、噪声过滤。
package processing

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func RenderFixed(text string, maxLines int, skipEmpty bool, lineSty lipgloss.Style) string {
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}
	var out strings.Builder
	for i := start; i < len(lines); i++ {
		if i > start {
			out.WriteString("\n")
		}
		if skipEmpty && lines[i] == "" {
			continue
		}
		if IsNoiseLine(lines[i]) {
			continue
		}
		out.WriteString("  " + lineSty.Render(lines[i]))
	}
	rendered := strings.Count(out.String(), "\n") + 1
	target := rendered
	if target < 2 {
		target = 2
	}
	for i := rendered; i < target; i++ {
		out.WriteString("\n")
	}
	return out.String()
}

func WrapPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	paragraphs := strings.Split(text, "\n")
	var result []string
	for _, para := range paragraphs {
		runes := []rune(para)
		if len(runes) <= width {
			result = append(result, para)
			continue
		}
		for i := 0; i < len(runes); i += width {
			end := i + width
			if end > len(runes) {
				end = len(runes)
			}
			result = append(result, string(runes[i:end]))
		}
	}
	return strings.Join(result, "\n")
}

func IsNoiseLine(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '.' {
			return false
		}
	}
	return true
}
