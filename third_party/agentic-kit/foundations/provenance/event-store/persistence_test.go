package eventstore_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
	es "github.com/geoffmilleraz/agentic-kit/foundations/provenance/event-store"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func fixedClockP(t time.Time) func() time.Time { return func() time.Time { return t } }

// buildChain appends n entries to s and returns them.
func buildChain(t *testing.T, s *es.Store, n int) []es.Entry {
	t.Helper()
	out := make([]es.Entry, n)
	for i := range out {
		e, err := s.Append("kind.test", []byte("payload"))
		if err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
		out[i] = e
	}
	return out
}

// ─── JSONLSink ────────────────────────────────────────────────────────────────

func TestJSONLSink_RoundTrip(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	s := es.NewStoreWithClock(fixedClockP(ts))

	var buf bytes.Buffer
	w := jsonllog.NewWriter(&buf)
	s.AttachSink(es.JSONLSink(w))

	const n = 5
	buildChain(t, s, n)

	if s.Len() != n {
		t.Fatalf("store has %d entries, want %d", s.Len(), n)
	}

	// Reload.
	loaded, err := es.LoadJSONL(&buf)
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	if len(loaded) != n {
		t.Fatalf("LoadJSONL: got %d entries, want %d", len(loaded), n)
	}

	// Verify hash fields survived the round-trip.
	orig := s.Entries()
	for i := range orig {
		if orig[i].Seq != loaded[i].Seq {
			t.Errorf("[%d] Seq: orig %d loaded %d", i, orig[i].Seq, loaded[i].Seq)
		}
		if orig[i].Kind != loaded[i].Kind {
			t.Errorf("[%d] Kind: orig %q loaded %q", i, orig[i].Kind, loaded[i].Kind)
		}
		if orig[i].Hash != loaded[i].Hash {
			t.Errorf("[%d] Hash: orig %q loaded %q", i, orig[i].Hash, loaded[i].Hash)
		}
		if orig[i].PrevHash != loaded[i].PrevHash {
			t.Errorf("[%d] PrevHash: orig %q loaded %q", i, orig[i].PrevHash, loaded[i].PrevHash)
		}
		if orig[i].PayloadHash != loaded[i].PayloadHash {
			t.Errorf("[%d] PayloadHash: orig %q loaded %q", i, orig[i].PayloadHash, loaded[i].PayloadHash)
		}
		if !orig[i].RecordedAt.Equal(loaded[i].RecordedAt) {
			t.Errorf("[%d] RecordedAt: orig %v loaded %v", i, orig[i].RecordedAt, loaded[i].RecordedAt)
		}
		// Payload is not stored in the wire format.
		if loaded[i].Payload != nil {
			t.Errorf("[%d] Payload: expected nil after load, got %q", i, loaded[i].Payload)
		}
	}
}

func TestJSONLSink_SinkErrorAbortsAppend(t *testing.T) {
	sentinel := errors.New("sink refused")
	s := es.NewStore()
	s.AttachSink(func(es.Entry) error { return sentinel })

	_, err := s.Append("kind.test", []byte("data"))
	if err == nil {
		t.Fatal("expected sink error to propagate from Append, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel in error chain, got %v", err)
	}
	// Chain must be unchanged.
	if s.Len() != 0 {
		t.Errorf("expected 0 entries after sink error, got %d", s.Len())
	}
}

func TestJSONLSink_ChainVerifiesAfterReload(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := es.NewStoreWithClock(fixedClockP(ts))

	var buf bytes.Buffer
	s.AttachSink(es.JSONLSink(jsonllog.NewWriter(&buf)))

	// Use nil payloads: the wire format does not store raw payload bytes, so
	// reloaded entries have Payload=nil.  Verify re-derives PayloadHash from
	// the (nil) payload and compares to the stored PayloadHash.  With nil
	// payloads both sides compute sha256([]) and the chain verifies.
	const n = 8
	for i := 0; i < n; i++ {
		if _, err := s.Append("kind.test", nil); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}

	reloaded, err := es.NewStoreFromJSONL(&buf)
	if err != nil {
		t.Fatalf("NewStoreFromJSONL: %v", err)
	}

	// Verify the reloaded chain.  Hash fields are preserved; nil payload →
	// sha256([]) which matches the stored PayloadHash for nil-payload entries.
	if err := reloaded.Verify(); err != nil {
		t.Fatalf("Verify on reloaded store: %v", err)
	}
	if reloaded.Len() != n {
		t.Fatalf("reloaded store has %d entries, want %d", reloaded.Len(), n)
	}
}

func TestJSONLSink_EmptyStore(t *testing.T) {
	var buf bytes.Buffer
	loaded, err := es.LoadJSONL(&buf)
	if err != nil {
		t.Fatalf("LoadJSONL on empty reader: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(loaded))
	}
}

func TestNewStoreFromJSONL_PartialTrailingLine(t *testing.T) {
	// Write a valid store, then corrupt the last byte to simulate a torn write.
	ts := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	s := es.NewStoreWithClock(fixedClockP(ts))

	var buf bytes.Buffer
	s.AttachSink(es.JSONLSink(jsonllog.NewWriter(&buf)))
	buildChain(t, s, 3)

	// Trim the trailing newline to produce a partial last line.
	data := buf.Bytes()
	torn := bytes.TrimRight(data, "\n")
	if bytes.Equal(torn, data) {
		t.Skip("buffer had no trailing newline to trim")
	}

	_, err := es.NewStoreFromJSONL(bytes.NewReader(torn))
	if !errors.Is(err, jsonllog.ErrPartialTrailingLine) {
		t.Errorf("expected ErrPartialTrailingLine, got %v", err)
	}
}

func TestLoadJSONL_InvalidJSON(t *testing.T) {
	bad := strings.NewReader("{not json}\n")
	_, err := es.LoadJSONL(bad)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadJSONL_MissingFields(t *testing.T) {
	// Valid JSON but missing required fields (Kind empty → decode succeeds but
	// wireToEntry/Validate reject it).
	bad := strings.NewReader(`{"Seq":0,"Kind":"","PayloadHash":"","PrevHash":"","Hash":"","RecordedAt":""}` + "\n")
	_, err := es.LoadJSONL(bad)
	if err == nil {
		t.Fatal("expected validation error for missing fields, got nil")
	}
}

// TestJSONLSink_SinkDoesNotRetainCallerPayload ensures the sink receives a
// deep copy of the payload so the caller cannot corrupt it.
func TestJSONLSink_SinkDoesNotRetainCallerPayload(t *testing.T) {
	var captured []byte
	s := es.NewStore()
	s.AttachSink(func(e es.Entry) error {
		captured = e.Payload
		return nil
	})

	payload := []byte("original")
	_, err := s.Append("ev", payload)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Mutate the payload; the sink's copy must be unaffected.
	payload[0] = 'X'
	if len(captured) > 0 && captured[0] == 'X' {
		t.Error("sink received a reference to the caller's payload slice, not a deep copy")
	}
}

// TestAttachSink_NoSinkNoPanic ensures a store without a sink still works normally.
func TestAttachSink_NoSinkNoPanic(t *testing.T) {
	s := es.NewStore()
	if _, err := s.Append("ev", []byte("data")); err != nil {
		t.Fatalf("Append without sink: %v", err)
	}
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify without sink: %v", err)
	}
}
