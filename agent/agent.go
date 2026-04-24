package agent

import (
	"context"
	"primusbot/llm"
)

type LLMClient interface {
	Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error)
}

type ChatManager interface {
	AddUserMessage(content string)
	AddAssistantMessage(content string)
	GetMessages() []llm.Message
	MessageCount() int
}

type Agent struct {
	ctx          context.Context
	chatManager  ChatManager
	llmClient    LLMClient
	cmdParser    interface{}
	toolRegistry *ToolRegistry
	memory       *Memory
	config       *Config

	maxIterations int
	currentStep   int
	finished      bool
}

func New(
	ctx context.Context,
	chatManager ChatManager,
	llmClient LLMClient,
	toolRegistry *ToolRegistry,
	opts ...Option,
) *Agent {
	cfg := NewConfig(opts...)
	return &Agent{
		ctx:          ctx,
		chatManager:  chatManager,
		llmClient:    llmClient,
		cmdParser:    nil,
		toolRegistry: toolRegistry,
		memory:       NewMemory(),
		config:       cfg,

		maxIterations: cfg.MaxIterations,
		currentStep:   0,
		finished:      false,
	}
}

func (a *Agent) Reset() {
	a.currentStep = 0
	a.finished = false
	a.memory.Clear()
}

func (a *Agent) IsFinished() bool {
	return a.finished
}

func (a *Agent) Step() int {
	return a.currentStep
}

func (a *Agent) GetMemory() *Memory {
	return a.memory
}

func (a *Agent) Perceive(input string) *PerceptionResult {
	return PerceiveInput(input)
}
