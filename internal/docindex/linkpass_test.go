package docindex

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func linkPassStore(t *testing.T, obs []core.Observation) *store.SQLiteStore {
	t.Helper()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "lp.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	if err := sq.PutObservations(context.Background(), obs); err != nil {
		t.Fatal(err)
	}
	return sq
}

func lpObs(ref, sym, hash string, ingested int64) core.Observation {
	return core.Observation{ServiceID: "svc", SourceRef: ref, Symbol: sym, ContentHash: hash,
		IngestedAt: time.Unix(0, ingested).UTC(), SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified}
}

// TestLinkPassSymbolIndexIsResolveSymbolNow (EW-49): the link pass's
// one-read symbol index answers every name exactly as the per-name point
// lookup drift keeps (resolveSymbolNow): unique, ambiguous, suffix either
// direction, a symbol seen twice (its last row in ingested order wins), and
// names nothing matches.
func TestLinkPassSymbolIndexIsResolveSymbolNow(t *testing.T) {
	sq := linkPassStore(t, []core.Observation{
		lpObs("a.go:1", "Frob", "h1", 30),
		lpObs("b.go:4", "pkg.Frob", "h2", 10),
		lpObs("c.go:9", "Solo", "h3a", 20),
		lpObs("c2.go:9", "Solo", "h3b", 5), // ingested first: h3a is the last row
		lpObs("d.go:2", "Other.Deep.Leaf", "h4", 10),
		lpObs("e.go:7", "Widget.Render", "h5", 1),
		lpObs("f.go:3", "Render", "h6", 40),
		lpObs("g.go:3", "x.Y.Z", "h7", 2),
		lpObs("h.go:5", "Alone.Method", "h8", 3),
		lpObs("i.go", "NoLine", "h9", 3),
	})
	ctx := context.Background()
	r := NewStoreResolver(ctx, sq, "svc").(*storeResolver)
	names := []string{"Frob", "pkg.Frob", "q.pkg.Frob", "Solo", "Leaf", "Deep.Leaf", "Other.Deep.Leaf", "Z.Other.Deep.Leaf",
		"Render", "Widget.Render", "Z", "Y.Z", "x.Y.Z", "Method", "Alone.Method", "NoLine", "Missing", "a.Missing"}
	for _, name := range names {
		want := resolveSymbolNow(ctx, sq, "svc", name)
		hash, ok := r.ResolveSymbol(name)
		if ok != want.Exists || hash != want.Hash {
			t.Errorf("%s: index (%q, %v), point lookup (%q, %v)", name, hash, ok, want.Hash, want.Exists)
		}
		if got := r.index.resolve(name, new(int)); got != want {
			t.Errorf("%s: index resolution %+v, point lookup %+v", name, got, want)
		}
	}
	if r.symbolReads != 1 {
		t.Fatalf("symbol table reads = %d for %d names, want 1", r.symbolReads, len(names))
	}
}

// linkWork links n doc units, each naming two of n symbols and one path,
// ten units a file through one LinkPass, and returns the resolver's store
// reads, symbol reads and index probes and the pass's open-link reads.
func linkWork(t *testing.T, n int) (storeReads, symbolReads, probes, openReads int) {
	t.Helper()
	var obs []core.Observation
	for i := 0; i < n; i++ {
		obs = append(obs, lpObs(fmt.Sprintf("pkg%d/f.go:%d", i, i+1), fmt.Sprintf("Type%d.Method%d", i, i), fmt.Sprintf("h%d", i), int64(i)))
		obs = append(obs, lpObs(fmt.Sprintf("pkg%d/f.go:%d", i, i+2), fmt.Sprintf("Type%d", i), fmt.Sprintf("t%d", i), int64(i)))
	}
	sq := linkPassStore(t, obs)
	ctx := context.Background()
	var units []store.DocUnit
	for i := 0; i < n; i++ {
		units = append(units, store.DocUnit{UnitID: fmt.Sprintf("u%d", i), Path: "docs/d.md", HeadingPath: "D", LineStart: 1, LineEnd: 3, Kind: "section",
			Body: fmt.Sprintf("`Method%d` and `Type%d.Method%d` live in `pkg%d/f.go`.\n", i, i, i, i)})
	}
	r := NewStoreResolver(ctx, sq, "svc").(*storeResolver)
	pass := NewLinkPass(sq, "svc", r, 1)
	links := 0
	for i := 0; i < len(units); i += 10 {
		n, err := pass.Link(ctx, units[i:min(i+10, len(units))], nil)
		if err != nil {
			t.Fatal(err)
		}
		links += n
	}
	if links == 0 {
		t.Fatal("nothing linked: the fixture does not exercise the resolver")
	}
	return r.storeReads, r.symbolReads, r.symbolProbes, pass.OpenLinkReads()
}

// TestLinkPassResolutionIsLinear (EW-49, EW-50; O362): the link pass's
// work grows with its input, never with names times symbols or units times
// links. Twice the units over twice the symbols costs one symbol-table read
// and one open-links read either way and at most 2.2 times the store reads
// and index probes. Counters, not a wall clock (EW-7).
func TestLinkPassResolutionIsLinear(t *testing.T) {
	r1, s1, p1, o1 := linkWork(t, 150)
	r2, s2, p2, o2 := linkWork(t, 300)
	if s1 != 1 || s2 != 1 {
		t.Fatalf("symbol table reads per pass = %d at N, %d at 2N; want 1", s1, s2)
	}
	if o1 != 1 || o2 != 1 {
		t.Fatalf("open doc_links reads per pass = %d at N, %d at 2N; want 1", o1, o2)
	}
	for _, c := range []struct {
		what   string
		n, two int
	}{{"store reads", r1, r2}, {"index probes", p1, p2}} {
		if c.n == 0 || float64(c.two) > 2.2*float64(c.n) {
			t.Errorf("%s: %d at N, %d at 2N; want at most 2.2x", c.what, c.n, c.two)
		}
	}
}
