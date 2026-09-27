package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestTenancyPanelRendersResourcesJSON (RT-121, design §14): Operate's
// Tenancy panel renders `fathom resources --json` — the page's section for
// that verb equals the verb's own JSON for the same inputs (ADR-045: every
// panel renders a verb) — as the 24 h footprint by hour against the budget
// and its marks (an hour with no sample hatched, never zero), the ladder
// timeline, lifecycle, in-flight work by class, the scratch census and the
// disk classes, through app-kit role utilities only.
func TestTenancyPanelRendersResourcesJSON(t *testing.T) {
	f := newConsoleFixture(t)
	rf := newResourcesFixture(t)
	rf.now = f.now
	rf.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
	rf.ledger(t, `{"row":"start","at":"`+at(f.now.Add(-5*60*60*1e9))+`","pid":777}`)
	prev := resourcesEnvNow
	resourcesEnvNow = rf.env
	t.Cleanup(func() { resourcesEnvNow = prev })
	resetOperateCache()
	t.Cleanup(resetOperateCache)

	env := f.pageJSON(t, "/operate")
	section, ok := env.Sections[operateResourcesVerb]
	if !ok {
		t.Fatalf("no section for %q; verbs %v", operateResourcesVerb, env.Verbs)
	}
	var out, errb bytes.Buffer
	runResources([]string{"--url", f.ts.URL, "--json"}, &out, &errb, rf.env())
	var fromVerb, fromPage any
	if err := json.Unmarshal(out.Bytes(), &fromVerb); err != nil {
		t.Fatalf("verb: %v\n%s", err, errb.String())
	}
	_ = json.Unmarshal(section, &fromPage)
	if !reflect.DeepEqual(fromVerb, fromPage) {
		a, _ := json.MarshalIndent(fromVerb, "", " ")
		b, _ := json.MarshalIndent(fromPage, "", " ")
		t.Fatalf("the Tenancy section differs from `fathom resources --json`:\nverb %s\npage %s", a, b)
	}
	var rep resourcesReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Footprint.Hourly) != 24 || len(rep.Ladder) == 0 || rep.Ladder[0].Level != "shed" {
		t.Fatalf("resources --json carries hourly %v and ladder %+v; want 24 hours and the shed transition", rep.Footprint.Hourly, rep.Ladder)
	}

	_, _, page := f.get(t, "/operate")
	mustContain(t, "tenancy panel", page,
		`id="tenancy"`, `data-console-verb="fathom resources --json"`,
		"footprint per hour, as a share of the budget", // the hourly heatmap
		"no sample this hour",                          // an hour without a sample, hatched
		"shed mark", "refuse mark", "exit mark",
		"ladder transitions", // the timeline
		"starts in 24 h", "in flight by class", "scratch daemons", "fathom home",
	)
	start := strings.Index(page, `id="tenancy"`)
	assertRoleUtilitiesOnly(t, page[start:])
}
