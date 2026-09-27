package tenancy

// gate.go — the admission gate (design §3.2; RT-53, RT-54). Every tool
// call asks for a ticket in its cost class before its handler runs. Slots
// are per-class semaphores; a waiter queues for at most its origin's wait
// (a hook never waits for tree or fleet) and leaves the moment its context
// ends. A request whose remaining deadline is below the class's observed
// p50 is refused at once. The ladder's refuse rung refuses tree, fleet,
// ingest and egress; safe mode refuses tree, fleet and egress. A ticket
// carries the class deadline, is registered in the in-flight registry (so
// the refuse rung can cancel it) and meters the bytes its producers read.
//
// mode "observe" (the default until RT-96) runs the whole decision, logs
// and counts every would-refuse, and admits — it never waits and its meter
// never stops a producer. mode "off" admits without deciding.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Mode is the admission mode ([tenancy].admission).
type Mode uint8

const (
	ModeObserve Mode = iota
	ModeEnforce
	ModeOff
)

func (m Mode) String() string {
	switch m {
	case ModeEnforce:
		return "enforce"
	case ModeOff:
		return "off"
	}
	return "observe"
}

// ParseMode reads [tenancy].admission; anything but enforce or off is
// observe (config resolution already rejects unknown values).
func ParseMode(s string) Mode {
	switch strings.TrimSpace(s) {
	case "enforce":
		return ModeEnforce
	case "off":
		return ModeOff
	}
	return ModeObserve
}

// Origin is who is asking (§3.2): each origin waits its own time.
type Origin uint8

const (
	OriginAgent Origin = iota
	OriginHook
	OriginConsole
	OriginConsoleInProcess
	OriginCLI
	OriginTest
	numOrigins
)

var originNames = [numOrigins]string{"agent", "hook", "console", "console-inprocess", "cli", "test"}

func (o Origin) String() string {
	if o < numOrigins {
		return originNames[o]
	}
	return "origin(" + strconv.Itoa(int(o)) + ")"
}

// ParseOrigin maps a declared X-Fathom-Origin (and the caller's test flag)
// to an Origin: "<kind>-hook" and "prime" are hooks, "console" and
// "console-inprocess" the console, any other declared origin a CLI, and
// none an agent. A test caller is a test whatever it declared.
func ParseOrigin(declared string, test bool) Origin {
	switch {
	case test:
		return OriginTest
	case declared == "":
		return OriginAgent
	case strings.HasSuffix(declared, "-hook") || declared == "prime":
		return OriginHook
	case declared == "console":
		return OriginConsole
	case declared == "console-inprocess":
		return OriginConsoleInProcess
	}
	return OriginCLI
}

// refuseWait marks a class an origin never waits for.
const refuseWait time.Duration = -1

// queueWait is §3.1's table: how long an origin may queue for a class.
func queueWait(c Class, o Origin) time.Duration {
	switch o {
	case OriginHook:
		if c == ClassPoint || c == ClassIndex {
			return 100 * time.Millisecond
		}
		return refuseWait
	case OriginConsole, OriginConsoleInProcess:
		switch c {
		case ClassFleet, ClassIngest, ClassEgress:
			return refuseWait
		}
		return 2 * time.Second
	case OriginCLI, OriginTest:
		if c == ClassEgress {
			return refuseWait
		}
		return 30 * time.Second
	}
	switch c { // agent
	case ClassFleet:
		return 20 * time.Second
	case ClassEgress:
		return refuseWait
	}
	return 5 * time.Second
}

// Refusal codes: the metric label and the reason's category.
const (
	RefusalLevel      = "level"
	RefusalSafeMode   = "safe_mode"
	RefusalDeadline   = "deadline"
	RefusalSlots      = "slots"
	RefusalCallerGone = "caller_gone"
)

// Refusal is a named refusal (§3.4): the reason is a sentence.
type Refusal struct {
	Code       string
	Reason     string
	Class      string
	Level      string
	RetryAfter time.Duration
	InFlight   []string
}

// Outcome is how a ticket ended (§13.3's call-log outcome).
type Outcome string

const (
	OutcomeOK        Outcome = "ok"
	OutcomePartial   Outcome = "partial"
	OutcomeRefused   Outcome = "refused"
	OutcomeAbandoned Outcome = "abandoned"
	OutcomeError     Outcome = "error"
)

// Request is one admission request.
type Request struct {
	Tool, Service string
	Class         Class
	Origin        Origin
	// Deadline is the client's (X-Fathom-Deadline); zero: none. The
	// admitting ctx's own deadline counts too; the earlier wins.
	Deadline time.Time
}

// GateConfig wires the gate; zero fields take defaults.
type GateConfig struct {
	Budget   Budget
	Mode     Mode
	Now      func() time.Time // nil: time.Now
	Level    func() Level     // nil: CurrentLevel
	SafeMode func() bool      // nil: never
	InFlight *InFlight        // nil: DefaultInFlight
	Log      func(string)     // would-refuse and refusal lines; nil: none
	// Timer is the queue-wait timer: a channel that fires after d, and its
	// stop. nil: time.NewTimer. Tests inject it with Now so queue waits
	// never depend on the wall clock (determinism contract).
	Timer func(d time.Duration) (<-chan time.Time, func() bool)
}

// Gate is the admission gate.
type Gate struct {
	cfg   GateConfig
	slots [numClasses]chan struct{}

	mu          sync.Mutex
	queued      [numClasses]int
	running     [numClasses][numOrigins]int
	durations   [numClasses]*durationRing
	toolDur     map[string]*durationRing
	rejected    map[counterKey]int64
	wouldRefuse map[counterKey]int64
	outcomes    map[outcomeKey]int64
}

type counterKey struct {
	class  Class
	code   string
	origin Origin
}

type outcomeKey struct {
	class   Class
	outcome Outcome
}

// NewGate builds a gate over the budget's slot counts.
func NewGate(cfg GateConfig) *Gate {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Level == nil {
		cfg.Level = CurrentLevel
	}
	if cfg.InFlight == nil {
		cfg.InFlight = DefaultInFlight
	}
	if cfg.Timer == nil {
		cfg.Timer = func(d time.Duration) (<-chan time.Time, func() bool) {
			t := time.NewTimer(d)
			return t.C, t.Stop
		}
	}
	g := &Gate{cfg: cfg, rejected: map[counterKey]int64{}, wouldRefuse: map[counterKey]int64{}, outcomes: map[outcomeKey]int64{}, toolDur: map[string]*durationRing{}}
	for _, c := range Classes {
		n := cfg.Budget.Slots[c]
		if n <= 0 {
			n = 1
		}
		g.slots[c] = make(chan struct{}, n)
		g.durations[c] = &durationRing{}
	}
	return g
}

// Mode is the gate's admission mode.
func (g *Gate) Mode() Mode { return g.cfg.Mode }

// Budget is the budget the gate was built over.
func (g *Gate) Budget() Budget { return g.cfg.Budget }

// Admit decides one request. It returns a ticket, or a named refusal.
func (g *Gate) Admit(ctx context.Context, r Request) (*Ticket, *Refusal) {
	now := g.cfg.Now()
	deadline := r.Deadline
	// The ctx's deadline is on the wall clock; carry it into the gate's
	// (injected) frame so every comparison below uses one clock.
	if dl, ok := ctx.Deadline(); ok {
		if d := now.Add(time.Until(dl)); deadline.IsZero() || d.Before(deadline) {
			deadline = d
		}
	}
	if g.cfg.Mode == ModeOff {
		return g.issue(ctx, r, deadline, 0, false), nil
	}
	observe := g.cfg.Mode == ModeObserve

	// A caller already gone is never admitted, in either mode: nobody is
	// there to serve, so running its handler (and holding a slot for it)
	// is work for nobody (RT-58, I49). Not a resource decision, so observe
	// refuses it too.
	if ctx.Err() != nil {
		return nil, g.refuse(r, g.callerGone(r, "before admission"))
	}

	if ref := g.decideLevel(r); ref != nil {
		if !observe {
			return nil, g.refuse(r, ref)
		}
		g.would(r, ref)
	}
	if ref := g.decideDeadline(r, now, deadline); ref != nil {
		if !observe {
			return nil, g.refuse(r, ref)
		}
		g.would(r, ref)
	}

	if observe {
		g.mu.Lock()
		full := g.runningLocked(r.Class) >= cap(g.slots[r.Class])
		g.mu.Unlock()
		if full && queueWait(r.Class, r.Origin) == refuseWait {
			g.would(r, g.slotsRefusal(r))
		}
		return g.issue(ctx, r, deadline, 0, false), nil
	}

	// Enforce: take a free slot, or queue for the origin's wait.
	select {
	case g.slots[r.Class] <- struct{}{}:
		return g.issue(ctx, r, deadline, 0, true), nil
	default:
	}
	wait := queueWait(r.Class, r.Origin)
	if wait != refuseWait && !deadline.IsZero() {
		if left := deadline.Sub(now); left < wait {
			wait = left
		}
	}
	if wait <= 0 {
		return nil, g.refuse(r, g.slotsRefusal(r))
	}
	g.mu.Lock()
	g.queued[r.Class]++
	g.mu.Unlock()
	start := g.cfg.Now()
	fire, stop := g.cfg.Timer(wait)
	defer stop()
	defer func() {
		g.mu.Lock()
		g.queued[r.Class]--
		g.mu.Unlock()
	}()
	select {
	case g.slots[r.Class] <- struct{}{}:
		return g.issue(ctx, r, deadline, g.cfg.Now().Sub(start), true), nil
	case <-fire:
		ref := g.slotsRefusal(r)
		ref.Reason += fmt.Sprintf(" after waiting %s", wait.Round(time.Millisecond))
		return nil, g.refuse(r, ref)
	case <-ctx.Done():
		return nil, g.refuse(r, g.callerGone(r, "while queued for a "+r.Class.String()+"-class slot"))
	}
}

func (g *Gate) callerGone(r Request, when string) *Refusal {
	return &Refusal{Code: RefusalCallerGone, Reason: "caller went away " + when, Class: r.Class.String(), Level: g.cfg.Level().String()}
}

func (g *Gate) decideLevel(r Request) *Refusal {
	lvl := g.cfg.Level()
	heavy := r.Class == ClassTree || r.Class == ClassFleet || r.Class == ClassIngest || r.Class == ClassEgress
	if lvl >= LevelExit || (lvl >= LevelRefuse && heavy) {
		return &Refusal{
			Code:       RefusalLevel,
			Reason:     fmt.Sprintf("fathom is over its memory budget (%s): %s-class work is refused — fathom resources", lvl, r.Class),
			Class:      r.Class.String(),
			Level:      lvl.String(),
			RetryAfter: 30 * time.Second,
		}
	}
	if g.cfg.SafeMode != nil && (r.Class == ClassTree || r.Class == ClassFleet || r.Class == ClassEgress) && g.cfg.SafeMode() {
		return &Refusal{
			Code:       RefusalSafeMode,
			Reason:     fmt.Sprintf("fathom is in safe mode: %s-class work is refused — fathom resources", r.Class),
			Class:      r.Class.String(),
			Level:      lvl.String(),
			RetryAfter: 60 * time.Second,
		}
	}
	return nil
}

func (g *Gate) decideDeadline(r Request, now, deadline time.Time) *Refusal {
	if deadline.IsZero() {
		return nil
	}
	// The tool's own recent median: a class median would let a whole-tree
	// search set the bar for a hook's obligations call.
	p50 := g.P50Tool(r.Tool)
	if p50 <= 0 {
		return nil
	}
	left := deadline.Sub(now)
	if left >= p50 {
		return nil
	}
	return &Refusal{
		Code:       RefusalDeadline,
		Reason:     fmt.Sprintf("cannot finish before your deadline: %s left, %s calls take %s (recent p50)", max(left, 0).Round(time.Millisecond), r.Tool, p50.Round(time.Millisecond)),
		Class:      r.Class.String(),
		Level:      g.cfg.Level().String(),
		RetryAfter: p50,
	}
}

// slotsRefusal names the full class and what holds it.
func (g *Gate) slotsRefusal(r Request) *Refusal {
	var names []string
	for _, e := range g.cfg.InFlight.Snapshot() {
		if e.Class == r.Class && e.Tool != "" {
			names = append(names, e.Tool)
		}
	}
	g.mu.Lock()
	running := g.runningLocked(r.Class)
	g.mu.Unlock()
	reason := fmt.Sprintf("%s-class slots full (%d/%d)", r.Class, running, cap(g.slots[r.Class]))
	if len(names) > 0 {
		reason += ": " + strings.Join(names, ", ")
	}
	retry := g.P50(r.Class)
	if def := defaultRetry[r.Class]; retry < def {
		retry = def
	}
	return &Refusal{Code: RefusalSlots, Reason: reason, Class: r.Class.String(), Level: g.cfg.Level().String(), RetryAfter: retry, InFlight: names}
}

var defaultRetry = [numClasses]time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 30 * time.Second, 30 * time.Second, 5 * time.Second}

func (g *Gate) refuse(r Request, ref *Refusal) *Refusal {
	g.mu.Lock()
	g.rejected[counterKey{r.Class, ref.Code, r.Origin}]++
	g.outcomes[outcomeKey{r.Class, OutcomeRefused}]++
	g.mu.Unlock()
	if g.cfg.Log != nil {
		g.cfg.Log(fmt.Sprintf("admission: refused %s (%s, %s): %s", r.Tool, r.Class, r.Origin, ref.Reason))
	}
	return ref
}

func (g *Gate) would(r Request, ref *Refusal) {
	g.mu.Lock()
	g.wouldRefuse[counterKey{r.Class, ref.Code, r.Origin}]++
	g.mu.Unlock()
	if g.cfg.Log != nil {
		g.cfg.Log(fmt.Sprintf("admission (observe): would refuse %s (%s, %s): %s", r.Tool, r.Class, r.Origin, ref.Reason))
	}
}

func (g *Gate) runningLocked(c Class) int {
	n := 0
	for _, v := range g.running[c] {
		n += v
	}
	return n
}

// issue builds a ticket: the class deadline, the in-flight registration and
// the meter.
func (g *Gate) issue(ctx context.Context, r Request, deadline time.Time, queued time.Duration, holdsSlot bool) *Ticket {
	now := g.cfg.Now()
	if d := g.cfg.Budget.Deadline[r.Class]; d > 0 {
		if cd := now.Add(d); deadline.IsZero() || cd.Before(deadline) {
			deadline = cd
		}
	}
	ictx, endInflight := g.cfg.InFlight.Begin(ctx, r.Class, r.Tool, r.Service, r.Origin.String())
	cancel := context.CancelFunc(func() {})
	if !deadline.IsZero() {
		// deadline is in the gate's frame; the context timer runs on the
		// wall clock, so it gets the time left, not the instant.
		ictx, cancel = context.WithTimeout(ictx, deadline.Sub(now))
	}
	t := &Ticket{
		g: g, req: r, queued: queued, holdsSlot: holdsSlot,
		start:   now,
		ceiling: g.cfg.Budget.PerCall[r.Class],
		enforce: g.cfg.Mode == ModeEnforce,
		release: func() { cancel(); endInflight() },
	}
	t.ctx = context.WithValue(ictx, ticketKey{}, t)
	g.mu.Lock()
	g.running[r.Class][r.Origin]++
	g.mu.Unlock()
	return t
}

// Ticket is one admitted call.
type Ticket struct {
	g         *Gate
	req       Request
	ctx       context.Context
	start     time.Time
	queued    time.Duration
	holdsSlot bool
	ceiling   int64
	enforce   bool
	release   func()
	bytes     atomic.Int64
	elided    atomic.Int64
	over      atomic.Bool
	once      sync.Once
}

// Context is cancelled on the deadline, the caller's cancellation, or the
// refuse rung's.
func (t *Ticket) Context() context.Context { return t.ctx }

// Class is the ticket's cost class.
func (t *Ticket) Class() Class { return t.req.Class }

// Origin is the ticket's origin.
func (t *Ticket) Origin() Origin { return t.req.Origin }

// Queued is how long the call waited for its slot.
func (t *Ticket) Queued() time.Duration { return t.queued }

// BytesRead is what the call's producers charged.
func (t *Ticket) BytesRead() int64 { return t.bytes.Load() }

// OverBudget reports whether the call's producers passed its per-call
// ceiling (in observe mode, would have been stopped).
func (t *Ticket) OverBudget() bool { return t.over.Load() }

// Stopped reports whether the meter stopped a producer: over the ceiling
// under an enforcing gate. Only a stopped call's answer is partial; an
// observe-mode call that passed the ceiling ran to completion.
func (t *Ticket) Stopped() bool { return t.enforce && t.over.Load() }

// Elide records n items a stopped producer did not reach; the budget
// block reports their sum as a lower bound.
func (t *Ticket) Elide(n int) {
	if n > 0 {
		t.elided.Add(int64(n))
	}
}

// Charge records n bytes read. At the class ceiling it returns an
// ErrOverBudget error in enforce mode (the producer stops and returns what
// it has, named partial); observe mode records it and returns nil.
func (t *Ticket) Charge(n int64) error {
	total := t.bytes.Add(n)
	if t.ceiling <= 0 || total <= t.ceiling {
		return nil
	}
	t.over.Store(true)
	if !t.enforce {
		return nil
	}
	return &CallBudgetError{Class: t.req.Class, Ceiling: t.ceiling, Read: total}
}

// PartialBudget is the `budget` block of a partial answer (§3.2).
type PartialBudget struct {
	Partial   bool   `json:"partial"`
	Elided    int    `json:"elided"`
	BytesRead int64  `json:"bytes_read"`
	Reason    string `json:"reason"`
}

// Partial names a partial answer: elided is what the producers reported
// they did not reach (Elide), a lower bound.
func (t *Ticket) Partial() PartialBudget {
	return PartialBudget{
		Partial:   true,
		Elided:    int(t.elided.Load()),
		BytesRead: t.BytesRead(),
		Reason:    fmt.Sprintf("stopped at the %s-class per-call ceiling of %s", t.req.Class, FormatBytes(t.ceiling)),
	}
}

// Release ends the ticket: the slot, the registration and the deadline go,
// and the call's duration feeds the class and tool medians — an ok or
// partial call's as measured, an abandoned call's as the lower bound it is
// (the slow calls are exactly the ones that get abandoned). Idempotent.
func (t *Ticket) Release(o Outcome) {
	t.once.Do(func() {
		t.release()
		g := t.g
		g.mu.Lock()
		g.running[t.req.Class][t.req.Origin]--
		g.outcomes[outcomeKey{t.req.Class, o}]++
		switch o {
		case OutcomeOK, OutcomePartial, OutcomeAbandoned:
			now := g.cfg.Now()
			took := now.Sub(t.start)
			g.durations[t.req.Class].add(took, now)
			ring := g.toolDur[t.req.Tool]
			if ring == nil {
				ring = &durationRing{}
				g.toolDur[t.req.Tool] = ring
			}
			ring.add(took, now)
		}
		g.mu.Unlock()
		if t.holdsSlot {
			<-g.slots[t.req.Class]
		}
	})
}

// CallBudgetError is the per-call meter's refusal; errors.Is ErrOverBudget.
type CallBudgetError struct {
	Class   Class
	Ceiling int64
	Read    int64
}

func (e *CallBudgetError) Error() string {
	return fmt.Sprintf("over budget: the %s-class per-call ceiling is %s (read %s)", e.Class, FormatBytes(e.Ceiling), FormatBytes(e.Read))
}

func (e *CallBudgetError) Is(target error) bool { return target == ErrOverBudget }

type ticketKey struct{}

// TicketFrom is ctx's admission ticket, or nil.
func TicketFrom(ctx context.Context) *Ticket {
	t, _ := ctx.Value(ticketKey{}).(*Ticket)
	return t
}

// Detach is ctx for work that outlives its request (an auto-refresh or
// explicit ingest, §3.5): never cancelled with the request, stripped of the
// request's admission ticket (whose per-call ceiling and release belong to
// the read) and marked as the ingest producer class. Other values (scope,
// origin, caller) are kept.
func Detach(ctx context.Context) context.Context {
	ctx = context.WithValue(context.WithoutCancel(ctx), ticketKey{}, (*Ticket)(nil))
	return WithProducerClass(ctx, ClassIngest)
}

// Charge charges ctx's ticket n bytes; no ticket, no charge.
func Charge(ctx context.Context, n int64) error {
	if t := TicketFrom(ctx); t != nil {
		return t.Charge(n)
	}
	return nil
}

// P50 is the class's observed median duration over its calls of the last
// p50Window; 0 until eight have finished in it.
func (g *Gate) P50(c Class) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.durations[c].p50(g.cfg.Now())
}

// P50Tool is the tool's own median over the last p50Window (the deadline
// feasibility bar); 0 until eight of its calls have finished in it.
func (g *Gate) P50Tool(tool string) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r := g.toolDur[tool]; r != nil {
		return r.p50(g.cfg.Now())
	}
	return 0
}

// Queued is how many callers wait for a class slot now.
func (g *Gate) Queued(c Class) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.queued[c]
}

// Rejected is the refusal count for (class, code, origin).
func (g *Gate) Rejected(c Class, code string, o Origin) int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rejected[counterKey{c, code, o}]
}

// WouldRefuse is observe mode's would-refuse count for (class, code, origin).
func (g *Gate) WouldRefuse(c Class, code string, o Origin) int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.wouldRefuse[counterKey{c, code, o}]
}

// RefusalTotals sums the refusal and would-refuse counters over every
// class, code and origin.
func (g *Gate) RefusalTotals() (refused, would int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, n := range g.rejected {
		refused += n
	}
	for _, n := range g.wouldRefuse {
		would += n
	}
	return refused, would
}

// MetricLines is the gate's /metrics exposition, sorted.
func (g *Gate) MetricLines() string {
	g.mu.Lock()
	var lines []string
	for _, c := range Classes {
		for o := range numOrigins {
			if n := g.running[c][o]; n > 0 {
				lines = append(lines, fmt.Sprintf("fathom_inflight{class=%q,origin=%q} %d", c.String(), Origin(o).String(), n))
			}
		}
		lines = append(lines, fmt.Sprintf("fathom_queue{class=%q} %d", c.String(), g.queued[c]))
		lines = append(lines, fmt.Sprintf("fathom_slots{class=%q} %d", c.String(), cap(g.slots[c])))
	}
	for k, n := range g.rejected {
		lines = append(lines, fmt.Sprintf("fathom_rejected_total{class=%q,reason=%q,origin=%q} %d", k.class.String(), k.code, k.origin.String(), n))
	}
	for k, n := range g.wouldRefuse {
		lines = append(lines, fmt.Sprintf("fathom_would_refuse_total{class=%q,reason=%q,origin=%q} %d", k.class.String(), k.code, k.origin.String(), n))
	}
	for k, n := range g.outcomes {
		lines = append(lines, fmt.Sprintf("fathom_calls_total{class=%q,outcome=%q} %d", k.class.String(), string(k.outcome), n))
	}
	g.mu.Unlock()
	lines = append(lines, fmt.Sprintf("fathom_admission_mode{mode=%q} 1", g.cfg.Mode.String()))
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// p50Window is how long a duration sample counts. Refused callers never
// sample the median, so without a window a slow spell would hold its
// refusals for as long as the refused traffic continued.
const p50Window = 15 * time.Minute

// durationRing keeps the last 64 durations (with when they ended).
type durationRing struct {
	buf [64]durationSample
	n   int
}

type durationSample struct {
	d  time.Duration
	at time.Time
}

func (r *durationRing) add(d time.Duration, at time.Time) {
	r.buf[r.n%len(r.buf)] = durationSample{d, at}
	r.n++
}

// p50 is the median of the samples no older than p50Window at now; 0 with
// fewer than eight.
func (r *durationRing) p50(now time.Time) time.Duration {
	n := min(r.n, len(r.buf))
	s := make([]time.Duration, 0, n)
	for _, x := range r.buf[:n] {
		if now.Sub(x.at) <= p50Window {
			s = append(s, x.d)
		}
	}
	if len(s) < 8 {
		return 0
	}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}
