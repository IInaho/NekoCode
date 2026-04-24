package components

import (
	"strings"

	"primusbot/ui/styles"
)

type Footer struct {
	Width  int
	Follow bool
}

func NewFooter(width int) *Footer {
	return &Footer{
		Width:  width,
		Follow: true,
	}
}

func (f *Footer) SetWidth(width int) {
	f.Width = width
}

func (f *Footer) SetFollow(follow bool) {
	f.Follow = follow
}

func (f *Footer) Height() int {
	return 2
}

func (f *Footer) View() string {
	w := maxInt(20, f.Width)
	line := strings.Repeat("─", w-2)

	followText := "Auto"
	if !f.Follow {
		followText = "Manual"
	}

	var b strings.Builder
	b.WriteString(styles.PrimaryStyle.Render("├"+line+"┤") + "\n")
	b.WriteString(styles.MutedStyle.Render("│ ") + styles.SubtleStyle.Render("↑↓:Scroll  End:Follow  Status:") + styles.GreenStyle.Render(followText) + "  " + styles.SubtleStyle.Render("Enter:Send  Ctrl+C:Quit") + strings.Repeat(" ", maxInt(0, w-55)) + styles.MutedStyle.Render("│") + "\n")
	b.WriteString(styles.PrimaryStyle.Render("╰"+line+"╯") + "\n")

	return b.String()
}
