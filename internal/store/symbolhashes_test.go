package store

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// TestSymbolHashesIsObservationsForSymbolsSuperset pins the bulk read's
// contract (EW-49): for every name, ObservationsForSymbol(name) is exactly
// the rows of SymbolHashes whose symbol is one of the name's candidates,
// in SymbolHashes' order — ingested_at, stable over rowid, written out of
// that order here — and a suppressed file's rows leave both alike.
func TestSymbolHashesIsObservationsForSymbolsSuperset(t *testing.T) {
	sup := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, sup)
	if _, err := scope.AppendSuppression(sup, scope.Suppression{
		Service: "svc", Pattern: "secret/**", Reason: "key material",
		CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI,
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "sh.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := func(n int64) time.Time { return time.Unix(0, n).UTC() }
	ob := func(ref, sym, hash string, ingested int64) core.Observation {
		return core.Observation{ServiceID: "svc", SourceRef: ref, Symbol: sym, ContentHash: hash, IngestedAt: at(ingested),
			SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified}
	}
	obs := []core.Observation{
		ob("a.go:1", "Frob", "h1", 30),
		ob("b.go:4", "pkg.Frob", "h2", 10),
		ob("c.go:9", "Frob", "h3", 20),
		ob("d.go:2", "Other.Frob.Deep", "h4", 10),
		ob("secret/k.go:1", "pkg.Frob", "h5", 5),
		ob("e.go:7", "Unrelated", "h6", 1),
		ob("f.go:3", "Frobnicate", "h7", 40),
	}
	if err := s.PutObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}
	all, err := s.SymbolHashes(ctx, "svc")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all {
		if strings.HasPrefix(r.SourceRef, "secret/") {
			t.Fatalf("a suppressed file's row was returned: %+v", r)
		}
	}
	for _, name := range []string{"Frob", "pkg.Frob", "x.pkg.Frob", "Deep", "Frob.Deep", "Unrelated", "Missing"} {
		cands := map[string]bool{}
		for _, c := range SymbolCandidates(name) {
			cands[c] = true
		}
		var want []SymbolHash
		for _, r := range all {
			if cands[r.Symbol] || strings.HasSuffix(r.Symbol, "."+name) {
				want = append(want, r)
			}
		}
		got, err := s.ObservationsForSymbol(ctx, "svc", name)
		if err != nil {
			t.Fatal(err)
		}
		var have []SymbolHash
		for _, o := range got {
			have = append(have, SymbolHash{Symbol: o.Symbol, SourceRef: o.SourceRef, ContentHash: o.ContentHash})
		}
		if !reflect.DeepEqual(have, want) {
			t.Errorf("%s: ObservationsForSymbol = %+v\nSymbolHashes' candidates = %+v", name, have, want)
		}
	}
}
