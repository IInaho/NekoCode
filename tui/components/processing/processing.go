// processing.go — ProcessingItem：流式渲染 output/reasoning 块 + 动态高度。
package processing
import (
	"strings"

	"primusbot/tui/styles"
	"primusbot/tui/components/block"

)

const (
	reasonLines = 6 // fixed height for reasoning section
	outputLines = 6 // fixed height for output section
)

type ProcessingItem struct {

	sty          *styles.Styles
	spinnerView  string
	statusText   string
	contentWidth int
	tokenPrompt  int
	tokenCompl   int
	todos        string

	blocks        []block.ContentBlock
	reasoningText string
	outputText    string

	cachedRender  string
	cachedRenderW int
	cachedHeight  int
	cachedToolN   int
	cachedToolW   int
	cachedTool    string
	cachedTodos   string
	cachedTodosW  int
}

func NewProcessingItem(sty *styles.Styles) *ProcessingItem {
	return &ProcessingItem{sty: sty, cachedTodosW: -1}
}

func (p *ProcessingItem) SetSpinnerView(view string) { p.spinnerView = view; p.invalidate() }
func (p *ProcessingItem) SetStatusText(text string)   { p.statusText = text; p.invalidate() }
func (p *ProcessingItem) SetTokens(prompt, completion int) {
	p.tokenPrompt = prompt; p.tokenCompl = completion; p.invalidate()
}
func (p *ProcessingItem) SetBlocks(blocks []block.ContentBlock) {
	p.blocks = blocks; p.reasoningText = ""; p.outputText = ""; p.invalidate()
}
func (p *ProcessingItem) SetTodos(text string) {
	if p.todos != text { p.todos = text; p.cachedTodosW = -1; p.invalidate() }
}

func (p *ProcessingItem) AppendReasoningText(delta string) { p.reasoningText += delta; p.invalidate() }
func (p *ProcessingItem) AppendStreamText(delta string)    { p.outputText += delta; p.invalidate() }
func (p *ProcessingItem) AddToolBlock(b block.ContentBlock) {
	if p.outputText != "" && !strings.HasSuffix(p.outputText, "\n") {
		p.outputText += "\n"
	}
	if p.reasoningText != "" && !strings.HasSuffix(p.reasoningText, "\n") {
		p.reasoningText += "\n"
	}
	p.blocks = append(p.blocks, b)
	p.invalidate()
}
func (p *ProcessingItem) AddDiffBlock(content string) {
	// Embed diff into the preceding edit tool block.
	for i := len(p.blocks) - 1; i >= 0; i-- {
		if p.blocks[i].Type == block.BlockTool && p.blocks[i].ToolName == "edit" {
			p.blocks[i].Content = content
			p.invalidate()
			return
		}
	}
}
func (p *ProcessingItem) AddThinkBlock(content string) {
	p.blocks = append(p.blocks, block.ContentBlock{Type: block.BlockThought, Content: content}); p.invalidate()
}
func (p *ProcessingItem) Clear() {
	p.blocks = nil; p.todos = ""; p.reasoningText = ""; p.outputText = ""; p.invalidate()
}
func (p *ProcessingItem) invalidate() { p.cachedRenderW = -1; p.cachedToolN = 0; p.cachedToolW = 0 }

func (p *ProcessingItem) Height(width int) int {
	if p.cachedRenderW != width {
		p.Render(width)
	}
	return p.cachedHeight
}

func (p *ProcessingItem) Blocks() []block.ContentBlock { return p.blocks }
