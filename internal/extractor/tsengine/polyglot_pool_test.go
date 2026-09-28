package tsengine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestPolyglotIngestHoldsModulePool is LB-F15's proof (design §9): an
// ingest of a fixture holding every registered tags dialect's fixtures —
// more grammar variants than the cap — never holds more than
// MaxLiveModules live modules, charges each compiled grammar to the memo
// registry, and leaves no module open once the pipeline closes.
func TestPolyglotIngestHoldsModulePool(t *testing.T) {
	dir := t.TempDir()
	seen := map[string]bool{}
	for _, id := range extractor.TagsDialects() {
		files, err := filepath.Glob(filepath.Join("..", "dialects", id, "testdata", "*"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, ".golden.json") {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(dir, id, filepath.Base(f))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, b, 0o644); err != nil {
				t.Fatal(err)
			}
			seen[id] = true
		}
	}
	for _, id := range extractor.TagsDialects() {
		if !seen[id] {
			t.Fatalf("dialect %s has no fixture to ingest", id)
		}
	}

	tsengine.CloseIdleParsers()
	memo.Default.Drop(memo.ReasonIdle)
	base := tsengine.LiveModules()
	tsengine.ResetPoolPeak()

	p := fusion.New(connector.NewLocalConnector(), store.NewMemoryStore(), fusion.Config{})
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "polyglot", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.TSEngineFaults != 0 {
		t.Fatalf("%d engine faults on the fixtures", rep.TSEngineFaults)
	}
	peak := tsengine.PoolPeak()
	t.Logf("ingest: %d dialects, pool peak %d live modules", len(seen), peak)
	if peak == 0 {
		t.Fatal("the ingest leased no parser from the pool")
	}
	if peak > tsengine.MaxLiveModules {
		t.Fatalf("pool peak %d live modules, cap %d", peak, tsengine.MaxLiveModules)
	}
	if live := tsengine.LiveModules() - base; live > tsengine.MaxLiveModules {
		t.Fatalf("%d live modules after the ingest, cap %d", live, tsengine.MaxLiveModules)
	}

	// The ingest above only compiles a dialect's grammar when fusion
	// actually reaches it: a dialect fusion still dispatches to its CGO/
	// regex incumbent (a port not yet merged, §11) is reached only through
	// the file-resolved-edges pass (design §7.2), and that pass skips any
	// dialect with no imports.scm (an L1/L2-target dialect, §13.2 step 13:
	// "L3 only"). So this warms every remaining registered grammar
	// directly — the assertion below is about every embedded blob's
	// tenancy accounting (LB-F15), not about fusion's own dispatch
	// routing, which other tests already cover.
	ds, err := extractor.LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.Grammar == nil {
			continue
		}
		wp, err := tsengine.NewParser(context.Background(), d.Grammar.Blob, d.Grammar.EntryPoint)
		if err != nil {
			t.Fatalf("warm %s (%s): %v", d.ID, d.Grammar.Blob, err)
		}
		wp.Close()
	}

	lock, err := tsengine.LoadGrammarLock()
	if err != nil {
		t.Fatal(err)
	}
	var want int64
	for _, r := range lock.Grammars {
		want += r.BlobBytes
	}
	var charged int64
	for _, g := range memo.Stats() {
		if g.Name == "grammar_code" {
			charged = g.Bytes
		}
	}
	if charged != want {
		t.Fatalf("compiled grammars charged %d bytes to the memo registry, want every grammar's %d", charged, want)
	}

	p.Close()
	if live := tsengine.LiveModules() - base; live != 0 {
		t.Fatalf("%d module(s) left open after Pipeline.Close", live)
	}
	if n := tsengine.PooledModules(); n != 0 {
		t.Fatalf("the pool still books %d module(s)", n)
	}

	// The fixture's variants fit the real cap exactly; under a cap of 3 the
	// same ingest evicts and re-instantiates as it goes, holds the cap, and
	// extracts the same symbols.
	restore := tsengine.SetPoolCapForTest(3)
	defer restore()
	tsengine.ResetPoolPeak()
	st := store.NewMemoryStore()
	p2 := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	defer p2.Close()
	rep2, err := p2.Ingest(context.Background(), core.ServiceRef{ID: "polyglot", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if peak := tsengine.PoolPeak(); peak > 3 {
		t.Fatalf("pool peak %d under a cap of 3", peak)
	}
	if rep2.TSEngineFaults != 0 || rep2.Symbols != rep.Symbols {
		t.Fatalf("under eviction: %d symbols, %d faults; at the full cap %d symbols", rep2.Symbols, rep2.TSEngineFaults, rep.Symbols)
	}
}
