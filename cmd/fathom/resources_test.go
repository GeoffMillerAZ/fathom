package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// resourcesFixture is a scratch HOME holding one managed daemon's state
// directory: a ring, a lifecycle ledger and a diagnostic capture.
type resourcesFixture struct {
	home, dir string
	now       time.Time
	census    []censusRow
	fetchErr  error
	live      *StatuszReport
}

const fixtureLabel = "dev.fathom.serve-http"

func newResourcesFixture(t *testing.T) *resourcesFixture {
	t.Helper()
	home := t.TempDir()
	f := &resourcesFixture{home: home, dir: filepath.Join(home, ".fathom", "daemon", fixtureLabel),
		now: time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC), fetchErr: errors.New("dial tcp 127.0.0.1:7581: connect: connection refused")}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return f
}

// writeRing writes minutes of samples ending 2 minutes before now (inside
// tenancySampleFresh: a daemon that just stopped answering), with a shed
// transition at the peak.
func (f *resourcesFixture) writeRing(t *testing.T, peak int64, lastLevel tenancy.Level, safe bool) {
	t.Helper()
	r, err := tenancy.OpenRing(filepath.Join(f.dir, tenancy.RingFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.WriteMeta(tenancy.RingMeta{PID: 777, Managed: true, StartedAt: f.now.Add(-5 * time.Hour), Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true}); err != nil {
		t.Fatal(err)
	}
	end := f.now.Add(-2 * time.Minute)
	for i := 0; i < 300; i++ {
		at := end.Add(-time.Duration(299-i) * time.Minute)
		rec := tenancy.RingRecord{At: at, Level: tenancy.LevelNormal, Footprint: 1200 * tenancy.MiB, Children: 100 * tenancy.MiB,
			Pressure: tenancy.PressureNormal, SwapUsed: 3 * tenancy.GiB, SwapTotal: 22 * tenancy.GiB, StoreFree: 41 * tenancy.GiB,
			TmpFree: 41 * tenancy.GiB, SwapVolumeFree: 41 * tenancy.GiB, DiskWritten: 300 * tenancy.MiB}
		if i == 150 {
			rec.Footprint, rec.Level = peak, tenancy.LevelShed
			tr := rec
			tr.Transition = true
			if err := r.WriteRingSample(tr); err != nil {
				t.Fatal(err)
			}
		}
		if i == 299 {
			rec.Level, rec.SafeMode = lastLevel, safe
			rec.InFlight[tenancy.ClassTree] = 1
			rec.OldestTool, rec.OldestClass, rec.OldestAge = "obligations", tenancy.ClassTree, 40*time.Second
		}
		if err := r.WriteRingSample(rec); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *resourcesFixture) ledger(t *testing.T, rows ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, "lifecycle.jsonl"), []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *resourcesFixture) env() resourcesEnv {
	return resourcesEnv{
		home: f.home,
		now:  func() time.Time { return f.now },
		fetch: func(string) (*StatuszReport, error) {
			if f.live != nil {
				return f.live, nil
			}
			return nil, f.fetchErr
		},
		census: func() ([]censusRow, error) { return f.census, nil },
		hooks:  func(string) hookguard.Decision { return hookguard.Decision{Verdict: hookguard.Allow} },
		budget: func() (tenancy.Budget, []tenancy.Note, error) {
			def, _ := tenancy.ResolveBudget(tenancy.Config{}, tenancy.HostFacts{}) // the default host triggers
			return tenancy.Budget{Budget: 2 * tenancy.GiB, Host: def.Host}, nil, nil
		},
		killSw: func(string, time.Time) tenancy.KillSwitchVerdict {
			return tenancy.KillSwitchVerdict{Action: tenancy.ActionRun}
		},
		size: func(string) int64 { return 212 * tenancy.MiB },
	}
}

func (f *resourcesFixture) run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runResources(args, &out, &errb, f.env())
	return code, out.String() + errb.String()
}

func at(ts time.Time) string { return ts.UTC().Format(time.RFC3339Nano) }

// RT-48 (§13.2): with the daemon down, `fathom resources` renders every
// section from files — the ring's last sample and its age, the 24 h peak,
// the lifecycle ledger, quarantine, host, in-flight, hooks, scratch
// daemons, disk and the last capture — and exits 0/1/2 by its rules.
func TestResourcesRendersFromRingWhenDaemonDown(t *testing.T) {
	t.Run("down, within budget: exit 0 and every section from files", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.ledger(t,
			`{"row":"start","at":"`+at(f.now.Add(-5*time.Hour))+`","pid":777}`,
			`{"row":"start","at":"`+at(f.now.Add(-3*time.Hour))+`","pid":778}`,
			`{"row":"exit","at":"`+at(f.now.Add(-3*time.Hour-time.Minute))+`","pid":777,"reason":"drain","code":0}`,
			`{"row":"quarantine","at":"`+at(f.now.Add(-2*time.Hour))+`","service":"app-kit","reason":"exceeded budget during ingest"}`,
		)
		if err := os.MkdirAll(filepath.Join(f.dir, "diag", "20260926T211400Z-refuse"), 0o700); err != nil {
			t.Fatal(err)
		}
		code, out := f.run(t)
		if code != 0 {
			t.Fatalf("exit %d, want 0 (within budget):\n%s", code, out)
		}
		for _, want := range []string{
			"fathom resources — " + fixtureLabel + " (managed)",
			"NOT answering at http://", "connection refused", "showing the ring's last sample, 2m0s old",
			"state      normal · mode normal · budget 4 GiB", "the daemon's ring header",
			"footprint  1.2 GiB now (+100 MiB children) [phys_footprint]", "24h max 3.4 GiB", "shed)",
			"2 starts/24h", "0 unclean", "quarantine quarantined: app-kit exceeded budget during ingest",
			"host       pressure normal · swap 3 GiB/22 GiB · store volume 41 GiB free",
			"in-flight  point 0 · index 0 · tree 1", "oldest obligations (tree) 40s", "refused: not counted",
			"hooks      breaker allow", "p99 not measured",
			"scratch    0 unmanaged daemon(s), 0 leaked",
			"disk       ~/.fathom 212 MiB", "written by the daemon 300 MiB",
			"diag       last capture 09-26 21:14 refuse",
			"verdict    within budget (exit 0)",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("report lacks %q:\n%s", want, out)
			}
		}
		// --json carries the same facts.
		code, js := f.run(t, "--json")
		if code != 0 {
			t.Fatalf("--json exit %d", code)
		}
		var rep resourcesReport
		if err := json.Unmarshal([]byte(js), &rep); err != nil {
			t.Fatalf("--json does not parse: %v\n%s", err, js)
		}
		if rep.Daemon.Reachable || rep.Tenancy == nil || rep.Tenancy.Source != "ring" || rep.Footprint.Samples != 301 || rep.Footprint.Max != 3500*tenancy.MiB {
			t.Fatalf("json report: reachable=%v tenancy=%+v footprint=%+v", rep.Daemon.Reachable, rep.Tenancy, rep.Footprint)
		}
	})

	t.Run("crash-looping, breaker-stopped and leaked scratch daemons: exit 1, each named", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, true)
		var rows []string
		for i := 1; i <= 3; i++ {
			rows = append(rows, `{"row":"exit","at":"`+at(f.now.Add(-time.Duration(i*5)*time.Minute))+`","pid":77`+string(rune('0'+i))+`,"reason":"over_budget","code":70,"detail":"own footprint 8.1 GiB ≥ exit mark 8.0 GiB"}`)
		}
		f.ledger(t, rows...)
		f.census = []censusRow{
			{PID: 901, Addr: "127.0.0.1:7941", Managed: false, Age: 22 * time.Hour, Verdict: "orphaned"},
			{PID: 902, Addr: "127.0.0.1:7942", Managed: false, Verdict: "ok"},
			{PID: 3, Record: "stale", Verdict: "stale-record"},
		}
		env := f.env()
		env.killSw = func(string, time.Time) tenancy.KillSwitchVerdict {
			return tenancy.KillSwitchVerdict{Action: tenancy.ActionExitStay, Record: tenancy.SwitchRecord{Reason: "breaker"}, Message: "disabled by breaker"}
		}
		var out bytes.Buffer
		code := runResources(nil, &out, &out, env)
		if code != 1 {
			t.Fatalf("exit %d, want 1:\n%s", code, out.String())
		}
		for _, want := range []string{"crash-looping: 3 counted exits in 30 min", "stopped by the breaker", "1 scratch daemon(s) leaked", "pid 901 127.0.0.1:7941 orphaned", "3 over-budget exits/24h", "mode safe", "last exit"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("report lacks %q:\n%s", want, out.String())
			}
		}
	})

	t.Run("over budget from the live daemon: exit 1", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.live = &StatuszReport{Tenancy: &governorSnapshot{Source: "live", Level: "shed", Mode: "normal", Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true,
			Footprint: 4200 * tenancy.MiB, Children: 0, Pressure: "normal", SampleAt: f.now.Format(time.RFC3339), InFlightByClass: map[string]int{}}}
		code, out := f.run(t)
		if code != 1 || !strings.Contains(out, "over budget: footprint + children 4.1 GiB > budget 4 GiB") || !strings.Contains(out, "answering at") {
			t.Fatalf("exit %d:\n%s", code, out)
		}
	})

	t.Run("an unreadable ring is exit 2, named", func(t *testing.T) {
		f := newResourcesFixture(t)
		if err := os.WriteFile(filepath.Join(f.dir, tenancy.RingFileName), []byte("garbage"), 0o600); err != nil {
			t.Fatal(err)
		}
		code, out := f.run(t)
		if code != 2 || !strings.Contains(out, "UNREADABLE resource ring") {
			t.Fatalf("exit %d:\n%s", code, out)
		}
	})

	t.Run("no daemon and no ring: exit 2, never an empty all-clear", func(t *testing.T) {
		f := newResourcesFixture(t)
		code, out := f.run(t)
		if code != 2 || !strings.Contains(out, "no sample") || !strings.Contains(out, "resolved from config") {
			t.Fatalf("exit %d:\n%s", code, out)
		}
	})
}
