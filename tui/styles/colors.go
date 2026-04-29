package styles

import "charm.land/lipgloss/v2"

const (
	fgText   = "#a0a0a0"
	fgMuted  = "#808080"
	fgSubtle = "#666666"
	fgBorder = "#333333"
	primary  = "#4ec9b0"
	blue     = "#7a8ba0"
	red      = "#e06c75"
	yellow   = "#c9a96e"
	catBody  = "#505050"
	catEye   = "#7ec8e3"
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
	Border    lipgloss.Style
	CatBody   lipgloss.Style
	CatEye    lipgloss.Style
	Scrollbar struct {
		Thumb lipgloss.Style
		Track lipgloss.Style
	}
}

func DefaultStyles() Styles {
	s := Styles{
		Base:    lipgloss.NewStyle().Foreground(lipgloss.Color(fgText)),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color(fgMuted)),
		Subtle:  lipgloss.NewStyle().Foreground(lipgloss.Color(fgSubtle)),
		Primary: lipgloss.NewStyle().Foreground(lipgloss.Color(primary)),
		Green:   lipgloss.NewStyle().Foreground(lipgloss.Color(primary)),
		Blue:    lipgloss.NewStyle().Foreground(lipgloss.Color(blue)),
		Red:     lipgloss.NewStyle().Foreground(lipgloss.Color(red)),
		Yellow:  lipgloss.NewStyle().Foreground(lipgloss.Color(yellow)),
		Border:  lipgloss.NewStyle().Foreground(lipgloss.Color(fgBorder)),
		CatBody: lipgloss.NewStyle().Foreground(lipgloss.Color(catBody)),
		CatEye:  lipgloss.NewStyle().Foreground(lipgloss.Color(catEye)),
	}

	s.Scrollbar.Thumb = lipgloss.NewStyle().Foreground(lipgloss.Color(fgMuted))
	s.Scrollbar.Track = lipgloss.NewStyle().Foreground(lipgloss.Color(fgBorder))

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
	BorderStyle  = defaultStyles.Border
	CatBodyStyle = defaultStyles.CatBody
	CatEyeStyle  = defaultStyles.CatEye
)
