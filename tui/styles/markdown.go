package styles

import (
	"regexp"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

var (
	fencedCodeRegex = regexp.MustCompile("^`{3}")
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	boldRegex       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex     = regexp.MustCompile(`\*([^*]+)\*`)
)

func RenderMarkdown(content string) string {
	return renderSimpleMarkdown(content, 80)
}

func RenderMarkdownWithWidth(content string, width int) string {
	return renderSimpleMarkdown(content, width)
}

func renderSimpleMarkdown(content string, width int) string {
	lines := strings.Split(content, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		if fencedCodeRegex.MatchString(line) {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			result = append(result, SubtleStyle.Render(Vertical+" "+line))
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

		if runewidth.StringWidth(line) > width {
			line = wrapText(line, width)
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
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
