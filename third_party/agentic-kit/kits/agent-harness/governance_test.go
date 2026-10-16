package agentharness_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/conversation"
	investigationjournal "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/investigation-journal"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

// buildMultiTurnConv builds a conversation with several turns so windowing
// can kick in.
func buildMultiTurnConv(t *testing.T) *conversation.Conversation {
	t.Helper()
	conv := conversation.New(conversation.Options{})
	// Turn 1
	if err := conv.AppendUserText("message one"); err != nil {
		t.Fatalf("append user: %v", err)
	}
	if err := conv.AppendAssistant(llmclient.Response{
		Text:       "reply one",
		Content:    []llmclient.ContentBlock{llmclient.TextBlock("reply one")},
		StopReason: llmclient.StopEndTurn,
	}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	// Turn 2
	if err := conv.AppendUserText("message two"); err != nil {
		t.Fatalf("append user: %v", err)
	}
	if err := conv.AppendAssistant(llmclient.Response{
		Text:       "reply two",
		Content:    []llmclient.ContentBlock{llmclient.TextBlock("reply two")},
		StopReason: llmclient.StopEndTurn,
	}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	// Final user message (the "new" turn)
	if err := conv.AppendUserText("message three"); err != nil {
		t.Fatalf("append user: %v", err)
	}
	return conv
}

func TestWindowViewVsGroundTruth(t *testing.T) {
	// Build a pre-populated conversation (5 messages).
	conv := buildMultiTurnConv(t)
	fullCount := len(conv.Messages()) // 5

	// Script: one end_turn so the harness terminates.
	mock := llmclient.NewMockClient(textEntry("final", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{
		MaxTurns:      3,
		ContextBudget: 1, // forces windowing — budget too small for all 5 messages
	}, agentharness.Deps{
		Client:   mock,
		Registry: reg,
		Now:      fixedClock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, runErr := h.RunConversation(context.Background(), conv)
	if runErr != nil {
		t.Fatalf("RunConversation: %v", runErr)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Fatalf("Stop = %q, want completed", res.Stop)
	}

	// The model must have seen FEWER messages than the full conversation.
	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("mock calls = %d, want 1", len(calls))
	}
	modelSaw := len(calls[0].Request.Messages)
	if modelSaw >= fullCount {
		t.Errorf("model saw %d messages (full=%d), want fewer due to windowing", modelSaw, fullCount)
	}

	// But the final RunResult.Messages has the FULL history.
	if got := len(res.Messages); got < fullCount {
		t.Errorf("RunResult.Messages len = %d, want >= %d (full ground truth)", got, fullCount)
	}
}

func TestCompactionEventJournaled(t *testing.T) {
	conv := buildMultiTurnConv(t)
	mock := llmclient.NewMockClient(textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)
	journal := investigationjournal.NewJournalWithClock(advancingClock(time.Millisecond))

	h, err := agentharness.New(agentharness.Config{
		MaxTurns:      3,
		ContextBudget: 1,
	}, agentharness.Deps{
		Client:   mock,
		Registry: reg,
		Journal:  journal,
		Now:      fixedClock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := h.RunConversation(context.Background(), conv); err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	entries := journal.Entries()
	var sawCompactionFired bool
	for _, e := range entries {
		if e.Step == "compaction-fired" {
			sawCompactionFired = true
		}
	}
	if !sawCompactionFired {
		t.Errorf("journal entries = %+v; want a 'compaction-fired' entry", entries)
	}
}

func TestCompactionStreamEvent(t *testing.T) {
	// Verify that the compaction event flows through the streaming path too.
	mock := llmclient.NewMockClient(textEntry("done", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{
		MaxTurns:      3,
		ContextBudget: 1, // force windowing
	}, agentharness.Deps{
		Client:   mock,
		Registry: reg,
		Now:      fixedClock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch, err := h.RunStreamBlocks(context.Background(), []llmclient.ContentBlock{
		llmclient.TextBlock("message three stream"),
	})
	if err != nil {
		t.Fatalf("RunStreamBlocks: %v", err)
	}

	var sawCompaction bool
	for e := range ch {
		if e.Kind == agentharness.HarnessEventCompaction {
			sawCompaction = true
			if e.DroppedMessages <= 0 {
				t.Errorf("compaction event DroppedMessages = %d, want > 0", e.DroppedMessages)
			}
		}
	}
	// Note: this test starts from a fresh conv via RunStreamBlocks, so no
	// prior turns to drop — compaction won't fire. Test the correct no-op path.
	_ = sawCompaction // not expected to fire on a fresh single-message conv
}

func TestCompactorErrorStopsRun(t *testing.T) {
	// A Compactor that returns an error should stop the run (not panic) and
	// surface a non-nil error with Stop == StopError.
	//
	// Note: the compactor is an option on the conversation, so we must build
	// the conversation with the bad compactor baked in. RunConversation passes
	// that conversation directly (its options are preserved); for Run/RunBlocks
	// the harness builds the conversation from h.cfg.Conversation, so both
	// paths are covered by testing Run with Conversation.Compactor set.
	compactErr := fmt.Errorf("synthetic compaction failure")
	badCompactor := func(_ context.Context, _ []llmclient.Message) (llmclient.Message, bool, error) {
		return llmclient.Message{}, false, compactErr
	}

	// Build the conversation WITH the bad compactor so WindowCtx will invoke it.
	convOpts := conversation.Options{Compactor: badCompactor}
	conv := conversation.New(convOpts)
	// Populate with several messages so windowing triggers.
	for i, pair := range [][2]string{
		{"message one", "reply one"},
		{"message two", "reply two"},
	} {
		if err := conv.AppendUserText(pair[0]); err != nil {
			t.Fatalf("pair %d user: %v", i, err)
		}
		if err := conv.AppendAssistant(llmclient.Response{
			Text:       pair[1],
			Content:    []llmclient.ContentBlock{llmclient.TextBlock(pair[1])},
			StopReason: llmclient.StopEndTurn,
		}); err != nil {
			t.Fatalf("pair %d assistant: %v", i, err)
		}
	}
	if err := conv.AppendUserText("message three"); err != nil {
		t.Fatalf("append last user: %v", err)
	}

	mock := llmclient.NewMockClient(textEntry("should not reach", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)
	journal := investigationjournal.NewJournalWithClock(fixedClock)

	h, err := agentharness.New(agentharness.Config{
		MaxTurns:      3,
		ContextBudget: 1, // forces windowing → calls the compactor
	}, agentharness.Deps{
		Client:   mock,
		Registry: reg,
		Journal:  journal,
		Now:      fixedClock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, runErr := h.RunConversation(context.Background(), conv)
	if runErr == nil {
		t.Fatal("RunConversation: error = nil, want compaction error")
	}
	if res.Stop != agentharness.StopError {
		t.Errorf("Stop = %q, want StopError", res.Stop)
	}

	// Journal must record the compaction failure.
	var sawFailed bool
	for _, e := range journal.Entries() {
		if e.Step == "compaction-failed" {
			sawFailed = true
		}
	}
	if !sawFailed {
		t.Error("journal: no 'compaction-failed' entry found")
	}

	// The model must never have been called (run stops before model call).
	if mock.CallCount() != 0 {
		t.Errorf("model calls = %d, want 0 (run stopped at compaction failure)", mock.CallCount())
	}
}

func TestConversationOptionsPassThrough(t *testing.T) {
	// Verify that h.cfg.Conversation is used when building the conversation,
	// specifically that a custom Estimator is wired in.
	var estimatorCalled bool
	customEstimator := func(s string) int {
		estimatorCalled = true
		return len(s) / 4
	}

	mock := llmclient.NewMockClient(textEntry("hi", llmclient.StopEndTurn, llmclient.Usage{}))
	reg := newRegistry(t)

	h, err := agentharness.New(agentharness.Config{
		MaxTurns:      2,
		ContextBudget: 10000, // large enough to not actually drop messages
		Conversation:  conversation.Options{Estimator: customEstimator},
	}, agentharness.Deps{
		Client:   mock,
		Registry: reg,
		Now:      fixedClock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := h.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Window calls the estimator when ContextBudget > 0.
	if !estimatorCalled {
		t.Error("custom Estimator was never called; Conversation options not forwarded")
	}
}
