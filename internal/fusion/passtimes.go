package fusion

// passtimes.go — per-pass wall and CPU time on the ingest Report (EW-48,
// wave ic; O362: the whole-service ingest cost grew superlinear and the
// report could not say which pass grew).
//
// An ingest's passes run one after another, so the timer is a lap counter:
// the recorder opens a lap when the ingest starts and each mark(name)
// closes the open lap as the top-level pass name and opens the next. A
// pass added later is one mark line after its call. Two refinements keep
// the entries honest:
//
//   - span(name) times a top-level pass that runs inside another pass's
//     lap (centrality, inside the final commit) by its own start and stop;
//     its time is subtracted from the lap it ran in, so the top-level
//     entries never count a nanosecond twice and sum to the ingest's time.
//   - sub(name) accumulates a part of the open lap (store writes inside
//     the extract stream, the reference resolution inside the file-resolved
//     pass, ...). A sub-pass entry names its Parent, follows it, and its
//     time is included in the parent's.
//
// CPU is the process's user + sys time (getrusage RUSAGE_SELF), read at
// the same points as wall time. It is process-wide: in the daemon
// (serve-http) a pass's CPU includes whatever else the process ran
// meanwhile (another service's ingest, a query); only a CLI ingest's CPU is
// its passes' own. A parallel pass (the extract stream's reads) can show
// more CPU than wall.
//
// The clock is injected (internal/AGENTS.md): Ingest reads WithPassClock's
// clock from its context and falls back to SystemPassClock. A zero
// PassClock turns timing off and leaves Report.PassTimes nil, so a test
// that pins output keeps it byte-identical.

import (
	"context"
	"sync"
	"syscall"
	"time"
)

// PassTime is one ingest pass's measured cost (Report.PassTimes). Parent is
// "" for a top-level pass, whose times exclude any top-level pass nested in
// it; a sub-pass names the pass its time is part of.
type PassTime struct {
	Name   string
	Parent string
	WallNS int64
	CPUNS  int64
}

// PassClock is the per-pass timer's clock: Now reads wall time and CPU the
// process's cumulative CPU time. A zero PassClock (either func nil) turns
// the timer off.
type PassClock struct {
	Now func() time.Time
	CPU func() time.Duration
}

// SystemPassClock is the production clock: time.Now and the process's
// user + sys CPU time from getrusage (process-wide; see the file comment).
func SystemPassClock() PassClock {
	return PassClock{Now: time.Now, CPU: processCPUTime}
}

// processCPUTime is the process's cumulative user + sys CPU time; 0 when
// getrusage fails (the entries then read 0 CPU, never a wrong number).
func processCPUTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

type passClockKey struct{}

// WithPassClock returns ctx carrying the clock the next Ingest under it
// times its passes with: a fake for a test, PassClock{} for none.
func WithPassClock(ctx context.Context, c PassClock) context.Context {
	return context.WithValue(ctx, passClockKey{}, c)
}

type passRecorderKey struct{}

// passRecorder collects one ingest's PassTime entries. Its methods are safe
// on a nil recorder (timing off) and from several goroutines.
type passRecorder struct {
	clock PassClock

	mu  sync.Mutex
	out []PassTime
	// the open lap: its start, the top-level spans recorded inside it, and
	// its sub-passes in first-seen order
	lapWall    time.Time
	lapCPU     time.Duration
	nestedWall int64
	nestedCPU  int64
	subs       []PassTime
}

// startPassTimes attaches a recorder for one ingest to ctx and opens its
// first lap; (ctx, nil) when ctx's clock is the zero PassClock.
func startPassTimes(ctx context.Context) (context.Context, *passRecorder) {
	clock := SystemPassClock()
	if c, ok := ctx.Value(passClockKey{}).(PassClock); ok {
		clock = c
	}
	if clock.Now == nil || clock.CPU == nil {
		return ctx, nil
	}
	r := &passRecorder{clock: clock}
	r.lapWall, r.lapCPU = clock.Now(), clock.CPU()
	return context.WithValue(ctx, passRecorderKey{}, r), r
}

// passTimesFrom is the recorder of the ingest ctx belongs to; nil outside
// one (a lazy ExtractPending, a test calling a pass directly) or with
// timing off.
func passTimesFrom(ctx context.Context) *passRecorder {
	r, _ := ctx.Value(passRecorderKey{}).(*passRecorder)
	return r
}

// mark closes the open lap as the top-level pass name (its time less the
// spans recorded inside it), appends its sub-passes after it, and opens
// the next lap.
func (r *passRecorder) mark(name string) {
	if r == nil {
		return
	}
	now, cpu := r.clock.Now(), r.clock.CPU()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.out = append(r.out, PassTime{
		Name:   name,
		WallNS: now.Sub(r.lapWall).Nanoseconds() - r.nestedWall,
		CPUNS:  int64(cpu-r.lapCPU) - r.nestedCPU,
	})
	for _, s := range r.subs {
		s.Parent = name
		r.out = append(r.out, s)
	}
	r.lapWall, r.lapCPU, r.nestedWall, r.nestedCPU, r.subs = now, cpu, 0, 0, nil
}

// noop is the stop func of a recorder that is off.
func noop() {}

// span starts top-level pass name inside the open lap; the returned stop
// records it and excludes its time from that lap.
func (r *passRecorder) span(name string) (stop func()) {
	if r == nil {
		return noop
	}
	w0, c0 := r.clock.Now(), r.clock.CPU()
	return func() {
		wall, cpu := r.clock.Now().Sub(w0).Nanoseconds(), int64(r.clock.CPU()-c0)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.out = append(r.out, PassTime{Name: name, WallNS: wall, CPUNS: cpu})
		r.nestedWall += wall
		r.nestedCPU += cpu
	}
}

// sub starts one span of sub-pass name of the open lap; the returned stop
// adds the span to the sub-pass's total.
func (r *passRecorder) sub(name string) (stop func()) {
	if r == nil {
		return noop
	}
	w0, c0 := r.clock.Now(), r.clock.CPU()
	return func() {
		wall, cpu := r.clock.Now().Sub(w0).Nanoseconds(), int64(r.clock.CPU()-c0)
		r.mu.Lock()
		defer r.mu.Unlock()
		for i := range r.subs {
			if r.subs[i].Name == name {
				r.subs[i].WallNS += wall
				r.subs[i].CPUNS += cpu
				return
			}
		}
		r.subs = append(r.subs, PassTime{Name: name, WallNS: wall, CPUNS: cpu})
	}
}

// times is the entries recorded so far, in execution order; nil when off.
// A lap still open (an ingest that returned before its next mark) is not
// an entry.
func (r *passRecorder) times() []PassTime {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]PassTime(nil), r.out...)
}
