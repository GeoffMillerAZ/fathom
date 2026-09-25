package main

// console_events.go — OC-13's feeders (docs/design/observatory-console.md
// §4). The bus itself is internal/console.EventBus; this file observes the
// sources that change under a reader and publishes names and IDs:
//
//   - ingest  — a service's indexed commit (service_meta) moved;
//   - compile — a service's newest compile run changed;
//   - verify  — the verify ledger grew (polled by size every 2 s);
//   - session — the session ledger grew;
//   - daemon  — this daemon started, restarted or adopted a new binary.
//
// Everything is polled rather than hooked so a change made by any process
// (a CLI verify, a hook's ledger row) is seen; the first poll of each source
// only records a baseline and publishes nothing.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
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
	ingest  map[string]string // service → indexed commit @ indexed-at
	compile map[string]string // service → newest compile run id
	primed  bool
}

func newConsoleEventSources(bus *console.EventBus, c *client.Client, verifyLedger, sessionLedger string) *consoleEventSources {
	return &consoleEventSources{
		bus: bus, c: c,
		verify:  console.LedgerTail{Path: verifyLedger},
		session: console.LedgerTail{Path: sessionLedger},
		ingest:  map[string]string{}, compile: map[string]string{},
	}
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
			_, _ = s.bus.Publish(console.Event{Kind: console.EventVerify, Service: row.Service, Name: filepath.Base(row.Plan), Ref: result})
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
			_, _ = s.bus.Publish(console.Event{Kind: console.EventSession, Service: row.Repo, Ref: row.SessionID})
		}
	}
}

// pollStore publishes ingest and compile events for services whose indexed
// commit or newest compile run changed since the previous poll.
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
	for _, svc := range services {
		if meta, found, merr := s.c.Store().GetServiceMeta(ctx, svc); merr == nil && found {
			key := meta.IndexedCommit + "@" + strconv.FormatInt(meta.IndexedAt, 10)
			if prev, seen := s.ingest[svc]; !first && (!seen || prev != key) {
				_, _ = s.bus.Publish(console.Event{Kind: console.EventIngest, Service: svc, Ref: meta.IndexedCommit})
			}
			s.ingest[svc] = key
		}
		if runs, rerr := s.c.CompileRunsFor(ctx, svc); rerr == nil && len(runs) > 0 {
			newest := runs[0] // newest first (Store.CompileRuns' contract)
			if prev, seen := s.compile[svc]; !first && (!seen || prev != newest.RunID) {
				_, _ = s.bus.Publish(console.Event{Kind: console.EventCompile, Service: svc, Name: newest.Layer, Ref: newest.RunID})
			}
			s.compile[svc] = newest.RunID
		}
	}
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
	_, _ = bus.Publish(console.Event{Kind: console.EventDaemon, Name: name, Ref: ref})
}
