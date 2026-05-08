// scrollbar.go — 垂直滚动条指示器。
package components
import (
	"strings"

	"primusbot/tui/styles"

	"charm.land/lipgloss/v2"
)

// Scrollbar renders a vertical scroll indicator as an independent component.
// When content fits in the viewport it returns an empty string.
type Scrollbar struct {
	totalHeight    int
	viewportHeight int
	scrollPercent  float64
	sty            *styles.Styles

	cachedView  string
	cachedDirty bool
}

func NewScrollbar(sty *styles.Styles) *Scrollbar {
	return &Scrollbar{sty: sty, cachedDirty: true}
}

func (s *Scrollbar) Update(totalHeight, viewportHeight int, scrollPercent float64) {
	if s.totalHeight == totalHeight && s.viewportHeight == viewportHeight && s.scrollPercent == scrollPercent {
		return
	}
	s.totalHeight = totalHeight
	s.viewportHeight = viewportHeight
	s.scrollPercent = scrollPercent
	s.cachedDirty = true
}

func (s *Scrollbar) View() string {
	if s.totalHeight <= s.viewportHeight || s.viewportHeight <= 0 {
		return ""
	}

	if !s.cachedDirty {
		return s.cachedView
	}

	thumbSize := max(1, s.viewportHeight*s.viewportHeight/s.totalHeight)
	thumbPos := 0
	trackSpace := s.viewportHeight - thumbSize
	if trackSpace > 0 {
		thumbPos = min(trackSpace, int(float64(trackSpace)*s.scrollPercent))
	}

	var sb strings.Builder
	for i := 0; i < s.viewportHeight; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(s.sty.Scrollbar.Thumb.Render(styles.HeavyVert))
		} else {
			sb.WriteString(s.sty.Scrollbar.Track.Render(styles.Vertical))
		}
	}

	s.cachedView = lipgloss.NewStyle().Width(1).Render(sb.String())
	s.cachedDirty = false
	return s.cachedView
}

