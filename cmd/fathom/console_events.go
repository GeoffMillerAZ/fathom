package main

// console_events.go — OC-13's feeders (docs/design/observatory-console.md
// §4). The bus itself is internal/console.EventBus; this file observes the
// sources that change under a reader and publishes names and IDs:
//
//   - ingest  — a service's indexed content moved (its commit or the
//     store's content digest; a hash-gated no-op re-ingest is not one);
//   - compile — a service recorded a compile run: one event per new run,
//     so two layers compiled inside one poll each name their layer;
//   - verify  — the verify ledger grew (polled by size every 2 s);
//   - session — the session ledger grew; the row's repo label is mapped to
//     the service whose root it names ("sextant" → fathom);
//   - daemon  — this daemon started, restarted or adopted a new binary.
//
// An event the bus refuses is reported through warn, never dropped
// silently.
//
// Everything is polled rather than hooked so a change made by any process
// (a CLI verify, a hook's ledger row) is seen; the first poll of each source
// only records a baseline and publishes nothing.

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// Poll intervals: the ledgers every 2 s (design §4), the store every 10 s.
const (
	consoleLedgerPoll = 2 * time.Second
	consoleStorePoll  = 10 * time.Second
)

// consoleEventSources holds each source's last-seen state.
type consoleEventSources struct {
	bus     *console.EventBus
	c       *client.Client
	verify  console.LedgerTail
	session console.LedgerTail
	ingest  map[string]string          // service → content key
	compile map[string]map[string]bool // service → compile run ids seen
	keys    contentKeyCache
	// roots maps each service to its recorded root, refreshed with the
	// store poll: a session row's repo label resolves through it.
	roots  map[string]string
	primed bool
	// ledgersPrimed is set once the ledgers' baseline is taken: a restart
	// of the feeders (RT-119) keeps their read positions, so what changed
	// while nobody listened is published when someone does.
	ledgersPrimed bool
	// compileCursor is the record order (store.CompileRunRef.Seq) of the
	// last compile run seen (RT-119): the poll reads only runs recorded
	// after it.
	compileCursor int64
	// compileCursorRun is that run's id: a purge's VACUUM renumbers the
	// record order, and a cursor whose run is no longer at its position
	// is re-based instead of skipping the runs recorded after the purge.
	compileCursorRun string
	// warn reports an event the bus refused (serve-http's log).
	warn func(string)

	// pollMu serializes the polls: the feeders' loop and a page's
	// catch-up (catchUp) never poll at once. ledgerAt and storeAt are when
	// each was last polled (by now, the clock tests pin); storePolls
	// counts the store polls run (tests); storeKicked is a catch-up's
	// store poll in flight.
	pollMu            sync.Mutex
	now               func() time.Time
	ledgerAt, storeAt time.Time
	storePolls        int
	storeKicked       atomic.Bool
}

func newConsoleEventSources(bus *console.EventBus, c *client.Client, verifyLedger, sessionLedger string) *consoleEventSources {
	return &consoleEventSources{
		bus: bus, c: c,
		verify:  console.LedgerTail{Path: verifyLedger},
		session: console.LedgerTail{Path: sessionLedger},
		ingest:  map[string]string{}, compile: map[string]map[string]bool{},
		roots: map[string]string{},
	}
}

// publish sends e, reporting a refusal instead of discarding it.
func (s *consoleEventSources) publish(e console.Event) {
	if _, err := s.bus.Publish(e); err != nil && s.warn != nil {
		s.warn("console events: " + err.Error())
	}
}

// sessionEventService maps a session ledger row's repo label — the Stop
// hook's lowered directory label, "sextant" for the fathom repository — to
// the service a panel keys on (sessions:fathom): the service named exactly
// by the label, else the one service whose recorded root has that base
// name, else the label itself when it is a name. A label that is not a
// name ("2026 job hunt") maps to "": the event still fires, for the
// unscoped "sessions" source only.
func sessionEventService(label string, roots map[string]string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	if l == "" {
		return ""
	}
	if svc := sessionLabelService(l, roots); svc != "" {
		return svc
	}
	if console.IsNameOrID(l) {
		return l
	}
	return ""
}

// sessionLabelKey maps a ledger row's repo label to the key the per-service
// session and verify counts use: the service it names (sessionLabelService),
// else the lowered label itself.
func sessionLabelKey(roots map[string]string) func(string) string {
	return func(label string) string {
		l := strings.ToLower(strings.TrimSpace(label))
		if svc := sessionLabelService(l, roots); svc != "" {
			return svc
		}
		return l
	}
}

// sessionLabelService is the service a lowered repo label names: the service
// ID itself, else the one service whose recorded root has that base name;
// "" when none (or two) do.
func sessionLabelService(l string, roots map[string]string) string {
	var byBase []string
	for name, root := range roots {
		if strings.ToLower(name) == l {
			return name
		}
		if root != "" && strings.ToLower(filepath.Base(root)) == l {
			byBase = append(byBase, name)
		}
	}
	if len(byBase) == 1 { // two services under one base name: ambiguous
		return byBase[0]
	}
	return ""
}

// pollLedgers publishes a verify event per appended verify-ledger row and a
// session event per appended session-ledger row.
func (s *consoleEventSources) pollLedgers() {
	if lines, err := s.verify.Poll(); err == nil {
		for _, line := range lines {
			var row verifyLedgerRow
			if json.Unmarshal(line, &row) != nil {
				continue
			}
			result := "pass"
			if row.Failing > 0 {
				result = "fail"
			}
			s.publish(console.Event{Kind: console.EventVerify, Service: row.Service, Name: filepath.Base(row.Plan), Ref: result})
		}
	}
	if lines, err := s.session.Poll(); err == nil {
		for _, line := range lines {
			var row struct {
				SessionID string `json:"session_id"`
				Repo      string `json:"repo"`
			}
			if json.Unmarshal(line, &row) != nil {
				continue
			}
			s.publish(console.Event{Kind: console.EventSession, Service: sessionEventService(row.Repo, s.roots), Ref: row.SessionID})
		}
	}
}

// pollStore publishes an ingest event for each service whose content key
// moved and a compile event for each compile run not seen before (oldest
// first), and refreshes the service roots the session events resolve by.
func (s *consoleEventSources) pollStore(ctx context.Context) {
	if s.c == nil {
		return
	}
	services, err := s.c.Services(ctx)
	if err != nil {
		return
	}
	sort.Strings(services)
	first := !s.primed
	s.primed = true
	// RT-119: the compile runs newer than the cursor, one read for every
	// service; a store without the capability is read per service.
	refs, rerr := compileRunRefsAfter(ctx, s.c, max(s.compileCursor-1, 0))
	compileFirst := first
	if rerr == nil && s.compileCursor > 0 {
		if len(refs) == 0 || refs[0].Seq != s.compileCursor || refs[0].RunID != s.compileCursorRun {
			// Renumbered (a purge's VACUUM): re-base on the whole record
			// without publishing it again.
			s.compileCursor, s.compileCursorRun, compileFirst = 0, "", true
			refs, rerr = compileRunRefsAfter(ctx, s.c, 0)
		}
	}
	haveRefs := rerr == nil
	roots := make(map[string]string, len(services))
	for _, svc := range services {
		if meta, key, found, kerr := s.keys.key(ctx, s.c.Store(), svc); kerr == nil && found {
			roots[svc] = meta.Path
			if prev, seen := s.ingest[svc]; !first && (!seen || prev != key) {
				s.publish(console.Event{Kind: console.EventIngest, Service: svc, Ref: meta.IndexedCommit})
			}
			s.ingest[svc] = key
		}
		if !haveRefs {
			s.pollCompileRunsOf(ctx, svc, first)
		}
	}
	if haveRefs {
		s.publishCompileRefs(refs, compileFirst)
	}
	s.roots = roots
}

// errNoCompileCursor is a store that cannot read compile runs by record
// order.
var errNoCompileCursor = errors.New("the store cannot read compile runs after a cursor")

// compileRunRefsAfter is the store's compile runs recorded after the
// cursor (store.CompileRunRefsAfter), when the store can answer it.
func compileRunRefsAfter(ctx context.Context, c *client.Client, after int64) ([]store.CompileRunRef, error) {
	r, ok := c.Store().(interface {
		CompileRunRefsAfter(context.Context, int64) ([]store.CompileRunRef, error)
	})
	if !ok {
		return nil, errNoCompileCursor
	}
	return r.CompileRunRefsAfter(ctx, after)
}

// publishCompileRefs publishes one compile event per run recorded since
// the cursor, in record order, and moves the cursor to the last one. The
// cursor is the record order, never ran_at: a run's ran_at is when it
// started, so a long run recorded after a shorter, later-started one would
// fall behind a time cursor and never be published.
func (s *consoleEventSources) publishCompileRefs(refs []store.CompileRunRef, first bool) {
	for _, r := range refs {
		if r.Seq <= s.compileCursor {
			continue
		}
		if !first {
			s.publish(console.Event{Kind: console.EventCompile, Service: r.ServiceID, Name: r.Layer, Ref: r.RunID})
		}
		s.compileCursor, s.compileCursorRun = r.Seq, r.RunID
	}
}

// pollCompileRunsOf is the per-service compile poll for a store without
// the cursor capability.
func (s *consoleEventSources) pollCompileRunsOf(ctx context.Context, svc string, first bool) {
	runs, rerr := s.c.CompileRunsFor(ctx, svc)
	if rerr != nil {
		return
	}
	seen := s.compile[svc]
	now := make(map[string]bool, len(runs))
	// runs is newest first (Store.CompileRuns' contract): publish the
	// unseen ones oldest first.
	for i := len(runs) - 1; i >= 0; i-- {
		run := runs[i]
		now[run.RunID] = true
		if !first && !seen[run.RunID] {
			s.publish(console.Event{Kind: console.EventCompile, Service: svc, Name: run.Layer, Ref: run.RunID})
		}
	}
	s.compile[svc] = now
}

// clock is the sources' now.
func (s *consoleEventSources) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// pollLedgersDue polls the ledgers — the first time, takes their baseline
// instead — unless they were polled within consoleLedgerPoll (force: the
// feeders' ticker or a restart polls regardless; a ledger poll reads only
// appended bytes).
func (s *consoleEventSources) pollLedgersDue(force bool) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	now := s.clock()
	switch {
	case !s.ledgersPrimed:
		s.verify.Prime()
		s.session.Prime()
		s.ledgersPrimed = true
	case force || now.Sub(s.ledgerAt) >= consoleLedgerPoll:
		s.pollLedgers()
	default:
		return
	}
	s.ledgerAt = now
}

// pollStoreDue polls the store unless it was polled within
// consoleStorePoll (force: the feeders' ticker): a navigation that closes
// and reopens /events restarts the feeders, and must not run a
// full-fleet poll each time.
func (s *consoleEventSources) pollStoreDue(ctx context.Context, force bool) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	now := s.clock()
	if !force && !s.storeAt.IsZero() && now.Sub(s.storeAt) < consoleStorePoll {
		return
	}
	s.pollStore(ctx)
	s.storePolls++
	s.storeAt = now
}

// catchUp is a page's read of the sources while the feeders are off
// (RT-119: they run only while /events has a subscriber): the ledgers are
// polled now, so a generation a memo keys on reflects what they gained;
// the store poll, a full-fleet read, is started off the request path at
// most once per consoleStorePoll, and the next view sees what it found.
// Paused like the feeders at shed and in safe mode.
func (s *consoleEventSources) catchUp() {
	if !backgroundAllowed() {
		return
	}
	s.pollLedgersDue(false)
	s.pollMu.Lock()
	due := s.storeAt.IsZero() || s.clock().Sub(s.storeAt) >= consoleStorePoll
	s.pollMu.Unlock()
	if due && s.storeKicked.CompareAndSwap(false, true) {
		go func() {
			defer s.storeKicked.Store(false)
			s.pollStoreDue(context.Background(), false)
		}()
	}
}

// runConsoleEventSources polls until ctx ends. The ledgers' baseline is
// taken on the first run only: a restart (RT-119) resumes from where the
// last run stopped, and polls the store only when consoleStorePoll has
// passed since the last store poll.
func runConsoleEventSources(ctx context.Context, s *consoleEventSources) {
	s.pollLedgersDue(true)
	s.pollStoreDue(ctx, false)
	ledgers := time.NewTicker(consoleLedgerPoll)
	store := time.NewTicker(consoleStorePoll)
	defer ledgers.Stop()
	defer store.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ledgers.C:
			if backgroundAllowed() { // RT-34: paused at shed and above; RT-39: off in safe mode
				s.pollLedgersDue(true)
			}
		case <-store.C:
			if backgroundAllowed() {
				s.pollStoreDue(ctx, true)
			}
		}
	}
}

// feederSwitch runs the event feeders only while GET /events has a
// subscriber (RT-119, design §14): the first subscriber starts them, the
// last one leaving stops them. An idle console polls nothing.
type feederSwitch struct {
	src *consoleEventSources
	run func(context.Context, *consoleEventSources)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newFeederSwitch(src *consoleEventSources) *feederSwitch {
	return &feederSwitch{src: src, run: runConsoleEventSources}
}

// subscribers is the bus's OnSubscribers callback: it only starts or
// cancels, never waits (it runs under the bus's lock).
func (f *feederSwitch) subscribers(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case n > 0 && f.cancel == nil:
		ctx, cancel := context.WithCancel(context.Background())
		prev := f.done
		done := make(chan struct{})
		f.cancel, f.done = cancel, done
		go func() {
			defer close(done)
			if prev != nil {
				<-prev // one feeder at a time: the last run has returned
			}
			f.run(ctx, f.src)
		}()
	case n == 0 && f.cancel != nil:
		f.cancel()
		f.cancel = nil
	}
}

// catchUp brings the sources up to date for a page when the feeders are
// off (consoleEventSources.catchUp); while they run they keep it current.
func (f *feederSwitch) catchUp() {
	if f == nil || f.running() {
		return
	}
	f.src.catchUp()
}

// running reports whether the feeders are on.
func (f *feederSwitch) running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancel != nil
}

// stoppedCleanly reports whether the last run has returned.
func (f *feederSwitch) stoppedCleanly() bool {
	f.mu.Lock()
	done := f.done
	on := f.cancel != nil
	f.mu.Unlock()
	if on {
		return false
	}
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// publishDaemonEvent announces this daemon: "start" when no previous record
// existed for the address, "adopt" when the previous daemon ran a different
// binary, "restart" otherwise. ref is the binary commit ("" when unstamped).
func publishDaemonEvent(bus *console.EventBus, prev console.DaemonRecord, hadPrev bool, commit string) {
	name := "start"
	switch {
	case hadPrev && prev.BinaryCommit != commit:
		name = "adopt"
	case hadPrev:
		name = "restart"
	}
	ref := commit
	if len(ref) > 12 {
		ref = ref[:12]
	}
	_, _ = bus.Publish(console.Event{Kind: console.EventDaemon, Name: name, Ref: ref}) // names and a commit: always valid
}

// contentKeyCache holds each service's content key: its indexed commit and
// the store's content digest (Store.ServiceContentDigest), recomputed only
// when service_meta.indexed_at moves. The ingest event and the search
// symbol cache both key on it, so a hash-gated no-op re-ingest — which
// stamps a new indexed_at over unchanged content — neither publishes an
// "ingest" event nor reloads a service's symbols.
type contentKeyCache struct {
	mu sync.Mutex
	m  map[string]contentKeyEntry
}

type contentKeyEntry struct {
	indexedAt int64
	key       string
}

// key returns svc's service_meta and content key; found is false when the
// service has no service_meta row.
func (k *contentKeyCache) key(ctx context.Context, st store.Store, svc string) (meta store.ServiceMeta, key string, found bool, err error) {
	meta, found, err = st.GetServiceMeta(ctx, svc)
	if err != nil || !found {
		return meta, "", found, err
	}
	k.mu.Lock()
	e, ok := k.m[svc]
	k.mu.Unlock()
	if ok && e.indexedAt == meta.IndexedAt {
		return meta, e.key, true, nil
	}
	digest, err := st.ServiceContentDigest(ctx, svc)
	if err != nil {
		return meta, "", true, err
	}
	key = meta.IndexedCommit + "@" + digest
	k.mu.Lock()
	if k.m == nil {
		k.m = map[string]contentKeyEntry{}
	}
	k.m[svc] = contentKeyEntry{indexedAt: meta.IndexedAt, key: key}
	k.mu.Unlock()
	return meta, key, true, nil
}
