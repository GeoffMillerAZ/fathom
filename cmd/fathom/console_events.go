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
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	// warn reports an event the bus refused (serve-http's log).
	warn func(string)
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
	roots := make(map[string]string, len(services))
	for _, svc := range services {
		if meta, key, found, kerr := s.keys.key(ctx, s.c.Store(), svc); kerr == nil && found {
			roots[svc] = meta.Path
			if prev, seen := s.ingest[svc]; !first && (!seen || prev != key) {
				s.publish(console.Event{Kind: console.EventIngest, Service: svc, Ref: meta.IndexedCommit})
			}
			s.ingest[svc] = key
		}
		if runs, rerr := s.c.CompileRunsFor(ctx, svc); rerr == nil {
			seen := s.compile[svc]
			now := make(map[string]bool, len(runs))
			// runs is newest first (Store.CompileRuns' contract): publish
			// the unseen ones oldest first.
			for i := len(runs) - 1; i >= 0; i-- {
				run := runs[i]
				now[run.RunID] = true
				if !first && !seen[run.RunID] {
					s.publish(console.Event{Kind: console.EventCompile, Service: svc, Name: run.Layer, Ref: run.RunID})
				}
			}
			s.compile[svc] = now
		}
	}
	s.roots = roots
}

// runConsoleEventSources polls until ctx ends.
func runConsoleEventSources(ctx context.Context, s *consoleEventSources) {
	s.verify.Prime()
	s.session.Prime()
	s.pollStore(ctx)
	ledgers := time.NewTicker(consoleLedgerPoll)
	store := time.NewTicker(consoleStorePoll)
	defer ledgers.Stop()
	defer store.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ledgers.C:
			s.pollLedgers()
		case <-store.C:
			s.pollStore(ctx)
		}
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
