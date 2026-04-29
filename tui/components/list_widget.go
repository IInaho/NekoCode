package components

import (
	"strings"
	"sync"
)

type Item interface {
	Render(width int) string
	Height(width int) int
}

type renderedItem struct {
	content string
	height  int
}

type List struct {
	width, height int
	items         []Item
	gap           int

	offsetIdx  int
	offsetLine int

	cache    map[int]renderedItem
	cacheMu  sync.RWMutex
	cacheWid int
}

func NewList(items ...Item) *List {
	return &List{
		items: items,
		cache: make(map[int]renderedItem),
	}
}

func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
}

func (l *List) SetGap(gap int) { l.gap = gap }

func (l *List) Width() int  { return l.width }
func (l *List) Height() int { return l.height }
func (l *List) Len() int    { return len(l.items) }

func (l *List) Items() []Item        { return l.items }
func (l *List) SetItems(items ...Item) {
	l.items = items
	l.offsetIdx = 0
	l.offsetLine = 0
	l.clearCache()
}

func (l *List) AppendItems(items ...Item) {
	l.items = append(l.items, items...)
}

func (l *List) getItem(idx int) renderedItem {
	if idx < 0 || idx >= len(l.items) {
		return renderedItem{}
	}

	l.cacheMu.RLock()
	if l.cacheWid == l.width {
		if cached, ok := l.cache[idx]; ok {
			l.cacheMu.RUnlock()
			return cached
		}
	}
	l.cacheMu.RUnlock()

	item := l.items[idx]
	content := item.Render(l.width)
	content = strings.TrimRight(content, "\n")
	height := strings.Count(content, "\n") + 1

	ri := renderedItem{content: content, height: height}

	l.cacheMu.Lock()
	if l.cacheWid != l.width {
		l.cache = make(map[int]renderedItem)
		l.cacheWid = l.width
	}
	l.cache[idx] = ri
	l.cacheMu.Unlock()

	return ri
}

func (l *List) clearCache() {
	l.cacheMu.Lock()
	l.cache = make(map[int]renderedItem)
	l.cacheMu.Unlock()
}

func (l *List) InvalidateItem(idx int) {
	l.cacheMu.Lock()
	delete(l.cache, idx)
	l.cacheMu.Unlock()
}

func (l *List) Invalidate() { l.clearCache() }

func (l *List) AtBottom() bool {
	if len(l.items) == 0 {
		return true
	}

	var totalHeight int
	for idx := l.offsetIdx; idx < len(l.items); idx++ {
		if totalHeight > l.height {
			return false
		}
		item := l.getItem(idx)
		itemHeight := item.height
		if l.gap > 0 && idx > l.offsetIdx {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
	}

	return totalHeight-l.offsetLine <= l.height
}

func (l *List) ScrollToTop() {
	l.offsetIdx = 0
	l.offsetLine = 0
}

func (l *List) ScrollToBottom() {
	if len(l.items) == 0 {
		return
	}

	var totalHeight int
	var idx int
	for idx = len(l.items) - 1; idx >= 0; idx-- {
		item := l.getItem(idx)
		itemHeight := item.height
		if l.gap > 0 && idx < len(l.items)-1 {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
		if totalHeight > l.height {
			break
		}
	}

	l.offsetIdx = max(idx, 0)
	l.offsetLine = max(totalHeight-l.height, 0)
}

func (l *List) ScrollBy(lines int) {
	if len(l.items) == 0 || lines == 0 {
		return
	}

	if lines > 0 {
		if l.AtBottom() {
			return
		}

		l.offsetLine += lines
		currentItem := l.getItem(l.offsetIdx)
		for l.offsetLine >= currentItem.height {
			l.offsetLine -= currentItem.height
			if l.gap > 0 {
				l.offsetLine = max(0, l.offsetLine-l.gap)
			}

			l.offsetIdx++
			if l.offsetIdx > len(l.items)-1 {
				l.ScrollToBottom()
				return
			}
			currentItem = l.getItem(l.offsetIdx)
		}
	} else {
		l.offsetLine += lines
		for l.offsetLine < 0 {
			l.offsetIdx--
			if l.offsetIdx < 0 {
				l.ScrollToTop()
				break
			}
			prevItem := l.getItem(l.offsetIdx)
			totalHeight := prevItem.height
			if l.gap > 0 {
				totalHeight += l.gap
			}
			l.offsetLine += totalHeight
		}
	}
}

func (l *List) VisibleItemIndices() (startIdx, endIdx int) {
	if len(l.items) == 0 {
		return 0, 0
	}

	startIdx = l.offsetIdx
	currentIdx := startIdx
	visibleHeight := -l.offsetLine

	for currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		visibleHeight += item.height
		if l.gap > 0 {
			visibleHeight += l.gap
		}

		if visibleHeight >= l.height {
			break
		}
		currentIdx++
	}

	endIdx = currentIdx
	if endIdx >= len(l.items) {
		endIdx = len(l.items) - 1
	}

	return startIdx, endIdx
}

func (l *List) Render() string {
	if len(l.items) == 0 {
		return ""
	}

	var lines []string
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine
	linesNeeded := l.height

	for linesNeeded > 0 && currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		itemLines := strings.Split(item.content, "\n")
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			startLine := currentOffset
			if startLine < len(itemLines) {
				lines = append(lines, itemLines[startLine:]...)
			}

			if l.gap > 0 {
				for i := 0; i < l.gap && len(lines) < l.height; i++ {
					lines = append(lines, "")
				}
			}
		} else if currentOffset >= itemHeight && l.gap > 0 {
			gapOffset := currentOffset - itemHeight
			gapRemaining := l.gap - gapOffset
			for i := 0; i < gapRemaining && len(lines) < l.height; i++ {
				lines = append(lines, "")
			}
		}

		linesNeeded = l.height - len(lines)
		currentIdx++
		currentOffset = 0
	}

	if len(lines) > l.height {
		lines = lines[:l.height]
	}

	return strings.Join(lines, "\n")
}

func (l *List) TotalContentHeight() int {
	var total int
	for i := 0; i < len(l.items); i++ {
		item := l.getItem(i)
		total += item.height
		if l.gap > 0 && i > 0 {
			total += l.gap
		}
	}
	return total
}

func (l *List) ScrollPercent() float64 {
	if len(l.items) == 0 {
		return 0
	}
	totalHeight := l.TotalContentHeight()
	if totalHeight <= l.height {
		return 0
	}

	var offsetPixels int
	for i := 0; i < l.offsetIdx; i++ {
		item := l.getItem(i)
		offsetPixels += item.height
		if l.gap > 0 {
			offsetPixels += l.gap
		}
	}
	offsetPixels += l.offsetLine

	maxOffset := totalHeight - l.height
	if maxOffset <= 0 {
		return 0
	}

	return float64(offsetPixels) / float64(maxOffset)
}
