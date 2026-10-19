package main

// tenancy_honesty_test.go — the wave-B visibility review's findings
// (RT-48..RT-52): what `resources`, doctor and the primer claim must be
// what their instruments earned (I38).

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// A ledger no build writes start rows into cannot report "0 starts" or
// "0 unclean": those counts are disclosed as not recorded, and the
// crash-loop judgement names that it sees only the exits this build writes.
func TestResourcesLifecycleDisclosesUnrecordedStarts(t *testing.T) {
	f := newResourcesFixture(t)
	f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
	f.ledger(t, `{"row":"exit","at":"`+at(f.now.Add(-time.Hour))+`","reason":"over_budget","code":70}`)
	rep := buildResources(f.env(), "", "", 24*time.Hour)
	if rep.Lifecycle.StartsRecorded {
		t.Fatal("a ledger with no start row must not claim starts are recorded")
	}
	if rep.Lifecycle.NotRecorded == "" {
		t.Fatal("the unrecorded counts must be disclosed")
	}
	_, out := f.run(t)
	if strings.Contains(out, "0 starts/24h") || strings.Contains(out, "0 unclean") {
		t.Fatalf("resources renders unmeasured zeros:\n%s", out)
	}
	if !strings.Contains(out, "starts and unclean exits not recorded") || !strings.Contains(out, "over-budget exits only") {
		t.Fatalf("resources does not disclose what the ledger lacks:\n%s", out)
	}
	c := checkDaemonLifecycle(rep)
	if strings.Contains(c.Reason, "0 starts/24h") || strings.Contains(c.Reason, "0 unclean") || !strings.Contains(c.Reason, "not recorded") {
		t.Fatalf("daemon_lifecycle claims unmeasured zeros: %+v", c)
	}

	// With start rows present the writer records them: counts are shown.
	f.ledger(t, `{"row":"start","at":"`+at(f.now.Add(-time.Hour))+`","pid":1}`)
	rep = buildResources(f.env(), "", "", 24*time.Hour)
	if !rep.Lifecycle.StartsRecorded || rep.Lifecycle.Starts24h != 1 {
		t.Fatalf("start rows present: %+v", rep.Lifecycle)
	}
	if c := checkDaemonLifecycle(rep); !strings.Contains(c.Reason, "1 starts/24h") {
		t.Fatalf("daemon_lifecycle: %+v", c)
	}
}

// writeOneSample writes a ring holding one record.
func (f *resourcesFixture) writeOneSample(t *testing.T, rec tenancy.RingRecord) {
	t.Helper()
	r, err := tenancy.OpenRing(filepath.Join(f.dir, tenancy.RingFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.WriteMeta(tenancy.RingMeta{PID: 777, Managed: true, StartedAt: rec.At.Add(-time.Hour), Budget: 4 * tenancy.GiB, Shed: 3 * tenancy.GiB, Refuse: 4 * tenancy.GiB, Exit: 8 * tenancy.GiB, ExitEnabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteRingSample(rec); err != nil {
		t.Fatal(err)
	}
}

// A ring sample older than the freshness window is shown with its age but
// never judged as the daemon's current state, by resources or doctor.
func TestResourcesStaleSampleNotJudgedAsNow(t *testing.T) {
	over := func(age time.Duration) tenancy.RingRecord {
		return tenancy.RingRecord{At: time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC).Add(-age), Level: tenancy.LevelRefuse,
			Footprint: 5 * tenancy.GiB, Children: 0, Pressure: tenancy.PressureWarn, SwapUsed: tenancy.GiB, SwapTotal: 2 * tenancy.GiB,
			SwapVolumeFree: 2 * tenancy.GiB, StoreFree: 40 * tenancy.GiB, TmpFree: 40 * tenancy.GiB}
	}
	f := newResourcesFixture(t)
	f.writeOneSample(t, over(72*time.Hour))
	rep := buildResources(f.env(), "", "", 24*time.Hour)
	if rep.Exit != 0 || len(rep.Reasons) != 0 {
		t.Fatalf("a 72 h old sample judged as now: exit %d %v", rep.Exit, rep.Reasons)
	}
	if !rep.Footprint.Stale {
		t.Fatal("a 72 h old sample must be marked stale")
	}
	_, out := f.run(t)
	if !strings.Contains(out, "no current sample") || strings.Contains(out, "within budget") {
		t.Fatalf("resources verdict for a stale sample:\n%s", out)
	}
	if c := checkHostPressure(rep); c.Status == doctorWarn || !strings.Contains(c.Reason, "old") {
		t.Fatalf("host_pressure judged a 72 h old sample as now: %+v", c)
	}

	// A fresh sample from a daemon that does not answer is still judged.
	f2 := newResourcesFixture(t)
	f2.writeOneSample(t, over(2*time.Minute))
	rep = buildResources(f2.env(), "", "", 24*time.Hour)
	if rep.Exit != 1 || rep.Footprint.Stale {
		t.Fatalf("a 2 min old over-budget sample: exit %d stale %v %v", rep.Exit, rep.Footprint.Stale, rep.Reasons)
	}
	if c := checkHostPressure(rep); c.Status != doctorWarn {
		t.Fatalf("host_pressure on a fresh warn sample: %+v", c)
	}
}

// resources' "leaked" is exactly reap's kill set: a record-less daemon
// whose parent is alive is kept by reap, so it is not a leak.
func TestResourcesLeakedIsReapsKillSet(t *testing.T) {
	kept := censusRow{PID: 903, Addr: "127.0.0.1:7943", Verdict: "no-record", ParentAlive: true, PPID: 55}
	orphan := censusRow{PID: 904, Addr: "127.0.0.1:7944", Verdict: "orphaned", PPID: 1}
	if d := reapDisposition(kept, 0, 0); d.kill {
		t.Fatalf("reap kills a no-record daemon with a live parent: %+v", d)
	}
	if d := reapDisposition(orphan, 0, 0); !d.kill {
		t.Fatalf("reap keeps an orphan: %+v", d)
	}
	f := newResourcesFixture(t)
	f.writeRing(t, 3400*tenancy.MiB, tenancy.LevelNormal, false)
	f.census = []censusRow{kept}
	rep := buildResources(f.env(), "", "", 24*time.Hour)
	if rep.Scratch.Leaked != 0 || rep.Exit != 0 {
		t.Fatalf("a daemon reap keeps counts as leaked: %+v exit %d %v", rep.Scratch, rep.Exit, rep.Reasons)
	}
	if c := checkScratchDaemons(rep); c.Status != doctorPass {
		t.Fatalf("scratch_daemons: %+v", c)
	}
	f.census = []censusRow{kept, orphan}
	rep = buildResources(f.env(), "", "", 24*time.Hour)
	if rep.Scratch.Leaked != 1 || rep.Exit != 1 {
		t.Fatalf("orphan: %+v exit %d", rep.Scratch, rep.Exit)
	}
	if c := checkScratchDaemons(rep); strings.Contains(c.Remedy, "whose verdict is not ok") {
		t.Fatalf("remedy misstates reap's predicate: %s", c.Remedy)
	}
}

// Design §1: FATHOM_TENANCY_ADMISSION=off and a memory_limit / gc_hint
// conflict each make tenancy_scope WARN.
func TestTenancyScopeWarnsOnAdmissionOffAndConflict(t *testing.T) {
	user := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(user, []byte("[tenancy]\nbudget = \"3GiB\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	off := func(k string) string {
		if k == "FATHOM_TENANCY_ADMISSION" {
			return "off"
		}
		return ""
	}
	if c := checkTenancyScope(t.TempDir(), user, off); c.Status != doctorWarn || !strings.Contains(c.Reason, "admission is off") {
		t.Fatalf("admission off: %+v", c)
	}
	conflict := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(conflict, []byte("[daemon]\nmemory_limit = \"3GiB\"\n\n[tenancy]\ngc_hint = \"exit\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if c := checkTenancyScope(t.TempDir(), conflict, func(string) string { return "" }); c.Status != doctorWarn || !strings.Contains(c.Reason, "conflict") {
		t.Fatalf("memory_limit + gc_hint conflict: %+v", c)
	}
}

// host_pressure reads the resolved [tenancy.host] triggers, not literals.
func TestHostPressureReadsConfiguredTriggers(t *testing.T) {
	f := newResourcesFixture(t)
	f.writeOneSample(t, tenancy.RingRecord{At: f.now.Add(-time.Minute), Level: tenancy.LevelNormal, Footprint: tenancy.GiB, Children: 0,
		Pressure: tenancy.PressureWarn, SwapUsed: tenancy.GiB, SwapTotal: 2 * tenancy.GiB, SwapVolumeFree: 6 * tenancy.GiB, StoreFree: 40 * tenancy.GiB, TmpFree: 40 * tenancy.GiB})
	env := f.env()
	env.budget = func() (tenancy.Budget, []tenancy.Note, error) {
		b, notes := tenancy.ResolveBudget(tenancy.Config{Host: tenancy.HostConfig{ShedOnPressure: "critical", RefuseOnPressure: "critical", MinSwapVolumeFree: 10 * tenancy.GiB, Allowance: 0.25}},
			tenancy.HostFacts{RAM: 32 * tenancy.GiB, NumCPU: 12, Managed: true})
		return b, notes, nil
	}
	rep := buildResources(env, "", "", 24*time.Hour)
	c := checkHostPressure(rep)
	if c.Status != doctorWarn || !strings.Contains(c.Reason, "10 GiB") || !strings.Contains(c.Reason, "sheds at critical") || strings.Contains(c.Reason, "5 GiB") {
		t.Fatalf("host_pressure ignores [tenancy.host]: %+v", c)
	}
	// RT-192: host_triggers names the allowance, and so does the doctor line.
	if rep.Triggers.Allowance != tenancy.GiB || !strings.Contains(c.Reason, "once its footprint reaches "+tenancy.FormatBytes(tenancy.GiB)) {
		t.Fatalf("the host allowance is not named: %d %+v", rep.Triggers.Allowance, c)
	}
}

// One /statusz block is one sample: the sample time and the footprint are
// read under the same lock, so a concurrent tick cannot split them.
func TestTenancyStatusReadsOneSample(t *testing.T) {
	r := newGovRig(t)
	base := r.now
	const n = 3000
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= n; i++ {
			r.host.set(int64(i)*tenancy.MiB, 0)
			r.g.tick(base.Add(time.Duration(i)*time.Second), 0)
		}
	}()
	bad := ""
	for i := 0; i < 20*n && bad == ""; i++ {
		s := tenancyStatus(r.g)
		if s.SampleAt == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339, s.SampleAt)
		if err != nil {
			t.Fatal(err)
		}
		if want := int64(at.Sub(base)/time.Second) * tenancy.MiB; s.Footprint != want {
			bad = s.SampleAt + " with footprint " + tenancy.FormatBytes(s.Footprint)
		}
	}
	wg.Wait()
	if bad != "" {
		t.Fatalf("one block mixed two samples: %s", bad)
	}
}
