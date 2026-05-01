package agent

import (
	"context"

	"primusbot/bot/tools"
	"primusbot/bot/types"
	"primusbot/ctxmgr"
	"primusbot/llm"
)

type Agent struct {
	ctx                  context.Context
	ctxMgr               *ctxmgr.Manager
	llmClient            llm.LLM
	toolRegistry         *tools.Registry
	confirmFn            types.ConfirmFunc
	phaseFn              types.PhaseFunc
	lastReasoningContent string
	maxIterations        int
	currentStep          int
	finished             bool
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
		maxIterations: 10,
	}
}

func (a *Agent) SetConfirmFn(fn types.ConfirmFunc) { a.confirmFn = fn }
func (a *Agent) SetPhaseFn(fn types.PhaseFunc)     { a.phaseFn = fn }

func (a *Agent) Reset() {
	a.currentStep = 0
	a.finished = false
	a.lastReasoningContent = ""
}
