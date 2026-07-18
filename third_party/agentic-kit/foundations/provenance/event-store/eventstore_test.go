package eventstore_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	es "github.com/geoffmilleraz/agentic-kit/foundations/provenance/event-store"
)

// fixedClock returns a clock that returns the same instant on every call.
// Useful for deterministic hash assertions.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// mustAppend appends to the store or fails the test.
func mustAppend(t *testing.T, s *es.Store, kind string, payload []byte) es.Entry {
	t.Helper()
	e, err := s.Append(kind, payload)
	if err != nil {
		t.Fatalf("Append(%q): unexpected error: %v", kind, err)
	}
	return e
}

// ---------------------------------------------------------------------------
// Entry.Validate
// ---------------------------------------------------------------------------

func TestEntryValidate_Happy(t *testing.T) {
	e := es.Entry{
		Seq:         0,
		Kind:        "test.event",
		PayloadHash: "abc123",
		PrevHash:    es.GenesisPrevHash,
		Hash:        "def456",
		RecordedAt:  time.Now().UTC(),
	}
	if err := e.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestEntryValidate_EmptyFields(t *testing.T) {
	base := es.Entry{
		Seq:         0,
		Kind:        "test.event",
		PayloadHash: "abc123",
		PrevHash:    es.GenesisPrevHash,
		Hash:        "def456",
		RecordedAt:  time.Now().UTC(),
	}
	cases := []struct {
		name string
		mut  func(*es.Entry)
	}{
		{"empty-Kind", func(e *es.Entry) { e.Kind = "" }},
		{"empty-PayloadHash", func(e *es.Entry) { e.PayloadHash = "" }},
		{"empty-Hash", func(e *es.Entry) { e.Hash = "" }},
		{"empty-PrevHash", func(e *es.Entry) { e.PrevHash = "" }},
		{"zero-RecordedAt", func(e *es.Entry) { e.RecordedAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.mut(&e)
			if err := e.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GenesisPrevHash constant
// ---------------------------------------------------------------------------

func TestGenesisPrevHash_Is64Zeros(t *testing.T) {
	if len(es.GenesisPrevHash) != 64 {
		t.Fatalf("GenesisPrevHash length %d, want 64", len(es.GenesisPrevHash))
	}
	for i, c := range es.GenesisPrevHash {
		if c != '0' {
			t.Fatalf("GenesisPrevHash[%d] = %q, want '0'", i, c)
		}
	}
}

// ---------------------------------------------------------------------------
// NewStore / NewStoreWithClock
// ---------------------------------------------------------------------------

func TestNewStore_Empty(t *testing.T) {
	s := es.NewStore()
	if s.Len() != 0 {
		t.Fatalf("new store: Len %d, want 0", s.Len())
	}
	_, ok := s.Head()
	if ok {
		t.Fatal("new store: Head returned true, want false")
	}
}

func TestNewStoreWithClock_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil clock, got none")
		}
	}()
	es.NewStoreWithClock(nil)
}

// ---------------------------------------------------------------------------
// Append
// ---------------------------------------------------------------------------

func TestAppend_EmptyKindErrors(t *testing.T) {
	s := es.NewStore()
	_, err := s.Append("", []byte("data"))
	if err == nil {
		t.Fatal("expected error for empty kind, got nil")
	}
}

func TestAppend_GenesisLinking(t *testing.T) {
	s := es.NewStore()
	e := mustAppend(t, s, "genesis.event", []byte("hello"))

	if e.Seq != 0 {
		t.Errorf("genesis: Seq %d, want 0", e.Seq)
	}
	if e.PrevHash != es.GenesisPrevHash {
		t.Errorf("genesis: PrevHash %q, want GenesisPrevHash", e.PrevHash)
	}
	if e.Hash == "" {
		t.Error("genesis: Hash must not be empty")
	}
	if e.PayloadHash == "" {
		t.Error("genesis: PayloadHash must not be empty")
	}
}

func TestAppend_ChainLinking(t *testing.T) {
	s := es.NewStore()
	e0 := mustAppend(t, s, "event.first", []byte("first"))
	e1 := mustAppend(t, s, "event.second", []byte("second"))
	e2 := mustAppend(t, s, "event.third", []byte("third"))

	if e1.PrevHash != e0.Hash {
		t.Errorf("e1.PrevHash %q != e0.Hash %q", e1.PrevHash, e0.Hash)
	}
	if e2.PrevHash != e1.Hash {
		t.Errorf("e2.PrevHash %q != e1.Hash %q", e2.PrevHash, e1.Hash)
	}
	if e0.Seq != 0 || e1.Seq != 1 || e2.Seq != 2 {
		t.Errorf("Seq: got %d,%d,%d want 0,1,2", e0.Seq, e1.Seq, e2.Seq)
	}
}

func TestAppend_NilPayload(t *testing.T) {
	s := es.NewStore()
	e := mustAppend(t, s, "empty.payload", nil)
	if e.PayloadHash == "" {
		t.Error("nil payload: PayloadHash must not be empty (SHA-256 of empty input)")
	}
}

// ---------------------------------------------------------------------------
// Deterministic hash with injected clock
// ---------------------------------------------------------------------------

func TestAppend_DeterministicHash(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	s1 := es.NewStoreWithClock(fixedClock(ts))
	s2 := es.NewStoreWithClock(fixedClock(ts))

	e1 := mustAppend(t, s1, "det.event", []byte("payload"))
	e2 := mustAppend(t, s2, "det.event", []byte("payload"))

	if e1.Hash != e2.Hash {
		t.Errorf("same inputs produced different hashes:\n  s1: %s\n  s2: %s", e1.Hash, e2.Hash)
	}
	if e1.PayloadHash != e2.PayloadHash {
		t.Errorf("same payload produced different PayloadHash:\n  s1: %s\n  s2: %s", e1.PayloadHash, e2.PayloadHash)
	}
}

func TestAppend_DifferentClocksDifferentHashes(t *testing.T) {
	ts1 := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2025, 1, 15, 10, 0, 0, 1, time.UTC) // 1 nanosecond later
	s1 := es.NewStoreWithClock(fixedClock(ts1))
	s2 := es.NewStoreWithClock(fixedClock(ts2))

	e1 := mustAppend(t, s1, "same.event", []byte("same"))
	e2 := mustAppend(t, s2, "same.event", []byte("same"))

	if e1.Hash == e2.Hash {
		t.Error("different RecordedAt should produce different entry Hashes")
	}
}

// ---------------------------------------------------------------------------
// Entries / Head / Len
// ---------------------------------------------------------------------------

func TestEntries_DefensiveCopy(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev", []byte("x"))
	snap := s.Entries()
	snap[0].Kind = "mutated"
	fresh := s.Entries()
	if fresh[0].Kind == "mutated" {
		t.Error("Entries returned a live slice; mutation propagated to store")
	}
}

func TestHead_ReturnsLatest(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "first", []byte("a"))
	last := mustAppend(t, s, "last", []byte("b"))

	h, ok := s.Head()
	if !ok {
		t.Fatal("Head returned false on non-empty store")
	}
	if h.Hash != last.Hash {
		t.Errorf("Head.Hash %q != last.Hash %q", h.Hash, last.Hash)
	}
}

// ---------------------------------------------------------------------------
// Verify — clean chain passes
// ---------------------------------------------------------------------------

func TestVerify_CleanChainPasses(t *testing.T) {
	s := es.NewStore()
	for i := 0; i < 10; i++ {
		mustAppend(t, s, fmt.Sprintf("kind.%d", i), []byte(fmt.Sprintf("payload-%d", i)))
	}
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify on clean chain: unexpected error: %v", err)
	}
}

func TestVerify_EmptyStorePasses(t *testing.T) {
	s := es.NewStore()
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify on empty store: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Verify — tampered chain fails
// ---------------------------------------------------------------------------

// tamperedStore constructs a Store whose internal state has been manipulated
// via the exported Entries view. Since Store is unexported-field encapsulated,
// we use a test-helper that appends known entries and then returns a copy
// with specific bytes changed. We do this by building a second store and
// using the low-level approach: append entries normally, then verify
// tampering is detected by comparing payload/hash mismatches through public API.
//
// Because Store does not expose internal mutation (by design), we test the
// tamper detection by using Replay to collect entries, altering them, and
// building a raw tampered store with an internal helper exposed only for tests.

func TestVerify_MutatedPayloadDetected(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("original payload"))
	mustAppend(t, s, "ev.b", []byte("second payload"))

	// Build tampered store: mutate payload of entry 0 without updating PayloadHash.
	entries := s.Entries()
	entries[0].Payload = []byte("tampered payload")

	tampered := es.NewStoreFromEntries(entries)
	if err := tampered.Verify(); err == nil {
		t.Fatal("expected Verify to fail on mutated payload, got nil")
	}
}

func TestVerify_MutatedHashDetected(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("data-a"))
	mustAppend(t, s, "ev.b", []byte("data-b"))

	entries := s.Entries()
	// Overwrite the Hash field of entry 0 with a plausible-looking but wrong value.
	entries[0].Hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tampered := es.NewStoreFromEntries(entries)
	if err := tampered.Verify(); err == nil {
		t.Fatal("expected Verify to fail on mutated Hash, got nil")
	}
}

func TestVerify_MutatedPayloadHashDetected(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("data-a"))

	entries := s.Entries()
	entries[0].PayloadHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	tampered := es.NewStoreFromEntries(entries)
	if err := tampered.Verify(); err == nil {
		t.Fatal("expected Verify to fail on mutated PayloadHash, got nil")
	}
}

func TestVerify_DroppedEntryDetected(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("a"))
	mustAppend(t, s, "ev.b", []byte("b"))
	mustAppend(t, s, "ev.c", []byte("c"))

	entries := s.Entries()
	// Drop the middle entry; indices become [0, 2] with Seq 0, 2.
	pruned := []es.Entry{entries[0], entries[2]}

	tampered := es.NewStoreFromEntries(pruned)
	if err := tampered.Verify(); err == nil {
		t.Fatal("expected Verify to fail on dropped entry, got nil")
	}
}

func TestVerify_ReorderedEntriesDetected(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("a"))
	mustAppend(t, s, "ev.b", []byte("b"))

	entries := s.Entries()
	// Swap order.
	reordered := []es.Entry{entries[1], entries[0]}

	tampered := es.NewStoreFromEntries(reordered)
	if err := tampered.Verify(); err == nil {
		t.Fatal("expected Verify to fail on reordered entries, got nil")
	}
}

// ---------------------------------------------------------------------------
// Replay
// ---------------------------------------------------------------------------

func TestReplay_OrderAndContent(t *testing.T) {
	s := es.NewStore()
	kinds := []string{"alpha", "beta", "gamma"}
	for _, k := range kinds {
		mustAppend(t, s, k, []byte(k+"-payload"))
	}

	var seen []string
	err := s.Replay(func(e es.Entry) error {
		seen = append(seen, e.Kind)
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: unexpected error: %v", err)
	}
	if len(seen) != len(kinds) {
		t.Fatalf("Replay visited %d entries, want %d", len(seen), len(kinds))
	}
	for i, k := range kinds {
		if seen[i] != k {
			t.Errorf("entry %d: kind %q, want %q", i, seen[i], k)
		}
	}
}

func TestReplay_StopsOnError(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "first", nil)
	mustAppend(t, s, "second", nil)
	mustAppend(t, s, "third", nil)

	sentinel := errors.New("stop here")
	count := 0
	err := s.Replay(func(e es.Entry) error {
		count++
		if count == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("Replay: got error %v, want sentinel", err)
	}
	if count != 2 {
		t.Errorf("Replay: fn called %d times, want 2", count)
	}
}

func TestReplay_EmptyStore(t *testing.T) {
	s := es.NewStore()
	called := false
	err := s.Replay(func(_ es.Entry) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Replay on empty store: unexpected error: %v", err)
	}
	if called {
		t.Error("Replay on empty store: fn should not be called")
	}
}

// ---------------------------------------------------------------------------
// FIX A1 regression: Payload deep-copy isolation
// ---------------------------------------------------------------------------

// TestEntries_PayloadIsolation verifies that mutating the Payload bytes
// returned by Entries() does not corrupt the store's internal state, and
// that Verify() still returns nil after the mutation.
func TestEntries_PayloadIsolation(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.payload", []byte("original"))
	mustAppend(t, s, "ev.second", []byte("second"))

	entries := s.Entries()
	// Mutate the returned payload bytes in place.
	for i := range entries[0].Payload {
		entries[0].Payload[i] = 'X'
	}

	// Store must still verify: internal bytes must be unaffected.
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify failed after mutating Entries() payload — store not isolated: %v", err)
	}
}

// TestReplay_PayloadIsolation verifies that mutating Entry.Payload inside the
// Replay callback does not corrupt the store's internal state.
func TestReplay_PayloadIsolation(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.one", []byte("hello"))

	err := s.Replay(func(e es.Entry) error {
		// Mutate the payload handed to the callback.
		for i := range e.Payload {
			e.Payload[i] = 'Z'
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Replay: unexpected error: %v", err)
	}

	// Internal state must be intact.
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify failed after mutating Replay callback payload — store not isolated: %v", err)
	}
}

// TestHead_PayloadIsolation verifies that mutating Entry.Payload returned by
// Head() does not corrupt the store's internal state.
func TestHead_PayloadIsolation(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.head", []byte("headdata"))

	h, ok := s.Head()
	if !ok {
		t.Fatal("Head returned false on non-empty store")
	}
	// Mutate the returned payload.
	for i := range h.Payload {
		h.Payload[i] = 'Q'
	}

	// Internal state must be intact.
	if err := s.Verify(); err != nil {
		t.Fatalf("Verify failed after mutating Head() payload — store not isolated: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Finding 1: NewStoreFromEntries Payload deep-copy isolation
// ---------------------------------------------------------------------------

// TestNewStoreFromEntries_PayloadIsolation verifies that mutating the caller's
// input slice Payload bytes after calling NewStoreFromEntries does NOT corrupt
// the store's internal state. The shallow copy bug allowed the store to share
// the backing array with the caller.
func TestNewStoreFromEntries_PayloadIsolation(t *testing.T) {
	s := es.NewStore()
	mustAppend(t, s, "ev.a", []byte("original"))
	mustAppend(t, s, "ev.b", []byte("second"))

	// Grab a copy of entries — these have their own isolated payloads from Entries().
	entries := s.Entries()

	// Build a new store from those entries.
	fromEntries := es.NewStoreFromEntries(entries)

	// Now mutate the input slice's payload bytes in-place.
	for i := range entries[0].Payload {
		entries[0].Payload[i] = 'X'
	}

	// The store built from these entries must still verify; it must have
	// deep-copied the payload bytes at construction time.
	if err := fromEntries.Verify(); err != nil {
		t.Fatalf("Verify failed after mutating NewStoreFromEntries input payload — store not isolated: %v", err)
	}

	// Also confirm the stored payload is genuinely "original", not "XXXXXXXX".
	got := fromEntries.Entries()
	if string(got[0].Payload) != "original" {
		t.Errorf("stored payload = %q, want %q", string(got[0].Payload), "original")
	}
}

// ---------------------------------------------------------------------------
// Finding 2: Verify() must call Validate() on each entry
// ---------------------------------------------------------------------------

// TestVerify_InvalidEntryZeroRecordedAt confirms that a store pre-populated via
// NewStoreFromEntries with a valid hash chain but an invalid entry (zero
// RecordedAt) is rejected by Verify(). Without the Validate() call inside
// Verify(), adversarially crafted entries with invalid fields pass the chain
// integrity check.
func TestVerify_InvalidEntryZeroRecordedAt(t *testing.T) {
	// Build a valid chain first.
	src := es.NewStore()
	mustAppend(t, src, "ev.a", []byte("data-a"))
	mustAppend(t, src, "ev.b", []byte("data-b"))

	entries := src.Entries()

	// Manually zero out RecordedAt on entry 0, then recompute PayloadHash and
	// Hash so the chain is mathematically consistent but the entry is semantically
	// invalid (zero timestamp). We can't recompute hashes from outside the package,
	// so instead we construct a brand-new entry with a zero time. We do this by
	// building a store with a zero-time clock — which Append accepts but Validate
	// rejects — then export its entries.
	zeroTime := time.Time{}
	zeroStore := es.NewStoreWithClock(func() time.Time { return zeroTime })
	_, err := zeroStore.Append("ev.zero", []byte("zero-payload"))
	if err != nil {
		t.Fatalf("Append with zero clock: %v", err)
	}

	// The zero-time entry must fail Validate independently.
	zeroEntries := zeroStore.Entries()
	if err := zeroEntries[0].Validate(); err == nil {
		t.Fatal("expected Validate to fail on zero RecordedAt entry, got nil")
	}

	// When Verify() calls Validate() on each entry, this store must fail.
	if err := zeroStore.Verify(); err == nil {
		t.Fatal("expected Verify to fail on entry with zero RecordedAt, got nil")
	}

	// A legitimately good chain must still pass.
	_ = entries
	goodStore := es.NewStoreFromEntries(entries)
	if err := goodStore.Verify(); err != nil {
		t.Fatalf("good chain: Verify returned unexpected error: %v", err)
	}
}
