package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-50 (§13.3): doctor gains daemon_lifecycle, host_pressure,
// scratch_daemons and tenancy_scope rows, each over the resources
// projection (or, for tenancy_scope, the config layers), each naming its
// remedy — and none restarts anything.
func TestDoctorTenancyRows(t *testing.T) {
	wants := func(t *testing.T, c doctorCheck, name string, st doctorStatus, reason, remedy []string) {
		t.Helper()
		if c.Name != name || c.Status != st {
			t.Fatalf("%s: got %s/%s, want %s/%s: %+v", name, c.Name, c.Status, name, st, c)
		}
		for _, w := range reason {
			if !strings.Contains(c.Reason, w) {
				t.Errorf("%s reason lacks %q: %s", name, w, c.Reason)
			}
		}
		for _, w := range remedy {
			if !strings.Contains(c.Remedy, w) {
				t.Errorf("%s remedy lacks %q: %s", name, w, c.Remedy)
			}
		}
		if strings.Contains(strings.ToLower(c.Remedy), "restart") {
			t.Errorf("%s: no restart remedy: %s", name, c.Remedy)
		}
	}

	t.Run("healthy: every row passes", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		f.ledger(t, `{"row":"start","at":"`+at(f.now.Add(-5*time.Hour))+`","pid":777}`)
		f.census = []censusRow{{PID: 902, Addr: "127.0.0.1:7942", Verdict: "ok"}}
		rep := buildResources(f.env(), "", "", 24*time.Hour)
		wants(t, checkDaemonLifecycle(rep), "daemon_lifecycle", doctorPass, []string{"1 starts/24h", "0 unclean", "switch run", "0 quarantined"}, nil)
		wants(t, checkHostPressure(rep), "host_pressure", doctorPass, []string{"pressure normal", "swap 3 GiB/22 GiB", "swap volume 41 GiB free"}, nil)
		wants(t, checkScratchDaemons(rep), "scratch_daemons", doctorPass, []string{"1 unmanaged", "0 leaked"}, nil)
	})

	t.Run("unhealthy: each row warns with its remedy", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
		var rows []string
		for i := 1; i <= 3; i++ {
			rows = append(rows, `{"row":"exit","at":"`+at(f.now.Add(-time.Duration(i*5)*time.Minute))+`","reason":"over_budget","code":70}`)
		}
		rows = append(rows, `{"row":"quarantine","at":"`+at(f.now.Add(-time.Minute))+`","service":"app-kit","reason":"exceeded budget during ingest"}`)
		f.ledger(t, rows...)
		f.census = []censusRow{{PID: 901, Addr: "127.0.0.1:7941", Age: 22 * time.Hour, Verdict: "orphaned"}, {PID: 7, Record: "stale", Verdict: "stale-record"}}
		env := f.env()
		env.killSw = func(string, time.Time) tenancy.KillSwitchVerdict {
			return tenancy.KillSwitchVerdict{Action: tenancy.ActionExitStay, Record: tenancy.SwitchRecord{Reason: "breaker"}, Message: "disabled.json: breaker"}
		}
		// A critical host and a nearly full swap volume in the newest sample.
		r, err := tenancy.OpenRing(filepath.Join(f.dir, tenancy.RingFileName))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.WriteRingSample(tenancy.RingRecord{At: f.now.Add(-time.Minute), Footprint: tenancy.GiB, Children: 0, Pressure: tenancy.PressureCritical,
			SwapUsed: 20 * tenancy.GiB, SwapTotal: 22 * tenancy.GiB, SwapVolumeFree: 3 * tenancy.GiB, StoreFree: 41 * tenancy.GiB}); err != nil {
			t.Fatal(err)
		}
		_ = r.Close()
		rep := buildResources(env, "", "", 24*time.Hour)
		wants(t, checkDaemonLifecycle(rep), "daemon_lifecycle", doctorWarn,
			[]string{"crash-looping: 3 counted exits in 30 min", "stopped by the breaker", "quarantined: app-kit"},
			[]string{"fathom daemon start", "fathom resources", "ingest"})
		wants(t, checkHostPressure(rep), "host_pressure", doctorWarn, []string{"pressure critical", "swap volume 3 GiB free"}, []string{"fathom resources"})
		wants(t, checkScratchDaemons(rep), "scratch_daemons", doctorWarn, []string{"1 leaked", "pid 901", "1 stale record"}, []string{"fathom daemon reap --apply"})
	})

	t.Run("no sample: host_pressure says why, never passes", func(t *testing.T) {
		f := newResourcesFixture(t)
		rep := buildResources(f.env(), "", "", 24*time.Hour)
		c := checkHostPressure(rep)
		if c.Status == doctorPass || !strings.Contains(c.Reason, "no sample") {
			t.Fatalf("host_pressure with no sample: %+v", c)
		}
		if c := checkScratchDaemons(resourcesReport{}); c.Status == doctorPass {
			t.Fatalf("scratch_daemons with no census passed: %+v", c)
		}
	})

	t.Run("tenancy_scope: a repository [tenancy] is ignored and named", func(t *testing.T) {
		root := t.TempDir()
		userDir := t.TempDir()
		user := filepath.Join(userDir, "config.toml")
		if err := os.WriteFile(user, []byte("[tenancy]\nbudget = \"3GiB\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		wants(t, checkTenancyScope(root, user, func(string) string { return "" }), "tenancy_scope", doctorPass, []string{"user scope", user}, nil)
		if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte("[tenancy]\nbudget = \"30GiB\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		wants(t, checkTenancyScope(root, user, func(string) string { return "" }), "tenancy_scope", doctorWarn,
			[]string{"ignored", "a repository cannot raise this machine's ceiling"}, []string{"~/.fathom/config.toml"})
		env := func(k string) string {
			if k == "FATHOM_TENANCY_BUDGET" {
				return "2GiB"
			}
			return ""
		}
		wants(t, checkTenancyScope(t.TempDir(), user, env), "tenancy_scope", doctorPass, []string{"FATHOM_TENANCY_BUDGET = 2GiB overrides"}, nil)
	})
}
