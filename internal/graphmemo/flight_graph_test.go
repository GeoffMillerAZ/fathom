package graphmemo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// isolateGraphStore gives a test clean snapshot, typed-edge and graph
// groups (they live in the registry they were registered in at package
// init, whatever Default is swapped to) and a normal level source,
// restoring both after — so a test that relies on a cold key passes under
// -count=N.
func isolateGraphStore(t *testing.T) *memo.Registry {
	t.Helper()
	reg := graphGroup.Registry()
	reset := func() {
		snapshotGroup.Reset(memo.ReasonWatermark)
		typedGroup.Reset(memo.ReasonWatermark)
		graphGroup.Reset(memo.ReasonWatermark)
		productGroup.Reset(memo.ReasonWatermark)
		reg.ResetWatermarks()
	}
	reset()
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelNormal })
	t.Cleanup(func() {
		reset()
		reg.SetLevelSource(tenancy.CurrentLevel)
	})
	return reg
}

// TestGraphFor_NestedFlightAtShedNeverWedges (review finding, RT-163): at
// shed one rebuild runs at a time, but a rebuild's nested flight (GraphFor
// loading typed edges through TypedFor) is part of that rebuild — it must
// not wait for the token its own parent holds, and no flight may wedge the
// token for everyone after it.
func TestGraphFor_NestedFlightAtShedNeverWedges(t *testing.T) {
	reg := isolateGraphStore(t)
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelShed })
	load := func(context.Context) ([]core.SymbolNode, error) { return fixtureNodes(), nil }
	none := func(context.Context) ([]core.Edge, error) { return nil, nil }
	sc := Scope{Store: "nested-shed", Service: "svc", Watermark: "w1"}

	ctx, cancel := context.WithTimeout(context.Background(), waitSafety)
	defer cancel()
	snap, err := SnapshotFor(ctx, sc, load)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GraphFor(ctx, snap, GraphBoth, none); err != nil {
		t.Fatalf("GraphFor at shed: %v (a nested flight waited on its parent's rebuild token)", err)
	}

	// A top-level TypedFor holds the token; a GraphFor meanwhile waits for
	// it only as long as its own ctx, and once the typed load finishes the
	// token is free for the next rebuild.
	sc2 := Scope{Store: "nested-shed", Service: "svc2", Watermark: "w1"}
	snap2, err := SnapshotFor(ctx, sc2, load)
	if err != nil {
		t.Fatal(err)
	}
	// svc2's graph build is measured (its syntax graph), so the 50 ms
	// waiter below starts a flight its deadline holds rather than being
	// declined (RT-191), and waits for the token.
	if _, err := GraphFor(ctx, snap2, GraphSyntax, nil); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	typedDone := make(chan error, 1)
	go func() {
		_, err := TypedFor(ctx, sc2, func(context.Context) ([]core.Edge, error) { <-gate; return nil, nil })
		typedDone <- err
	}()
	time.Sleep(20 * time.Millisecond)
	short, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()
	// The typed load holds the token until gate closes below, so a waiter
	// that outstayed its ctx could never return.
	var werr error
	returnsUnaided(t, "a waiter for the rebuild token", func() {
		_, werr = GraphFor(short, snap2, GraphResolved, none)
	})
	if werr != nil && !errors.Is(werr, context.DeadlineExceeded) {
		t.Fatalf("waiter err = %v", werr)
	}
	close(gate)
	if err := <-typedDone; err != nil {
		t.Fatal(err)
	}
	if _, err := GraphFor(ctx, snap2, GraphResolved, none); err != nil {
		t.Fatalf("GraphFor after the typed rebuild: %v (token wedged)", err)
	}
}

// TestPatchPackage_KeepsSnapshotDisclosure (review finding, SP-3): a
// single-file refresh patches the snapshot, and the patched snapshot still
// carries what the load returned beside the nodes — the suppression
// disclosure cached readers replay.
func TestPatchPackage_KeepsSnapshotDisclosure(t *testing.T) {
	isolateGraphStore(t)
	ctx := context.Background()
	sc := Scope{Store: "patch-extra", Service: "svc", Watermark: "w1"}
	if _, err := SnapshotForExtra(ctx, sc, func(context.Context) ([]core.SymbolNode, any, error) {
		return fixtureNodes(), "2 symbols suppressed", nil
	}); err != nil {
		t.Fatal(err)
	}
	PatchPackage(sc, "c", []core.SymbolNode{{Service: "svc", Package: "c", Name: "Log2", Kind: core.KindGoFunc, FilePath: "c/l.go", Line: 1}}, nil)
	snap, ok := snapshotGroup.Peek(snapKey{sc})
	if !ok {
		t.Fatal("snapshot not cached")
	}
	if len(snap.Named("Log2")) != 1 {
		t.Fatal("patch did not land")
	}
	if snap.Extra != "2 symbols suppressed" {
		t.Fatalf("patched snapshot lost its disclosure: Extra = %v", snap.Extra)
	}
}
