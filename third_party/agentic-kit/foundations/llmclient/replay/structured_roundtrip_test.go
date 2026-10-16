package llmreplay_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llmreplay "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/replay"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/structured"
)

// TestStructuredCompleteRoundTripsThroughFileCassette proves the exact chain
// the fathom gke-expertchat plan (GKE-EC-5) relies on: a structured.Complete
// call recorded to an on-disk JSONL cassette replays byte-identical, with no
// network involved on either side. This closes the audit gap — until this
// test, record→replay fidelity was proven for the raw llmclient.LLMClient
// surface (TestReplayClientConformance) but never through the structured
// seam a downstream caller actually uses.
func TestStructuredCompleteRoundTripsThroughFileCassette(t *testing.T) {
	type answer struct {
		Verdict string `json:"verdict"`
		Score   int    `json:"score"`
	}

	schema := json.RawMessage(`{"type":"object","properties":{"verdict":{"type":"string"},"score":{"type":"integer"}},"required":["verdict","score"]}`)
	req := llmclient.Request{Prompt: "evaluate the fixture"}

	cassettePath := filepath.Join(t.TempDir(), "structured.jsonl")

	// --- record: a live (mock, not network) client wrapped in a recorder,
	// writing straight to a real file on disk.
	f, err := os.OpenFile(cassettePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open cassette for write: %v", err)
	}
	mock := llmclient.NewMockClient(llmclient.MockEntryOK(`{"verdict":"pass","score":97}`))
	rec := llmreplay.NewRecordingClient(mock, f)

	wantResp, wantErr := structured.Complete(context.Background(), rec, req, schema, new(answer))
	if wantErr != nil {
		t.Fatalf("recording structured.Complete: %v", wantErr)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close cassette: %v", err)
	}

	// Decode what the recording side saw, for the byte-identical comparison
	// below.
	var wantAnswer answer
	if err := json.Unmarshal([]byte(wantResp.Text), &wantAnswer); err != nil {
		t.Fatalf("decode recorded response text: %v", err)
	}

	// --- replay: load the cassette back from disk (no network, no mock —
	// just the file written above) and run the identical structured.Complete
	// call against it.
	entries, err := llmreplay.LoadFile(cassettePath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 recorded entry, got %d", len(entries))
	}
	replayClient := llmreplay.NewReplayClient(entries)

	var gotAnswer answer
	gotResp, err := structured.Complete(context.Background(), replayClient, req, schema, &gotAnswer)
	if err != nil {
		t.Fatalf("replayed structured.Complete: %v", err)
	}

	if gotResp.Text != wantResp.Text {
		t.Fatalf("replayed response text = %q, want byte-identical %q", gotResp.Text, wantResp.Text)
	}
	if !reflect.DeepEqual(gotResp, wantResp) {
		t.Fatalf("replayed Response = %+v, want %+v", gotResp, wantResp)
	}
	if !reflect.DeepEqual(gotAnswer, wantAnswer) {
		t.Fatalf("replayed decoded struct = %+v, want %+v", gotAnswer, wantAnswer)
	}

	// A second call past the single recorded entry must fail loudly, not
	// fall through to any live/network path — there is none to fall through
	// to, but the contract must still hold under the structured seam.
	if _, err := structured.Complete(context.Background(), replayClient, req, schema, new(answer)); err == nil {
		t.Fatal("expected an error when replaying past the last recorded entry, got nil")
	}
}
