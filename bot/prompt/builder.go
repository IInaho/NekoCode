// builder.go — System prompt assembly with static/dynamic split.
//
// Pattern taken from Claude Code's getSystemPrompt() / splitSysPromptPrefix() /
// buildEffectiveSystemPrompt() design.
//
// The system prompt is split at SYSTEM_PROMPT_DYNAMIC_BOUNDARY into:
//   - Static prefix: identity, tool rules, engineering principles, safety.
//     This never changes between turns and benefits from prompt caching.
//   - Dynamic suffix: env info, skill list, todo, summary, closing instruction.
//     These change per-turn but are section-level cached where possible.
//
// The boundary enables cache scope optimization: static prefix can use
// server-side KV cache reuse across turns, minimizing token costs.

package prompt

import (
	_ "embed"
	"strings"
)

//go:embed system_static.md
var systemStatic string

//go:embed system_claude.md
var systemClaudeReasoning string

//go:embed system_deepseek.md
var systemDeepSeekReasoning string

// Builder assembles the system prompt from static prefix + dynamic sections.
type Builder struct {
	staticPrefix  string
	sections      *SectionManager
	override      string // non-empty when a mode override is active (e.g. plan mode)
	appendPrompt  string // always appended at the end, regardless of override
}

// NewBuilder creates a Builder with the appropriate reasoning strategy.
// provider: "anthropic" → think-first, "deepseek"/"glm"/other → act-first.
func NewBuilder(provider string) *Builder {
	reasoning := systemDeepSeekReasoning
	if provider == "anthropic" {
		reasoning = systemClaudeReasoning
	}
	staticPrefix := strings.Replace(systemStatic, "{{REASONING}}", reasoning, 1)
	return &Builder{
		staticPrefix: staticPrefix,
		sections:     NewSectionManager(),
	}
}

// SetOverride replaces the entire system prompt (static + dynamic) with
// a custom prompt. Used for plan mode and other special modes.
// Set to "" to restore normal behavior.
func (b *Builder) SetOverride(prompt string) {
	b.override = prompt
}

// SetAppend sets text that is always appended at the end of the system prompt,
// after dynamic sections. Used for mode-specific instructions that should
// layer on top of the default prompt rather than replace it.
func (b *Builder) SetAppend(prompt string) {
	b.appendPrompt = prompt
}

// AddCachedSection registers a cached dynamic section.
func (b *Builder) AddCachedSection(s *cachedSection) {
	b.sections.AddCached(s)
}

// AddUncachedSection registers an uncached dynamic section.
func (b *Builder) AddUncachedSection(s *uncachedSection) {
	b.sections.AddUncached(s)
}

// ClearSectionCache invalidates all cached sections (call on /clear, /compact).
func (b *Builder) ClearSectionCache() {
	b.sections.ClearAllCached()
}

// Build assembles the complete system prompt.
func (b *Builder) Build() string {
	// Override: completely replaces the system prompt.
	if b.override != "" {
		result := b.override
		if b.appendPrompt != "" {
			result += "\n\n" + b.appendPrompt
		}
		return result
	}

	var parts []string

	// Static prefix: never changes between turns.
	if b.staticPrefix != "" {
		parts = append(parts, b.staticPrefix)
	}

	// Dynamic suffix: section-level cached, changes per turn.
	if dynamic := b.sections.BuildDynamicSuffix(); dynamic != "" {
		parts = append(parts, dynamic)
	}

	// Append prompt: always at the end, even with override.
	if b.appendPrompt != "" {
		parts = append(parts, b.appendPrompt)
	}

	return strings.Join(parts, "\n\n")
}

