// Package refresh is the daemon's refresh queue (docs/design/
// responsible-tenant.md §7.1, RT-97..RT-100): every whole-service ingest
// the daemon runs — an auto-refresh after drift, the `ingest` tool, the
// post-commit hook's POST /refresh-request, /refresh-file's whole-service
// case — is a request here, and one worker runs them.
//
//   - One ingest per service: a request while its ingest runs sets dirty,
//     which becomes exactly one follow-up run — never a parallel one.
//     Requests for a queued service coalesce into the queued job.
//   - One heavy slot: the worker runs one job at a time, process-wide.
//   - Jobs run under the worker's context (the daemon's lifetime), never a
//     requester's: a caller that waits does so under its own context and
//     returns when it is done, and the shared job completes.
//   - Order: explicit ingests (someone is waiting) first, then the most
//     recently queried service, then the oldest drift (Orderer is an
//     interface — the decision-port option stays free). After a start the
//     queue waits StartWait before background work and, during warm-up,
//     starts one background job per Stagger.
//   - The gate (Verdict) pauses background work at shed and refuses at
//     refuse, in safe mode, for a quarantined service and in store
//     distress; an explicit ingest passes a quarantine (only it lifts one)
//     and safe mode (which refuses tree and fleet reads, not ingest).
//   - Every request's answer names the run that serves it (Status.Run), and
//     RunStatus reads that run's state later without requesting anything
//     (EW-66): a caller answered before its run finished can learn when it
//     did, and how, from the queue's own per-service state.
package refresh

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Reason names why a refresh was requested.
type Reason string

// The callers of the queue.
const (
	ReasonAuto        Reason = "auto-refresh"
	ReasonIngestTool  Reason = "ingest"
	ReasonPostCommit  Reason = "post-commit"
	ReasonRefreshFile Reason = "refresh-file"
)

// Explicit reports whether r is a caller's explicit request to ingest (the
// `ingest` tool): it is ordered first and passes a quarantine and safe
// mode.
func (r Reason) Explicit() bool { return r == ReasonIngestTool }

// State is one service's place in the queue.
type State string

// Service states. A running service with a follow-up pending is
// StateRunning with Status.FollowUp.
const (
	StateIdle    State = "idle"
	StateQueued  State = "queued"
	StateRunning State = "running"
	StateRefused State = "refused"
)

// Request asks for one service's refresh.
type Request struct {
	Service string
	// Path is an explicit corpus root (onboarding); "" means the service's
	// recorded root. A later request's non-empty Path replaces an earlier
	// one's in the coalesced job.
	Path   string
	Reason Reason
	// Done, when set, is called once with the result of the run that
	// satisfied this request, or with the refusal that dropped it.
	Done func(error)
}

// Job is what the worker hands Run: one service, the union of the requests
// it serves.
type Job struct {
	Service  string
	Path     string
	Reasons  []Reason // sorted, distinct
	Explicit bool
	run      uint64 // the run's sequence number (RunStatus)
}

// Status answers a request.
type Status struct {
	Service string
	State   State
	// Coalesced is true when the request joined a job already queued or
	// running instead of creating one.
	Coalesced bool
	// FollowUp is true when the request arrived while the service's ingest
	// ran: it is served by the one follow-up run.
	FollowUp bool
	// Refused names why the request was refused (State == StateRefused).
	Refused string
	// Paused names why the queue is not starting background work now.
	Paused string
	// Done is true when a waiting request's run finished; Err is its
	// result (or the waiter's own context error when it stopped waiting).
	Done bool
	Err  error
	// Run identifies the run that serves the request (the queued job, or
	// the one follow-up of a running one), for RunStatus. "" when refused.
	Run string
}

// RunState is one run's place, read by RunStatus.
type RunState struct {
	// Known is false for a run id this queue did not issue for the
	// service: malformed, another service's, or another queue's (the
	// daemon restarted since; its in-flight ingest resumes on the next
	// start, RT-109).
	Known bool
	// Done is true once the run finished; Err names its failure ("" for a
	// success). Superseded: a later run of the service has finished since,
	// and Err is that later run's outcome.
	Done       bool
	Err        string
	Superseded bool
	// State is the run's queue state while it has not finished: queued
	// (waiting, or the follow-up of a running run) or running.
	State State
}

// ErrRefused wraps every refusal a request or a dropped job reports.
var ErrRefused = errors.New("refresh refused")

// Verdict is the gate's answer for one service now: Refuse drops the
// request (and a queued job), Pause holds it queued.
type Verdict struct {
	Pause  string
	Refuse string
}

// Gate decides whether a service's refresh may start now.
type Gate func(service string, explicit bool, now time.Time) Verdict

// Candidate is a queued job as the ordering sees it.
type Candidate struct {
	Service     string
	Explicit    bool
	Queued      time.Time // when the job (or its drift) was first queued
	LastQueried time.Time // the most recent read of the service (demand)
}

// Orderer picks the next job among eligible candidates.
type Orderer interface {
	// Less reports whether a should start before b.
	Less(a, b Candidate) bool
}

// DemandOrder is the default ordering: explicit first, then the most
// recently queried, then the oldest queued, then by name.
type DemandOrder struct{}

// Less implements Orderer.
func (DemandOrder) Less(a, b Candidate) bool {
	if a.Explicit != b.Explicit {
		return a.Explicit
	}
	if !a.LastQueried.Equal(b.LastQueried) {
		return a.LastQueried.After(b.LastQueried)
	}
	if !a.Queued.Equal(b.Queued) {
		return a.Queued.Before(b.Queued)
	}
	return a.Service < b.Service
}

// Timing is the clock state nextRefresh decides from.
type Timing struct {
	Now       time.Time
	Started   time.Time     // the process start
	StartWait time.Duration // no background job before Started+StartWait
	Warm      bool          // warm-up after a start is in effect
	LastStart time.Time     // the last job start
	Stagger   time.Duration // during warm-up, background starts at most one per Stagger
}

// nextRefresh picks the candidate to start now, or -1 with how long to wait
// before one becomes eligible (0 when none is queued). Pure. An explicit
// candidate is never held by the start wait or the stagger (someone is
// waiting on it; admission already admitted it).
func nextRefresh(cands []Candidate, order Orderer, tm Timing) (int, time.Duration) {
	if order == nil {
		order = DemandOrder{}
	}
	var hold time.Duration
	if tm.StartWait > 0 && !tm.Started.IsZero() {
		if until := tm.Started.Add(tm.StartWait); tm.Now.Before(until) {
			hold = until.Sub(tm.Now)
		}
	}
	if tm.Warm && tm.Stagger > 0 && !tm.LastStart.IsZero() {
		if until := tm.LastStart.Add(tm.Stagger); tm.Now.Before(until) && until.Sub(tm.Now) > hold {
			hold = until.Sub(tm.Now)
		}
	}
	best := -1
	for i, c := range cands {
		if !c.Explicit && hold > 0 {
			continue
		}
		if best < 0 || order.Less(c, cands[best]) {
			best = i
		}
	}
	if best >= 0 {
		return best, 0
	}
	if len(cands) == 0 {
		return -1, 0
	}
	return -1, hold
}

// Config configures a Queue.
type Config struct {
	// Run ingests one job under ctx (the worker's). Required.
	Run func(ctx context.Context, job Job) error
	// Gate decides pause/refuse; nil = TenancyGate.
	Gate Gate
	// Order ranks eligible jobs; nil = DemandOrder.
	Order Orderer
	// Now is the clock; nil = time.Now.
	Now func() time.Time
	// StartWait holds background work after the process start (default
	// 60 s; negative = none). Started is the start (zero = New's now).
	StartWait time.Duration
	Started   time.Time
	// Stagger spaces background starts during warm-up (default 30 s;
	// negative = none). Warmup reports whether warm-up is in effect (nil =
	// tenancy.InWarmup).
	Stagger time.Duration
	Warmup  func(time.Time) bool
	// PausePoll is how often a paused queue re-asks the gate (default 2 s,
	// the governor's sample interval).
	PausePoll time.Duration
}

type waiter struct {
	done chan struct{}
	err  error
	fn   func(error)
}

type entry struct {
	state    State
	path     string
	reasons  map[Reason]bool
	explicit bool
	queued   time.Time
	waiters  []*waiter
	// run is the queued or running job's sequence number; nextRun the one
	// follow-up's, issued to the first request that made the entry dirty.
	run     uint64
	nextRun uint64
	// dirty: a request arrived while running; next* describe the one
	// follow-up it becomes.
	dirty        bool
	nextPath     string
	nextReasons  map[Reason]bool
	nextExplicit bool
	nextWaiters  []*waiter
}

// Queue is the refresh queue. The zero value is not usable; use New.
type Queue struct {
	cfg     Config
	mu      sync.Mutex
	entries map[string]*entry
	queried map[string]time.Time
	last    time.Time // last job start
	running string
	kick    chan struct{}
	stopped chan struct{}
	started bool
	failed  map[string]Failure // service -> its last job's failure (cleared by a success)
	// seq numbers runs, in issue order; finished is each service's last
	// finished run and its outcome (RunStatus): one row per service, like
	// failed, never one per job.
	seq      uint64
	finished map[string]finishedRun
}

// finishedRun is a service's last finished run: its sequence number and
// error text ("" for a success).
type finishedRun struct {
	run uint64
	err string
}

// runID is the opaque id Status.Run carries: the queue's start (so another
// queue's ids never read as this one's) and the run's sequence number.
func (q *Queue) runID(seq uint64) string {
	return strconv.FormatInt(q.cfg.Started.UnixNano(), 36) + "-" + strconv.FormatUint(seq, 10)
}

// runSeq parses a runID of this queue; 0 when it is not one.
func (q *Queue) runSeq(id string) uint64 {
	epoch, n, ok := strings.Cut(id, "-")
	if !ok || epoch != strconv.FormatInt(q.cfg.Started.UnixNano(), 36) {
		return 0
	}
	seq, err := strconv.ParseUint(n, 10, 64)
	if err != nil {
		return 0
	}
	return seq
}

// RunStatus reads the run id a request was answered with (Status.Run):
// queued or running, done with its outcome, or unknown. It requests
// nothing.
func (q *Queue) RunStatus(service, run string) RunState {
	seq := q.runSeq(run)
	q.mu.Lock()
	defer q.mu.Unlock()
	if seq == 0 || seq > q.seq {
		return RunState{}
	}
	if e, ok := q.entries[service]; ok {
		switch {
		case e.run == seq:
			return RunState{Known: true, State: e.state}
		case e.dirty && e.nextRun == seq:
			return RunState{Known: true, State: StateQueued}
		}
	}
	if f, ok := q.finished[service]; ok && f.run >= seq {
		return RunState{Known: true, Done: true, Err: f.err, Superseded: f.run > seq}
	}
	return RunState{}
}

// Failure is a service's last failed job (I38): recorded whether or not
// anyone waited on it, so a queued job's failure is never silent.
type Failure struct {
	At      time.Time
	Err     string
	Reasons []Reason
}

// LastFailure is service's last failed job, if its most recent job failed.
func (q *Queue) LastFailure(service string) (Failure, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	f, ok := q.failed[service]
	return f, ok
}

// New returns a queue; Start runs its worker.
func New(cfg Config) *Queue {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Gate == nil {
		cfg.Gate = TenancyGate
	}
	if cfg.Order == nil {
		cfg.Order = DemandOrder{}
	}
	if cfg.StartWait == 0 {
		cfg.StartWait = 60 * time.Second
	}
	if cfg.Stagger == 0 {
		cfg.Stagger = 30 * time.Second
	}
	if cfg.Warmup == nil {
		cfg.Warmup = tenancyWarmup
	}
	if cfg.PausePoll <= 0 {
		cfg.PausePoll = 2 * time.Second
	}
	if cfg.Started.IsZero() {
		cfg.Started = cfg.Now()
	}
	return &Queue{
		cfg:      cfg,
		entries:  map[string]*entry{},
		queried:  map[string]time.Time{},
		failed:   map[string]Failure{},
		finished: map[string]finishedRun{},
		kick:     make(chan struct{}, 1),
		stopped:  make(chan struct{}),
	}
}

// Start runs the worker until ctx is done; a job in flight when ctx ends is
// cancelled with it (the ingest stops at its next batch boundary and stays
// in_progress for the next start to resume).
func (q *Queue) Start(ctx context.Context) {
	q.mu.Lock()
	if q.started {
		q.mu.Unlock()
		return
	}
	q.started = true
	q.mu.Unlock()
	go q.work(ctx)
}

// Wait blocks until the worker has stopped (after Start's ctx ended).
func (q *Queue) Wait() { <-q.stopped }

// NoteQueried records a read of service now — the demand the ordering
// prefers.
func (q *Queue) NoteQueried(service string) {
	if service == "" {
		return
	}
	now := q.cfg.Now()
	q.mu.Lock()
	q.queried[service] = now
	q.mu.Unlock()
}

// StateOf is service's state now.
func (q *Queue) StateOf(service string) State {
	q.mu.Lock()
	defer q.mu.Unlock()
	if e, ok := q.entries[service]; ok {
		return e.state
	}
	return StateIdle
}

// Pending lists the services queued or running, sorted, with their state.
func (q *Queue) Pending() map[string]State {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make(map[string]State, len(q.entries))
	for s, e := range q.entries {
		out[s] = e.state
	}
	return out
}

// RequestRefresh asks for service's refresh. With wait it blocks, under
// ctx, until the run that satisfies it finishes (a ctx that ends first
// returns its error; the job runs on). Without wait it returns at once
// with the queue's answer.
func (q *Queue) RequestRefresh(ctx context.Context, req Request, wait bool) Status {
	now := q.cfg.Now()
	st := Status{Service: req.Service}
	explicit := req.Reason.Explicit()
	v := q.cfg.Gate(req.Service, explicit, now)
	if v.Refuse != "" {
		st.State, st.Refused = StateRefused, v.Refuse
		st.Err = refusedErr(v.Refuse)
		if req.Done != nil {
			req.Done(st.Err)
		}
		return st
	}
	st.Paused = v.Pause
	w := &waiter{done: make(chan struct{}), fn: req.Done}

	q.mu.Lock()
	e, ok := q.entries[req.Service]
	switch {
	case !ok:
		q.seq++
		e = &entry{state: StateQueued, path: req.Path, reasons: map[Reason]bool{req.Reason: true}, explicit: explicit, queued: now, waiters: []*waiter{w}, run: q.seq}
		q.entries[req.Service] = e
		st.State, st.Run = StateQueued, q.runID(e.run)
	case e.state == StateQueued:
		if req.Path != "" {
			e.path = req.Path
		}
		e.reasons[req.Reason] = true
		e.explicit = e.explicit || explicit
		e.waiters = append(e.waiters, w)
		st.State, st.Coalesced, st.Run = StateQueued, true, q.runID(e.run)
	default: // running
		if req.Path != "" {
			e.nextPath = req.Path
		}
		if e.nextReasons == nil {
			e.nextReasons = map[Reason]bool{}
		}
		e.nextReasons[req.Reason] = true
		e.nextExplicit = e.nextExplicit || explicit
		e.nextWaiters = append(e.nextWaiters, w)
		e.dirty = true
		if e.nextRun == 0 {
			q.seq++
			e.nextRun = q.seq
		}
		st.State, st.Coalesced, st.FollowUp, st.Run = StateRunning, true, true, q.runID(e.nextRun)
	}
	q.mu.Unlock()
	q.wake()

	if !wait {
		return st
	}
	select {
	case <-w.done:
		st.Done, st.Err = true, w.err
	case <-ctx.Done():
		st.Err = ctx.Err()
	}
	return st
}

func refusedErr(reason string) error {
	return &refusalError{reason: reason}
}

type refusalError struct{ reason string }

func (e *refusalError) Error() string   { return "refresh refused: " + e.reason }
func (e *refusalError) Is(t error) bool { return t == ErrRefused }

func (q *Queue) wake() {
	select {
	case q.kick <- struct{}{}:
	default:
	}
}

func finish(ws []*waiter, err error) {
	for _, w := range ws {
		w.err = err
		close(w.done)
		if w.fn != nil {
			w.fn(err)
		}
	}
}

func sortedReasons(m map[Reason]bool) []Reason {
	out := make([]Reason, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// pick chooses the next job under the gate, dropping refused jobs and
// skipping paused ones. It returns the job (ok) or how long to sleep.
func (q *Queue) pick() (Job, []*waiter, bool, time.Duration) {
	now := q.cfg.Now()
	q.mu.Lock()
	var cands []Candidate
	var dropped []struct {
		ws  []*waiter
		err error
	}
	paused := false
	names := make([]string, 0, len(q.entries))
	for s := range q.entries {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, s := range names {
		e := q.entries[s]
		if e.state != StateQueued {
			continue
		}
		v := q.cfg.Gate(s, e.explicit, now)
		if v.Refuse != "" {
			err := refusedErr(v.Refuse)
			dropped = append(dropped, struct {
				ws  []*waiter
				err error
			}{e.waiters, err})
			// EW-66: a dropped run finished, refused; RunStatus names that,
			// never "unknown" (which reads as a restarted daemon).
			if e.run != 0 {
				q.finished[s] = finishedRun{run: e.run, err: err.Error()}
			}
			delete(q.entries, s)
			continue
		}
		if v.Pause != "" {
			paused = true
			continue
		}
		cands = append(cands, Candidate{Service: s, Explicit: e.explicit, Queued: e.queued, LastQueried: q.queried[s]})
	}
	tm := Timing{Now: now, Started: q.cfg.Started, StartWait: q.cfg.StartWait, Warm: q.cfg.Warmup(now), LastStart: q.last, Stagger: q.cfg.Stagger}
	i, wait := nextRefresh(cands, q.cfg.Order, tm)
	var job Job
	var ws []*waiter
	if i >= 0 {
		s := cands[i].Service
		e := q.entries[s]
		e.state = StateRunning
		job = Job{Service: s, Path: e.path, Reasons: sortedReasons(e.reasons), Explicit: e.explicit, run: e.run}
		ws = e.waiters
		e.waiters = nil
		q.last = now
		q.running = s
	}
	q.mu.Unlock()
	for _, d := range dropped {
		finish(d.ws, d.err)
	}
	if i >= 0 {
		return job, ws, true, 0
	}
	if paused && (wait == 0 || wait > q.cfg.PausePoll) {
		wait = q.cfg.PausePoll
	}
	return Job{}, nil, false, wait
}

// complete records job's result and turns a dirty service into its one
// follow-up.
func (q *Queue) complete(job Job, ws []*waiter, err error) {
	now := q.cfg.Now()
	q.mu.Lock()
	e := q.entries[job.Service]
	q.running = ""
	switch {
	case err == nil:
		delete(q.failed, job.Service)
	case !errors.Is(err, ErrRefused):
		q.failed[job.Service] = Failure{At: now, Err: err.Error(), Reasons: job.Reasons}
	}
	if job.run != 0 {
		f := finishedRun{run: job.run}
		if err != nil {
			f.err = err.Error()
		}
		q.finished[job.Service] = f
	}
	if e != nil {
		if e.dirty {
			e.state = StateQueued
			e.path = e.nextPath
			if e.path == "" {
				e.path = job.Path
			}
			e.reasons, e.explicit, e.waiters = e.nextReasons, e.nextExplicit, e.nextWaiters
			e.queued = now
			e.run = e.nextRun
			e.dirty, e.nextPath, e.nextReasons, e.nextExplicit, e.nextWaiters, e.nextRun = false, "", nil, false, nil, 0
		} else {
			delete(q.entries, job.Service)
		}
	}
	q.mu.Unlock()
	finish(ws, err)
}

func (q *Queue) work(ctx context.Context) {
	defer close(q.stopped)
	for {
		if ctx.Err() != nil {
			q.drain(ctx.Err())
			return
		}
		job, ws, ok, wait := q.pick()
		if ok {
			err := q.cfg.Run(ctx, job)
			q.complete(job, ws, err)
			continue
		}
		var timer <-chan time.Time
		if wait > 0 {
			t := time.NewTimer(wait)
			timer = t.C
			select {
			case <-ctx.Done():
				t.Stop()
			case <-q.kick:
				t.Stop()
			case <-timer:
			}
			continue
		}
		select {
		case <-ctx.Done():
		case <-q.kick:
		}
	}
}

// drain fails every queued job's waiters when the worker stops.
func (q *Queue) drain(cause error) {
	q.mu.Lock()
	var ws []*waiter
	for s, e := range q.entries {
		ws = append(ws, e.waiters...)
		ws = append(ws, e.nextWaiters...)
		delete(q.entries, s)
	}
	q.mu.Unlock()
	finish(ws, cause)
}
