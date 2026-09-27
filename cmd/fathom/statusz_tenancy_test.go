package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// RT-32 review finding 7 (design §2, §13.3): /statusz carries a tenancy
// block — the level and every unknown reading, "ladder off (footprint
// unknown)" included — not only the log and /metrics.
func TestStatuszDisclosesLadderLevelAndUnknowns(t *testing.T) {
	r := newGovRig(t)
	r.host.set(tenancy.Unknown, tenancy.Unknown)
	r.ticks(2)
	activeGovernor.Store(r.g)
	defer activeGovernor.Store(nil)

	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	raw, err := json.Marshal(buildStatuszReport(context.Background(), c, time.Now().Add(-time.Second), time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tenancy *struct {
			Level     string   `json:"level"`
			LadderOff bool     `json:"ladder_off"`
			Budget    int64    `json:"budget"`
			Unknowns  []string `json:"unknowns"`
		} `json:"tenancy"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Tenancy == nil {
		t.Fatalf("/statusz lacks the tenancy block: %s", raw)
	}
	if m.Tenancy.Level != "normal" || !m.Tenancy.LadderOff || m.Tenancy.Budget != 4*tenancy.GiB {
		t.Fatalf("tenancy block %+v", *m.Tenancy)
	}
	if !strings.Contains(strings.Join(m.Tenancy.Unknowns, "; "), "footprint unknown") {
		t.Fatalf("unknown footprint not disclosed in /statusz: %v", m.Tenancy.Unknowns)
	}

	// A CLI verb (no governor) has no block.
	activeGovernor.Store(nil)
	raw, _ = json.Marshal(buildStatuszReport(context.Background(), c, time.Now(), time.Now()))
	if strings.Contains(string(raw), `"tenancy"`) {
		t.Fatalf("no governor, no tenancy block: %s", raw)
	}
}
