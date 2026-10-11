package refresh

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// waitRun polls q until run's status reads done (a safety bound, never
// asserted on as a duration).
func waitRun(t *testing.T, q *Queue, service, run string) RunState {
	t.Helper()
	deadline := time.Now().Add(waitSafety)
	for {
		rs := q.RunStatus(service, run)
		if rs.Done {
			return rs
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s of %s never finished: %+v", run, service, rs)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// steppedRunner hands each run's outcome to the test: a run announces its
// start, then returns whatever the test sends.
type steppedRunner struct {
	started chan Job
	result  chan error
}

func (s *steppedRunner) run(ctx context.Context, job Job) error {
	s.started <- job
	select {
	case err := <-s.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *steppedRunner) waitStart(t *testing.T) Job {
	t.Helper()
	select {
	case j := <-s.started:
		return j
	case <-time.After(waitSafety):
		t.Fatal("no run started")
		return Job{}
	}
}

// TestRunStatusFollowsARequestToItsFinish is EW-66 (G4): a request's answer
// names the run that serves it, and RunStatus reads that run's state from
// the queue without requesting anything: queued or running, then done with
// its outcome (a success, then a failure named). A request while the
// service runs is served by the one follow-up, a run of its own; a later
// finished run marks an earlier one superseded; a run id this queue never
// issued, or one from another queue (a restarted daemon), reads unknown.
func TestRunStatusFollowsARequestToItsFinish(t *testing.T) {
	sr := &steppedRunner{started: make(chan Job, 4), result: make(chan error)}
	q := New(Config{Run: sr.run, Gate: openGate, StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false }, PausePoll: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	t.Cleanup(func() { cancel(); q.Wait() })

	first := q.RequestRefresh(ctx, Request{Service: "a", Reason: ReasonIngestTool}, false)
	if first.Run == "" {
		t.Fatalf("request answered %+v with no run", first)
	}
	if rs := q.RunStatus("a", first.Run); !rs.Known || rs.Done || (rs.State != StateQueued && rs.State != StateRunning) {
		t.Fatalf("RunStatus before the run finished = %+v, want known, not done, queued or running", rs)
	}
	sr.waitStart(t)
	if rs := q.RunStatus("a", first.Run); rs.Done || rs.State != StateRunning {
		t.Fatalf("RunStatus while it runs = %+v, want running", rs)
	}

	// Joined while running: the one follow-up, a run of its own, shared by
	// every later mid-run request.
	follow := q.RequestRefresh(ctx, Request{Service: "a", Reason: ReasonPostCommit}, false)
	if !follow.FollowUp || follow.Run == "" || follow.Run == first.Run {
		t.Fatalf("mid-run request = %+v, want the follow-up's own run (first was %s)", follow, first.Run)
	}
	if again := q.RequestRefresh(ctx, Request{Service: "a", Reason: ReasonAuto}, false); again.Run != follow.Run {
		t.Fatalf("a second mid-run request got run %s, want the one follow-up's %s", again.Run, follow.Run)
	}
	if rs := q.RunStatus("a", follow.Run); !rs.Known || rs.Done || rs.State != StateQueued {
		t.Fatalf("follow-up's status while the first runs = %+v, want queued", rs)
	}

	sr.result <- nil
	if rs := waitRun(t, q, "a", first.Run); !rs.Known || rs.Err != "" || rs.Superseded {
		t.Fatalf("first run finished = %+v, want known, no error, not superseded", rs)
	}
	sr.waitStart(t) // the follow-up
	if rs := q.RunStatus("a", follow.Run); rs.Done || rs.State != StateRunning {
		t.Fatalf("follow-up while it runs = %+v, want running", rs)
	}
	sr.result <- errors.New("database or disk is full")
	if rs := waitRun(t, q, "a", follow.Run); rs.Err != "database or disk is full" || rs.Superseded {
		t.Fatalf("follow-up finished = %+v, want its failure named", rs)
	}
	if rs := q.RunStatus("a", first.Run); !rs.Done || !rs.Superseded {
		t.Fatalf("the first run after a later one finished = %+v, want done and superseded", rs)
	}
	// A queued request joining a queued job shares its run.
	q2 := q.RequestRefresh(ctx, Request{Service: "b", Reason: ReasonAuto}, false)
	if j := sr.waitStart(t); j.Service != "b" {
		t.Fatalf("started %q, want b", j.Service)
	}
	sr.result <- nil
	waitRun(t, q, "b", q2.Run)

	// b's run is not a's (a's last finished run is older than it).
	for _, bogus := range []string{"", "nonsense", first.Run + "0", "zz-1", q2.Run} {
		if rs := q.RunStatus("a", bogus); rs.Known {
			t.Errorf("RunStatus(a, %q) = %+v, want unknown", bogus, rs)
		}
	}
	// Another queue's ids (a daemon that restarted) are unknown there.
	other := New(Config{Run: sr.run, Gate: openGate, Started: time.Now().Add(-time.Hour)})
	if rs := other.RunStatus("a", first.Run); rs.Known {
		t.Errorf("another queue read this queue's run as %+v, want unknown", rs)
	}
}

// TestRunStatusNamesARunTheGateDropped (EW-66): a queued run the gate drops
// before it starts (store distress, a refusal level reached while it waited)
// finished, with the refusal as its outcome. Reading it as unknown told a
// polling caller the daemon had restarted, which it had not.
func TestRunStatusNamesARunTheGateDropped(t *testing.T) {
	var level atomic.Value
	level.Store("shed")
	sr := &steppedRunner{started: make(chan Job, 4), result: make(chan error)}
	q := New(Config{Run: sr.run, StartWait: -1, Stagger: -1, Warmup: func(time.Time) bool { return false }, PausePoll: 10 * time.Millisecond,
		Gate: func(_ string, _ bool, _ time.Time) Verdict {
			switch level.Load().(string) {
			case "shed":
				return Verdict{Pause: "paused (shed)"}
			case "refuse":
				return Verdict{Refuse: "store distress: disk full"}
			}
			return Verdict{}
		}})
	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	t.Cleanup(func() { cancel(); q.Wait() })

	dropped := make(chan error, 1)
	st := q.RequestRefresh(ctx, Request{Service: "a", Reason: ReasonAuto, Done: func(err error) { dropped <- err }}, false)
	if st.Run == "" || st.State != StateQueued {
		t.Fatalf("request at shed = %+v, want queued with a run", st)
	}
	level.Store("refuse")
	select {
	case <-dropped:
	case <-time.After(waitSafety):
		t.Fatal("the gate never dropped the paused run")
	}
	rs := q.RunStatus("a", st.Run)
	if !rs.Known || !rs.Done || !strings.Contains(rs.Err, "store distress: disk full") || rs.Superseded {
		t.Fatalf("RunStatus of a run the gate dropped = %+v, want known, done, the refusal named", rs)
	}
}
