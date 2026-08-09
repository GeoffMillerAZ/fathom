package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fake_test.go — the test doubles every phase-W test shares.

// fakeIndex is a hand-driven Index. It COUNTS probes, because "drift is
// evaluated at read time, cached per index generation" is only a real claim
// if something counts the hashing.
//
// Concurrency: safe for concurrent use via sync.Mutex (the -race gate runs
// the cache test through concurrent evaluations).
type fakeIndex struct {
	mu     sync.Mutex
	gen    string
	hashes map[string]string // anchor key → current hash; absent = deleted
	probes int
	gens   int
}

func newFakeIndex(gen string) *fakeIndex {
	return &fakeIndex{gen: gen, hashes: map[string]string{}}
}

func (f *fakeIndex) set(anchorKey, hash string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hashes[anchorKey] = hash
}

func (f *fakeIndex) remove(anchorKey string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.hashes, anchorKey)
}

func (f *fakeIndex) setGeneration(gen string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gen = gen
}

func (f *fakeIndex) probeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.probes
}

func (f *fakeIndex) Generation(_ context.Context, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gens++
	return f.gen, nil
}

func (f *fakeIndex) Probe(_ context.Context, _ string, a Anchor) (AnchorProbe, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probes++
	h, ok := f.hashes[a.Key()]
	if !ok {
		return AnchorProbe{Exists: false, Covers: "not in the fake index"}, nil
	}
	return AnchorProbe{Hash: h, Exists: true, Covers: "the fake index entry for " + a.Key()}, nil
}

// fixedClock returns a clock that always reports at.
func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed.UTC()
}

// writeFixture writes one well-formed memory anchored to symbol:parseConfig
// with index evidence, and returns it.
func writeFixture(t *testing.T, ctx context.Context, ms Store, idx *fakeIndex, at time.Time) Memory {
	t.Helper()
	idx.set("symbol:parseConfig", "hash-v1")
	m, err := WriteMemory(ctx, ms, idx, fixedClock(at), Draft{
		Service:  "svc",
		Claim:    "parseConfig validates the schema before returning.",
		Anchors:  []Anchor{{Kind: AnchorSymbol, Ref: "parseConfig"}},
		Evidence: []Evidence{{Kind: EvidenceIndex, Ref: "svc::cfg.parseConfig"}},
		Author:   Actor{Kind: ActorHuman, Name: "alex"},
		Commit:   "commit:aaaa111",
	})
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	return m
}
