package toolregistry

import (
	"context"
	"fmt"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// handlerResult carries one handler outcome across the dispatch goroutine
// boundary. panicked distinguishes a recovered panic from an ordinary error.
type handlerResult struct {
	out      string
	err      error
	panicked bool
	panicVal any
}

// Dispatch executes the tool call described by block and returns the paired
// tool_result ContentBlock, with ToolUseID set from the tool_use block's ID.
// It never returns a Go error and never panics — every failure mode is
// surfaced to the model as an is_error tool_result (see the package doc,
// "Dispatch returns a ContentBlock, never a Go error"):
//
//   - block is not a tool_use block (caller bug, but still reported in-band)
//   - unknown tool name
//   - policy denial (the Policy error text is the result content)
//   - handler error
//   - handler panic (recovered)
//   - timeout (the result names the timeout duration)
//   - context cancellation
//
// The handler runs in its own goroutine under a context that carries the
// per-tool deadline (Tool.Timeout, else Options.DefaultTimeout, else none).
// On deadline or cancellation Dispatch returns immediately and the handler
// goroutine is abandoned; see the package doc, "Timeouts". A handler result
// that arrives before Dispatch observes the deadline is used as-is.
func (r *Registry) Dispatch(ctx context.Context, block llmclient.ContentBlock) llmclient.ContentBlock {
	if block.Type != llmclient.BlockToolUse {
		return llmclient.ToolResultBlock(block.ID,
			fmt.Sprintf("toolregistry: Dispatch requires a tool_use block, got %q", block.Type), true)
	}

	tool, ok := r.Get(block.Name)
	if !ok {
		return llmclient.ToolResultBlock(block.ID,
			fmt.Sprintf("unknown tool %q", block.Name), true)
	}

	if r.opts.Policy != nil {
		if err := r.opts.Policy(block.Name, block.Input); err != nil {
			return llmclient.ToolResultBlock(block.ID,
				fmt.Sprintf("tool %q denied by policy: %v", block.Name, err), true)
		}
	}

	timeout := tool.Timeout
	if timeout == 0 {
		timeout = r.opts.DefaultTimeout
	}
	dctx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		dctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Buffered so an abandoned handler's send never blocks: a handler that
	// eventually returns after a timeout does not leak its goroutine.
	resCh := make(chan handlerResult, 1)
	go func() {
		defer func() {
			if v := recover(); v != nil {
				resCh <- handlerResult{panicked: true, panicVal: v}
			}
		}()
		out, err := tool.Handler(dctx, block.Input)
		resCh <- handlerResult{out: out, err: err}
	}()

	timedOut := func() bool {
		return timeout > 0 && dctx.Err() == context.DeadlineExceeded
	}

	select {
	case res := <-resCh:
		switch {
		case res.panicked:
			return llmclient.ToolResultBlock(block.ID,
				fmt.Sprintf("tool %q panicked: %v", block.Name, res.panicVal), true)
		case res.err != nil:
			// A cooperative handler that returns ctx.Err() on deadline is
			// reported as a timeout, not a generic failure.
			if timedOut() {
				return llmclient.ToolResultBlock(block.ID,
					fmt.Sprintf("tool %q timed out after %v", block.Name, timeout), true)
			}
			return llmclient.ToolResultBlock(block.ID,
				fmt.Sprintf("tool %q failed: %v", block.Name, res.err), true)
		default:
			return llmclient.ToolResultBlock(block.ID, res.out, false)
		}
	case <-dctx.Done():
		if timedOut() {
			return llmclient.ToolResultBlock(block.ID,
				fmt.Sprintf("tool %q timed out after %v", block.Name, timeout), true)
		}
		return llmclient.ToolResultBlock(block.ID,
			fmt.Sprintf("tool %q canceled: %v", block.Name, dctx.Err()), true)
	}
}

// DispatchAll dispatches every tool_use block in blocks — other block types
// (text, tool_result, unknown) are skipped — and returns the tool_result
// blocks in the same order, ready to wrap in a user Message for the next
// turn. It returns nil when blocks contains no tool_use blocks.
//
// Dispatch is sequential by design: tools have side effects, and the model
// emitted the calls in an order it may depend on. See the package doc,
// "Sequential dispatch", for the parallel escape hatch.
func (r *Registry) DispatchAll(ctx context.Context, blocks []llmclient.ContentBlock) []llmclient.ContentBlock {
	var results []llmclient.ContentBlock
	for _, b := range blocks {
		if b.Type != llmclient.BlockToolUse {
			continue
		}
		results = append(results, r.Dispatch(ctx, b))
	}
	return results
}
