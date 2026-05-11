package ctxmgr

import (
	"unicode"

	"nekocode/llm"
)

const (
	asciiCharsPerToken = 4 // heuristic: ~4 ASCII chars per token
	toolCallOverhead   = 8 // per-tool-call token overhead
	toolOverheadTokens = 200
)

// estimatedTokens must be called with the lock held.
func (m *Manager) estimatedTokens() int {
	return estimateTokens(m.messages) + estimateTokensSystem(m.systemPrompt, m.summary)
}

// estimateTokens uses a language-aware heuristic: ASCII content ≈ 4 chars/token,
// CJK content ≈ 1.5 chars/token. ceiling division avoids zero results for short strings.
func estimateTokens(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += estimateString(m.Role)
		n += estimateString(m.Content)
		n += estimateString(m.ReasoningContent)
		n += estimateString(m.Name)
		for _, tc := range m.ToolCalls {
			n += estimateString(tc.ID)
			n += estimateString(tc.Function.Name)
			n += estimateString(tc.Function.Arguments)
			n += toolCallOverhead
		}
	}
	return n
}

func estimateTokensSystem(prompt, summary string) int {
	n := 0
	if prompt != "" {
		n += estimateString("system") + estimateString(prompt)
	}
	if summary != "" {
		n += estimateString("system") + estimateString(summary)
	}
	return n
}

func estimateString(s string) int {
	if len(s) == 0 {
		return 0
	}
	asciiChars := 0
	cjkChars := 0
	for _, r := range s {
		if r <= 127 {
			asciiChars++
		} else if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			cjkChars++
		} else {
			asciiChars++
		}
	}
	tokens := (asciiChars + asciiCharsPerToken - 1) / asciiCharsPerToken
	tokens += (cjkChars*2 + 2) / 3 // CJK: ~1.5 chars/token, ceiling via (2n+2)/3
	return tokens
}

func tokenOverhead(withTools bool) int {
	if withTools {
		return toolOverheadTokens
	}
	return 0
}
