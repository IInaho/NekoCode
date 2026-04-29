package bot

import (
	"context"
	"fmt"
	"sync"

	_ "embed"
	"primusbot/agent"
	"primusbot/chat"
	"primusbot/command"
	"primusbot/config"
	"primusbot/llm"
)

type Bot struct {
	Cfg         *config.Config
	Ctx         context.Context
	Cancel      context.CancelFunc
	ChatMgr     *chat.ChatManager
	CmdParser   *command.Parser
	Agent       *agent.Agent
	streamMu    sync.Mutex
	streamCancel context.CancelFunc
}

//go:embed prompt/system.md
var SystemPrompt string

func New() *Bot {
	ctx, cancel := context.WithCancel(context.Background())

	cfg, _ := config.LoadConfig()
	chatMgr := chat.NewChatManager(SystemPrompt)

	var llmClient llm.LLM
	switch cfg.Provider {
	case "anthropic":
		llmClient = llm.NewAnthropic(cfg.APIKey, cfg.Model)
	case "glm":
		llmClient = llm.NewGLM(cfg.APIKey, cfg.BaseURL, cfg.Model)
	default:
		llmClient = llm.NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.Model)
	}
	chatMgr.SetLLM(llmClient)

	cmdParser := command.NewParser()
	callbacks := &command.CommandCallbacks{
		ClearHistory: chatMgr.ClearHistory,
		Quit:         cancel,
		GetConfig:    func() string { return fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model) },
		SetModel:     func(model string) { cfg.Model = model },
	}
	command.RegisterDefaultCommands(cmdParser, callbacks)

	toolRegistry := agent.NewToolRegistry()
	agent.RegisterDefaultTools(toolRegistry)

	botAgent := agent.New(ctx, chatMgr, llmClient, toolRegistry)

	return &Bot{
		Cfg:       cfg,
		Ctx:       ctx,
		Cancel:    cancel,
		ChatMgr:   chatMgr,
		CmdParser: cmdParser,
		Agent:     botAgent,
	}
}

func (b *Bot) Chat(input string, onToken func(string, string), onDone func()) error {
	streamCtx, streamCancel := context.WithCancel(b.Ctx)
	b.streamMu.Lock()
	b.streamCancel = streamCancel
	b.streamMu.Unlock()

	return b.ChatMgr.ChatStream(streamCtx, input, onToken, onDone)
}

func (b *Bot) CancelStream() {
	b.streamMu.Lock()
	if b.streamCancel != nil {
		b.streamCancel()
		b.streamCancel = nil
	}
	b.streamMu.Unlock()
}

func (b *Bot) ExecuteCommand(input string) (string, bool) {
	cmd := b.CmdParser.Parse(input)
	if cmd.Name == "" {
		return "", false
	}
	resp, handle := b.CmdParser.Execute(cmd)
	if cmd.Name == "quit" || cmd.Name == "exit" {
		b.Cancel()
	}
	return resp, handle
}

func (b *Bot) RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string)) (string, error) {
	result := b.Agent.Run(input, onStep)
	return result.FinalOutput, result.Error
}

func (b *Bot) CommandNames() []string {
	return b.CmdParser.Commands()
}
