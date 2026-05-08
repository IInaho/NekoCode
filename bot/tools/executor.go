// executor.go — 工具执行器：并行/串行调度、危险分级检查、用户确认。
package tools

import (
	"context"
	"fmt"
	"sync"
)

type Executor struct {
	registry   *Registry
	confirmFn  ConfirmFunc
	phaseFn    PhaseFunc
	maxWorkers int
}

func NewExecutor(r *Registry) *Executor {
	return &Executor{
		registry:   r,
		maxWorkers: 10,
	}
}

func (e *Executor) SetConfirmFn(fn ConfirmFunc) { e.confirmFn = fn }
func (e *Executor) SetPhaseFn(fn PhaseFunc)     { e.phaseFn = fn }

// ExecuteBatch partitions tools into read-only (parallel) and mutable (sequential)
// groups, then runs read-only concurrently first, then mutable in order.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCallItem) []ToolCallResult {
	if len(calls) == 0 {
		return nil
	}
	ro, mw := e.partition(calls)
	results := make([]ToolCallResult, 0, len(calls))
	if len(ro) > 0 {
		results = append(results, e.runParallel(ctx, ro)...)
	}
	if len(mw) > 0 {
		results = append(results, e.runSequential(ctx, mw)...)
	}
	return results
}

func (e *Executor) partition(calls []ToolCallItem) (readOnly, mutable []ToolCallItem) {
	for _, c := range calls {
		t, err := e.registry.Get(c.Name)
		if err != nil || t.ExecutionMode(c.Args) == ModeSequential {
			mutable = append(mutable, c)
		} else {
			readOnly = append(readOnly, c)
		}
	}
	return
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
		select {
		case <-ctx.Done():
			results[i] = ToolCallResult{ID: call.ID, Name: call.Name, Error: ctx.Err().Error()}
			continue
		default:
		}
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
	if level == LevelForbidden {
		return ToolCallResult{
			ID: tc.ID, Name: tc.Name,
			Error: fmt.Sprintf("operation rejected: %s is forbidden", tc.Name),
		}
	}

	if e.phaseFn != nil {
		e.phaseFn(PhaseRunning + " " + tc.Name)
	}
	if level >= LevelWrite && e.confirmFn != nil {
		if !e.confirmFn(ConfirmRequest{
			ToolName: tc.Name, Args: tc.Args, Level: level,
			Response: make(chan bool, 1),
		}) {
			return ToolCallResult{
				ID: tc.ID, Name: tc.Name,
				Error: "operation cancelled by user",
			}
		}
	}

	output, err := tool.Execute(ctx, tc.Args)
	if err != nil {
		return ToolCallResult{ID: tc.ID, Name: tc.Name, Error: err.Error()}
	}
	return ToolCallResult{ID: tc.ID, Name: tc.Name, Output: output}
}
