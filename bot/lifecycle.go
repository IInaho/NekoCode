package bot

import (
	"fmt"
	"time"

	"nekocode/bot/ctxmgr"
	"nekocode/bot/session"
	"nekocode/bot/skill"
)

func (b *Bot) SummarizeIfNeeded() {
	if !b.ctxMgr.NeedsSummarization() {
		return
	}

	// Try session memory first (free, no API call).
	if b.sessMem != nil {
		if content := b.sessMem.ReadContent(); b.sessMem.HasSubstance() && len(content) > 100 {
			_ = b.ctxMgr.SummarizeWithSessionMemory(content)
			return
		}
	}

	// Fall back to LLM summarizer.
	_ = b.ctxMgr.Summarize()
}

func (b *Bot) ForceSummarize() (string, error) {
	count, tokens, hadSummary := b.ctxMgr.Stats()
	if count <= 2 {
		return "Conversation too short, nothing to compact.", nil
	}
	if !b.ctxMgr.NeedsSummarization() {
		return fmt.Sprintf("Not needed: %d messages, ~%d tokens — well under budget", count, tokens), nil
	}
	if err := b.ctxMgr.Summarize(); err != nil {
		return "", err
	}
	_, newTokens, _ := b.ctxMgr.Stats()
	if newTokens >= tokens {
		return fmt.Sprintf("Already compact: %d messages, ~%d tokens — nothing to compress", count, tokens), nil
	}
	action := "Compacted"
	if hadSummary {
		action = "Summary updated"
	}
	return fmt.Sprintf("%s: %d messages, ~%d → ~%d tokens", action, count, tokens, newTokens), nil
}

func (b *Bot) ContextStats() string {
	count, tokens, hasSummary := b.ctxMgr.Stats()
	summary := "none"
	if hasSummary {
		summary = "yes"
	}
	return fmt.Sprintf("Messages: %d, ~%d tokens, summary: %s", count, tokens, summary)
}

func (b *Bot) ContextReport() string {
	r := b.ctxMgr.Report()
	r.ToolDefCount = len(b.toolRegistry.Descriptors())
	r.ToolDefTokens = estimateToolDefTokens(b.toolRegistry.Descriptors())
	return ctxmgr.FormatContextReport(r)
}

func (b *Bot) ForceFreshStart() (string, error) {
	count, oldTokens, _ := b.ctxMgr.Stats()
	b.skillReg.ClearLoaded()
	b.ctxMgr.SetSkillList(skill.BuildSkillListText(b.skillReg.List(), nil, b.cfg.TokenBudget))

	oldSessContent := ""
	oldSessHadSubstance := false
	if b.sessMem != nil {
		oldSessContent = b.sessMem.ReadContent()
		oldSessHadSubstance = b.sessMem.HasSubstance()
	}

	newSessID := fmt.Sprintf("session-%d", time.Now().Unix())
	newSess, err := session.New(newSessID, "")
	if err != nil {
		newSess = nil
	}
	b.sessMem = newSess

	if count <= 2 {
		b.ctxMgr.FreshStart()
		return fmt.Sprintf("New session %s started.", newSessID), nil
	}

	// Use old session memory as summary if available (free, no API call).
	if oldSessHadSubstance && oldSessContent != "" {
		b.ctxMgr.SetSummary(oldSessContent)
		b.ctxMgr.FreshStart()
		_, newTokens, _ := b.ctxMgr.Stats()
		return fmt.Sprintf("New session %s. %d messages, ~%d tokens → session memory (~%d tokens)", newSessID, count, oldTokens, newTokens), nil
	}

	// Fall back to API summarizer.
	if b.ctxMgr.NeedsSummarization() {
		if err := b.ctxMgr.Summarize(); err != nil {
			return "", err
		}
	}
	b.ctxMgr.FreshStart()
	_, newTokens, hasSummary := b.ctxMgr.Stats()
	detail := "no summary"
	if hasSummary {
		detail = "with summary"
	}
	return fmt.Sprintf("New session %s. %d messages, ~%d tokens → %s (~%d tokens)", newSessID, count, oldTokens, detail, newTokens), nil
}
