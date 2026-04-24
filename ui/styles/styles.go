package styles

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	codeBlockRegex  = regexp.MustCompile("`([^`]+)`")
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	boldRegex       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRegex     = regexp.MustCompile(`\*([^*]+)\*`)
)

const (
	fgMuted   = "#808080"
	fgSubtle  = "#5a5a5a"
	primary   = "#4ec9b0"
	greenDark = "#4ec9b0"
	blue      = "#569cd6"
	red       = "#f44747"
	yellow    = "#dcdcaa"
)

type Styles struct {
	Base      lipgloss.Style
	Muted     lipgloss.Style
	Subtle    lipgloss.Style
	Primary   lipgloss.Style
	Green     lipgloss.Style
	Blue      lipgloss.Style
	Red       lipgloss.Style
	Yellow    lipgloss.Style
	Chat      ChatStyles
	Scrollbar struct {
		Thumb lipgloss.Style
		Track lipgloss.Style
	}
}

type ChatStyles struct {
	Message MessageStyles
}

type MessageStyles struct {
	UserBlurred      lipgloss.Style
	UserFocused      lipgloss.Style
	AssistantBlurred lipgloss.Style
	AssistantFocused lipgloss.Style
	SystemBlurred    lipgloss.Style
	SystemFocused    lipgloss.Style
	ErrorBlurred     lipgloss.Style
	ErrorFocused     lipgloss.Style
	Processing       lipgloss.Style
}

func DefaultStyles() Styles {
	s := Styles{
		Base:    lipgloss.NewStyle().Foreground(lipgloss.Color(fgMuted)),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color(fgMuted)),
		Subtle:  lipgloss.NewStyle().Foreground(lipgloss.Color(fgSubtle)),
		Primary: lipgloss.NewStyle().Foreground(lipgloss.Color(primary)),
		Green:   lipgloss.NewStyle().Foreground(lipgloss.Color(greenDark)),
		Blue:    lipgloss.NewStyle().Foreground(lipgloss.Color(blue)),
		Red:     lipgloss.NewStyle().Foreground(lipgloss.Color(red)),
		Yellow:  lipgloss.NewStyle().Foreground(lipgloss.Color(yellow)),
	}

	s.Chat.Message = MessageStyles{
		UserBlurred: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(yellow)),
		UserFocused: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(yellow)),
		AssistantBlurred: lipgloss.NewStyle().
			PaddingLeft(2),
		AssistantFocused: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(primary)),
		SystemBlurred: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(blue)),
		SystemFocused: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(blue)),
		ErrorBlurred: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(red)),
		ErrorFocused: lipgloss.NewStyle().
			PaddingLeft(1).
			BorderLeft(true).
			BorderForeground(lipgloss.Color(red)),
		Processing: lipgloss.NewStyle().
			Foreground(lipgloss.Color(primary)),
	}

	s.Scrollbar.Thumb = lipgloss.NewStyle().Foreground(lipgloss.Color(primary))
	s.Scrollbar.Track = lipgloss.NewStyle().Foreground(lipgloss.Color(fgSubtle))

	return s
}

var defaultStyles = DefaultStyles()

var (
	MutedStyle   = defaultStyles.Muted
	SubtleStyle  = defaultStyles.Subtle
	PrimaryStyle = defaultStyles.Primary
	GreenStyle   = defaultStyles.Green
	BlueStyle    = defaultStyles.Blue
	RedStyle     = defaultStyles.Red
	YellowStyle  = defaultStyles.Yellow
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

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "```") {
			continue
		}

		if strings.HasPrefix(line, "# ") {
			result = append(result, PrimaryStyle.Render(line[2:]))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			result = append(result, PrimaryStyle.Render(line[3:]))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			result = append(result, PrimaryStyle.Render(line[4:]))
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result = append(result, MutedStyle.Render("• ")+line[2:])
			continue
		}

		if matches := codeBlockRegex.FindStringSubmatch(line); len(matches) > 0 {
			result = append(result, SubtleStyle.Render(matches[1]))
			continue
		}

		line = inlineCodeRegex.ReplaceAllString(line, SubtleStyle.Render("$1"))
		line = boldRegex.ReplaceAllString(line, PrimaryStyle.Render("$1"))
		line = italicRegex.ReplaceAllString(line, MutedStyle.Render("$1"))

		if len(line) > width {
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
		if len(currentLine)+len(word)+1 > width {
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
