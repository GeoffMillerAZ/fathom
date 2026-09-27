package main

import (
	"encoding/json"
	"strings"
	"testing"

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
