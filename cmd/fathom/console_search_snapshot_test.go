package main

// console_search_snapshot_test.go — RT-120 (docs/design/responsible-tenant.md
// §14): the ⌘K palette's Symbols group reads the daemon's one shared symbol
// snapshot (internal/memo, RT-75), never a private per-console copy of the
// fleet's symbols.

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

func TestConsoleSearchUsesSharedSnapshot(t *testing.T) {
	f := newConsoleFixture(t)
	ctx := context.Background()
	services, err := f.client.Services(ctx)
	if err != nil || len(services) == 0 {
		t.Fatalf("services = %v, %v", services, err)
	}
	sc := &consoleSearchCache{}
	if _, total, err := sc.lookupSymbols(ctx, f.client, services, []string{"e"}, 8); err != nil || total == 0 {
		t.Fatalf("lookupSymbols = %d, %v; want matches", total, err)
	}
	// Every service's snapshot is now the shared one: a reader keyed on
	// the same (store, service, watermark) gets it without a load.
	checked := 0
	for _, svc := range services {
		sc := consoleScope(ctx, f.client, svc)
		if sc.Watermark == "" {
			continue // an empty watermark is never cached (I51)
		}
		checked++
		loads := 0
		snap, err := memo.SnapshotFor(ctx, sc, func(context.Context) ([]core.SymbolNode, error) {
			loads++
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if loads != 0 || snap == nil || snap.Len() == 0 {
			t.Errorf("%s: a reader after the palette loaded %d times (len %d), want the palette's shared snapshot", svc, loads, snap.Len())
		}
	}
	if checked == 0 {
		t.Fatal("no fixture service has a watermark: nothing shared was checked")
	}
	// The palette holds no symbols of its own.
	typ := reflect.TypeOf(consoleSearchCache{})
	for i := 0; i < typ.NumField(); i++ {
		if ft := typ.Field(i).Type.String(); ft == "[]main.consoleSymbolEntry" || ft == "map[string][]main.consoleSymbolEntry" || ft == "[]core.SymbolNode" {
			t.Errorf("consoleSearchCache.%s keeps a private symbol copy (%s)", typ.Field(i).Name, ft)
		}
	}
}
