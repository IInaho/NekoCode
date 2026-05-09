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
	"nekocode/bot/ctxmgr"
	"nekocode/bot/session"
	"nekocode/bot/tools"
	"nekocode/llm"
)

type Bot struct {
	cfg       *Config
	ctxMgr    *ctxmgr.Manager
	cmdParser *Parser
	ag        *agent.Agent
	sessMem   *session.Memory
	extractor *session.Extractor
}

//go:embed prompt/system.md
var SystemPrompt string

func New() *Bot {
	ctx := context.Background()

	cfg, _ := LoadConfig()

	systemPrompt := SystemPrompt

	ctxMgr := ctxmgr.New(systemPrompt)

	// Inject environment as <system-reminder> user message (not system prompt)
	// so the system prompt stays lean for prompt caching.
	if cwd, err := os.Getwd(); err == nil {
		ctxMgr.Add("user", fmt.Sprintf("<system-reminder>\nWorking directory: %s\nToday's date is %s. Use this for web searches and file timestamps.\nUse list/glob to explore, read when needed.\n</system-reminder>", cwd, time.Now().Format("2006-01-02")))
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
	tools.RegisterDefaults(toolRegistry)

	cmdParser := NewParser()

	sessID := fmt.Sprintf("session-%d", time.Now().Unix())
	sessMem, err := session.New(sessID, "")
	if err != nil {
		sessMem, _ = session.New("default", "") // fallback to /tmp-based default
	}
	sessExt := session.NewExtractor(llmClient)

	b := &Bot{
		cfg:       cfg,
		ctxMgr:    ctxMgr,
		cmdParser: cmdParser,
		ag:        agent.New(ctx, ctxMgr, llmClient, toolRegistry),
		sessMem:   sessMem,
		extractor: sessExt,
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
		t.(*tools.TaskTool).Wire(func(ctx context.Context, prompt, agentType string) (string, error) {
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

	callbacks := &CommandCallbacks{
		ClearHistory:   ctxMgr.Clear,
		GetConfig:      func() string { return fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model) },
		ForceSummarize: func() (string, error) { return b.ForceSummarize() },
		ContextStats:   func() string { return b.ContextStats() },
		FreshStart:     func() (string, error) { return b.ForceFreshStart() },
	}
	RegisterDefaultCommands(b.cmdParser, callbacks)

	// Wire snip tool so the model can proactively prune history.
	b.WireSnip()

	return b
}

// Public API

func (b *Bot) Provider() string { return b.cfg.Provider }
func (b *Bot) Model() string    { return b.cfg.Model }

func (b *Bot) ExecuteCommand(input string) (string, bool) {
	cmd := b.cmdParser.Parse(input)
	if cmd.Name == "" {
		return "", false
	}
	return b.cmdParser.Execute(cmd)
}

func (b *Bot) RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)) (string, error) {
	result := b.ag.Run(input, onStep)
	b.SummarizeIfNeeded()

	// Trigger async session memory extraction if thresholds are met.
	_, tokens, _ := b.ctxMgr.Stats()
	toolCount := b.ctxMgr.ToolResultCount()
	hasToolCall := b.ctxMgr.LastAssistantHasToolCall()
	if b.sessMem.ShouldExtract(tokens, toolCount, hasToolCall, session.DefaultExtractConfig) {
		b.extractor.RunAsync(b.sessMem, b.ctxMgr, nil)
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
	if b.ctxMgr.NeedsSummarization() {
		_ = b.ctxMgr.Summarize()
	}
}

func (b *Bot) ForceSummarize() (string, error) {
	count, tokens, hadSummary := b.ctxMgr.Stats()
	if count <= 2 {
		return "Conversation too short, nothing to compact.", nil
	}
	if err := b.ctxMgr.Summarize(); err != nil {
		return "", err
	}
	_, newTokens, _ := b.ctxMgr.Stats()
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
// Uses session memory content as summary if available (free), otherwise calls the
// configured summarizer.
func (b *Bot) ForceFreshStart() (string, error) {
	count, oldTokens, _ := b.ctxMgr.Stats()
	if count <= 2 {
		b.ctxMgr.FreshStart()
		return "New conversation started.", nil
	}

	// Try session memory first (free, no API call).
	if content := b.sessMem.ReadContent(); b.sessMem.HasSubstance() {
		b.ctxMgr.SetSummary(content)
		b.ctxMgr.FreshStart()
		_, newTokens, _ := b.ctxMgr.Stats()
		return fmt.Sprintf("New conversation. %d messages, ~%d tokens → session memory (~%d tokens)", count, oldTokens, newTokens), nil
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
	return fmt.Sprintf("New conversation. %d messages, ~%d tokens → %s (~%d tokens)", count, oldTokens, detail, newTokens), nil
}

// cloneLLM creates a copy of the LLM client with the same provider/model config.
func cloneLLM(client llm.LLM, cfg *Config) llm.LLM {
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
