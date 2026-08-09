package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// index_test.go — StoreIndex against a real store, proving the anchor hashes
// come from data fathom ALREADY maintains (observations and fingerprints),
// not from a new sweep or a new column.

func seedStore(t *testing.T) *store.MemoryStore {
	t.Helper()
	st := store.NewMemoryStore()
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{
		{
			ServiceID: "svc", Package: "cfg", Symbol: "parseConfig", SourceKind: core.KindGoFunc,
			SourceRef: "internal/cfg/cfg.go", Claim: "func parseConfig(b []byte) (Config, error)", ContentHash: "file-hash-v1",
		},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.SetFingerprint(ctx, "svc", "internal/cfg/cfg.go", "file-hash-v1"); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: "/repo", IndexedCommit: "aaa111", IndexedAt: 1}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	return st
}

func TestStoreIndexResolvesEveryAnchorKind(t *testing.T) {
	ctx := context.Background()
	st := seedStore(t)
	idx := NewStoreIndex(st)

	gen, err := idx.Generation(ctx, "svc")
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if gen != "commit:aaa111" {
		t.Errorf("Generation = %q, want commit:aaa111", gen)
	}

	cases := []struct {
		name   string
		anchor Anchor
		exists bool
	}{
		{"symbol by bare name", Anchor{Kind: AnchorSymbol, Ref: "parseConfig"}, true},
		{"symbol by key", Anchor{Kind: AnchorSymbol, Ref: core.SymbolKey("svc", "cfg", "parseConfig")}, true},
		{"symbol missing", Anchor{Kind: AnchorSymbol, Ref: "nope"}, false},
		{"file", Anchor{Kind: AnchorFile, Ref: "internal/cfg/cfg.go"}, true},
		{"file missing", Anchor{Kind: AnchorFile, Ref: "internal/cfg/gone.go"}, false},
		{"path glob", Anchor{Kind: AnchorPath, Ref: "internal/**/*.go"}, true},
		{"path glob missing", Anchor{Kind: AnchorPath, Ref: "cmd/**/*.go"}, false},
		{"commit", Anchor{Kind: AnchorCommit, Ref: "aaa111"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := idx.Probe(ctx, "svc", tc.anchor)
			if err != nil {
				t.Fatalf("Probe: %v", err)
			}
			if p.Exists != tc.exists {
				t.Fatalf("Exists = %v, want %v (covers: %s)", p.Exists, tc.exists, p.Covers)
			}
			if p.Covers == "" {
				t.Error("Covers is empty; a probe must say what its hash covers (law 3)")
			}
			if tc.exists && p.Hash == "" {
				t.Error("an existing anchor must carry a hash")
			}
			if !tc.exists && p.Hash != "" {
				t.Errorf("a missing anchor must carry no hash; got %q", p.Hash)
			}
		})
	}
}

// TestStoreIndexSymbolDriftsWithItsFile: the granularity is file-level and
// the probe says so — the drift signal fires when the containing file's
// content hash moves.
func TestStoreIndexSymbolDriftsWithItsFile(t *testing.T) {
	ctx := context.Background()
	st := seedStore(t)
	idx := NewStoreIndex(st)
	a := Anchor{Kind: AnchorSymbol, Ref: "parseConfig"}

	before, err := idx.Probe(ctx, "svc", a)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}

	// Re-ingest the same file with new content and a new commit.
	if err := st.PurgeSource(ctx, "svc", "internal/cfg/cfg.go"); err != nil {
		t.Fatalf("PurgeSource: %v", err)
	}
	if err := st.PutObservations(ctx, []core.Observation{
		{
			ServiceID: "svc", Package: "cfg", Symbol: "parseConfig", SourceKind: core.KindGoFunc,
			SourceRef: "internal/cfg/cfg.go", Claim: "func parseConfig(b []byte) (Config, error)", ContentHash: "file-hash-v2",
		},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: "/repo", IndexedCommit: "bbb222", IndexedAt: 2}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	after, err := idx.Probe(ctx, "svc", a)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if before.Hash == after.Hash {
		t.Error("the symbol's hash did not move when its file's content hash did")
	}
	if after.Covers == "" {
		t.Error("Covers must state the file granularity honestly")
	}
}

// TestStoreIndexUnindexedServiceIsNamed — law 3: a service with no metadata
// reports a named generation, not an empty string that would silently share a
// cache slot with every other unindexed service.
func TestStoreIndexUnindexedServiceIsNamed(t *testing.T) {
	ctx := context.Background()
	idx := NewStoreIndex(store.NewMemoryStore())
	gen, err := idx.Generation(ctx, "ghost")
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if gen != "unindexed" {
		t.Errorf("Generation = %q, want %q", gen, "unindexed")
	}
}

// TestGenerationDoesNotDoubleLabelANonGitWatermark — law 3 applied to
// labels: internal/fusion records a non-git corpus's watermark as
// "manifest:<hash>", which already names its scheme. Prefixing it again
// produced the flatly untrue "commit:manifest:…" in a live smoke run.
func TestGenerationDoesNotDoubleLabelANonGitWatermark(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{
		ServiceID: "svc", Path: "/repo", IndexedCommit: "manifest:abc123", IndexedAt: 1,
	}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	gen, err := NewStoreIndex(st).Generation(ctx, "svc")
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if gen != "manifest:abc123" {
		t.Errorf("Generation = %q, want %q — a scheme-bearing watermark must not be re-prefixed", gen, "manifest:abc123")
	}
}

func TestShortWatermarkKeepsTheSchemeAndTruncatesTheHash(t *testing.T) {
	cases := map[string]string{
		"":                                 "",
		"aaa111":                           "aaa111",
		"commit:aaa111":                    "commit:aaa111",
		"commit:0123456789abcdef0123":      "commit:0123456789ab…",
		"manifest:0123456789abcdef0123456": "manifest:0123456789ab…",
	}
	for in, want := range cases {
		if got := ShortWatermark(in); got != want {
			t.Errorf("ShortWatermark(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFileStoreRoundTrip: the JSONL store round-trips a memory whole and
// returns rows in a deterministic order regardless of insertion order.
func TestFileStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), ".fathom", FileName)
	fs := NewFileStore(path)

	at := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	mk := func(id, service, claim string) Memory {
		return Memory{
			ID: id, Service: service, Claim: claim,
			Anchors:      []Anchor{{Kind: AnchorSymbol, Ref: "sym" + id}},
			Evidence:     []Evidence{{Kind: EvidenceIndex, Ref: "ref" + id}},
			Tier:         core.TierPreScan,
			Confidence:   3,
			AnchorHashes: map[string]string{"symbol:sym" + id: "h" + id},
			CreatedAt:    at, LastConfirmedAt: at,
			ConfirmedBy: Actor{Kind: ActorHuman, Name: "alex"},
			Author:      Actor{Kind: ActorHuman, Name: "alex"},
		}
	}
	for _, m := range []Memory{mk("ccc", "svc", "third"), mk("aaa", "svc", "first"), mk("bbb", "other", "second")} {
		if err := fs.Put(ctx, m); err != nil {
			t.Fatalf("Put(%s): %v", m.ID, err)
		}
	}

	all, err := fs.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 || all[0].ID != "aaa" || all[1].ID != "bbb" || all[2].ID != "ccc" {
		t.Fatalf("List order = %v, want aaa bbb ccc", memIDs(all))
	}

	svc, err := fs.List(ctx, "svc")
	if err != nil {
		t.Fatalf("List(svc): %v", err)
	}
	if len(svc) != 2 {
		t.Fatalf("List(svc) = %v, want 2 rows", memIDs(svc))
	}

	got, found, err := fs.Get(ctx, "bbb")
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.Claim != "second" || got.AnchorHashes["symbol:symbbb"] != "hbbb" {
		t.Errorf("round-trip lost data: %+v", got)
	}

	// Replacing by id must not duplicate.
	replaced := mk("bbb", "other", "second, rewritten")
	if err := fs.Put(ctx, replaced); err != nil {
		t.Fatalf("Put(replace): %v", err)
	}
	all, _ = fs.List(ctx, "")
	if len(all) != 3 {
		t.Fatalf("replace produced %d rows, want 3", len(all))
	}

	// A second FileStore over the same path reads the same content — the
	// file, not the process, is the store.
	reopened := NewFileStore(path)
	again, err := reopened.List(ctx, "")
	if err != nil {
		t.Fatalf("List after reopen: %v", err)
	}
	if len(again) != 3 || again[1].Claim != "second, rewritten" {
		t.Errorf("reopened store = %+v", memIDs(again))
	}
}

func TestPathIsProjectScoped(t *testing.T) {
	got := Path("/repo")
	want := filepath.Join("/repo", ".fathom", "memories.jsonl")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func memIDs(ms []Memory) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}
