package tenancy

import (
	"strings"
	"testing"
	"time"
)

// fakeHost is an injectable Host: every reading is a field.
type fakeHost struct {
	footprint, children int64
	childCount          int
	diskWritten         int64
	pressure            PressureLevel
	swapUsed, swapTotal int64
	storeFree, tmpFree  int64
	swapVolFree         int64
}

func (h *fakeHost) ReadHost(s *Sample) {
	s.Footprint, s.Children, s.ChildCount = h.footprint, h.children, h.childCount
	s.DiskWritten = h.diskWritten
	s.Pressure = h.pressure
	s.SwapUsed, s.SwapTotal = h.swapUsed, h.swapTotal
	s.StoreFree, s.TmpFree, s.SwapVolumeFree = h.storeFree, h.tmpFree, h.swapVolFree
}

// normalHost is a healthy 32 GiB machine: pressure normal, swap 80% used
// (macOS grows swap on demand; this is routine), plenty of disk.
func normalHost(footprint int64) *fakeHost {
	return &fakeHost{
		footprint: footprint, pressure: PressureNormal,
		swapUsed: 4900 * MiB, swapTotal: 6100 * MiB,
		storeFree: 40 * GiB, tmpFree: 40 * GiB, swapVolFree: 40 * GiB,
	}
}

// ladderRig drives StepLadder with a fake host and an injected clock.
type ladderRig struct {
	t   *testing.T
	b   Budget
	st  LadderState
	h   *fakeHost
	rt  *RuntimeReader
	now time.Time
	s   Sample
}

func newRig(t *testing.T) *ladderRig {
	b, _ := ResolveBudget(Config{}, HostFacts{RAM: 32 * GiB, NumCPU: 12, Managed: true}) // 4 GiB budget, exit 8 GiB
	return &ladderRig{t: t, b: b, h: normalHost(1 * GiB), rt: NewRuntimeReader(), now: t0}
}

// step advances the clock by one cadence (plus late) and steps the ladder.
func (r *ladderRig) step(late time.Duration) Transition {
	r.now = r.now.Add(SampleEvery + late)
	SampleHost(r.h, r.rt, r.now, late, &r.s)
	return StepLadder(&r.st, &r.s, &r.b)
}

func (r *ladderRig) stepN(n int) (last Transition) {
	for range n {
		last = r.step(0)
	}
	return last
}

func (r *ladderRig) want(l Level, ctx string) {
	r.t.Helper()
	if r.st.Level != l {
		r.t.Fatalf("%s: level %s, want %s", ctx, r.st.Level, l)
	}
}

// RT-33 (§2.1): normal → shed → refuse → exit, entry on two consecutive
// samples, step-down after 60 s below 0.6 × the current rung's mark, one
// rung at a time.
func TestLadder_TransitionsAndHysteresis(t *testing.T) {
	r := newRig(t)
	r.stepN(5)
	r.want(LevelNormal, "1 GiB of a 4 GiB budget")

	// One sample over the shed mark (3 GiB) is not enough.
	r.h.footprint = 3200 * MiB
	if tr := r.step(0); tr.Changed {
		t.Fatalf("one over-mark sample moved the ladder: %+v", tr)
	}
	tr := r.step(0)
	r.want(LevelShed, "two samples over the shed mark")
	if !tr.Changed || tr.From != LevelNormal || tr.To != LevelShed || !strings.Contains(tr.Reason, "shed mark") {
		t.Fatalf("shed transition must name its trigger: %+v", tr)
	}

	// Over the refuse mark (4 GiB) for two samples.
	r.h.footprint = 4500 * MiB
	r.step(0)
	r.want(LevelShed, "one sample over refuse")
	tr = r.step(0)
	r.want(LevelRefuse, "two samples over refuse")
	if !strings.Contains(tr.Reason, "refuse mark") {
		t.Fatalf("refuse reason: %q", tr.Reason)
	}

	// Back to 3 GiB: above 0.6 × refuse (2.4 GiB) — refuse holds.
	r.h.footprint = 3 * GiB
	r.stepN(60)
	r.want(LevelRefuse, "above 0.6× the refuse mark")

	// 2 GiB: below 0.6 × refuse. Not before 60 s...
	r.h.footprint = 2 * GiB
	r.stepN(29) // 58 s
	r.want(LevelRefuse, "58 s below 0.6× refuse")
	// ...a blip back above resets the window.
	r.h.footprint = 3 * GiB
	r.step(0)
	r.h.footprint = 2 * GiB
	r.stepN(29)
	r.want(LevelRefuse, "the blip reset the 60 s window")
	r.stepN(2)
	r.want(LevelShed, "60 s below 0.6× refuse steps down one rung")

	// Shed steps down only below 0.6 × shed (1.8 GiB) for 60 s.
	r.stepN(40)
	r.want(LevelShed, "2 GiB is above 0.6× the shed mark")
	r.h.footprint = 1 * GiB
	r.stepN(31)
	r.want(LevelNormal, "60 s below 0.6× shed")

	// Exit: own footprint ≥ the exit mark (8 GiB) for two samples.
	r.h.footprint = 9 * GiB
	r.step(0)
	r.want(LevelNormal, "one sample over the exit mark moves nothing")
	tr = r.step(0)
	r.want(LevelExit, "two samples over the exit mark")
	if !strings.Contains(tr.Reason, "exit mark") {
		t.Fatalf("exit reason: %q", tr.Reason)
	}
	// The exit rung is an action: when the next sample is below the exit
	// mark (an ingest was halted), the ladder holds refuse, not normal.
	r.h.footprint = 5 * GiB
	r.step(0)
	r.want(LevelRefuse, "exit not sustained")

	// Children count toward shed and refuse, never toward exit (§2).
	r = newRig(t)
	r.h.footprint, r.h.children = 2*GiB, 3*GiB
	r.stepN(2)
	r.want(LevelRefuse, "footprint + children ≥ the refuse mark")
	r.h.footprint, r.h.children = 1*GiB, 9*GiB
	r.stepN(4)
	r.want(LevelRefuse, "children alone never reach the exit rung")

	// exit_enabled = false caps the ladder at refuse.
	r = newRig(t)
	r.b.ExitEnabled = false
	r.h.footprint = 9 * GiB
	r.stepN(4)
	r.want(LevelRefuse, "exit disabled")

	// Pressure critical for 60 s with the footprint over the budget exits.
	r = newRig(t)
	r.h.pressure = PressureCritical
	r.h.footprint = 4200 * MiB
	r.stepN(30) // critical since the first of these: 58 s
	r.want(LevelRefuse, "critical pressure, 58 s")
	r.stepN(1)
	r.want(LevelExit, "critical pressure for 60 s with footprint > budget")

	// Host pressure warn sheds; critical refuses (two samples each). Warn
	// sheds only at or above the host allowance (1.8 GiB of 4 GiB, RT-192):
	// 2 GiB keeps this sequence's meaning.
	r = newRig(t)
	r.h.pressure = PressureWarn
	r.h.footprint = 2 * GiB
	r.stepN(2)
	r.want(LevelShed, "pressure warn")
	r.h.pressure = PressureCritical
	r.stepN(2)
	r.want(LevelRefuse, "pressure critical")
	// A host trigger holds its rung however low the footprint.
	r.stepN(40)
	r.want(LevelRefuse, "critical pressure holds refuse")
	r.h.pressure = PressureNormal
	r.stepN(31)
	r.want(LevelShed, "pressure cleared: 60 s, one rung")
}

// RT-154 (§2.1): the swap-used fraction alone never moves the ladder; the
// host triggers are pressure, swap growth > 2 GiB in 5 min while fathom is
// at or above the shed mark, and swap-volume free space.
func TestLadder_SwapFractionAloneNeverSheds(t *testing.T) {
	r := newRig(t)
	// 99% swap used, flat, pressure normal: the machine this ran on sat at
	// 80% at pressure level 1 (4.9 of 6.1 GB, 2026-09-26).
	r.h.swapUsed, r.h.swapTotal = 6050*MiB, 6100*MiB
	r.stepN(200)
	r.want(LevelNormal, "a full swap file alone")

	// Swap grows 3 GiB in 2 min while fathom is BELOW the shed mark: not
	// fathom's doing — no rung.
	for range 60 {
		r.h.swapUsed += 51 * MiB
		r.step(0)
	}
	r.want(LevelNormal, "swap growth while fathom is below the shed mark")

	// The same growth while fathom is at the shed mark: refuse.
	r = newRig(t)
	r.h.footprint = 3100 * MiB
	r.stepN(3)
	r.want(LevelShed, "at the shed mark")
	var tr Transition
	for range 60 {
		r.h.swapUsed += 51 * MiB
		if x := r.step(0); x.Changed {
			tr = x
		}
	}
	r.want(LevelRefuse, "swap grew > 2 GiB in 5 min while fathom ≥ shed")
	if !strings.Contains(tr.Reason, "swap grew") {
		t.Fatalf("swap growth reason: %q", tr.Reason)
	}

	// Growth spread over more than the window does not count.
	r = newRig(t)
	r.h.footprint = 3100 * MiB
	for range 400 { // 800 s, 3 GiB in all: ~1.15 GiB per 5 min
		r.h.swapUsed += 8 * MiB
		r.step(0)
	}
	r.want(LevelShed, "slow swap growth")

	// Swap-volume free space below 5 GiB refuses whatever the footprint.
	r = newRig(t)
	r.h.swapVolFree = 4 * GiB
	r.stepN(2)
	r.want(LevelRefuse, "swap volume under 5 GiB free")
}

// RT-171 (§2): a sample more than 3× late (≥ 6 s after the previous one:
// the GC limiter or swap stalled it) counts as a second over-mark sample,
// and the sampler does not allocate.
func TestLadder_LateSampleEscalatesAndSamplerZeroAlloc(t *testing.T) {
	r := newRig(t)
	r.h.footprint = 3200 * MiB
	r.step(3 * time.Second) // 5 s after the last: late, not 3× late
	r.want(LevelNormal, "a 2.5× late sample is one sample")
	r = newRig(t)
	r.h.footprint = 3200 * MiB
	tr := r.step(4 * time.Second) // 6 s: 3× the cadence
	r.want(LevelShed, "a 3× late over-mark sample counts twice")
	if !strings.Contains(tr.Reason, "late") {
		t.Fatalf("the reason names the late sample: %q", tr.Reason)
	}
	r = newRig(t)
	r.h.footprint = 9 * GiB
	r.step(10 * time.Second)
	r.want(LevelExit, "a stalled sampler over the exit mark exits on the first sample")

	// Zero allocation: the sampler (host + runtime/metrics) and the step.
	r = newRig(t)
	allocs := testing.AllocsPerRun(200, func() {
		r.now = r.now.Add(SampleEvery)
		SampleHost(r.h, r.rt, r.now, 0, &r.s)
	})
	if allocs != 0 {
		t.Fatalf("SampleHost allocates %.1f per sample; the sampler must not allocate", allocs)
	}
	allocs = testing.AllocsPerRun(200, func() {
		r.now = r.now.Add(SampleEvery)
		SampleHost(r.h, r.rt, r.now, 0, &r.s)
		StepLadder(&r.st, &r.s, &r.b)
	})
	if allocs != 0 {
		t.Fatalf("a steady-state ladder step allocates %.1f", allocs)
	}
}

// RT-32 (§2): an Unknown pressure or swap reading is disclosed and never a
// trigger; an Unknown footprint disables the ladder, disclosed.
func TestHostSampler_UnknownDisclosedNeverTriggers(t *testing.T) {
	r := newRig(t)
	r.h.pressure = PressureUnknown
	r.h.swapUsed, r.h.swapTotal, r.h.swapVolFree = -1, -1, -1
	r.h.storeFree, r.h.tmpFree = -1, -1
	r.stepN(100)
	r.want(LevelNormal, "unknown pressure and swap")
	u := strings.Join(r.s.Unknowns(), "; ")
	for _, want := range []string{"pressure unknown", "swap unknown", "swap volume free unknown", "store volume free unknown"} {
		if !strings.Contains(u, want) {
			t.Fatalf("unknown readings must be disclosed: %q lacks %q", u, want)
		}
	}
	// The runtime/metrics half is always read.
	if r.s.HeapLive < 0 || r.s.GoTotalMinusReleased <= 0 { // live heap is 0 before the first GC
		t.Fatalf("runtime/metrics readings missing: %+v", r.s)
	}

	// An Unknown footprint turns the ladder off — whatever else says.
	r = newRig(t)
	r.h.footprint = 5 * GiB
	r.stepN(2)
	r.want(LevelRefuse, "precondition")
	r.h.footprint = -1
	r.h.pressure = PressureCritical
	tr := r.step(0)
	r.want(LevelNormal, "footprint unknown")
	if !r.st.Off || !tr.Changed || !strings.Contains(tr.Reason, "ladder: off (footprint unknown)") {
		t.Fatalf("the ladder is off and says so: off=%v %+v", r.st.Off, tr)
	}
	if !strings.Contains(strings.Join(r.s.Unknowns(), ";"), "footprint unknown") {
		t.Fatalf("the unknown footprint is disclosed: %v", r.s.Unknowns())
	}
	r.stepN(50)
	r.want(LevelNormal, "still off")
	// Readable again: the ladder resumes (two samples to enter).
	r.h.footprint = 5 * GiB
	tr = r.step(0)
	if r.st.Off || !tr.Changed || !strings.Contains(tr.Reason, "ladder: on") {
		t.Fatalf("the ladder comes back on, named: %+v", tr)
	}
	r.step(0)
	r.want(LevelRefuse, "resumed")
}

// RT-192 (§2.1 amended 2026-10-19, ADR-047): host pressure at or above
// shed_on_pressure moves the ladder only while footprint + children is at
// or above the host allowance ([tenancy.host] allowance × budget, default
// 0.45 = 0.6 × shed_at; an explicit 0 is the old rule). A pressure-entered
// rung steps down when pressure clears, or when combined stays under 0.6 ×
// the allowance for 60 s. Pressure critical and the swap-volume floor are
// unchanged.
func TestLadder_PressureBelowAllowanceStaysNormal(t *testing.T) {
	host := HostFacts{RAM: 32 * GiB, NumCPU: 12, Managed: true} // 4 GiB budget, shed 3 GiB
	t.Run("the allowance resolves with its derivation", func(t *testing.T) {
		b, notes := ResolveBudget(Config{}, host)
		frac := 0.45
		if want := int64(float64(4*GiB) * frac); b.Host.Allowance != want {
			t.Fatalf("default allowance %d, want %d (0.45 × budget)", b.Host.Allowance, want)
		}
		var got string
		for _, n := range notes {
			if n.Key == "allowance" {
				got = n.Value + " " + n.Derivation
			}
		}
		if !strings.Contains(got, "0.45 × budget") || !strings.Contains(got, "0.6 × shed_at") {
			t.Fatalf("allowance note %q does not name its derivation", got)
		}
		b, _ = ResolveBudget(Config{Host: HostConfig{Allowance: 0.25}}, host)
		if b.Host.Allowance != GiB {
			t.Fatalf("configured 0.25 resolved to %d, want 1 GiB", b.Host.Allowance)
		}
		b, notes = ResolveBudget(Config{Host: HostConfig{Allowance: AllowanceOff}}, host)
		if b.Host.Allowance != 0 {
			t.Fatalf("an explicit 0 resolved to %d, want off", b.Host.Allowance)
		}
		for _, n := range notes {
			if n.Key == "allowance" && n.Value != "off" {
				t.Fatalf("an explicit 0 reads %q, want off", n.Value)
			}
		}
	})
	t.Run("warn under the allowance stays normal", func(t *testing.T) {
		r := newRig(t)
		r.h.pressure = PressureWarn
		r.h.footprint, r.h.children = 1*GiB, 300*MiB // 1.3 GiB < 1.8 GiB
		r.stepN(300)
		r.want(LevelNormal, "warn with fathom under its allowance")
		r.h.footprint, r.h.children = r.b.Host.Allowance-1, 0
		r.stepN(50)
		r.want(LevelNormal, "warn one byte under the allowance")
	})
	t.Run("warn at or over the allowance sheds after two samples", func(t *testing.T) {
		r := newRig(t)
		r.h.pressure = PressureWarn
		r.h.footprint = 1 * GiB
		r.stepN(10)
		r.want(LevelNormal, "precondition")
		r.h.footprint, r.h.children = r.b.Host.Allowance-200*MiB, 200*MiB // children count
		if tr := r.step(0); tr.Changed {
			t.Fatalf("one sample at the allowance moved the ladder: %+v", tr)
		}
		tr := r.step(0)
		r.want(LevelShed, "two samples at the allowance")
		if tr.Cause.Kind != CauseHost || tr.Cause.Trigger != TriggerPressure || !strings.Contains(tr.Reason, "allowance") {
			t.Fatalf("a pressure shed names its trigger and the allowance: %+v", tr)
		}
	})
	t.Run("an explicit 0 sheds as today", func(t *testing.T) {
		r := newRig(t)
		r.b, _ = ResolveBudget(Config{Host: HostConfig{Allowance: AllowanceOff}}, host)
		r.h.pressure = PressureWarn
		r.h.footprint = 70 * MiB
		r.step(0)
		r.want(LevelNormal, "one sample")
		r.step(0)
		r.want(LevelShed, "allowance off: warn sheds at any footprint")
	})
	t.Run("critical refuses whatever the footprint", func(t *testing.T) {
		r := newRig(t)
		r.h.pressure = PressureCritical
		r.h.footprint = 70 * MiB
		tr := r.stepN(2)
		r.want(LevelRefuse, "critical at 70 MB")
		if tr.Cause.Trigger != TriggerPressure {
			t.Fatalf("critical names the pressure trigger: %+v", tr.Cause)
		}
	})
	t.Run("a pressure-entered rung steps down after 60 s under 0.6 × the allowance", func(t *testing.T) {
		r := newRig(t)
		r.h.pressure = PressureWarn
		r.h.footprint = 2 * GiB
		r.stepN(2)
		r.want(LevelShed, "entered on warn at 2 GiB")
		// Under the allowance but over 0.6 × it, still at warn: held.
		r.h.footprint = 1200 * MiB
		r.stepN(100)
		r.want(LevelShed, "warn holds between 0.6 × and 1 × the allowance")
		// Under 0.6 × the allowance (1.08 GiB) at warn: 60 s, then normal.
		r.h.footprint = 1 * GiB
		r.stepN(30)
		r.want(LevelShed, "58 s under 0.6 × the allowance")
		tr := r.stepN(1)
		r.want(LevelNormal, "60 s under 0.6 × the allowance")
		if !strings.Contains(tr.Reason, "allowance") {
			t.Fatalf("the step-down names the allowance: %+v", tr)
		}
		// Pressure clearing steps down by today's rule (0.6 × the shed mark).
		r = newRig(t)
		r.h.pressure = PressureWarn
		r.h.footprint = 2 * GiB
		r.stepN(2)
		r.want(LevelShed, "entered on warn")
		r.h.pressure, r.h.footprint = PressureNormal, 1500*MiB
		r.stepN(30)
		r.want(LevelShed, "pressure cleared, 58 s")
		r.stepN(1)
		r.want(LevelNormal, "pressure cleared: 60 s below 0.6 × the shed mark")
	})
	t.Run("a floor refuse at normal pressure is unchanged", func(t *testing.T) {
		r := newRig(t)
		r.h.footprint = 70 * MiB
		r.h.swapVolFree = 3 * GiB
		tr := r.stepN(2)
		r.want(LevelRefuse, "swap volume under 5 GiB at normal pressure")
		if tr.Cause.Trigger != TriggerSwapVolume || !strings.Contains(tr.Reason, "swap volume free") {
			t.Fatalf("the floor names itself: %+v", tr)
		}
	})
}
