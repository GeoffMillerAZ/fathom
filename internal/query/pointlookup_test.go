package query_test

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// noFullLoad is a store whose whole-service observation load is counted.
type noFullLoad struct {
	store.Store
	fullLoads int
}

func (n *noFullLoad) QueryObservations(ctx context.Context, svc, concern string) ([]core.Observation, error) {
	n.fullLoads++
	return n.Store.QueryObservations(ctx, svc, concern)
}

func pointLookupFixture() []core.Observation {
	return []core.Observation{
		{ServiceID: "s", SourceRef: "a.go", SourceKind: core.KindGoFunc, Symbol: "ValidateToken", Package: "auth", Calls: []string{"strings.TrimSpace"}},
		{ServiceID: "s", SourceRef: "b.go", SourceKind: core.KindGoMethod, Symbol: "Registry.CloseCase", Package: "reg", Calls: []string{"auth.ValidateToken", "Helper"}},
		{ServiceID: "s", SourceRef: "c.go", SourceKind: core.KindGoFunc, Symbol: "closecase", Package: "x", Calls: []string{"pkg/auth.ValidateToken"}},
		{ServiceID: "s", SourceRef: "d.go", SourceKind: core.KindGoFunc, Symbol: "Other", Package: "x", Calls: []string{"ValidateTokenX", "Helper"}},
		{ServiceID: "t", SourceRef: "e.go", SourceKind: core.KindGoFunc, Symbol: "ValidateToken", Package: "auth", Calls: []string{"ValidateToken"}},
	}
}

func names(ns []core.SymbolNode) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.FilePath+":"+n.Name)
	}
	sort.Strings(out)
	return out
}

// RT-85 (§4.5): find_symbol and find_callers are SQL point lookups — the
// (service_id, symbol) index and the observation_calls index — never a
// load of the service's whole observation set, and they answer exactly
// what the full-load match answered.
func TestFindSymbol_PointLookupNoFullLoad(t *testing.T) {
	ctx := context.Background()
	for _, backend := range []string{"sqlite", "memory"} {
		t.Run(backend, func(t *testing.T) {
			var base store.Store
			if backend == "sqlite" {
				st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { st.Close() })
				base = st
			} else {
				base = store.NewMemoryStore()
			}
			if err := base.PutObservations(ctx, pointLookupFixture()); err != nil {
				t.Fatal(err)
			}
			if _, err := base.BackfillDerivedTables(ctx, "s", store.BackfillOptions{Calls: true}); err != nil {
				t.Fatal(err)
			}
			want := query.New(base)
			st := &noFullLoad{Store: base}
			e := query.New(st)
			for _, q := range []string{"ValidateToken", "CloseCase", "Registry.CloseCase", "reg.Registry.CloseCase", "(*Registry).CloseCase", "auth.ValidateToken", "Nope"} {
				got, err := e.FindSymbol(ctx, "s", q)
				if err != nil {
					t.Fatal(err)
				}
				exp, _ := want.FindSymbolFullScan(ctx, "s", q)
				if !reflect.DeepEqual(names(got), names(exp)) {
					t.Errorf("FindSymbol(%q) = %v, want %v", q, names(got), names(exp))
				}
			}
			for _, q := range []string{"ValidateToken", "auth.ValidateToken", "Helper", "TrimSpace", "Nope"} {
				got, err := e.FindCallers(ctx, "s", q)
				if err != nil {
					t.Fatal(err)
				}
				exp, _ := want.FindCallersFullScan(ctx, "s", q)
				if !reflect.DeepEqual(names(got), names(exp)) {
					t.Errorf("FindCallers(%q) = %v, want %v", q, names(got), names(exp))
				}
			}
			if st.fullLoads != 0 {
				t.Fatalf("FindSymbol/FindCallers loaded the whole service %d times, want 0", st.fullLoads)
			}
		})
	}
}
