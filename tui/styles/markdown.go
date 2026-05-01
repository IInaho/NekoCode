// Markdown 渲染器：标题、列表、粗体、斜体、行内代码、代码块着色、diff 高亮。
// RenderMarkdown / RenderMarkdownWithWidth。
package styles

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"

	runewidth "github.com/mattn/go-runewidth"
)

var (
	fencedCodeRegex = regexp.MustCompile("^`{3}")
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	boldRegex       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex     = regexp.MustCompile(`\*([^*]+)\*`)
	diffLineRegex   = regexp.MustCompile(`^[+\-@] `)
)

var (
	diffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#98c379"))
	diffDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e06c75"))
	diffHdrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7a8ba0"))
)

func RenderMarkdown(content string) string {
	return renderSimpleMarkdown(content, 80)
}

func RenderMarkdownWithWidth(content string, width int) string {
	return renderSimpleMarkdown(content, width)
}

func isDiffLine(line string) (isDiff bool, kind byte) {
	if !diffLineRegex.MatchString(line) {
		return false, 0
	}
	switch line[0] {
	case '+':
		return true, '+'
	case '-':
		return true, '-'
	case '@':
		return true, '@'
	}
	return false, 0
}

func detectDiffBlocks(lines []string) map[int]bool {
	diffSet := make(map[int]bool)

	groupStart := -1
	hasPlus := false

	for i, line := range lines {
		isDiff, _ := isDiffLine(line)

		if isDiff {
			if groupStart == -1 {
				groupStart = i
				hasPlus = false
			}
			if line[0] == '+' {
				hasPlus = true
			}
		} else {
			if groupStart != -1 && hasPlus {
				for j := groupStart; j < i; j++ {
					if d, _ := isDiffLine(lines[j]); d {
						diffSet[j] = true
					}
				}
			}
			groupStart = -1
			hasPlus = false
		}
	}

	if groupStart != -1 && hasPlus {
		for j := groupStart; j < len(lines); j++ {
			if d, _ := isDiffLine(lines[j]); d {
				diffSet[j] = true
			}
		}
	}

	return diffSet
}

func renderSimpleMarkdown(content string, width int) string {
	lines := strings.Split(content, "\n")
	diffSet := detectDiffBlocks(lines)
	var result []string
	inCodeBlock := false

	for i, line := range lines {
		if fencedCodeRegex.MatchString(line) {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			wrapped := hardWrapText(line, width)
			renderWrappedLines(&result, wrapped, func(s string) string {
				return SubtleStyle.Render(Vertical+" "+s)
			})
			continue
		}

		if diffSet[i] {
			switch line[0] {
			case '+':
				wrapped := hardWrapText("+ "+strings.TrimPrefix(line, "+ "), width)
				renderWrappedLines(&result, wrapped, func(s string) string {
					return diffAddStyle.Render(s)
				})
			case '-':
				wrapped := hardWrapText("- "+strings.TrimPrefix(line, "- "), width)
				renderWrappedLines(&result, wrapped, func(s string) string {
					return diffDelStyle.Render(s)
				})
			case '@':
				wrapped := hardWrapText(line, width)
				renderWrappedLines(&result, wrapped, func(s string) string {
					return diffHdrStyle.Render(s)
				})
			}
			continue
		}

		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "# ") {
			result = append(result, PrimaryStyle.Bold(true).Render(line[2:]))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			result = append(result, PrimaryStyle.Bold(true).Render(line[3:]))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			result = append(result, PrimaryStyle.Bold(true).Render(line[4:]))
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result = append(result, MutedStyle.Render(Bullet+" ")+line[2:])
			continue
		}

		line = inlineCodeRegex.ReplaceAllString(line, SubtleStyle.Render("$1"))
		line = boldRegex.ReplaceAllString(line, PrimaryStyle.Render("$1"))
		line = italicRegex.ReplaceAllString(line, MutedStyle.Render("$1"))

		if lipgloss.Width(line) > width {
			line = wrapText(line, width)
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func hardWrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result []string
	remaining := text
	for runewidth.StringWidth(remaining) > width {
		cut := runewidth.Truncate(remaining, width, "")
		result = append(result, cut)
		remaining = remaining[len(cut):]
	}
	if len(remaining) > 0 {
		result = append(result, remaining)
	}
	if len(result) == 0 {
		return text
	}
	return strings.Join(result, "\n")
}

func renderWrappedLines(result *[]string, text string, styleFn func(string) string) {
	for _, l := range strings.Split(text, "\n") {
		*result = append(*result, styleFn(l))
	}
}

func wrapText(text string, width int) string {
	var result []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		wordW := runewidth.StringWidth(word)
		lineW := runewidth.StringWidth(currentLine)
		if lineW+wordW+1 > width {
			if currentLine != "" {
				result = append(result, currentLine)
			}
			currentLine = word
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}

	if currentLine != "" {
		result = append(result, currentLine)
	}

	return strings.Join(result, "\n")
}
