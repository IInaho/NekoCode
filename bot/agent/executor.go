// 工具执行器：单工具执行 + 批量并行/串行调度。
// ExecutionMode 由 Tool 接口声明；批次中有任何 Sequential 工具则整批串行。
package agent

import (
	"context"
	"fmt"
	"sync"

	"primusbot/bot/tools"
	"primusbot/bot/types"
)

type ActionResult struct {
	Thought     string
	Action      ActionType
	Output      string
	Error       string
	IsFinal     bool
	ShouldRetry bool
}

type ToolCallResult struct {
	ID     string
	Name   string
	Output string
	Error  string
}

type Executor struct {
	registry   *tools.Registry
	confirmFn  types.ConfirmFunc
	phaseFn    types.PhaseFunc
	maxWorkers int
}

func NewExecutor(r *tools.Registry) *Executor {
	return &Executor{
		registry:   r,
		maxWorkers: 10,
	}
}

func (e *Executor) SetConfirmFn(fn types.ConfirmFunc) { e.confirmFn = fn }
func (e *Executor) SetPhaseFn(fn types.PhaseFunc)     { e.phaseFn = fn }

// ExecuteBatch runs all tool calls. If any tool has ModeSequential, the whole
// batch runs serially; otherwise they run concurrently via a worker pool.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCallItem) []ToolCallResult {
	if len(calls) == 0 {
		return nil
	}
	if e.needsSequential(calls) {
		return e.runSequential(ctx, calls)
	}
	return e.runParallel(ctx, calls)
}

func (e *Executor) needsSequential(calls []ToolCallItem) bool {
	for _, c := range calls {
		t, err := e.registry.Get(c.Name)
		if err != nil {
			continue
		}
		if t.ExecutionMode(c.Args) == tools.ModeSequential {
			return true
		}
	}
	return false
}

func (e *Executor) runSequential(ctx context.Context, calls []ToolCallItem) []ToolCallResult {
	results := make([]ToolCallResult, len(calls))
	for i, c := range calls {
		results[i] = e.executeOne(ctx, c)
	}
	return results
}

func (e *Executor) runParallel(ctx context.Context, calls []ToolCallItem) []ToolCallResult {
	results := make([]ToolCallResult, len(calls))
	sem := make(chan struct{}, e.maxWorkers)
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc ToolCallItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = e.executeOne(ctx, tc)
		}(i, call)
	}
	wg.Wait()
	return results
}

func (e *Executor) executeOne(ctx context.Context, tc ToolCallItem) ToolCallResult {
	tool, err := e.registry.Get(tc.Name)
	if err != nil {
		return ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}

	level := tool.DangerLevel(tc.Args)
	if level == tools.LevelForbidden {
		return ToolCallResult{
			ID: tc.ID, Name: tc.Name,
			Error: fmt.Sprintf("操作被拒绝: %s 属于禁止操作", tc.Name),
		}
	}

	if level >= tools.LevelWrite && e.confirmFn != nil {
		if !e.confirmFn(types.ConfirmRequest{
			ToolName: tc.Name, Args: tc.Args, Level: level,
			Response: make(chan bool, 1),
		}) {
			return ToolCallResult{
				ID: tc.ID, Name: tc.Name,
				Error: "操作被用户取消",
			}
		}
	}

	if e.phaseFn != nil {
		e.phaseFn("Running " + tc.Name)
	}

	output, err := tool.Execute(ctx, tc.Args)
	if err != nil {
		return ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}
	return ToolCallResult{ID: tc.ID, Name: tc.Name, Output: output}
}
