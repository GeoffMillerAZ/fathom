package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memory"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// memFakeIndex is a hand-driven memory.Index + memory.SymbolLocator.
type memFakeIndex struct {
	gen    string
	hashes map[string]string
	files  map[string]string
}

func (f *memFakeIndex) Generation(_ context.Context, _ string) (string, error) { return f.gen, nil }

func (f *memFakeIndex) Probe(_ context.Context, _ string, a memory.Anchor) (memory.AnchorProbe, error) {
	h, ok := f.hashes[a.Key()]
	if !ok {
		return memory.AnchorProbe{Exists: false, Covers: "not in the fake index"}, nil
	}
	return memory.AnchorProbe{Hash: h, Exists: true, Covers: "the fake index entry for " + a.Key()}, nil
}

func (f *memFakeIndex) SymbolFile(_ context.Context, _ string, ref string) (string, bool, error) {
	file, ok := f.files[ref]
	return file, ok, nil
}

var memNow = func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }

func memWrite(t *testing.T, ms memory.Store, idx memory.Index, claim string, anchors []memory.Anchor, ev memory.Evidence) memory.Memory {
	t.Helper()
	m, err := memory.WriteMemory(context.Background(), ms, idx, memNow, memory.Draft{
		Service: "svc", Claim: claim, Anchors: anchors, Evidence: []memory.Evidence{ev},
		Author: memory.Actor{Kind: memory.ActorHuman, Name: "alex"},
	})
	if err != nil {
		t.Fatalf("WriteMemory(%q): %v", claim, err)
	}
	return m
}

// TestMemorySearchAlwaysLabels (AM-8): every surfaced memory carries a
// tier and a state, a retired memory never appears, and the order is
// tier → specificity → id.
func TestMemorySearchAlwaysLabels(t *testing.T) {
	ctx := context.Background()
	ms := memory.NewMemStore()
	idx := &memFakeIndex{
		gen: "commit:aaaa111",
		hashes: map[string]string{
			"symbol:parseConfig":       "h1",
			"path:internal/cfg/**":     "h2",
			"file:internal/cfg/io.go":  "h3",
			"file:internal/cfg/env.go": "h4",
		},
		files: map[string]string{"parseConfig": "internal/cfg/parse.go"},
	}
	verified := memWrite(t, ms, idx, "parseConfig validates the schema before returning.",
		[]memory.Anchor{{Kind: memory.AnchorSymbol, Ref: "parseConfig"}}, memory.Evidence{Kind: memory.EvidenceMachineCheck, Ref: "TestParseConfig"})
	doc := memWrite(t, ms, idx, "The cfg package owns every config default.",
		[]memory.Anchor{{Kind: memory.AnchorPath, Ref: "internal/cfg/**"}}, memory.Evidence{Kind: memory.EvidenceDoc, Ref: "docs/cfg.md"})
	guess := memWrite(t, ms, idx, "io.go is only read at startup.",
		[]memory.Anchor{{Kind: memory.AnchorFile, Ref: "internal/cfg/io.go"}}, memory.Evidence{Kind: memory.EvidenceModel, Ref: "claude"})
	retired := memWrite(t, ms, idx, "env.go once shadowed flags.",
		[]memory.Anchor{{Kind: memory.AnchorFile, Ref: "internal/cfg/env.go"}}, memory.Evidence{Kind: memory.EvidenceHuman, Ref: "alex"})
	if _, err := memory.Resolve(ctx, ms, idx, memNow, retired.ID, memory.ResolveNoLongerTrue, memory.Actor{Kind: memory.ActorHuman, Name: "alex"}, "", "", "gone"); err != nil {
		t.Fatalf("retire: %v", err)
	}

	res, err := memorySearchTool(ctx, ms, idx, memory.NewDriftCache(), memNow, memorySearchParams{Service: "svc"})
	if err != nil {
		t.Fatalf("memorySearchTool: %v", err)
	}
	if res.Total != 3 || res.Shown != 3 || res.Displaced != 0 {
		t.Fatalf("total/shown/displaced = %d/%d/%d, want 3/3/0", res.Total, res.Shown, res.Displaced)
	}
	// Tier first (verified > pre-scan > doc-claimed > sme-stated > assumed),
	// so doc (doc-claimed, path anchor) outranks guess (assumed, file
	// anchor) despite the sharper anchor.
	wantOrder := []string{verified.ID, doc.ID, guess.ID}
	for i, e := range res.Memories {
		if e.Tier == "" || e.State == "" {
			t.Errorf("memory %s surfaced unlabelled: tier=%q state=%q", e.ID, e.Tier, e.State)
		}
		if !strings.Contains(e.Line, string(e.Tier)) || !strings.Contains(e.Line, string(e.State)) {
			t.Errorf("line %q omits tier %q or state %q", e.Line, e.Tier, e.State)
		}
		if e.ID == retired.ID {
			t.Errorf("retired memory %s surfaced", e.ID)
		}
		if len(e.Anchors) == 0 {
			t.Errorf("memory %s carries no anchors", e.ID)
		}
		if i < len(wantOrder) && e.ID != wantOrder[i] {
			t.Errorf("position %d = %s, want %s", i, e.ID, wantOrder[i])
		}
	}

	// The wire form keeps the labels: id, tier and state survive JSON.
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire struct {
		Memories []map[string]any `json:"memories"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range wire.Memories {
		if m["tier"] == "" || m["tier"] == nil || m["state"] == "" || m["state"] == nil {
			t.Errorf("wire entry %v lacks tier/state", m)
		}
	}

	// query narrows, case-insensitively, over claim AND anchor keys ("cfg"
	// occurs only in doc's and guess's anchors); limit cuts the
	// lowest-ranked tail and the cut is disclosed twice over — `displaced`
	// (the obligations/edit-hook shape) and BA-1's `elided` block.
	res, err = memorySearchTool(ctx, ms, idx, nil, memNow, memorySearchParams{Service: "svc", Query: "CFG", Limit: 1})
	if err != nil {
		t.Fatalf("memorySearchTool(query): %v", err)
	}
	if res.Total != 2 || res.Shown != 1 || res.Displaced != 1 || res.Memories[0].ID != doc.ID {
		t.Fatalf("query/limit = total %d shown %d displaced %d first %s", res.Total, res.Shown, res.Displaced, res.Memories[0].ID)
	}
	if res.Elided == nil || res.Elided.Count != 1 || res.Elided.Of != 2 || res.Elided.Ordering == "" || res.Elided.HowToGetMore == "" {
		t.Fatalf("elided = %+v, want count 1 of 2 with ordering and how_to_get_more", res.Elided)
	}

	// file narrows to anchors touching the file (symbol placed by the locator, glob matches).
	res, err = memorySearchTool(ctx, ms, idx, nil, memNow, memorySearchParams{Service: "svc", File: "internal/cfg/parse.go"})
	if err != nil {
		t.Fatalf("memorySearchTool(file): %v", err)
	}
	if res.Shown != 2 || res.Memories[0].ID != verified.ID || res.Memories[1].ID != doc.ID {
		t.Fatalf("file scope = %+v", res.Memories)
	}

	// A drifted anchor still surfaces — labeled suspect, tier dropped.
	idx.hashes["symbol:parseConfig"] = "h1-moved"
	res, err = memorySearchTool(ctx, ms, idx, nil, memNow, memorySearchParams{Service: "svc", Query: "schema"})
	if err != nil {
		t.Fatalf("memorySearchTool(drift): %v", err)
	}
	if res.Shown != 1 || res.Memories[0].State != memory.StateSuspect || res.Memories[0].Tier != core.TierPreScan {
		t.Fatalf("drifted = %+v", res.Memories)
	}

	// No index → refusal, never an unlabelled answer.
	if _, err := memorySearchTool(ctx, ms, nil, nil, memNow, memorySearchParams{Service: "svc"}); err == nil {
		t.Fatal("memorySearchTool without an index should refuse")
	}
}

// memFakeReader is the memory.Reader slice the StoreIndex needs, over one
// symbol and one fingerprint. queries counts QueryObservations calls —
// the observable cost of (re)building a StoreIndex symbol snapshot.
type memFakeReader struct {
	obs   []core.Observation
	fps   map[string]string
	meta  store.ServiceMeta
	found bool

	mu      sync.Mutex
	queries int
}

func (r *memFakeReader) QueryObservations(_ context.Context, _, _ string) ([]core.Observation, error) {
	r.mu.Lock()
	r.queries++
	r.mu.Unlock()
	return r.obs, nil
}

// queryCount returns how many times the symbol snapshot was loaded.
func (r *memFakeReader) queryCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.queries
}

func (r *memFakeReader) Fingerprint(_ context.Context, _, ref string) (string, bool, error) {
	h, ok := r.fps[ref]
	return h, ok, nil
}

func (r *memFakeReader) ListSourceRefs(_ context.Context, _ string) ([]string, error) {
	var out []string
	for ref := range r.fps {
		out = append(out, ref)
	}
	return out, nil
}

func (r *memFakeReader) GetServiceMeta(_ context.Context, _ string) (store.ServiceMeta, bool, error) {
	return r.meta, r.found, nil
}

func (r *memFakeReader) ClaimsForService(_ context.Context, _, _ string) ([]core.SemanticClaim, error) {
	return nil, nil
}

// memBackend is a QueryInterface whose ServiceMeta names a corpus root and
// which exposes the memory.Reader slice the way pkg/client.Client does.
type memBackend struct {
	*elisionFakeBackend
	root   string
	reader memory.Reader
}

func (b *memBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return b.root, "aaaa111", b.root != "", nil
}

func (b *memBackend) MemoryReader() memory.Reader { return b.reader }

// TestMemorySearchOverTheServer drives memory_search through tools/call
// against the service's own .fathom/memories.jsonl, with drift evaluated
// by a StoreIndex over the backend's reader.
func TestMemorySearchOverTheServer(t *testing.T) {
	root := t.TempDir()
	reader := &memFakeReader{
		obs: []core.Observation{{
			ServiceID: "svc", Package: "cfg", Symbol: "parseConfig",
			SourceRef: "internal/cfg/parse.go", ContentHash: "c1", Claim: "func parseConfig()",
		}},
		fps:   map[string]string{"internal/cfg/parse.go": "c1"},
		meta:  store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "aaaa111"},
		found: true,
	}
	ms := memory.NewFileStore(memory.Path(root))
	idx := memory.NewStoreIndex(reader)
	m := memWrite(t, ms, idx, "parseConfig validates the schema before returning.",
		[]memory.Anchor{{Kind: memory.AnchorSymbol, Ref: "parseConfig"}}, memory.Evidence{Kind: memory.EvidenceMachineCheck, Ref: "TestParseConfig"})
	if _, err := os.Stat(filepath.Join(root, ".fathom", "memories.jsonl")); err != nil {
		t.Fatalf("memories file not written: %v", err)
	}

	srv := NewServer(&memBackend{elisionFakeBackend: &elisionFakeBackend{}, root: root, reader: reader})
	payload := callTool(t, srv, "memory_search", map[string]any{"service": "svc", "query": "schema"})
	mems, _ := payload["memories"].([]any)
	if len(mems) != 1 {
		t.Fatalf("memories = %v, want 1", payload)
	}
	first := mems[0].(map[string]any)
	if first["id"] != m.ID || first["tier"] != string(core.TierVerified) || first["state"] != string(memory.StateFresh) {
		t.Fatalf("entry = %v, want %s verified fresh", first, m.ID)
	}

	// file-scoped: the symbol anchor is placed in parse.go by the StoreIndex.
	payload = callTool(t, srv, "memory_search", map[string]any{"service": "svc", "file": "internal/cfg/parse.go"})
	if mems, _ := payload["memories"].([]any); len(mems) != 1 {
		t.Fatalf("file-scoped memories = %v, want 1", payload)
	}
	payload = callTool(t, srv, "memory_search", map[string]any{"service": "svc", "file": "cmd/other.go"})
	if mems, _ := payload["memories"].([]any); len(mems) != 0 {
		t.Fatalf("unrelated file memories = %v, want 0", payload)
	}

	// BA-1, proven live: a limit that cuts the list discloses `elided`.
	memWrite(t, ms, idx, "parse.go has no side effects.",
		[]memory.Anchor{{Kind: memory.AnchorFile, Ref: "internal/cfg/parse.go"}}, memory.Evidence{Kind: memory.EvidenceDoc, Ref: "docs/cfg.md"})
	assertElided(t, "memory_search", callTool(t, srv, "memory_search", map[string]any{"service": "svc", "limit": 1}))

	// The reader's file moved: suspect, labeled, on the wire.
	reader.fps["internal/cfg/parse.go"] = "c2"
	reader.obs[0].ContentHash = "c2"
	reader.meta.IndexedCommit = "bbbb222"
	payload = callTool(t, srv, "memory_search", map[string]any{"service": "svc"})
	mems, _ = payload["memories"].([]any)
	if len(mems) != 2 {
		t.Fatalf("after drift = %v, want both memories", payload)
	}
	for _, raw := range mems {
		e := raw.(map[string]any)
		if e["state"] != string(memory.StateSuspect) || e["tier"] == "" {
			t.Fatalf("after drift entry = %v, want suspect and labeled", e)
		}
	}
}

// TestMemorySearchIndexCachedPerGeneration (A6 review finding 1): the
// server keeps ONE anchor index across tools/call, so the StoreIndex's
// per-(service, generation) symbol snapshot is loaded once per generation
// — a file-scoped search (ForFile → AnchorTouchesFile → SymbolFile) and
// the symbol probe behind it share it. Two calls at the same generation
// query the observation table once; a moved generation rebuilds it
// exactly once more; a further call at the new generation reuses that.
func TestMemorySearchIndexCachedPerGeneration(t *testing.T) {
	root := t.TempDir()
	reader := &memFakeReader{
		obs: []core.Observation{{
			ServiceID: "svc", Package: "cfg", Symbol: "parseConfig",
			SourceRef: "internal/cfg/parse.go", ContentHash: "c1", Claim: "func parseConfig()",
		}},
		fps:   map[string]string{"internal/cfg/parse.go": "c1"},
		meta:  store.ServiceMeta{ServiceID: "svc", Path: root, IndexedCommit: "aaaa111"},
		found: true,
	}
	ms := memory.NewFileStore(memory.Path(root))
	memWrite(t, ms, memory.NewStoreIndex(reader), "parseConfig validates the schema before returning.",
		[]memory.Anchor{{Kind: memory.AnchorSymbol, Ref: "parseConfig"}}, memory.Evidence{Kind: memory.EvidenceMachineCheck, Ref: "TestParseConfig"})
	baseline := reader.queryCount() // the write-side index's own load

	backend := &memBackend{elisionFakeBackend: &elisionFakeBackend{}, root: root, reader: reader}
	srv := NewServer(backend)
	search := func() {
		t.Helper()
		payload := callTool(t, srv, "memory_search", map[string]any{"service": "svc", "file": "internal/cfg/parse.go"})
		if mems, _ := payload["memories"].([]any); len(mems) != 1 {
			t.Fatalf("file-scoped memories = %v, want 1", payload)
		}
	}

	search()
	search()
	if n := reader.queryCount() - baseline; n != 1 {
		t.Fatalf("two searches at one generation loaded the symbol snapshot %d times, want 1", n)
	}

	// Generation moved (the file was re-ingested): rebuilt once, then reused.
	reader.meta.IndexedCommit = "bbbb222"
	search()
	search()
	if n := reader.queryCount() - baseline; n != 2 {
		t.Fatalf("after a generation change the snapshot was loaded %d times in total, want 2", n)
	}

	// The cache hands out one index for one backend; a backend with no
	// reader yields nil and is NOT pinned as nil.
	var c memoryIndexCache
	if a, b := c.get(backend), c.get(backend); a == nil || a != b {
		t.Fatalf("memoryIndexCache.get returned %v then %v, want one shared non-nil index", a, b)
	}
	var empty memoryIndexCache
	if idx := empty.get(&elisionFakeBackend{}); idx != nil {
		t.Fatalf("backend without a reader yielded an index: %v", idx)
	}
	if idx := empty.get(backend); idx == nil {
		t.Fatal("a nil result was pinned: the cache refused to build an index once a reader was available")
	}
}

// TestMemorySearchDegradesHonestly: a backend with no reader (or no
// recorded root) answers with a note and zero memories — never an error
// the agent must guess at, never an unlabelled memory.
func TestMemorySearchDegradesHonestly(t *testing.T) {
	srv := NewServer(&elisionFakeBackend{})
	payload := callTool(t, srv, "memory_search", map[string]any{"service": "svc"})
	if note, _ := payload["note"].(string); !strings.Contains(note, "not measurable") {
		t.Fatalf("payload = %v, want a not-measurable note", payload)
	}
	if mems, _ := payload["memories"].([]any); len(mems) != 0 {
		t.Fatalf("memories = %v, want none", payload)
	}
}

// TestMemorySearchOnAgentSurface: memory_search is on the agent allowlist.
func TestMemorySearchOnAgentSurface(t *testing.T) {
	on := false
	for _, name := range AgentSurfaceTools() {
		if name == "memory_search" {
			on = true
		}
	}
	if !on {
		t.Fatal("memory_search is not on the agent surface allowlist")
	}
}
