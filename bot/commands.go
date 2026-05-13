package bot

import (
	"fmt"
	"strings"

	"nekocode/bot/command"
	"nekocode/bot/prompt"
	"nekocode/bot/skill"
)

// clearSkillContext removes skill messages from the previous turn so they
// don't consume context tokens when the current turn doesn't need the skill.
func (b *Bot) clearSkillContext() {
	if b.skillMsgStart < 0 || b.skillMsgEnd <= b.skillMsgStart {
		return
	}
	b.ctxMgr.RemoveMessages(b.skillMsgStart, b.skillMsgEnd-1)
	b.skillMsgStart = -1
	b.skillMsgEnd = 0
}

// registerCustomCommands registers /plan and per-skill slash commands.
// Must be called after b.ag and b.ctxMgr are initialized.
func (b *Bot) registerCustomCommands() {
	callbacks := &command.Callbacks{
		ClearHistory:   b.ctxMgr.Clear,
		GetConfig:      func() string { return fmt.Sprintf("%s/%s", b.cfg.Provider, b.cfg.Model) },
		ForceSummarize: func() (string, error) { return b.ForceSummarize() },
		ContextStats:   func() string { return b.ContextStats() },
		ContextReport:  func() string { return b.ContextReport() },
		FreshStart:     func() (string, error) { return b.ForceFreshStart() },
	}
	command.RegisterDefaults(b.cmdParser, callbacks)

	// /plan command: enter read-only exploration mode, design approach, get user approval.
	b.cmdParser.Register("plan", func(cmd *command.Command) (string, bool) {
		if len(cmd.Args) == 0 {
			return "Usage: /plan <task description> — enter read-only exploration mode to design an approach before coding.", true
		}
		task := strings.Join(cmd.Args, " ")
		b.ag.SetPlanMode(true)
		b.ctxMgr.SetSystemPrompt(prompt.PlanModePrompt(task))
		b.ctxMgr.Add("user", task)
		b.wantsAgent = true
		return "", false
	})

	// Register each skill as a slash command (/skill-name).
	for _, sk := range b.skillReg.List() {
		name := sk.Name
		b.cmdParser.Register(name, func(cmd *command.Command) (string, bool) {
			sk, ok := b.skillReg.Get(name)
			if !ok {
				return fmt.Sprintf("Skill %q not found.", name), true
			}
			b.skillMsgStart = b.ctxMgr.Len()
			b.ctxMgr.Add("user", skill.FormatForContext(sk))
			b.skillReg.MarkLoaded(name)
			b.ctxMgr.SetSkillList(skill.BuildSkillListText(b.skillReg.List(), b.skillReg.LoadedSet(), b.cfg.TokenBudget))

			if len(cmd.Args) == 0 {
				b.skillMsgStart = -1
				return fmt.Sprintf("Loaded skill %q. Type your request to use it.", name), true
			}

			b.ctxMgr.Add("user", strings.Join(cmd.Args, " "))
			b.skillMsgEnd = b.ctxMgr.Len()
			b.skillHint = name
			b.wantsAgent = true
			return "", false
		})
	}

	// Wire skill tool OnLoad so model-loaded skills are marked and excluded.
	if t, err := b.toolRegistry.Get("skill"); err == nil {
		t.(*skill.SkillTool).SetOnLoad(func(name string) {
			b.skillReg.MarkLoaded(name)
			b.ctxMgr.SetSkillList(skill.BuildSkillListText(b.skillReg.List(), b.skillReg.LoadedSet(), b.cfg.TokenBudget))
		})
	}
}
