package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// The Operate page carries the tenancy projection (design §13.3; RT-52's
// block): the ladder level, footprint (self + children) against the
// budget, and the mode — the same block /statusz serves — and says in
// words when no governor runs, never a blank tile (I38).
func TestOperatePageShowsTenancy(t *testing.T) {
	f := newConsoleFixture(t)

	// No governor in this process: the tile says so.
	resetOperateCache()
	_, _, body := f.get(t, "/operate")
	if !strings.Contains(body, "no governor runs in this process") {
		t.Errorf("/operate without a governor must say so; body lacks the absent tenancy tile")
	}

	r := newGovRig(t)
	r.ticks(40)
	r.host.set(3200*tenancy.MiB, 200*tenancy.MiB)
	r.ticks(2) // → shed
	activeGovernor.Store(r.g)
	defer activeGovernor.Store(nil)
	resetOperateCache()

	_, _, body = f.get(t, "/operate")
	for _, want := range []string{"tenancy", "shed", "fathom resources"} {
		if !strings.Contains(body, want) {
			t.Errorf("/operate missing %q in its tenancy tile", want)
		}
	}
	var om struct {
		Tenancy *governorSnapshot `json:"tenancy"`
	}
	if err := json.Unmarshal(f.pageJSON(t, "/operate").Model, &om); err != nil {
		t.Fatal(err)
	}
	if om.Tenancy == nil || om.Tenancy.Level != "shed" || om.Tenancy.Source != "live" {
		t.Fatalf("/operate model tenancy = %+v, want the live shed block", om.Tenancy)
	}
}

// TestOperateResourcesOffTheRequestPath (RT-121, RT-113's rule for the
// Operate projection): the Tenancy panel's report — a process census and
// a walk of ~/.fathom — is kept for its own TTL and refreshed by one flight
// outside every lock while the last report is served, and the Operate
// projection builds outside its cache's lock, so no request waits behind
// a directory walk another request started.
func TestOperateResourcesOffTheRequestPath(t *testing.T) {
	resetOperateCache()
	t.Cleanup(resetOperateCache)
	release := make(chan struct{})
	var builds atomic.Int32
	prev := operateResourcesBuilder
	operateResourcesBuilder = func(selfURL string) resourcesReport {
		if builds.Add(1) == 2 {
			<-release
		}
		return resourcesReport{GeneratedAt: fmt.Sprint(builds.Load())}
	}
	t.Cleanup(func() { operateResourcesBuilder = prev })
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	if rep := operateResourcesCached("http://x", t0); rep.GeneratedAt != "1" {
		t.Fatalf("cold = %q", rep.GeneratedAt)
	}
	if rep := operateResourcesCached("http://x", t0.Add(time.Second)); rep.GeneratedAt != "1" || builds.Load() != 1 {
		t.Fatalf("inside the TTL = %q after %d builds", rep.GeneratedAt, builds.Load())
	}
	later := t0.Add(operateResourcesTTL + time.Second)
	for i := 0; i < 4; i++ {
		// The census build stays blocked (release closes below), so a stale
		// read that waited on it could not return: returning is "served
		// the last report at once".
		var rep resourcesReport
		returnsUnaided(t, "a stale operate read while the census rebuilds", func() {
			rep = operateResourcesCached("http://x", later)
		})
		if rep.GeneratedAt != "1" {
			t.Errorf("a stale read got %q", rep.GeneratedAt)
		}
	}
	waitUntil(t, func() bool { return builds.Load() == 2 })
	close(release)
	waitUntil(t, func() bool { return operateResourcesCached("http://x", later).GeneratedAt == "2" })
	if n := builds.Load(); n != 2 {
		t.Errorf("%d builds; want one flight for the stale reads", n)
	}

	// The projection builds outside the operate cache's lock.
	hold := make(chan struct{})
	prevP := operateProjectionBuilder
	operateProjectionBuilder = func(d *consoleDeps, m observatoryModel, resolved console.ResolvedCallLog, now time.Time) operateProjection {
		<-hold
		return operateProjection{GeneratedAt: now}
	}
	t.Cleanup(func() { operateProjectionBuilder = prevP })
	d := &consoleDeps{selfURL: "http://x"}
	got := make(chan operateProjection, 2)
	go func() { got <- operateNow(d, observatoryModel{}, t0) }()
	time.Sleep(20 * time.Millisecond)
	go func() { got <- operateNow(d, observatoryModel{}, t0) }()
	locked := make(chan struct{})
	go func() {
		operateCache.Lock()
		operateCache.Unlock()
		close(locked)
	}()
	select {
	case <-locked:
	case <-time.After(waitSafety):
		t.Fatal("the operate cache's lock was held while the projection built")
	}
	close(hold)
	for i := 0; i < 2; i++ {
		if p := <-got; !p.GeneratedAt.Equal(t0) {
			t.Errorf("projection %d = %v", i, p.GeneratedAt)
		}
	}
}
