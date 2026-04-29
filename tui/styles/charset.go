package styles

import (
	"os"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

// Box-drawing character set. Initialized by init() based on terminal capabilities.
var (
	TopLeft     = "╭"
	TopRight    = "╮"
	BottomLeft  = "╰"
	BottomRight = "╯"
	Vertical    = "│"
	Horizontal  = "─"
	LeftTee     = "├"
	RightTee    = "┤"
	HeavyVert   = "┃"
	Bullet      = "•"
	Fisheye     = "◉"
)

func init() {
	runewidth.DefaultCondition.EastAsianWidth = true

	if !supportsUnicode() {
		TopLeft = "+"
		TopRight = "+"
		BottomLeft = "+"
		BottomRight = "+"
		Vertical = "|"
		Horizontal = "-"
		LeftTee = "+"
		RightTee = "+"
		HeavyVert = "|"
		Bullet = "*"
		Fisheye = "o"
	}
}

func supportsUnicode() bool {
	for _, env := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		v := strings.ToUpper(os.Getenv(env))
		if strings.Contains(v, "UTF-8") || strings.Contains(v, "UTF8") {
			return true
		}
	}
	switch os.Getenv("TERM") {
	case "dumb", "vt100", "vt52", "linux":
		return false
	}
	return true
}
