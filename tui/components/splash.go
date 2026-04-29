package components

import (
	"fmt"
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

type Splash struct {
	width   int
	height  int
	version string
	blink   bool
}

func NewSplash(width, height int, version string) *Splash {
	return &Splash{width: width, height: height, version: version}
}

func (s *Splash) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Splash) Blink() {
	s.blink = !s.blink
}

func (s *Splash) View() string {
	w := maxInt(60, s.width)
	h := maxInt(20, s.height)

	center := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)
	cat := s.renderCat()
	title := s.renderTitle()
	sep := s.renderSeparator()
	subtitle := s.renderSubtitle()

	var lines []string

	// Cat: block-center to preserve internal structure.
	catLines := strings.Split(cat, "\n")
	maxCatW := 0
	for _, l := range catLines {
		if cw := lipgloss.Width(l); cw > maxCatW {
			maxCatW = cw
		}
	}
	catPad := maxInt(0, (w-maxCatW)/2)
	for _, l := range catLines {
		lines = append(lines, strings.Repeat(" ", catPad)+l)
	}

	lines = append(lines, "") // gap
	for _, line := range strings.Split(title, "\n") {
		lines = append(lines, center.Render(line))
	}
	lines = append(lines, center.Render(sep))
	lines = append(lines, center.Render(subtitle))

	contentBlock := strings.Join(lines, "\n")
	contentH := len(lines)

	// Input.Height()=5 + 2 separator lines in tui.go View().
	reserved := 7
	topPad := maxInt(0, (h-reserved-contentH)/2)

	var b strings.Builder
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}
	b.WriteString(contentBlock)
	return b.String()
}

func (s *Splash) renderCat() string {
	// Black cat with glowing teal eyes
	//
	//      /\___/\
	//     ( o   o )
	//      =  V  =
	//     /|     |\
	//    (_|     |_)
	//       || ||
	//

	body := styles.CatBodyStyle
	eyeStyle := styles.CatEyeStyle
	if s.blink {
		eyeStyle = styles.SubtleStyle // dim eyes on blink, keeps width constant
	}

	var b strings.Builder
	b.WriteString(body.Render("   /\\___/\\") + "\n")
	b.WriteString(body.Render("  ( ") + eyeStyle.Render("o") + body.Render("   ") + eyeStyle.Render("o") + body.Render(" )") + "\n")
	b.WriteString(body.Render("   =  V  =") + "\n")
	b.WriteString(body.Render("  /|     |\\") + "\n")
	b.WriteString(body.Render(" (_|     |_)") + "\n")
	b.WriteString(body.Render("    || ||"))

	return b.String()
}

func (s *Splash) renderTitle() string {
	titleLine := styles.PrimaryStyle.Bold(true).Render("P R I M U S")
	versionLine := styles.SubtleStyle.Render(fmt.Sprintf("v%s", s.version))
	return titleLine + "\n" + versionLine
}

func (s *Splash) renderSeparator() string {
	seg := strings.Repeat(styles.Horizontal, 12)
	return styles.MutedStyle.Render(seg) + styles.PrimaryStyle.Render(" ◆ ") + styles.MutedStyle.Render(seg)
}

func (s *Splash) renderSubtitle() string {
	return styles.MutedStyle.Render("Ready to chat  >^.^<")
}
