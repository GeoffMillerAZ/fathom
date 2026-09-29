package memo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// isolateGraphStore gives a test clean snapshot, typed-edge and graph
// groups (they live in the registry they were registered in at package
// init, whatever Default is swapped to) and a normal level source,
// restoring both after — so a test that relies on a cold key passes under
// -count=N.
func isolateGraphStore(t *testing.T) *Registry {
	t.Helper()
	reg := graphGroup.reg
	reset := func() {
		snapshotGroup.Reset(ReasonWatermark)
		typedGroup.Reset(ReasonWatermark)
		graphGroup.Reset(ReasonWatermark)
		productGroup.Reset(ReasonWatermark)
		reg.mu.Lock()
		reg.marks = make(map[string]string)
		reg.mu.Unlock()
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
	gate := make(chan struct{})
	typedDone := make(chan error, 1)
	go func() {
		_, err := TypedFor(ctx, sc2, func(context.Context) ([]core.Edge, error) { <-gate; return nil, nil })
		typedDone <- err
	}()
	time.Sleep(20 * time.Millisecond)
	short, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()
	start := time.Now()
	if _, err := GraphFor(short, snap2, GraphResolved, none); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("a waiter for the rebuild token outstayed its ctx")
	}
	close(gate)
	if err := <-typedDone; err != nil {
		t.Fatal(err)
	}
	if _, err := GraphFor(ctx, snap2, GraphResolved, none); err != nil {
		t.Fatalf("GraphFor after the typed rebuild: %v (token wedged)", err)
	}
}

// TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights (review finding,
// RT-72): Keep holds a flight only past half a stated expected cost — a
// Keep group with no expected cost cancels like any other — and the refuse
// drop cancels every flight in progress, kept or not, and never caches its
// result.
func TestGroup_KeepNeedsExpectedCostAndRefuseCancelsFlights(t *testing.T) {
	reg := NewRegistry()
	reg.SetLevelSource(func() tenancy.Level { return tenancy.LevelNormal })
	g := NewGroupIn[skey, int](reg, "keep-no-cost", Options{WatermarkScoped: true, Keep: true})
	cancelled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	_, _ = g.Get(ctx, skey{"svc", "w1", "a"}, func(bctx context.Context) (int, int64, error) {
		<-bctx.Done()
		close(cancelled)
		return 0, 0, bctx.Err()
	})
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("a Keep flight with no expected cost outlived its last waiter")
	}

	// Past half its expected cost the flight is kept… until refuse.
	kg := NewGroupIn[skey, int](reg, "keep-cost", Options{WatermarkScoped: true, Keep: true, ExpectedCost: time.Nanosecond})
	k := skey{"svc", "w1", "b"}
	started := make(chan struct{})
	stopped := make(chan struct{})
	kctx, kcancel := context.WithCancel(context.Background())
	go func() { <-started; kcancel() }()
	_, _ = kg.Get(kctx, k, func(bctx context.Context) (int, int64, error) {
		close(started)
		<-bctx.Done()
		close(stopped)
		return 9, 1, nil
	})
	select {
	case <-stopped:
		t.Fatal("a kept flight was cancelled when its waiter left")
	case <-time.After(30 * time.Millisecond):
	}
	reg.Drop(ReasonRefuse)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("refuse did not cancel the flight in progress")
	}
	time.Sleep(10 * time.Millisecond)
	if _, ok := kg.Peek(k); ok {
		t.Fatal("a flight cancelled at refuse was cached")
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
