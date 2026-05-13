package bot

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"nekocode/bot/agent"
	"nekocode/bot/agent/subagent"
	"nekocode/bot/command"
	"nekocode/bot/config"
	"nekocode/bot/projctx"
	"nekocode/bot/ctxmgr"
	"nekocode/bot/prompt"
	"nekocode/bot/session"
	"nekocode/bot/skill"
	"nekocode/bot/skill/bundled"
	"nekocode/bot/tools"
	"nekocode/bot/tools/builtin"
	"nekocode/llm"
)

type Bot struct {
	cfg           *config.Config
	ctxMgr        *ctxmgr.Manager
	cmdParser     *command.Parser
	ag            *agent.Agent
	sessMem       *session.Memory
	skillReg      *skill.Registry
	promptBuilder *prompt.Builder
	skillHint     string // skill activation hint for the TUI
	wantsAgent    bool   // true if the last command should continue to agent
	skillMsgStart int    // index of first skill message in ctxMgr, -1 if none
	skillMsgEnd   int    // index after last skill message
	toolRegistry  *tools.Registry
}

func New() *Bot {
	ctx := context.Background()

	cfg, _ := config.Load()

	promptBuilder := prompt.NewBuilder(cfg.Provider)

	// Register environment info as a cached system prompt section.
	// Stays stable across turns — only changes on cd or date change.
	// Having it in the system prompt (not a user message) keeps it
	// out of summarization and preserves prompt cache stability.
	if cwd, err := os.Getwd(); err == nil {
		now := time.Now().Format("2006-01-02")
		envSection := prompt.CachedSection(func() string {
			return fmt.Sprintf("<env>\nWorking directory: %s\nToday: %s\n</env>", cwd, now)
		})
		promptBuilder.AddCachedSection(envSection)
	}

	systemPrompt := promptBuilder.Build()

	ctxMgr := ctxmgr.New(systemPrompt)

	// Preload project context (NEKOCODE.md files) to avoid repeated
	// glob/grep/read exploration at the start of every conversation.
	// Environment info (CWD, date) is already in the cached system section.
	var projCtx string
	if cwd, err := os.Getwd(); err == nil {
		projCtx = projctx.LoadProjectContext(cwd)
		if projCtx != "" {
			ctxMgr.Add("system", projCtx)
		}
	}
	ctxMgr.SetTokenBudget(cfg.TokenBudget)

	llmClient := llm.NewClient(cfg.Provider, cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.ThinkingBudget)

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

	b := &Bot{
		toolRegistry:  toolRegistry,
		cfg:           cfg,
		ctxMgr:        ctxMgr,
		cmdParser:     cmdParser,
		ag:            agent.New(ctx, ctxMgr, llmClient, toolRegistry),
		sessMem:       sessMem,
		skillReg:      skillReg,
		promptBuilder: promptBuilder,
		skillMsgStart: -1,
	}

	// Sub-agent engine uses a separate LLM client with thinking disabled.
	// Sub-agents execute — they don't need extended reasoning.
	subLLM := llm.Clone(cfg.Provider, cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.ThinkingBudget)
	engine := subagent.NewEngine(subLLM, toolRegistry)
	names := make([]string, 0)
	for _, a := range subagent.List() {
		names = append(names, a.Name)
	}
	if t, err := toolRegistry.Get("task"); err == nil {
		t.(*builtin.TaskTool).Wire(func(ctx context.Context, prompt, agentType, thoroughness string) (*subagent.Result, error) {
			at, ok := subagent.Get(agentType)
			if !ok {
				return nil, fmt.Errorf("unknown sub-agent type: %s (available: %s)", agentType, strings.Join(names, ", "))
			}
			cwd, _ := os.Getwd()
			cfg := subagent.RunConfig{
				Prompt:          prompt,
				AgentType:       at,
				Cwd:             cwd,
				ProjectContext:  projCtx,
				Thoroughness:    thoroughness,
				DisableThinking: true, // subagents execute, don't ponder
			}
			if fn := b.ag.PhaseFn(); fn != nil {
				cfg.OnPhase = func(p string) { fn(at.Name + " · " + p) }
			}
			cfg.AddTokens = b.ag.AddTokens
			return engine.Run(ctx, cfg)
		}, names)
	}

	// Search circuit breaker: searching heavily without fetching may mean
	// the model is hallucinating. Inject a prompt first at threshold 4;
	// only force-stop at threshold 8.
	lastSearchPromptStep := 0
	b.ag.SetShouldStop(func(info agent.StopInfo) bool {
		if info.State.SearchCount >= 8 && info.State.FetchCount == 0 {
			return true
		}
		if info.State.SearchCount >= 4 && info.State.FetchCount == 0 && info.Step > lastSearchPromptStep+1 {
			lastSearchPromptStep = info.Step
			b.ctxMgr.Add("user", "[System] You've done "+strconv.Itoa(info.State.SearchCount)+" web searches without fetching any results. If the search snippets are sufficient, summarize findings. Otherwise, use web_fetch to get full content from the most relevant results.")
		}
		return false
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
		if toolResults > 40 {
			msgs = append(msgs, llm.Message{
				Role:    "user",
				Content: "[System] " + strconv.Itoa(toolResults) + " tool results accumulated. Check for unfinished sub-tasks — if any, continue with task. If all done, call task(verify) to validate, then report results.",
			})
		}
		return msgs
	})

		b.registerCustomCommands()

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
	b.ag.SetPlanMode(false) // plan mode is single-turn only
	// Restore default system prompt after exiting plan mode.
	b.ctxMgr.SetSystemPrompt(b.promptBuilder.Build())
	b.SummarizeIfNeeded()

	// Update goal from session memory after each turn.
	if b.sessMem != nil {
		if anchor := b.ctxMgr.Anchor(); anchor != nil {
			if content := b.sessMem.ReadContent(); b.sessMem.HasSubstance() {
				anchor.ExtractGoalFromSessionMemory(content)
			}
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

func (b *Bot) SetPhaseFn(fn tools.PhaseFunc) {
	b.ag.SetPhaseFn(fn)
}

func (b *Bot) SetCtxTodos(text string) { b.ctxMgr.SetTodos(text) }

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

func estimateToolDefTokens(descs []tools.Descriptor) int {
	n := 0
	for _, d := range descs {
		n += len(d.Name) + len(d.Description) + 80
		for _, p := range d.Parameters {
			n += len(p.Name) + len(p.Description) + len(p.Type) + 20
		}
	}
	return n / 4
}

