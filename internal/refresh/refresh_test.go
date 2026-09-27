package refresh

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRunner records every run and lets a test hold a run open.
type fakeRunner struct {
	mu      sync.Mutex
	runs    []Job
	starts  []time.Time
	active  int32
	maxSeen int32
	hold    map[string]chan struct{} // service -> closed to let its run finish
	started chan string
	err     error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{hold: map[string]chan struct{}{}, started: make(chan string, 64)}
}

func (f *fakeRunner) gate(service string) chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.hold[service]
	if !ok {
		ch = make(chan struct{})
		f.hold[service] = ch
	}
	return ch
}

func (f *fakeRunner) release(service string) { close(f.gate(service)) }

// rearm makes service's next run block again.
func (f *fakeRunner) rearm(service string) {
	f.mu.Lock()
	f.hold[service] = make(chan struct{})
	f.mu.Unlock()
}

func (f *fakeRunner) run(ctx context.Context, job Job) error {
	n := atomic.AddInt32(&f.active, 1)
	for {
		m := atomic.LoadInt32(&f.maxSeen)
		if n <= m || atomic.CompareAndSwapInt32(&f.maxSeen, m, n) {
			break
		}
	}
	f.mu.Lock()
	f.runs = append(f.runs, job)
	f.starts = append(f.starts, time.Now())
	f.mu.Unlock()
	f.started <- job.Service
	select {
	case <-f.gate(job.Service):
	case <-ctx.Done():
	}
	atomic.AddInt32(&f.active, -1)
	return f.err
}

func (f *fakeRunner) count(service string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, j := range f.runs {
		if j.Service == service {
			n++
		}
	}
	return n
}

func waitStarted(t *testing.T, f *fakeRunner, want string) {
	t.Helper()
	select {
	case got := <-f.started:
		if got != want {
			t.Fatalf("started %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no run of %q started", want)
	}
}

func openGate(string, bool, time.Time) Verdict { return Verdict{} }

func newTestQueue(t *testing.T, f *fakeRunner, mut func(*Config)) *Queue {
	t.Helper()
	cfg := Config{Run: f.run, Gate: openGate, StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false }, PausePoll: 10 * time.Millisecond}
	if mut != nil {
		mut(&cfg)
	}
	q := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	t.Cleanup(func() {
		cancel()
		q.Wait()
	})
	return q
}

// TestRefresh_OneInFlightOneFollowUp is RT-97 (design §7.1): one ingest per
// service — a request while it runs sets dirty for exactly one follow-up,
// never a parallel run — and one heavy slot machine-wide, whichever caller
// (auto-refresh, the ingest tool, post-commit, /refresh-file) asked.
func TestRefresh_OneInFlightOneFollowUp(t *testing.T) {
	f := newFakeRunner()
	q := newTestQueue(t, f, nil)
	ctx := context.Background()

	st := q.RequestRefresh(ctx, Request{Service: "a", Reason: ReasonAuto}, false)
	if st.State != StateQueued && st.State != StateRunning {
		t.Fatalf("first request: state %q", st.State)
	}
	waitStarted(t, f, "a")

	// Three requests from three callers while a runs: one follow-up.
	for _, r := range []Reason{ReasonPostCommit, ReasonRefreshFile, ReasonAuto} {
		st := q.RequestRefresh(ctx, Request{Service: "a", Reason: r}, false)
		if !st.Coalesced || !st.FollowUp {
			t.Fatalf("request mid-run (%s): %+v; want coalesced into one follow-up", r, st)
		}
	}
	// Another service queued behind the one heavy slot: it never starts
	// while a runs.
	q.RequestRefresh(ctx, Request{Service: "b", Reason: ReasonIngestTool}, false)
	select {
	case s := <-f.started:
		t.Fatalf("%q started while a held the heavy slot", s)
	case <-time.After(50 * time.Millisecond):
	}

	// Finish a's run. b is explicit and a's follow-up is background: b
	// goes first, then a's single follow-up.
	f.release("a")
	waitStarted(t, f, "b")
	f.release("b")
	waitStarted(t, f, "a")
	deadline := time.After(5 * time.Second)
	for q.StateOf("a") != StateIdle {
		select {
		case <-deadline:
			t.Fatalf("a never went idle: %q", q.StateOf("a"))
		case <-time.After(5 * time.Millisecond):
		}
	}
	if n := f.count("a"); n != 2 {
		t.Fatalf("a ran %d times; want 2 (the run plus exactly one follow-up)", n)
	}
	if m := atomic.LoadInt32(&f.maxSeen); m != 1 {
		t.Fatalf("%d ingests ran at once; the heavy slot allows 1", m)
	}
	// Every caller's reason is carried by the job that served it.
	f.mu.Lock()
	follow := f.runs[len(f.runs)-1]
	f.mu.Unlock()
	if !strings.Contains(strings.Join(reasonStrings(follow.Reasons), ","), "post-commit") {
		t.Fatalf("follow-up job reasons %v do not carry post-commit", follow.Reasons)
	}
}

func reasonStrings(rs []Reason) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

// TestIngestSurvivesDisconnectAndCoalesces is RT-98 (§7.1): a waiting
// caller that disconnects returns at once and the shared ingest completes
// under the queue's own context; duplicate requests coalesce into the one
// queued job and every waiter gets its result.
func TestIngestSurvivesDisconnectAndCoalesces(t *testing.T) {
	f := newFakeRunner()
	q := newTestQueue(t, f, nil)

	// Hold the slot with another service so "a" stays queued while the
	// duplicates arrive.
	q.RequestRefresh(context.Background(), Request{Service: "busy", Reason: ReasonAuto}, false)
	waitStarted(t, f, "busy")

	var wg sync.WaitGroup
	results := make([]Status, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = q.RequestRefresh(context.Background(), Request{Service: "a", Reason: ReasonIngestTool}, true)
		}(i)
	}
	// A fourth waiter disconnects while a is still queued.
	dctx, disconnect := context.WithCancel(context.Background())
	gone := make(chan Status, 1)
	go func() { gone <- q.RequestRefresh(dctx, Request{Service: "a", Reason: ReasonIngestTool}, true) }()
	time.Sleep(30 * time.Millisecond)
	disconnect()
	select {
	case st := <-gone:
		if !errors.Is(st.Err, context.Canceled) {
			t.Fatalf("disconnected waiter: %+v; want its own context's error", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a disconnected waiter did not return at once")
	}

	f.release("busy")
	waitStarted(t, f, "a")
	f.release("a")
	wg.Wait()
	for i, st := range results {
		if st.Err != nil || !st.Done {
			t.Fatalf("waiter %d: %+v; want the completed run's result", i, st)
		}
	}
	if n := f.count("a"); n != 1 {
		t.Fatalf("a ran %d times for 4 duplicate requests; want 1", n)
	}
	coalesced := 0
	for _, st := range results {
		if st.Coalesced {
			coalesced++
		}
	}
	if coalesced < 2 {
		t.Fatalf("%d of 3 concurrent duplicates coalesced; want at least 2", coalesced)
	}

	// A run whose requester disconnected mid-run still completes: the job
	// runs under the queue's context, not the caller's.
	f.rearm("a")
	rctx, rcancel := context.WithCancel(context.Background())
	done := make(chan Status, 1)
	go func() { done <- q.RequestRefresh(rctx, Request{Service: "a", Reason: ReasonIngestTool}, true) }()
	waitStarted(t, f, "a")
	rcancel()
	<-done
	if q.StateOf("a") != StateRunning {
		t.Fatalf("the ingest stopped with its requester: state %q", q.StateOf("a"))
	}
	f.release("a")
}

// TestRefresh_DemandOrderAndStartupStagger is RT-100 (§7.1): the queue
// starts the most recently queried service first (then the oldest drift),
// waits after a start and staggers during warm-up; pauses background work
// at shed and refuses it at refuse, in safe mode, for a quarantined service
// and in store distress (explicit ingests pass what they may).
func TestRefresh_DemandOrderAndStartupStagger(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)

	t.Run("demand order", func(t *testing.T) {
		cands := []Candidate{
			{Service: "old", Queued: t0.Add(1 * time.Second)},
			{Service: "older", Queued: t0},
			{Service: "hot", Queued: t0.Add(5 * time.Second), LastQueried: t0.Add(9 * time.Second)},
			{Service: "warm", Queued: t0.Add(2 * time.Second), LastQueried: t0.Add(3 * time.Second)},
		}
		var order []string
		for len(cands) > 0 {
			i, wait := nextRefresh(cands, DemandOrder{}, Timing{Now: t0.Add(time.Minute)})
			if i < 0 || wait != 0 {
				t.Fatalf("nextRefresh = %d, %s with eligible candidates", i, wait)
			}
			order = append(order, cands[i].Service)
			cands = append(cands[:i], cands[i+1:]...)
		}
		if got, want := strings.Join(order, ","), "hot,warm,older,old"; got != want {
			t.Fatalf("order %s; want %s (most recently queried, then oldest drift)", got, want)
		}
		// An explicit ingest is someone waiting: it goes first.
		cands = []Candidate{{Service: "hot", LastQueried: t0}, {Service: "asked", Explicit: true}}
		if i, _ := nextRefresh(cands, DemandOrder{}, Timing{Now: t0}); cands[i].Service != "asked" {
			t.Fatalf("explicit request not first: %s", cands[i].Service)
		}
	})

	t.Run("start wait and warm-up stagger", func(t *testing.T) {
		bg := []Candidate{{Service: "a", Queued: t0}}
		tm := Timing{Now: t0.Add(10 * time.Second), Started: t0, StartWait: 60 * time.Second, Stagger: 30 * time.Second}
		if i, wait := nextRefresh(bg, DemandOrder{}, tm); i >= 0 || wait != 50*time.Second {
			t.Fatalf("10 s after a start: %d, %s; want to wait 50 s", i, wait)
		}
		// An explicit ingest is not held by the start wait.
		ex := []Candidate{{Service: "a", Explicit: true}}
		if i, _ := nextRefresh(ex, DemandOrder{}, tm); i != 0 {
			t.Fatalf("explicit ingest held by the start wait")
		}
		// During warm-up, one service per stagger interval.
		tm = Timing{Now: t0.Add(70 * time.Second), Started: t0, StartWait: 60 * time.Second, Stagger: 30 * time.Second, Warm: true, LastStart: t0.Add(61 * time.Second)}
		if i, wait := nextRefresh(bg, DemandOrder{}, tm); i >= 0 || wait != 21*time.Second {
			t.Fatalf("9 s after the last start in warm-up: %d, %s; want to wait 21 s", i, wait)
		}
		tm.Warm = false
		if i, _ := nextRefresh(bg, DemandOrder{}, tm); i != 0 {
			t.Fatalf("after warm-up the stagger must not hold")
		}
	})

	t.Run("live stagger", func(t *testing.T) {
		f := newFakeRunner()
		q := newTestQueue(t, f, func(c *Config) {
			c.StartWait = -1
			c.Stagger = 120 * time.Millisecond
			c.Warmup = func(time.Time) bool { return true }
		})
		for _, s := range []string{"x", "y"} {
			f.release(s)
			q.RequestRefresh(context.Background(), Request{Service: s, Reason: ReasonAuto}, false)
		}
		<-f.started
		<-f.started
		f.mu.Lock()
		gap := f.starts[1].Sub(f.starts[0])
		f.mu.Unlock()
		if gap < 100*time.Millisecond {
			t.Fatalf("second start %s after the first during warm-up; want ≥ the 120 ms stagger", gap)
		}
	})

	t.Run("gates", func(t *testing.T) {
		var level atomic.Value
		level.Store("")
		f := newFakeRunner()
		q := newTestQueue(t, f, func(c *Config) {
			c.Gate = func(service string, explicit bool, now time.Time) Verdict {
				switch l := level.Load().(string); {
				case l == "shed" && !explicit:
					return Verdict{Pause: "paused (shed)"}
				case l == "refuse":
					return Verdict{Refuse: "refused (refuse)"}
				case l == "safe" && !explicit:
					return Verdict{Refuse: "refused (safe mode)"}
				case l == "distress":
					return Verdict{Refuse: "store distress: disk full"}
				case service == "quarantined" && !explicit:
					return Verdict{Refuse: "refused (quarantined)"}
				}
				return Verdict{}
			}
		})
		for _, l := range []string{"refuse", "safe", "distress"} {
			level.Store(l)
			st := q.RequestRefresh(context.Background(), Request{Service: "a", Reason: ReasonPostCommit}, false)
			if st.State != StateRefused || st.Refused == "" {
				t.Fatalf("%s: background request %+v; want refused with the reason", l, st)
			}
		}
		level.Store("")
		if st := q.RequestRefresh(context.Background(), Request{Service: "quarantined", Reason: ReasonPostCommit}, false); st.State != StateRefused {
			t.Fatalf("quarantined service re-queued by a hook: %+v", st)
		}
		// Shed pauses background work: queued, not started.
		level.Store("shed")
		st := q.RequestRefresh(context.Background(), Request{Service: "p", Reason: ReasonAuto}, false)
		if st.State != StateQueued || st.Paused == "" {
			t.Fatalf("at shed: %+v; want queued and paused", st)
		}
		select {
		case s := <-f.started:
			t.Fatalf("%q started while paused at shed", s)
		case <-time.After(60 * time.Millisecond):
		}
		// Refuse drops the paused job with its reason.
		done := make(chan error, 1)
		q.RequestRefresh(context.Background(), Request{Service: "p", Reason: ReasonAuto, Done: func(err error) { done <- err }}, false)
		level.Store("refuse")
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "refuse") {
				t.Fatalf("paused job dropped at refuse with %v; want the refusal", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("a paused job was not dropped at refuse")
		}
		if f.count("p") != 0 {
			t.Fatal("a paused job ran")
		}
		// Back to normal: an explicit request for the quarantined service
		// runs (only an explicit ingest lifts a quarantine).
		level.Store("")
		f.release("quarantined")
		st = q.RequestRefresh(context.Background(), Request{Service: "quarantined", Reason: ReasonIngestTool}, true)
		if st.Err != nil || !st.Done {
			t.Fatalf("explicit ingest of a quarantined service: %+v", st)
		}
	})
}
