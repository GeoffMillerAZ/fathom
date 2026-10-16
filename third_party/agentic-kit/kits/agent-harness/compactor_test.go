package agentharness_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	evidencecompactor "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/evidence-compactor"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	jsonllog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/persistence/jsonl-log"
	agentharness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/agent-harness"
)

func newTestCompactor(t *testing.T, journal *bytes.Buffer) (*evidencecompactor.Compactor, *llmclient.MockClient) {
	t.Helper()
	summarizer := llmclient.NewMockClient(
		textEntry("COMPACTION SUMMARY:\n- earlier turns exchanged messages one and two", llmclient.StopEndTurn, llmclient.Usage{}),
	)
	ec, err := evidencecompactor.New(evidencecompactor.Config{}, summarizer, jsonllog.NewWriter(journal), fixedClock)
	if err != nil {
		t.Fatalf("evidencecompactor.New: %v", err)
	}
	return ec, summarizer
}

func TestWithEvidenceCompactorValidation(t *testing.T) {
	ec, _ := newTestCompactor(t, &bytes.Buffer{})

	if _, err := agentharness.WithEvidenceCompactor(agentharness.Config{MaxTurns: 1, ContextBudget: 10}, nil); err == nil || !strings.Contains(err.Error(), "non-nil") {
		t.Errorf("nil compactor: err = %v, want a non-nil error", err)
	}
	if _, err := agentharness.WithEvidenceCompactor(agentharness.Config{MaxTurns: 1}, ec); err == nil || !strings.Contains(err.Error(), "ContextBudget") {
		t.Errorf("no ContextBudget: err = %v, want an error naming ContextBudget (a compactor that can never run is the drift this fixes)", err)
	}
	cfg, err := agentharness.WithEvidenceCompactor(agentharness.Config{MaxTurns: 1, ContextBudget: 10}, ec)
	if err != nil {
		t.Fatalf("WithEvidenceCompactor: %v", err)
	}
	if cfg.Conversation.Compactor == nil {
		t.Error("Conversation.Compactor not installed")
	}
}

// TestWithEvidenceCompactorWired proves the option installs the REAL
// compactor into the harness loop: when windowing drops messages, the
// dropped ground truth lands in the journal BEFORE the summary call, and the
// model sees a provenance-marked summary in place of the dropped turns.
func TestWithEvidenceCompactorWired(t *testing.T) {
	var journal bytes.Buffer
	ec, summarizer := newTestCompactor(t, &journal)

	cfg, err := agentharness.WithEvidenceCompactor(agentharness.Config{
		MaxTurns:      3,
		ContextBudget: 1, // forces windowing
	}, ec)
	if err != nil {
		t.Fatalf("WithEvidenceCompactor: %v", err)
	}
	model := llmclient.NewMockClient(textEntry("final", llmclient.StopEndTurn, llmclient.Usage{}))
	h, err := agentharness.New(cfg, agentharness.Deps{Client: model, Registry: newRegistry(t), Now: fixedClock})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// The compactor lives on Config.Conversation, which the harness applies
	// to every conversation IT constructs (Run, RunBlocks, RunFromCheckpoint).
	// Resume a checkpoint of a five-message history so the harness builds the
	// conversation itself and windowing has clean cut points to drop.
	snap, err := buildMultiTurnConv(t).Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	res, err := h.RunFromCheckpoint(context.Background(), &agentharness.Checkpoint{Version: 1, ConvSnapshot: snap, TurnCursor: 2})
	if err != nil {
		t.Fatalf("RunFromCheckpoint: %v", err)
	}
	if res.Stop != agentharness.StopCompleted {
		t.Fatalf("Stop = %q, want completed", res.Stop)
	}

	if summarizer.CallCount() != 1 {
		t.Errorf("summary LLM calls = %d, want 1 (the compactor ran once)", summarizer.CallCount())
	}
	lines := strings.Split(strings.TrimSpace(journal.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"kind":"compaction-dropped"`) || !strings.Contains(lines[0], "message one") {
		t.Errorf("journal = %q, want one compaction-dropped record carrying the dropped ground truth verbatim", journal.String())
	}

	calls := model.Calls()
	if len(calls) != 1 {
		t.Fatalf("model calls = %d, want 1", len(calls))
	}
	var sawMarker bool
	for _, m := range calls[0].Request.Messages {
		for _, b := range m.Content {
			if strings.HasPrefix(b.Text, "[COMPACTION: ") {
				sawMarker = true
			}
		}
	}
	if !sawMarker {
		t.Errorf("model did not see a [COMPACTION: …] provenance-marked summary; messages = %+v", calls[0].Request.Messages)
	}
	if got := len(res.Messages); got < 5 {
		t.Errorf("RunResult.Messages = %d, want the full ground truth (>= 5)", got)
	}
}
