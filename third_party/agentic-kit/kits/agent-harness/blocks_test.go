package agentharness_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

func TestRunBlocksImageFlowsThrough(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("ok", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{MaxTurns: 2},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// An image block plus a text description.
	blocks := []llmclient.ContentBlock{
		llmclient.ImageBlock("image/png", "base64datahere"),
		llmclient.TextBlock("what is in this image?"),
	}

	res, err := h.RunBlocks(context.Background(), blocks)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", res.Stop)
	}

	// The first model call should contain the image block.
	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("mock calls = %d, want 1", len(calls))
	}
	msgs := calls[0].Request.Messages
	if len(msgs) == 0 {
		t.Fatal("request has no messages")
	}
	var foundImage bool
	for _, b := range msgs[0].Content {
		if b.Type == llmclient.BlockImage {
			foundImage = true
			if b.MediaType != "image/png" || b.Data != "base64datahere" {
				t.Errorf("image block = %+v, want MediaType=image/png Data=base64datahere", b)
			}
		}
	}
	if !foundImage {
		t.Errorf("no image block in first message content: %+v", msgs[0].Content)
	}
}

func TestRunBlocksEmptyError(t *testing.T) {
	mock := llmclient.NewMockClient()
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{MaxTurns: 2},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("nil blocks", func(t *testing.T) {
		res, err := h.RunBlocks(context.Background(), nil)
		if err == nil {
			t.Fatal("RunBlocks(nil): error = nil, want error")
		}
		if res.Stop != agentharness.StopError {
			t.Errorf("Stop = %q, want %q", res.Stop, agentharness.StopError)
		}
	})

	t.Run("empty blocks", func(t *testing.T) {
		res, err := h.RunBlocks(context.Background(), []llmclient.ContentBlock{})
		if err == nil {
			t.Fatal("RunBlocks([]): error = nil, want error")
		}
		if res.Stop != agentharness.StopError {
			t.Errorf("Stop = %q, want %q", res.Stop, agentharness.StopError)
		}
	})
}

func TestRunStreamBlocksBasic(t *testing.T) {
	mock := llmclient.NewMockClient(textEntry("stream ok", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{MaxTurns: 2},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	blocks := []llmclient.ContentBlock{llmclient.TextBlock("hello from blocks")}
	ch, err := h.RunStreamBlocks(context.Background(), blocks)
	if err != nil {
		t.Fatalf("RunStreamBlocks: %v", err)
	}

	var resultEvent *agentharness.HarnessEvent
	for e := range ch {
		if e.Kind == agentharness.HarnessEventResult {
			ev := e
			resultEvent = &ev
		}
	}

	if resultEvent == nil {
		t.Fatal("no terminal event received")
	}
	if resultEvent.Err != nil {
		t.Fatalf("terminal event error = %v", resultEvent.Err)
	}
	if resultEvent.Result == nil {
		t.Fatal("terminal event Result is nil")
	}
	if resultEvent.Result.Stop != agentharness.StopCompleted {
		t.Errorf("Stop = %q, want completed", resultEvent.Result.Stop)
	}
	if resultEvent.Result.FinalText != "stream ok" {
		t.Errorf("FinalText = %q, want %q", resultEvent.Result.FinalText, "stream ok")
	}
}

func TestRunStreamBlocksEmptyError(t *testing.T) {
	mock := llmclient.NewMockClient()
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{MaxTurns: 2},
		agentharness.Deps{Client: mock, Registry: reg, Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = h.RunStreamBlocks(context.Background(), nil)
	if err == nil {
		t.Fatal("RunStreamBlocks(nil): error = nil, want error")
	}
}
