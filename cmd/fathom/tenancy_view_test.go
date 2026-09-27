package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-52 (§13.3): /statusz carries a tenancy block, and it is the same
// projection `fathom resources` renders — built live from the governor,
// or rebuilt from the ring when the daemon is down, the fields both can
// carry agree.
func TestStatuszTenancyBlockMatchesResources(t *testing.T) {
	r := newGovRig(t)
	dir := filepath.Join(t.TempDir(), "daemon", fixtureLabel)
	ring, err := tenancy.OpenRing(filepath.Join(dir, tenancy.RingFileName))
	if err != nil {
		t.Fatal(err)
	}
	b := r.g.budget
	if err := ring.WriteMeta(tenancy.RingMeta{PID: 4242, Managed: true, StartedAt: r.now, Budget: b.Budget, Shed: b.Shed, Refuse: b.Refuse, Exit: b.Exit, ExitEnabled: b.ExitEnabled}); err != nil {
		t.Fatal(err)
	}
	r.g.ring = ring
	_, end := r.g.inflight.Begin(context.Background(), tenancy.ClassTree, "references", "fathom", "agent")
	defer end()
	_, end2 := r.g.inflight.Begin(context.Background(), tenancy.ClassPoint, "find_symbol", "fathom", "hook")
	defer end2()
	r.ticks(40)
	r.host.set(3200*tenancy.MiB, 200*tenancy.MiB)
	r.ticks(2) // → shed: a transition record carries the latest sample

	live := tenancyStatus(r.g)
	if live == nil || live.Source != "live" || live.Level != "shed" || live.Mode != "normal" {
		t.Fatalf("live block: %+v", live)
	}
	if len(live.InFlight) != 2 || live.InFlight[0].Tool != "references" || live.InFlight[0].Origin != "agent" || live.InFlight[1].Origin != "hook" {
		t.Fatalf("live block names every in-flight call with its origin, oldest first: %+v", live.InFlight)
	}
	if live.Refusals == "" {
		t.Fatal("refusals must be disclosed (not counted until RT-53), never omitted")
	}

	// /statusz serves exactly this block.
	activeGovernor.Store(r.g)
	defer activeGovernor.Store(nil)
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	rep := buildStatuszReport(context.Background(), c, r.now.Add(-time.Minute), r.now)
	raw, _ := json.Marshal(rep)
	var round StatuszReport
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatal(err)
	}
	if round.Tenancy == nil || !reflect.DeepEqual(*round.Tenancy, *live) {
		t.Fatalf("/statusz tenancy block differs from tenancyStatus:\n got %+v\nwant %+v", round.Tenancy, live)
	}
	if !strings.Contains(string(raw), `"tenancy":{`) || !strings.Contains(string(raw), `"in_flight_by_class"`) {
		t.Fatalf("/statusz JSON lacks the tenancy block: %s", raw)
	}

	// The ring rebuilds the same projection.
	if err := ring.Close(); err != nil {
		t.Fatal(err)
	}
	r.g.ring = nil
	rd, err := tenancy.ReadRingFile(filepath.Join(dir, tenancy.RingFileName))
	if err != nil {
		t.Fatal(err)
	}
	fromRing, ok := tenancyFromRing(rd, live.Quarantined)
	if !ok {
		t.Fatal("ring holds no record")
	}
	common := func(s *governorSnapshot) governorSnapshot {
		c := *s
		c.Source, c.InFlight, c.LastTransition, c.LastDiag = "", nil, "", nil
		if c.Oldest != nil {
			o := *c.Oldest
			o.Service, o.Origin, o.AgeSec = "", "", 0 // the ring keeps the oldest call's tool and class
			c.Oldest = &o
		}
		return c
	}
	if got, want := common(fromRing), common(live); !reflect.DeepEqual(got, want) {
		t.Fatalf("ring projection differs from the live block:\n ring %+v\n live %+v", got, want)
	}

	// And `resources` renders that block whichever source it read.
	env := resourcesEnv{home: t.TempDir(), now: func() time.Time { return r.now },
		fetch: func(string) (*StatuszReport, error) { return &round, nil }, size: func(string) int64 { return 0 },
		killSw: func(string, time.Time) tenancy.KillSwitchVerdict { return tenancy.KillSwitchVerdict{} }}
	viaLive := buildResources(env, "", "http://127.0.0.1:7949", 24*time.Hour)
	if viaLive.Tenancy == nil || !reflect.DeepEqual(*viaLive.Tenancy, *live) {
		t.Fatalf("resources (live) renders a different block: %+v", viaLive.Tenancy)
	}
}
