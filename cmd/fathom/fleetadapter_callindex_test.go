package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// RT-84: the daemon's fleet adapter carries the call-index and name-lookup
// capabilities mcp.fleetCallers and crossServiceCallees use, and answers
// them from a real store's indexes.
func TestFleetAdapter_CallIndexCapabilities(t *testing.T) {
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "a", Symbol: "Caller", Package: "p", SourceRef: "a.go", SourceKind: core.KindGoFunc, Calls: []string{"out.FormatCents"}},
		{ServiceID: "b", Symbol: "FormatCents", Package: "out", SourceRef: "b.go", SourceKind: core.KindGoFunc},
	}); err != nil {
		t.Fatal(err)
	}
	for _, svc := range []string{"a", "b"} {
		if _, err := st.BackfillDerivedTables(ctx, svc, store.BackfillOptions{Calls: true}); err != nil {
			t.Fatal(err)
		}
	}
	f := newTestFleetAdapter(newCountingConnector(), st)
	var idx interface {
		CallersOfKeys(context.Context, []string) ([]store.CallerRow, error)
		CallsIndexed(context.Context, string) bool
		SymbolSourcesForNames(context.Context, string, []string) ([]store.SymbolSource, error)
	} = f
	if !idx.CallsIndexed(ctx, "a") {
		t.Fatal("a backfilled store reports service a not call-indexed")
	}
	rows, err := idx.CallersOfKeys(ctx, []string{"FormatCents"})
	if err != nil || len(rows) != 1 || rows[0].ServiceID != "a" || rows[0].Symbol != "Caller" {
		t.Fatalf("CallersOfKeys = %+v, %v", rows, err)
	}
	srcs, err := idx.SymbolSourcesForNames(ctx, "b", []string{"FormatCents"})
	if err != nil || len(srcs) != 1 || srcs[0].Package != "out" {
		t.Fatalf("SymbolSourcesForNames = %+v, %v", srcs, err)
	}
}
