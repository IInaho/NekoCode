// processing.go — ProcessingItem：流式渲染 output/reasoning 块 + 动态高度。
package processing

import (
	"strings"

	"nekocode/tui/components/block"
	"nekocode/tui/styles"
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
	compactCount int
	todos        string

	blocks         []block.ContentBlock
	reasoningText  strings.Builder
	outputText     strings.Builder

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
func (p *ProcessingItem) SetCompactCount(n int) {
	if p.compactCount != n { p.compactCount = n; p.invalidate() }
}
func (p *ProcessingItem) SetBlocks(blocks []block.ContentBlock) {
	p.blocks = blocks; p.reasoningText.Reset(); p.outputText.Reset(); p.invalidate()
}
func (p *ProcessingItem) SetTodos(text string) {
	if p.todos != text { p.todos = text; p.cachedTodosW = -1; p.invalidate() }
}

func (p *ProcessingItem) AppendReasoningText(delta string) { p.reasoningText.WriteString(delta); p.invalidate() }
func (p *ProcessingItem) AppendStreamText(delta string)    { p.outputText.WriteString(delta); p.invalidate() }
func (p *ProcessingItem) AddToolBlock(b block.ContentBlock) {
	if out := p.outputText.String(); out != "" && !strings.HasSuffix(out, "\n") {
		p.outputText.WriteString("\n")
	}
	if r := p.reasoningText.String(); r != "" && !strings.HasSuffix(r, "\n") {
		p.reasoningText.WriteString("\n")
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
func (p *ProcessingItem) AddTaskOutput(output string) {
	// Attach sub-agent output to the most recent task tool block so the
	// user can see what the sub-agent produced.
	for i := len(p.blocks) - 1; i >= 0; i-- {
		if p.blocks[i].Type == block.BlockTool && p.blocks[i].ToolName == "task" {
			p.blocks[i].Content = output
			p.invalidate()
			return
		}
	}
}

func (p *ProcessingItem) AddThinkBlock(content string) {
	p.blocks = append(p.blocks, block.ContentBlock{Type: block.BlockThought, Content: content}); p.invalidate()
}
func (p *ProcessingItem) Clear() {
	p.blocks = nil; p.todos = ""; p.reasoningText.Reset(); p.outputText.Reset(); p.invalidate()
}
func (p *ProcessingItem) invalidate() { p.cachedRenderW = -1; p.cachedToolN = 0; p.cachedToolW = 0 }

func (p *ProcessingItem) Height(width int) int {
	if p.cachedRenderW != width {
		p.Render(width)
	}
	return p.cachedHeight
}

func (p *ProcessingItem) Blocks() []block.ContentBlock { return p.blocks }

func (p *ProcessingItem) OutputText() string  { return p.outputText.String() }
func (p *ProcessingItem) ReasoningText() string { return p.reasoningText.String() }
