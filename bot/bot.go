package bot

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "embed"
	"primusbot/bot/agent"
	"primusbot/bot/agent/subagent"
	"primusbot/bot/extensions"
	"primusbot/bot/tools"
	"primusbot/bot/types"
	"primusbot/bot/ctxmgr"
	"primusbot/llm"
)

type Bot struct {
	cfg       *Config
	ctxMgr    *ctxmgr.Manager
	cmdParser *Parser
	ag        *agent.Agent
}

//go:embed prompt/system.md
var SystemPrompt string

func New(exts ...extensions.Extension) *Bot {
	ctx := context.Background()

	cfg, _ := LoadConfig()

	systemPrompt := SystemPrompt

	ctxMgr := ctxmgr.New(systemPrompt)

	// Inject environment as <system-reminder> user message (not system prompt)
	// so the system prompt stays lean for prompt caching.
	if cwd, err := os.Getwd(); err == nil {
		ctxMgr.Add("user", fmt.Sprintf("<system-reminder>\n当前工作目录: %s\n用 list/glob 探索目录结构，需要时再 read。\n</system-reminder>", cwd))
	}
	ctxMgr.SetTokenBudget(cfg.TokenBudget)

	var llmClient llm.LLM
	switch cfg.Provider {
	case "anthropic":
		llmClient = llm.NewAnthropic(cfg.APIKey, cfg.Model)
	case "glm":
		llmClient = llm.NewGLM(cfg.APIKey, cfg.BaseURL, cfg.Model)
	default:
		llmClient = llm.NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.Model)
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

	// Register extension tools and commands.
	for _, ext := range exts {
		for _, t := range ext.Tools() {
			toolRegistry.Register(t)
		}
		for _, c := range ext.Commands() {
			name := c.Name
			handler := c.Handler
			cmdParser.Register(name, func(cmd *Command) (string, bool) {
				result, err := handler(cmd.Args)
				if err != nil {
					return "Error: " + err.Error(), true
				}
				return result, true
			})
		}
	}

	b := &Bot{
		cfg:       cfg,
		ctxMgr:    ctxMgr,
		cmdParser: cmdParser,
		ag:        agent.New(ctx, ctxMgr, llmClient, toolRegistry),
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
				return "", fmt.Errorf("未知的子 agent 类型: %s（可用: %s）", agentType, strings.Join(names, ", "))
			}
			cwd, _ := os.Getwd()
			cfg := subagent.RunConfig{
				Prompt:    prompt,
				AgentType: at,
				Cwd:       cwd,
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
				Content: "【系统指令】已经有 " + strconv.Itoa(toolResults) + " 个工具结果了。检查是否有未完成的子任务——如果有，继续调用 task 执行。如果全部完成，调用 task(verify) 验证，然后汇报结果。",
			})
		}
		return msgs
	})

	callbacks := &CommandCallbacks{
		ClearHistory:   ctxMgr.Clear,
		GetConfig:      func() string { return fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model) },
		ForceSummarize: func() (string, error) { return b.ForceSummarize() },
		ContextStats:   func() string { return b.ContextStats() },
	}
	RegisterDefaultCommands(b.cmdParser, callbacks)

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
	return result.FinalOutput, result.Error
}

// Steer injects a user message mid-agent-loop, like Pi's steering messages.
func (b *Bot) Steer(msg string)                { b.ag.Steer(msg) }
func (b *Bot) Abort()                            { b.ag.Abort() }
func (b *Bot) SetStreamFn(fn func(delta string)) {
	b.ag.SetStreamFn(func(delta string, _ bool) { fn(delta) })
}

func (b *Bot) SetReasoningStreamFn(fn func(delta string)) {
	b.ag.SetReasoningStreamFn(fn)
}

func (b *Bot) SetConfirmFn(fn types.ConfirmFunc) {
	b.ag.SetConfirmFn(fn)
}
func (b *Bot) WireTodoWrite(fn tools.TodoFunc) {
	b.ag.WireTodoWrite(fn)
}

func (b *Bot) SetPhaseFn(fn types.PhaseFunc) {
	b.ag.SetPhaseFn(fn)
}

func (b *Bot) SetCtxTodos(text string)   { b.ctxMgr.SetTodos(text) }

func (b *Bot) SummarizeIfNeeded() {
	if b.ctxMgr.NeedsSummarization() {
		_ = b.ctxMgr.Summarize()
	}
}

func (b *Bot) ForceSummarize() (string, error) {
	count, tokens, hadSummary := b.ctxMgr.Stats()
	if count <= 2 {
		return "对话太短，无需压缩", nil
	}
	if err := b.ctxMgr.Summarize(); err != nil {
		return "", err
	}
	_, newTokens, _ := b.ctxMgr.Stats()
	prev := "已压缩"
	if hadSummary {
		prev = "已更新摘要"
	}
	return fmt.Sprintf("%s：%d 条消息 → %d tokens", prev, tokens, newTokens), nil
}

func (b *Bot) ContextStats() string {
	count, tokens, hasSummary := b.ctxMgr.Stats()
	summary := "无"
	if hasSummary {
		summary = "有"
	}
	return fmt.Sprintf("消息: %d 条, 约 %d tokens, 摘要: %s", count, tokens, summary)
}

func (b *Bot) TokenUsage() (prompt, completion int) {
	return b.ag.TokenUsage()
}

func (b *Bot) ContextTokens() int {
	return b.ag.ContextTokens()
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
