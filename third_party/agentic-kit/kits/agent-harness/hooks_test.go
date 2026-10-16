package agentharness_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

func TestAllThreeHooksFire(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "alpha-result"))

	var (
		preModelCalls   int32
		preDispatch     int32
		postToolResults int32
	)

	var gotFirstPreModelTurn int = -1
	var gotToolCallName string
	var gotPostTurn int

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PreModelCall: func(_ context.Context, v agentharness.PreModelCallView) error {
				if atomic.AddInt32(&preModelCalls, 1) == 1 {
					gotFirstPreModelTurn = v.Turn
				}
				return nil
			},
			PreDispatch: func(_ context.Context, v agentharness.ToolCallView) (bool, string, error) {
				atomic.AddInt32(&preDispatch, 1)
				gotToolCallName = v.ToolName
				return true, "", nil
			},
			PostToolResult: func(_ context.Context, v agentharness.ToolResultView, result llmclient.ContentBlock) (llmclient.ContentBlock, error) {
				atomic.AddInt32(&postToolResults, 1)
				gotPostTurn = v.Turn
				return result, nil
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", res.Stop)
	}

	if n := atomic.LoadInt32(&preModelCalls); n != 2 {
		t.Errorf("PreModelCall called %d times, want 2 (one per model call)", n)
	}
	if n := atomic.LoadInt32(&preDispatch); n != 1 {
		t.Errorf("PreDispatch called %d times, want 1", n)
	}
	if n := atomic.LoadInt32(&postToolResults); n != 1 {
		t.Errorf("PostToolResult called %d times, want 1", n)
	}
	if gotFirstPreModelTurn != 0 {
		t.Errorf("PreModelCall Turn = %d, want 0 on first model call", gotFirstPreModelTurn)
	}
	if gotToolCallName != "alpha" {
		t.Errorf("PreDispatch ToolName = %q, want %q", gotToolCallName, "alpha")
	}
	if gotPostTurn != 0 {
		t.Errorf("PostToolResult Turn = %d, want 0", gotPostTurn)
	}
}

func TestHookPreModelCallError(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)
	hookErr := errors.New("pre_model_call hook failure")

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 3,
		Hooks: &agentharness.HookConfig{
			PreModelCall: func(_ context.Context, _ agentharness.PreModelCallView) error {
				return hookErr
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, runErr := h.Run(context.Background(), "go")
	if runErr == nil {
		t.Fatal("Run: error = nil, want hook error")
	}
	if !errors.Is(runErr, hookErr) {
		t.Errorf("Run error = %v, want to wrap %v", runErr, hookErr)
	}
	if res.Stop != agentharness.StopHook {
		t.Errorf("Stop = %q, want %q", res.Stop, agentharness.StopHook)
	}
	if mock.CallCount() != 0 {
		t.Errorf("model calls = %d, want 0 (hook fires before model call)", mock.CallCount())
	}
}

func TestHookPreDispatchError(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "alpha-result"))
	hookErr := errors.New("pre_dispatch hook failure")

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PreDispatch: func(_ context.Context, _ agentharness.ToolCallView) (bool, string, error) {
				return false, "", hookErr
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, runErr := h.Run(context.Background(), "go")
	if runErr == nil {
		t.Fatal("Run: error = nil, want hook error")
	}
	if !errors.Is(runErr, hookErr) {
		t.Errorf("Run error = %v, want to wrap %v", runErr, hookErr)
	}
	if res.Stop != agentharness.StopHook {
		t.Errorf("Stop = %q, want %q", res.Stop, agentharness.StopHook)
	}
}

func TestHookPostToolResultError(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "alpha-result"))
	hookErr := errors.New("post_tool_result hook failure")

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PostToolResult: func(_ context.Context, _ agentharness.ToolResultView, result llmclient.ContentBlock) (llmclient.ContentBlock, error) {
				return llmclient.ContentBlock{}, hookErr
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, runErr := h.Run(context.Background(), "go")
	if runErr == nil {
		t.Fatal("Run: error = nil, want hook error")
	}
	if !errors.Is(runErr, hookErr) {
		t.Errorf("Run error = %v, want to wrap %v", runErr, hookErr)
	}
	if res.Stop != agentharness.StopHook {
		t.Errorf("Stop = %q, want %q", res.Stop, agentharness.StopHook)
	}
}

func TestPreDispatchDenyAllowsContinue(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done after denial", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "alpha-result"))

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PreDispatch: func(_ context.Context, v agentharness.ToolCallView) (bool, string, error) {
				// Deny all calls.
				return false, "not permitted in this context", nil
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed (denial lets loop continue)", res.Stop)
	}
	if res.FinalText != "done after denial" {
		t.Errorf("FinalText = %q", res.FinalText)
	}

	// The denial tool_result should be in the transcript — it's an is_error block.
	// Messages: [user, assistant(tool_use), user(tool_result=denial), assistant(done)]
	if len(res.Messages) < 4 {
		t.Fatalf("Messages len = %d, want >= 4", len(res.Messages))
	}
	// The tool result message (index 2) should have an is_error block.
	toolResultMsg := res.Messages[2]
	if len(toolResultMsg.Content) == 0 {
		t.Fatalf("tool result message has no content")
	}
	b := toolResultMsg.Content[0]
	if !b.IsError {
		t.Errorf("denial block IsError = false, want true")
	}
	if !strings.Contains(b.Content, "not permitted") {
		t.Errorf("denial block content = %q, want it to mention the reason", b.Content)
	}
}

func TestPostToolResultReplacement(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "original-result"))

	replacement := llmclient.ToolResultBlock("tu-1", "replaced-result", false)

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PostToolResult: func(_ context.Context, _ agentharness.ToolResultView, _ llmclient.ContentBlock) (llmclient.ContentBlock, error) {
				return replacement, nil
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := h.Run(context.Background(), "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Fatalf("Stop = %q, want completed", res.Stop)
	}

	// The tool_result in the transcript must be the replacement.
	// Messages: [user, assistant(tool_use), user(tool_result), assistant]
	if len(res.Messages) < 4 {
		t.Fatalf("Messages len = %d, want >= 4", len(res.Messages))
	}
	toolResultContent := res.Messages[2].Content[0]
	if toolResultContent.Content != "replaced-result" {
		t.Errorf("tool_result content = %q, want %q", toolResultContent.Content, "replaced-result")
	}
	// Also verify the second model call SAW the replacement.
	calls := mock.Calls()
	if len(calls) < 2 {
		t.Fatalf("mock calls = %d, want >= 2", len(calls))
	}
	call2msgs := calls[1].Request.Messages
	var found bool
	for _, m := range call2msgs {
		for _, b := range m.Content {
			if b.Content == "replaced-result" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("second model call did not see the replaced result")
	}
}

func TestHooksOnStreamPath(t *testing.T) {
	in := json.RawMessage(`{}`)
	mock := llmclient.NewMockClient(
		toolUseEntry("tu-1", "alpha", in, llmclient.Usage{}),
		textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	reg := newRegistry(t, echoTool("alpha", "alpha-result"))

	var preModelCount int32

	h, err := agentharness.New(agentharness.Config{
		MaxTurns: 5,
		Hooks: &agentharness.HookConfig{
			PreModelCall: func(_ context.Context, _ agentharness.PreModelCallView) error {
				atomic.AddInt32(&preModelCount, 1)
				return nil
			},
		},
	}, agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch, err := h.RunStream(context.Background(), "go")
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var finalResult *agentharness.HarnessEvent
	for e := range ch {
		if e.Kind == agentharness.HarnessEventResult {
			ev := e
			finalResult = &ev
		}
	}
	if finalResult == nil {
		t.Fatal("no terminal event received")
	}
	if finalResult.Err != nil {
		t.Fatalf("terminal event error = %v", finalResult.Err)
	}
	if n := atomic.LoadInt32(&preModelCount); n != 2 {
		t.Errorf("PreModelCall called %d times on stream path, want 2", n)
	}
}
