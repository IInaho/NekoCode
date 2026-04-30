// Package bot 是应用的核心组装层。Bot 结构体聚合所有依赖
//（ctxmgr、agent、tools、llm、command、config），提供统一的公开 API。
// New() 完成依赖注入和生命周期管理。
package bot

import (
	"context"
	"fmt"

	_ "embed"
	"primusbot/bot/agent"
	"primusbot/bot/tools"
	"primusbot/ctxmgr"
	"primusbot/llm"
)

type Bot struct {
	Cfg       *Config
	CtxMgr    *ctxmgr.Manager
	CmdParser *Parser
	Agent     *agent.Agent
}

//go:embed prompt/system.md
var SystemPrompt string

func New() *Bot {
	ctx := context.Background()

	cfg, _ := LoadConfig()
	ctxMgr := ctxmgr.New(SystemPrompt)

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

	b := &Bot{
		Cfg:       cfg,
		CtxMgr:    ctxMgr,
		CmdParser: NewParser(),
		Agent:     agent.New(ctx, ctxMgr, llmClient, toolRegistry),
	}

	callbacks := &CommandCallbacks{
		ClearHistory:   ctxMgr.Clear,
		GetConfig:      func() string { return fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model) },
		ForceSummarize: func() (string, error) { return b.ForceSummarize() },
		ContextStats:   func() string { return b.ContextStats() },
	}
	RegisterDefaultCommands(b.CmdParser, callbacks)

	return b
}

func (b *Bot) ExecuteCommand(input string) (string, bool) {
	cmd := b.CmdParser.Parse(input)
	if cmd.Name == "" {
		return "", false
	}
	return b.CmdParser.Execute(cmd)
}

func (b *Bot) RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string)) (string, error) {
	result := b.Agent.Run(input, onStep)
	b.SummarizeIfNeeded()
	return result.FinalOutput, result.Error
}

func (b *Bot) SetConfirmFn(fn agent.ConfirmFunc) {
	b.Agent.SetConfirmFn(fn)
}

func (b *Bot) SummarizeIfNeeded() {
	if b.CtxMgr.NeedsSummarization() {
		_ = b.CtxMgr.Summarize()
	}
}

func (b *Bot) ForceSummarize() (string, error) {
	count, tokens, hadSummary := b.CtxMgr.Stats()
	if count <= 2 {
		return "对话太短，无需压缩", nil
	}
	if err := b.CtxMgr.Summarize(); err != nil {
		return "", err
	}
	_, newTokens, _ := b.CtxMgr.Stats()
	prev := "已压缩"
	if hadSummary {
		prev = "已更新摘要"
	}
	return fmt.Sprintf("%s：%d 条消息 → %d tokens", prev, tokens, newTokens), nil
}

func (b *Bot) ContextStats() string {
	count, tokens, hasSummary := b.CtxMgr.Stats()
	summary := "无"
	if hasSummary {
		summary = "有"
	}
	return fmt.Sprintf("消息: %d 条, 约 %d tokens, 摘要: %s", count, tokens, summary)
}

func (b *Bot) TokenUsage() (int, int) {
	return b.CtxMgr.TokenUsage()
}

func (b *Bot) CommandNames() []string {
	return b.CmdParser.Commands()
}
