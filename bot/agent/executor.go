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

// ExecuteBatch partitions tools into read-only (parallel) and mutable (sequential)
// groups, then runs read-only concurrently first, then mutable in order.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []tools.ToolCallItem) []tools.ToolCallResult {
	if len(calls) == 0 {
		return nil
	}
	ro, mw := e.partition(calls)
	results := make([]tools.ToolCallResult, 0, len(calls))
	if len(ro) > 0 {
		results = append(results, e.runParallel(ctx, ro)...)
	}
	if len(mw) > 0 {
		results = append(results, e.runSequential(ctx, mw)...)
	}
	return results
}

func (e *Executor) partition(calls []tools.ToolCallItem) (readOnly, mutable []tools.ToolCallItem) {
	for _, c := range calls {
		t, err := e.registry.Get(c.Name)
		if err != nil || t.ExecutionMode(c.Args) == tools.ModeSequential {
			mutable = append(mutable, c)
		} else {
			readOnly = append(readOnly, c)
		}
	}
	return
}

func (e *Executor) runSequential(ctx context.Context, calls []tools.ToolCallItem) []tools.ToolCallResult {
	results := make([]tools.ToolCallResult, len(calls))
	for i, c := range calls {
		results[i] = e.executeOne(ctx, c)
	}
	return results
}

func (e *Executor) runParallel(ctx context.Context, calls []tools.ToolCallItem) []tools.ToolCallResult {
	results := make([]tools.ToolCallResult, len(calls))
	sem := make(chan struct{}, e.maxWorkers)
	var wg sync.WaitGroup

	for i, call := range calls {
		select {
		case <-ctx.Done():
			results[i] = tools.ToolCallResult{ID: call.ID, Name: call.Name, Error: ctx.Err().Error()}
			continue
		default:
		}
		wg.Add(1)
		go func(idx int, tc tools.ToolCallItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = e.executeOne(ctx, tc)
		}(i, call)
	}
	wg.Wait()
	return results
}

func (e *Executor) executeOne(ctx context.Context, tc tools.ToolCallItem) tools.ToolCallResult {
	tool, err := e.registry.Get(tc.Name)
	if err != nil {
		return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}

	level := tool.DangerLevel(tc.Args)
	if level == tools.LevelForbidden {
		return tools.ToolCallResult{
			ID: tc.ID, Name: tc.Name,
			Error: fmt.Sprintf("操作被拒绝: %s 属于禁止操作", tc.Name),
		}
	}

	if e.phaseFn != nil {
		e.phaseFn(types.PhaseRunning + " " + tc.Name)
	}
	if level >= tools.LevelWrite && e.confirmFn != nil {
		if !e.confirmFn(types.ConfirmRequest{
			ToolName: tc.Name, Args: tc.Args, Level: level,
			Response: make(chan bool, 1),
		}) {
			return tools.ToolCallResult{
				ID: tc.ID, Name: tc.Name,
				Error: "操作被用户取消",
			}
		}
	}


	output, err := tool.Execute(ctx, tc.Args)
	if err != nil {
		return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}
	return tools.ToolCallResult{ID: tc.ID, Name: tc.Name, Output: output}
}
