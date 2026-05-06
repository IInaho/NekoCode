package styles

import (
	"sync"

	"github.com/charmbracelet/glamour"
)

var zeroMarginStyle = []byte(`{"document":{"margin":0}}`)

var (
	mu        sync.Mutex
	renderers = map[int]*glamour.TermRenderer{}
	warmedUp  bool
)

func Warmup() {
	mu.Lock()
	defer mu.Unlock()
	if warmedUp {
		return
	}
	for w := 40; w <= 160; w++ {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithStylesFromJSONBytes(zeroMarginStyle),
			glamour.WithWordWrap(w),
		)
		if err != nil {
			panic("failed to warm up markdown renderer: " + err.Error())
		}
		renderers[w] = r
	}
	warmedUp = true
}

func getRenderer(width int) *glamour.TermRenderer {
	mu.Lock()
	defer mu.Unlock()
	if r, ok := renderers[width]; ok {
		return r
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithStylesFromJSONBytes(zeroMarginStyle),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		panic("failed to create markdown renderer: " + err.Error())
	}
	renderers[width] = r
	return r
}

func RenderMarkdown(content string) string {
	return RenderMarkdownWithWidth(content, 80)
}

func RenderMarkdownWithWidth(content string, width int) string {
	if width <= 0 {
		width = 80
	}
	out, err := getRenderer(width).Render(content)
	if err != nil {
		return content
	}
	return out
}
