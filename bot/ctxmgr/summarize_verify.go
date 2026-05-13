// summarize_verify.go — post-summarization constraint verification.
//
// After the LLM generates a summary, we check that any critical constraints
// extracted from user messages are still present in the summary. If a
// constraint was lost during compression, we force re-summary with an
// explicit preservation instruction.

package ctxmgr

import (
	"strings"

	"nekocode/llm"
)

// verifySummary checks that critical constraints from the anchor are
// preserved in the generated summary. Returns a list of missing constraints.
func (m *Manager) verifySummary(summary string) []string {
	if m.anchor == nil {
		return nil
	}
	return checkConstraints(m.anchor.Constraints(), summary)
}

// checkConstraints is the lock-free core of verifySummary, accepting a
// pre-snapshotted constraint list so it can be called outside m.mu.
func checkConstraints(constraints []string, summary string) []string {
	if len(constraints) == 0 {
		return nil
	}
	var missing []string
	summaryLower := strings.ToLower(summary)
	for _, c := range constraints {
		cLower := strings.ToLower(c)
		if !strings.Contains(summaryLower, cLower) {
			longestWord := longestWord(cLower)
			if longestWord == "" || !strings.Contains(summaryLower, longestWord) {
				missing = append(missing, c)
			}
		}
	}
	return missing
}

// BuildVerifyPrompt creates a re-summarization prompt that explicitly
// requires preserving the missing constraints.
func BuildVerifyPrompt(msgs []llm.Message, prevSummary string, missing []string) string {
	prompt := BuildPrompt(msgs, prevSummary)

	var constraintList strings.Builder
	for _, c := range missing {
		constraintList.WriteString("- \"" + c + "\"\n")
	}

	return prompt + "\n\nCRITICAL: The following user constraints were LOST in the previous summary. You MUST include them VERBATIM in the [Critical Context] section this time:\n" +
		constraintList.String() +
		"\nRe-generate the summary now."
}

func longestWord(s string) string {
	words := strings.Fields(s)
	longest := ""
	for _, w := range words {
		if len(w) > len(longest) {
			longest = w
		}
	}
	// Only return if it has substance (>3 chars)
	if len(longest) > 3 {
		return longest
	}
	return ""
}
