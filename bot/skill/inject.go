package skill

import (
	"fmt"
	"strings"
)

// BuildSkillListText generates the available-skills text injected into context.
// Loaded skills are excluded to prevent the model from double-loading.
// tokenBudget constrains the output to ~1% of the context window (default ~2500 chars).
// Returns empty string when no skills are available (or all are already loaded).
func BuildSkillListText(skills []*Skill, loaded map[string]bool, tokenBudget int) string {
	if len(skills) == 0 {
		return ""
	}

	// Budget: 1% of context window, min 500 chars, max 3000 chars.
	maxChars := tokenBudget / 100
	if maxChars < 500 {
		maxChars = 500
	}
	if maxChars > 3000 {
		maxChars = 3000
	}

	header := "## Available Skills\n\n"
	header += "Use the skill tool to load detailed workflow instructions. Loaded skills are excluded:\n\n"
	headerChars := len([]rune(header))

	var entries []string
	for _, sk := range skills {
		if loaded[sk.Name] {
			continue
		}
		entry := fmt.Sprintf("- **%s**: %s\n", sk.Name, sk.Description)
		if sk.WhenToUse != "" {
			entry += fmt.Sprintf("  When to use: %s\n", sk.WhenToUse)
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return ""
	}

	// Build until budget exhausted.
	var sb strings.Builder
	sb.WriteString(header)
	remaining := maxChars - headerChars
	listed := 0

	for _, entry := range entries {
		entryChars := len([]rune(entry))
		if remaining < entryChars {
			if listed == 0 {
				// At least include the first skill even if over budget.
				sb.WriteString(entry)
				listed++
			}
			break
		}
		sb.WriteString(entry)
		remaining -= entryChars
		listed++
	}

	if listed < len(entries) {
		sb.WriteString(fmt.Sprintf("\n(%d more skills available but omitted due to token budget)\n", len(entries)-listed))
	}

	return sb.String()
}

// FormatForContext formats a skill's content for injection into the conversation
// context, used by slash commands (/skill-name) and the skill tool.
func FormatForContext(sk *Skill) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<skill_content name=\"%s\">\n", sk.Name))
	sb.WriteString(fmt.Sprintf("# Skill: %s\n\n", sk.Name))
	sb.WriteString(fmt.Sprintf("**This skill is already loaded. Do NOT call the skill tool for %q.**\n\n", sk.Name))

	if sk.Dir != "" {
		sb.WriteString(fmt.Sprintf("**Skill files (templates, references, scripts): `%s`** — Read input files from here using absolute paths. Do NOT glob or search.\n", sk.Dir))
		sb.WriteString("**Output files go to the current working directory**, NOT the skill directory.\n\n")
	} else {
		sb.WriteString("(This is a built-in skill with no file-system directory.)\n\n")
	}

	sb.WriteString(sk.Content)

	if len(sk.Files) > 0 {
		sb.WriteString("\n\n## Skill files (absolute paths):\n")
		for _, f := range sk.Files {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	sb.WriteString("</skill_content>")
	return sb.String()
}
