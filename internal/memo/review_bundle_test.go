package memo

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func bundleNodes(name string) []core.SymbolNode {
	return []core.SymbolNode{{Service: "svc", Package: "p", Name: name, Kind: core.KindGoFunc, FilePath: "p/f.go", Line: 1}}
}

func TestReviewBundleRejectsSnapshotPublishedBeforeGraph(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	load := func(context.Context) ([]core.SymbolNode, any, error) { return bundleNodes("Old"), nil, nil }
	typed := func(context.Context) ([]core.Edge, error) { return nil, nil }
	if _, _, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		patchPackage(sc, "p", bundleNodes("New"), nil, func(phase string) {
			if phase == "snapshot" {
				close(entered)
				<-release
			}
		})
		close(done)
	}()
	<-entered
	_, _, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if !errors.Is(err, ErrCoherentGraphChanged) {
		t.Fatalf("published snapshot with old graph accepted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, _, _, err := SnapshotGraphBothForExtra(ctx, sc, load, typed); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled reader: %v", err)
	}
	close(release)
	<-done
	snap, graph, _, _, seq, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if err != nil || !CoherentGraphCurrent(seq) || snap.Nodes[0].Name != "New" || !callgraph.Reachable(graph, snap.Nodes[0].Key(), snap.Nodes[0].Key()) {
		t.Fatalf("after patch: snap=%v graph=%v err=%v", snap, graph, err)
	}
}

func TestReviewBundleRetriesOldSnapshotAfterNewGraph(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	load := func(context.Context) ([]core.SymbolNode, any, error) { return bundleNodes("Old"), nil, nil }
	typed := func(context.Context) ([]core.Edge, error) { return nil, nil }
	if _, _, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	type result struct {
		snap  *Snapshot
		graph *callgraph.Graph
		err   error
	}
	done := make(chan result, 1)
	go func() {
		snap, graph, _, _, _, err := snapshotGraphBothForExtra(context.Background(), sc, load, typed, func() { once.Do(func() { close(entered); <-release }) })
		done <- result{snap, graph, err}
	}()
	<-entered
	PatchPackage(sc, "p", bundleNodes("New"), nil)
	close(release)
	r := <-done
	if r.err != nil || r.snap.Nodes[0].Name != "New" || !callgraph.Reachable(r.graph, r.snap.Nodes[0].Key(), r.snap.Nodes[0].Key()) {
		t.Fatalf("old snapshot/new graph was returned: %+v", r)
	}
}

func TestReviewBundleDropScopeIsOnePublication(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	var rows atomic.Value
	rows.Store(bundleNodes("Old"))
	load := func(context.Context) ([]core.SymbolNode, any, error) {
		return rows.Load().([]core.SymbolNode), nil, nil
	}
	typed := func(context.Context) ([]core.Edge, error) { return nil, nil }
	if _, _, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	rows.Store(bundleNodes("New"))
	go func() {
		dropScope(sc, func(phase string) {
			if phase == "snapshot" {
				close(entered)
				<-release
			}
		})
		close(done)
	}()
	<-entered
	_, _, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if !errors.Is(err, ErrCoherentGraphChanged) {
		t.Fatalf("partial drop accepted: %v", err)
	}
	close(release)
	<-done
	snap, graph, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if err != nil || snap.Nodes[0].Name != "New" || !callgraph.Reachable(graph, snap.Nodes[0].Key(), snap.Nodes[0].Key()) {
		t.Fatalf("after drop: snap=%v graph=%v err=%v", snap, graph, err)
	}
	// The opposite interleaving holds the old snapshot while DropScope
	// removes the graph, then resumes the graph read at the same watermark.
	entered2, release2 := make(chan struct{}), make(chan struct{})
	var once sync.Once
	type result struct {
		snap  *Snapshot
		graph *callgraph.Graph
		err   error
	}
	readDone := make(chan result, 1)
	go func() {
		s, g, _, _, _, e := snapshotGraphBothForExtra(context.Background(), sc, load, typed, func() { once.Do(func() { close(entered2); <-release2 }) })
		readDone <- result{s, g, e}
	}()
	<-entered2
	rows.Store(bundleNodes("After"))
	DropScope(sc)
	if cached, ok := PeekSnapshot(sc); ok {
		t.Fatalf("drop retained snapshot: %s", cached.Nodes[0].Name)
	}
	close(release2)
	r := <-readDone
	if r.err != nil || r.snap.Nodes[0].Name != "After" || !callgraph.Reachable(r.graph, r.snap.Nodes[0].Key(), r.snap.Nodes[0].Key()) {
		t.Fatalf("old snapshot after drop: name=%s snap=%v graph=%v err=%v", r.snap.Nodes[0].Name, r.snap, r.graph, r.err)
	}
}

func TestReviewBundleTypedRefreshDoesNotWaitOnReadLock(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	load := func(context.Context) ([]core.SymbolNode, any, error) { return bundleNodes("Old"), nil, nil }
	var calls atomic.Int32
	typed := func(context.Context) ([]core.Edge, error) {
		if calls.Add(1) == 1 {
			PatchPackage(sc, "p", bundleNodes("New"), []core.Edge{})
		}
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, _, _, _, err := SnapshotGraphBothForExtra(ctx, sc, load, typed)
	if err != nil && !errors.Is(err, ErrCoherentGraphChanged) {
		t.Fatalf("typed refresh blocked graph acquisition: %v", err)
	}
	if calls.Load() == 0 {
		t.Fatal("typed loader did not run")
	}
}

func TestLegacyGraphForTypedCannotRepublishAfterDropScope(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	var rows atomic.Value
	rows.Store(bundleNodes("Old"))
	load := func(context.Context) ([]core.SymbolNode, any, error) {
		return rows.Load().([]core.SymbolNode), nil, nil
	}
	typed := func(context.Context) ([]core.Edge, error) { return nil, nil }
	old, oldGraph, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if err != nil || !callgraph.Reachable(oldGraph, old.Nodes[0].Key(), old.Nodes[0].Key()) {
		t.Fatalf("old graph prerequisite: %v", err)
	}
	rows.Store(bundleNodes("New"))
	DropScope(sc)
	if _, _, _, err := GraphForTyped(context.Background(), old, GraphBoth, typed); !errors.Is(err, ErrCoherentGraphChanged) {
		t.Fatalf("legacy held snapshot republished graph: %v", err)
	}
	if _, ok := graphGroup.Peek(graphKey{sc, GraphBoth}); ok {
		t.Fatal("legacy held snapshot cached graph after drop")
	}
	fresh, graph, _, _, _, err := SnapshotGraphBothForExtra(context.Background(), sc, load, typed)
	if err != nil || fresh.Nodes[0].Name != "New" || !callgraph.Reachable(graph, fresh.Nodes[0].Key(), fresh.Nodes[0].Key()) || callgraph.Reachable(graph, old.Nodes[0].Key(), old.Nodes[0].Key()) {
		t.Fatalf("fresh bundle mixed with legacy graph: fresh=%v graph=%v err=%v", fresh, graph, err)
	}
}

func TestSnapshotFlightCannotRepublishAfterDropScope(t *testing.T) {
	isolateGraphStore(t)
	sc := Scope{Store: t.Name(), Service: "svc", Watermark: "w1"}
	entered, release := make(chan struct{}), make(chan struct{})
	loader := func(context.Context) ([]core.SymbolNode, any, error) {
		close(entered)
		<-release
		return bundleNodes("Old"), nil, nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := SnapshotForExtra(context.Background(), sc, loader)
		done <- err
	}()
	<-entered
	DropScope(sc)
	close(release)
	if err := <-done; !errors.Is(err, ErrCoherentGraphChanged) {
		t.Fatalf("old snapshot flight accepted after drop: %v", err)
	}
	if snap, ok := PeekSnapshot(sc); ok {
		t.Fatalf("old snapshot flight cached after drop: %v", snap)
	}
}
