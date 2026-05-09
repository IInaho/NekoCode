// session/memory.go — Session Memory：持久化结构化会话记忆。
//
// 在对话进行中，用异步 subagent 周期性将对话精华提取到 Markdown 文件。
// 压缩上下文时优先使用该文件作为摘要（免费），而非调用 compact API。
package session

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nekocode/bot/ctxmgr"
	"nekocode/llm"
)

//go:embed memory_template.md
var DefaultTemplate string

// Memory manages the session memory file.
type Memory struct {
	mu       sync.Mutex
	path     string
	template string

	// Extraction tracking
	tokenAtLast int
	initialized bool

	// Async control
	extracting  bool
	extractDone chan struct{}
}

// ExtractConfig for extraction thresholds.
type ExtractConfig struct {
	MinTokensInit   int // min tokens before first extraction
	MinTokensUpdate int // min token growth since last extraction
	MinToolCalls    int // min tool calls since last extraction
}

var DefaultExtractConfig = ExtractConfig{
	MinTokensInit:   10000,
	MinTokensUpdate: 5000,
	MinToolCalls:    3,
}

func New(sessionID, template string) (*Memory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".nekocode", "sessions", sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("session memory: mkdir: %w", err)
	}
	if template == "" {
		template = DefaultTemplate
	}
	path := filepath.Join(dir, "memory.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(template), 0644); err != nil {
			return nil, fmt.Errorf("session memory: write template: %w", err)
		}
	}
	return &Memory{
		path:     path,
		template: template,
	}, nil
}

// ShouldExtract checks if extraction should trigger based on thresholds.
// lastTurnHasToolCall should be false when the last assistant message had no tool calls
// (meaning it's a natural conversation break — extract even if tool count unmet).
func (m *Memory) ShouldExtract(tokenCount, toolCallCount int, lastTurnHasToolCall bool, cfg ExtractConfig) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.extracting {
		return false
	}
	if !m.initialized {
		if tokenCount >= cfg.MinTokensInit {
			m.initialized = true
			m.tokenAtLast = tokenCount
		}
		return false
	}
	hasMetToken := (tokenCount - m.tokenAtLast) >= cfg.MinTokensUpdate
	hasMetTools := toolCallCount >= cfg.MinToolCalls
	return hasMetToken && (hasMetTools || !lastTurnHasToolCall)
}

// MarkExtraction records extraction start. Returns (done channel, wasNew).
// tokenAtLast is NOT reset here — ShouldExtract already set it when triggering.
func (m *Memory) MarkExtraction() (chan struct{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.extracting {
		return m.extractDone, false
	}
	m.extracting = true
	m.extractDone = make(chan struct{})
	return m.extractDone, true
}

// FinishExtraction marks extraction complete.
func (m *Memory) FinishExtraction() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extracting = false
	close(m.extractDone)
}

// ReadContent returns the current memory file content.
func (m *Memory) ReadContent() string {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return ""
	}
	return string(data)
}

// HasSubstance returns true if the memory file differs from the empty template.
func (m *Memory) HasSubstance() bool {
	c := m.ReadContent()
	return c != "" && c != m.template
}

// Extractor runs async session memory extraction.
type Extractor struct {
	llmClient llm.LLM
}

func NewExtractor(llmClient llm.LLM) *Extractor {
	return &Extractor{llmClient: llmClient}
}

// RunAsync spawns extraction in a goroutine. Returns immediately.
// The extraction reads current messages, asks the LLM to update the memory file,
// and writes the result back to disk.
func (ex *Extractor) RunAsync(memory *Memory, mgr *ctxmgr.Manager, phaseFn func(string)) {
	ch, isNew := memory.MarkExtraction()
	if !isNew {
		return
	}

	go func() {
		defer memory.FinishExtraction()
		_ = ch

		currentContent := memory.ReadContent()
		messages := mgr.Build(false)
		prompt := buildExtractPrompt(messages, currentContent)

		resp, err := ex.llmClient.Chat(
			context.Background(),
			[]llm.Message{{Role: "user", Content: prompt}},
			nil,
		)
		if err != nil {
			return
		}
		if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
			return
		}

		newContent := resp.Choices[0].Message.Content
		// Only write if the LLM produced meaningful output.
		if len(newContent) > 50 && strings.Contains(newContent, "#") {
			_ = os.WriteFile(memory.path, []byte(newContent), 0644)
		}

		if phaseFn != nil {
			phaseFn("session memory updated")
		}
	}()
}

// buildExtractPrompt creates the extraction prompt.
func buildExtractPrompt(messages []llm.Message, currentMemory string) string {
	var b strings.Builder
	for _, m := range messages {
		limit := 300
		if m.Role == "tool" {
			limit = 500
		}
		content := m.Content
		runes := []rune(content)
		if len(runes) > limit {
			content = string(runes[:limit]) + "..."
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, content)
	}
	conversation := b.String()

	template := `You are a session note-taking assistant. Based on the conversation above, update the structured session notes file below.

Rules:
- Only update content below section headers (lines starting with #)
- Do NOT modify or delete any # section header lines or *italic instruction* lines
- Do NOT add new sections
- Be specific: file paths, function names, error messages, exact commands
- Keep each section under 500 chars
- Current State should always reflect the latest progress
- Worklog uses short entries, one per step`

	if currentMemory != "" && currentMemory != DefaultTemplate {
		return fmt.Sprintf("%s\n\n[Current Notes]\n%s\n\n[New Conversation]\n%s\n\nOutput the updated complete notes file:", template, currentMemory, conversation)
	}
	return fmt.Sprintf("%s\n\n[Conversation]\n%s\n\nOutput the complete notes file:", template, conversation)
}
