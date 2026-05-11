package bot

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "embed"
	"nekocode/bot/agent"
	"nekocode/bot/agent/subagent"
	"nekocode/bot/command"
	"nekocode/bot/config"
	bctx "nekocode/bot/context"
	"nekocode/bot/ctxmgr"
	"nekocode/bot/session"
	"nekocode/bot/skill"
	"nekocode/bot/skill/bundled"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin"
	"nekocode/llm"
)

type Bot struct {
	cfg        *config.Config
	ctxMgr     *ctxmgr.Manager
	cmdParser  *command.Parser
	ag         *agent.Agent
	sessMem    *session.Memory
	extractor  *session.Extractor
	skillReg      *skill.Registry
	skillHint     string // skill activation hint for the TUI
	wantsAgent    bool   // true if the last command should continue to agent
	skillMsgStart int    // index of first skill message in ctxMgr, -1 if none
	skillMsgEnd   int    // index after last skill message
}

//go:embed prompt/system.md
var SystemPrompt string

func New() *Bot {
	ctx := context.Background()

	cfg, _ := config.Load()

	systemPrompt := SystemPrompt

	ctxMgr := ctxmgr.New(systemPrompt)

	// Inject environment as <system-reminder> user message (not system prompt)
	// so the system prompt stays lean for prompt caching.
	if cwd, err := os.Getwd(); err == nil {
		ctxMgr.Add("user", fmt.Sprintf("<system-reminder>\nWorking directory: %s\nToday's date is %s. Use this for web searches and file timestamps.\nUse list/glob to explore, read when needed.\n</system-reminder>", cwd, time.Now().Format("2006-01-02")))

		// Preload project context (NEKOCODE.md files) to avoid repeated
		// glob/grep/read exploration at the start of every conversation.
		if projCtx := bctx.LoadProjectContext(cwd); projCtx != "" {
			ctxMgr.Add("system", projCtx)
		}
	} else {
		ctxMgr.Add("user", fmt.Sprintf("<system-reminder>\nToday's date is %s.\n</system-reminder>", time.Now().Format("2006-01-02")))
	}
	ctxMgr.SetTokenBudget(cfg.TokenBudget)

	var llmClient llm.LLM
	switch cfg.Provider {
	case "anthropic":
		c := llm.NewAnthropic(cfg.APIKey, cfg.Model)
		// Separate thinking budget from output budget so reasoning
		// doesn't consume output tokens (OpenCode pattern).
		c.SetThinkingBudget(cfg.ThinkingBudget)
		llmClient = c
	case "glm":
		c := llm.NewGLM(cfg.APIKey, cfg.BaseURL, cfg.Model)
		c.SetDisableThinking(true) // DeepSeek auto-max can't be controlled, disable by default
		llmClient = c
	default:
		c := llm.NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.Model)
		c.SetDisableThinking(true) // DeepSeek auto-max can't be controlled, disable by default
		llmClient = c
	}

	ctxMgr.SetSummarizer(func(msgs []llm.Message, prevSummary string) (string, error) {
		prompt := ctxmgr.BuildPrompt(msgs, prevSummary)
		resp, err := llmClient.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, nil)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) > 0 {
			return resp.Choices[0].Message.Content, nil
		}
		return "", nil
	})

	toolRegistry := tools.NewRegistry()
	builtin.RegisterAll(toolRegistry)

	// Init global file state cache for read dedup (Phase 2).
	tools.GlobalFileCache = tools.NewFileStateCache()

	// Skill system: bundled first (take priority), then file-based.
	skillReg := skill.NewRegistry()
	skillReg.RegisterBundled(bundled.All())
	if err := skillReg.Load(skill.DefaultDirs()); err != nil {
		fmt.Fprintf(os.Stderr, "skill: load error: %v\n", err)
	}
	toolRegistry.Register(skill.NewSkillTool(skillReg))
	ctxMgr.SetSkillList(skill.BuildSkillListText(skillReg.List(), nil, cfg.TokenBudget))

	cmdParser := command.NewParser()

	sessID := fmt.Sprintf("session-%d", time.Now().Unix())
	sessMem, err := session.New(sessID, "")
	if err != nil {
		if sessMem, err = session.New("default", ""); err != nil {
			fmt.Fprintf(os.Stderr, "session memory: %v — running without session persistence\n", err)
		}
	}
	sessExt := session.NewExtractor(llmClient)

	b := &Bot{
		cfg:       cfg,
		ctxMgr:    ctxMgr,
		cmdParser: cmdParser,
		ag:        agent.New(ctx, ctxMgr, llmClient, toolRegistry),
		sessMem:   sessMem,
		extractor: sessExt,
		skillReg:  skillReg,
		skillMsgStart: -1,
	}

	// Sub-agent engine uses a separate LLM client with thinking disabled.
	// Sub-agents execute — they don't need extended reasoning.
	subLLM := cloneLLM(llmClient, cfg)
	subLLM.SetDisableThinking(true)
	engine := subagent.NewEngine(subLLM, toolRegistry)
	names := make([]string, 0)
	for _, a := range subagent.List() {
		names = append(names, a.Name)
	}
	if t, err := toolRegistry.Get("task"); err == nil {
		t.(*builtin.TaskTool).Wire(func(ctx context.Context, prompt, agentType string) (string, error) {
			at, ok := subagent.Get(agentType)
			if !ok {
				return "", fmt.Errorf("unknown sub-agent type: %s (available: %s)", agentType, strings.Join(names, ", "))
			}
			cwd, _ := os.Getwd()
			cfg := subagent.RunConfig{
				Prompt:          prompt,
				AgentType:       at,
				Cwd:             cwd,
				DisableThinking: true, // subagents execute, don't ponder
			}
			if fn := b.ag.PhaseFn(); fn != nil {
				cfg.OnPhase = func(p string) { fn(at.Name + " · " + p) }
			}
			cfg.AddTokens = b.ag.AddTokens
			return engine.Run(ctx, cfg)
		}, names)
	}

	// Circuit breaker: force synthesis when searching without fetching.
	b.ag.SetShouldStop(func(info agent.StopInfo) bool {
		return info.State.SearchCount >= 4 && info.State.FetchCount == 0
	})

	// Prompt the agent to make progress when accumulating many tool results.
	// Raised from 6 to 20 to accommodate subagent (task) workflows.
	b.ag.SetContextTransform(func(msgs []llm.Message) []llm.Message {
		toolResults := 0
		for _, m := range msgs {
			if m.Role == "tool" {
				toolResults++
			}
		}
		if toolResults > 20 {
			msgs = append(msgs, llm.Message{
				Role:    "user",
				Content: "[System] " + strconv.Itoa(toolResults) + " tool results accumulated. Check for unfinished sub-tasks — if any, continue with task. If all done, call task(verify) to validate, then report results.",
			})
		}
		return msgs
	})

	callbacks := &command.Callbacks{
		ClearHistory:   ctxMgr.Clear,
		GetConfig:      func() string { return fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model) },
		ForceSummarize: func() (string, error) { return b.ForceSummarize() },
		ContextStats:   func() string { return b.ContextStats() },
		FreshStart:     func() (string, error) { return b.ForceFreshStart() },
	}
	command.RegisterDefaults(b.cmdParser, callbacks)

	// Register each skill as a slash command (/skill-name).
	for _, sk := range skillReg.List() {
		name := sk.Name
		b.cmdParser.Register(name, func(cmd *command.Command) (string, bool) {
			sk, ok := skillReg.Get(name)
			if !ok {
				return fmt.Sprintf("Skill %q not found.", name), true
			}
			// Track skill message range so we can snip them when
			// the next turn doesn't need this skill.
			b.skillMsgStart = ctxMgr.Len()
			ctxMgr.Add("user", skill.FormatForContext(sk))
			skillReg.MarkLoaded(name)
			ctxMgr.SetSkillList(skill.BuildSkillListText(skillReg.List(), skillReg.LoadedSet(), cfg.TokenBudget))

			if len(cmd.Args) == 0 {
				b.skillMsgStart = -1
				return fmt.Sprintf("Loaded skill %q. Type your request to use it.", name), true
			}

			// Has arguments: inject clean prompt, set skill hint, start agent.
			ctxMgr.Add("user", strings.Join(cmd.Args, " "))
			b.skillMsgEnd = ctxMgr.Len()
			b.skillHint = name
			b.wantsAgent = true
			return "", false
		})
	}

	// Wire skill tool OnLoad so model-loaded skills are marked and excluded.
	if t, err := toolRegistry.Get("skill"); err == nil {
		t.(*skill.SkillTool).SetOnLoad(func(name string) {
			skillReg.MarkLoaded(name)
			ctxMgr.SetSkillList(skill.BuildSkillListText(skillReg.List(), skillReg.LoadedSet(), cfg.TokenBudget))
		})
	}

	// Wire snip tool so the model can proactively prune history.
	b.WireSnip()

	return b
}

// Public API

func (b *Bot) Provider() string { return b.cfg.Provider }
func (b *Bot) Model() string    { return b.cfg.Model }

func (b *Bot) ExecuteCommand(input string) (string, bool) {
	b.wantsAgent = false
	cmd := b.cmdParser.Parse(input)
	if cmd.Name == "" {
		// Plain text: clear skill context from previous turn.
		b.clearSkillContext()
		return "", false
	}
	return b.cmdParser.Execute(cmd)
}

// clearSkillContext removes skill messages from the previous turn so they
// don't consume context tokens when the current turn doesn't need the skill.
func (b *Bot) clearSkillContext() {
	if b.skillMsgStart < 0 || b.skillMsgEnd <= b.skillMsgStart {
		return
	}
	b.ctxMgr.Snip(b.skillMsgStart, b.skillMsgEnd-1)
	b.skillMsgStart = -1
	b.skillMsgEnd = 0
}

// SkillHint returns the skill activation hint and whether the agent should
// continue running. Call after ExecuteCommand.
func (b *Bot) SkillHint() (string, bool) {
	hint := b.skillHint
	cont := b.wantsAgent
	b.skillHint = ""
	b.wantsAgent = false
	return hint, cont
}

func (b *Bot) RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)) (string, error) {
	// Extract goal from first substantive user message.
	if anchor := b.ctxMgr.Anchor(); anchor != nil && anchor.Goal() == "" {
		anchor.ExtractGoalFromUserMessage(input)
	}

	result := b.ag.Run(input, onStep)
	b.SummarizeIfNeeded()

	// Update goal from session memory after each turn.
	if b.sessMem != nil {
		if anchor := b.ctxMgr.Anchor(); anchor != nil {
			if content := b.sessMem.ReadContent(); b.sessMem.HasSubstance() {
				anchor.ExtractGoalFromSessionMemory(content)
			}
		}

		// Trigger async session memory extraction if thresholds are met.
		_, tokens, _ := b.ctxMgr.Stats()
		toolCount := b.ctxMgr.ToolResultCount()
		hasToolCall := b.ctxMgr.LastAssistantHasToolCall()
		if b.sessMem.ShouldExtract(tokens, toolCount, hasToolCall, session.DefaultExtractConfig) {
			b.extractor.RunAsync(b.sessMem, b.ctxMgr, nil)
		}
	}

	return result.FinalOutput, result.Error
}

// Steer injects a user message mid-agent-loop, like Pi's steering messages.
func (b *Bot) Steer(msg string) { b.ag.Steer(msg) }
func (b *Bot) Abort()           { b.ag.Abort() }
func (b *Bot) SetStreamFn(fn func(delta string)) {
	b.ag.SetStreamFn(func(delta string, _ bool) { fn(delta) })
}

func (b *Bot) SetReasoningStreamFn(fn func(delta string)) {
	b.ag.SetReasoningStreamFn(fn)
}

func (b *Bot) SetConfirmFn(fn tools.ConfirmFunc) {
	b.ag.SetConfirmFn(fn)
}
func (b *Bot) WireTodoWrite(fn tools.TodoFunc) {
	b.ag.WireTodoWrite(fn)
}

func (b *Bot) WireSnip() {
	b.ag.WireSnip(func(startIdx, endIdx int) string {
		return b.ctxMgr.Snip(startIdx, endIdx)
	})
}

func (b *Bot) SetPhaseFn(fn tools.PhaseFunc) {
	b.ag.SetPhaseFn(fn)
}

func (b *Bot) SetCtxTodos(text string) { b.ctxMgr.SetTodos(text) }

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

func (b *Bot) TokenUsage() (prompt, completion int) {
	return b.ag.TokenUsage()
}

func (b *Bot) ContextTokens() int {
	return b.ag.ContextTokens()
}

func (b *Bot) CompactCount() int {
	return b.ctxMgr.CompactCount()
}

func (b *Bot) Duration() string {
	d := b.ag.Duration()
	if d == 0 {
		return ""
	}
	if d < time.Second {
		return "0s"
	}
	return d.Truncate(100 * time.Millisecond).String()
}

func (b *Bot) CommandNames() []string {
	return b.cmdParser.Commands()
}

// ForceFreshStart summarizes the current conversation (if enough content) then
// clears all messages, keeping the summary for future context.
// Creates a new session memory file so old context doesn't leak.
func (b *Bot) ForceFreshStart() (string, error) {
	count, oldTokens, _ := b.ctxMgr.Stats()
	b.skillReg.ClearLoaded()
	b.ctxMgr.SetSkillList(skill.BuildSkillListText(b.skillReg.List(), nil, b.cfg.TokenBudget))

	// Snapshot old session content before creating a new one.
	oldSessContent := ""
	oldSessHadSubstance := false
	if b.sessMem != nil {
		oldSessContent = b.sessMem.ReadContent()
		oldSessHadSubstance = b.sessMem.HasSubstance()
	}

	// Create a new session memory file for the new conversation.
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

// cloneLLM creates a copy of the LLM client with the same provider/model config.
func cloneLLM(client llm.LLM, cfg *config.Config) llm.LLM {
	switch cfg.Provider {
	case "anthropic":
		c := llm.NewAnthropic(cfg.APIKey, cfg.Model)
		c.SetBaseURL(cfg.BaseURL)
		return c
	case "glm":
		c := llm.NewGLM(cfg.APIKey, cfg.BaseURL, cfg.Model)
		return c
	default:
		c := llm.NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.Model)
		return c
	}
}
