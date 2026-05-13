// priority.go — Prompt priority stack.
//
// Pattern taken from Claude Code's buildEffectiveSystemPrompt() which
// implements a 5-level override chain. NekoCode uses a simplified 2-level:
//
//   0. Override (plan mode) — completely replaces the system prompt
//   1. Default — standard NekoCode system prompt
//   + Append — always added at the end, regardless of level

package prompt

// Priority represents the current prompt priority level.
type Priority int

const (
	PriorityDefault  Priority = iota // normal system prompt
	PriorityOverride                 // plan mode: replace with mode-specific prompt
)

// SetPriorityMode configures the builder for a specific priority level.
func (b *Builder) SetPriorityMode(p Priority, prompt string) {
	switch p {
	case PriorityDefault:
		b.override = ""
		b.appendPrompt = ""
	case PriorityOverride:
		b.override = prompt
	}
}

// SetPlanModePrompt sets the plan mode system prompt, which enforces
// read-only exploration. Call SetPriorityMode(PriorityDefault, "") to exit.
func (b *Builder) SetPlanModePrompt(task string) string {
	prompt := PlanModePrompt(task)
	b.SetPriorityMode(PriorityOverride, prompt)
	return prompt
}
