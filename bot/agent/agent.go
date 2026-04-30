// Package agent 实现 Agent 循环：感知输入、LLM 推理、工具执行、状态反馈。
// Agent 通过 ctxmgr.Manager 获取上下文，通过 tools.Registry 调用工具，
// 通过 llm.LLM 与语言模型交互。循环上限由 maxIterations 控制。
package agent

import (
	"context"
	"primusbot/bot/tools"
	"primusbot/ctxmgr"
	"primusbot/llm"
)

type Agent struct {
	ctx           context.Context
	ctxMgr        *ctxmgr.Manager
	llmClient     llm.LLM
	toolRegistry  *tools.Registry
	memory        *Memory
	confirmFn     ConfirmFunc
	maxIterations int
	currentStep   int
	finished      bool
}

func New(
	ctx context.Context,
	ctxMgr *ctxmgr.Manager,
	llmClient llm.LLM,
	toolRegistry *tools.Registry,
) *Agent {
	return &Agent{
		ctx:           ctx,
		ctxMgr:        ctxMgr,
		llmClient:     llmClient,
		toolRegistry:  toolRegistry,
		memory:        NewMemory(),
		confirmFn:     nil,
		maxIterations: 10,
		currentStep:   0,
		finished:      false,
	}
}

func (a *Agent) SetConfirmFn(fn ConfirmFunc) {
	a.confirmFn = fn
}

func (a *Agent) Reset() {
	a.currentStep = 0
	a.finished = false
	a.memory.Clear()
}
