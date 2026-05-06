package tui

import (
	"strings"

	"primusbot/bot/tools"
)

// formatBriefArgs extracts key identifying args for a clean one-line tool display.
func formatBriefArgs(toolName, toolArgs string) string {
	parse := func(s string) map[string]string {
		m := make(map[string]string)
		for _, pair := range tools.SplitPairs(s) {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
		return m
	}
	args := parse(toolArgs)

	switch toolName {
	case "filesystem":
		op := args["operation"]
		path := args["path"]
		if op != "" && path != "" {
			return op + " " + path
		}
		return path
	case "edit":
		return args["path"]
	case "bash":
		cmd := args["command"]
		if len(cmd) > 50 {
			cmd = tools.TruncateByRune(cmd, 47)
		}
		return cmd
	case "glob":
		return args["pattern"]
	case "grep":
		pat := args["pattern"]
		p := args["path"]
		if p != "" {
			return pat + " " + p
		}
		return pat
	default:
		for _, v := range args {
			return tools.TruncateByRune(v, 27)
		}
		return ""
	}
}

// --- suggestions ---

func (m *Model) refreshSuggestions() {
	m.Suggestions.Refresh(m.Input.Value(), m.Bot.CommandNames())
}

func (m *Model) acceptSuggestion() {
	if val := m.Suggestions.Accept(); val != "" {
		m.Input.SetValue(val)
		m.Input.SetCursorEnd()
	}
}

func (m *Model) cycleSuggestion(delta int) {
	m.Suggestions.Cycle(delta)
}
