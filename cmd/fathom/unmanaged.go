package main

// unmanaged.go — RT-20..RT-22 (docs/design/responsible-tenant.md §10.1,
// I52): a serve-http nobody supervises is a scratch daemon, and a scratch
// daemon is bounded. Two ran 22-23 h after their parents died (INC-23), and
// scratch daemons opened the operator's store and call log. An unmanaged
// serve-http now exits when its parent dies (SIGKILL included), after
// 20 min idle or 8 h, or when its executable or store is unlinked; it runs
// no auto-refresh, self-restart watcher or scavenger tick, starts the
// console feeders only when /events is first asked for, and refuses the
// managed daemon's store and call log without --allow-operator-state.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// detectManaged reports whether this serve-http is the managed daemon:
// started with --managed (plist schema 2), or by launchd under the default
// label or the Label key of an installed fathom agent (XPC_SERVICE_NAME — a
// schema-1 plist has no --managed, and the installed daemon must stay
// managed across the deploy that brings this code). A systemd unit passes
// --managed. The label is returned for the record id and the lifecycle
// ledger. It is the one managed rule serve-http uses after flag parsing:
// selfRestartManaged decides it, and isManagedInvocation (the kill switch,
// before parsing) applies the same label rule (isFathomLaunchdLabel); no
// substring of the label is assumed (ADR-011).
func detectManaged(flagManaged bool, getenv func(string) string, installed func() []string) (bool, string) {
	if installed == nil {
		installed = func() []string { return nil }
	}
	return selfRestartManaged(flagManaged, getenv("XPC_SERVICE_NAME"), installed)
}

// watchParentExit calls onExit once, with the reason, when the process
// that started this one (ppid) exits. getppid is polled every poll
// (reparenting changes it; this survives SIGKILL of the parent, traps and
// Setpgid); where the platform has it, a kqueue NOTE_EXIT wakes it at
// once. ppid <= 1 means the parent is already init: no watch.
func watchParentExit(ppid int, poll time.Duration, getppid func() int, onExit func(reason string)) (watching bool) {
	if ppid <= 1 {
		return false
	}
	var once sync.Once
	fire := func() { once.Do(func() { onExit(fmt.Sprintf("parent %d exited", ppid)) }) }
	go func() {
		t := time.NewTicker(poll)
		defer t.Stop()
		for range t.C {
			if getppid() != ppid {
				fire()
				return
			}
		}
	}()
	go func() {
		if waitProcessExit(ppid) == nil {
			fire()
		}
	}()
	return true
}

// fileIdentity is (device, inode) of a path at start; unlinked() reports
// whether the path no longer names that file (removed or replaced), which
// is when this process holds a deleted file open. Path-based on purpose:
// an extra fd on the SQLite store, closed later, would drop the store's
// POSIX locks.
type fileIdentity struct {
	path     string
	dev, ino uint64
	ok       bool
}

func identify(path string) fileIdentity {
	id := fileIdentity{path: path}
	if path == "" {
		return id
	}
	fi, err := os.Stat(path)
	if err != nil {
		return id
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		id.dev, id.ino, id.ok = uint64(st.Dev), uint64(st.Ino), true
	}
	return id
}

func (f fileIdentity) unlinked() bool {
	if !f.ok {
		return false
	}
	now := identify(f.path)
	return !now.ok || now.dev != f.dev || now.ino != f.ino
}

// activity counts requests in flight (an /events stream is one for as long
// as it is open) and the time of the last request's start or end.
type activity struct {
	inFlight atomic.Int64
	last     atomic.Int64 // unix nanos
	now      func() time.Time
}

func newActivity(now func() time.Time) *activity {
	a := &activity{now: now}
	a.last.Store(now().UnixNano())
	return a
}

func (a *activity) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.inFlight.Add(1)
		a.last.Store(a.now().UnixNano())
		defer func() {
			a.last.Store(a.now().UnixNano())
			a.inFlight.Add(-1)
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *activity) snapshot() (inFlight int, last time.Time) {
	return int(a.inFlight.Load()), time.Unix(0, a.last.Load()).UTC()
}

// daemonLifetime is the 60 s tick every serve-http runs: it refreshes the
// record's heartbeat and, for an unmanaged daemon, evaluates the exit
// bounds.
type daemonLifetime struct {
	managed    bool
	bounds     tenancy.UnmanagedBounds
	startedAt  time.Time
	act        *activity
	exe, store fileIdentity
	recordPath string
	now        func() time.Time
	logf       func(string)
	exit       func(code int)
	// readRecord reads the record for the heartbeat (nil:
	// console.ReadDaemonRecord); a seam for the shutdown race test.
	readRecord func(string) (console.DaemonRecord, error)
	// onExit records the exit (its lifecycle row) before the record goes;
	// nil records nothing.
	onExit func(reason string, code int)

	// mu orders the heartbeat's write against shutdown's removal: once
	// closed, no heartbeat writes the record back.
	mu     sync.Mutex
	closed bool

	// stopped (nil: never) ends run's ticker when serve-http returns
	// without exiting (a failed start in an in-process caller): a
	// leftover tick must never apply an exit bound to the caller's
	// process. Closed by stop, once.
	stopped  chan struct{}
	stopOnce sync.Once
}

// stop ends run's ticker; idempotent.
func (l *daemonLifetime) stop() {
	l.stopOnce.Do(func() {
		if l.stopped != nil {
			close(l.stopped)
		}
	})
}

// tick runs one evaluation; it returns the exit reason when it exited.
func (l *daemonLifetime) tick() string {
	inFlight, last := l.act.snapshot()
	now := l.now()
	// The lifetime starts before the store wait (RT-42): a store created
	// by its first open is identified at the first tick that sees it.
	if !l.store.ok && l.store.path != "" {
		l.store = identify(l.store.path)
	}
	if l.recordPath != "" {
		read := l.readRecord
		if read == nil {
			read = console.ReadDaemonRecord
		}
		if rec, err := read(l.recordPath); err == nil {
			rec.HeartbeatAt = now.UTC().Format(time.RFC3339)
			rec.LastActivityAt = last.Format(time.RFC3339)
			if inFlight > 0 {
				// A request in flight (an open /events stream) is
				// activity now: the census must not read this daemon idle.
				rec.LastActivityAt = rec.HeartbeatAt
			}
			l.mu.Lock()
			if !l.closed {
				_ = console.WriteDaemonRecord(l.recordPath, rec)
			}
			l.mu.Unlock()
		}
	}
	if l.managed {
		return ""
	}
	reason, exit := tenancy.EvaluateUnmanagedExit(l.bounds, tenancy.UnmanagedState{
		Now: now, StartedAt: l.startedAt, LastActivity: last, InFlight: inFlight,
		ExeUnlinked: l.exe.unlinked(), StoreUnlinked: l.store.unlinked(),
	})
	if !exit {
		return ""
	}
	l.shutdown(reason)
	return reason
}

// shutdown logs the reason, removes the record and exits 0.
func (l *daemonLifetime) shutdown(reason string) { l.shutdownWith(reason, 0) }

// shutdownWith is shutdown with an exit code (128+signo for a signal).
func (l *daemonLifetime) shutdownWith(reason string, code int) {
	l.logf(fmt.Sprintf("serve-http: unmanaged daemon exiting: %s (§10.1; --idle-exit/--ttl/--exit-with-parent, or --managed)", reason))
	if l.onExit != nil {
		l.onExit(reason, code)
	}
	l.closeRecord()
	l.exit(code)
}

// closeRecord removes the record; no heartbeat writes it back after.
func (l *daemonLifetime) closeRecord() {
	l.mu.Lock()
	l.closed = true
	removeDaemonRecord(l.recordPath)
	l.mu.Unlock()
}

// lifetimeExitReason maps a lifetime exit's reason to the ledger's (§2.3).
func lifetimeExitReason(reason string) string {
	switch {
	case strings.HasPrefix(reason, "idle"):
		return tenancy.ExitIdle
	case strings.HasPrefix(reason, "ttl"):
		return tenancy.ExitTTL
	case strings.Contains(reason, "unlinked"):
		return tenancy.ExitUnlinked
	case strings.HasPrefix(reason, "parent"):
		return tenancy.ExitParent
	case strings.HasPrefix(reason, "signal"):
		return tenancy.ExitSignal
	}
	return reason
}

// removePerProcessLedger removes an addr-<port>-<pid> daemon's ledger (both
// generations) and an empty crash.log on its clean exit: nothing reads a
// per-process ledger after its process, and its directory would otherwise
// stay for good (I48). A diagnostic capture keeps the directory.
func removePerProcessLedger(ledger string) {
	_ = os.Remove(ledger)
	_ = os.Remove(ledger + ".1")
	crash := filepath.Join(filepath.Dir(ledger), "crash.log")
	if fi, err := os.Stat(crash); err == nil && fi.Size() == 0 {
		_ = os.Remove(crash)
	}
}

// exitOnSignal shuts an unmanaged daemon down on the first signal from
// sigs, removing its record and exiting 128+signo as the default action
// would. kill, pkill, `daemon reap` and a script's trap all send SIGTERM;
// without this each left the record behind for good (its id is unique per
// process, so nothing overwrote it).
func exitOnSignal(sigs <-chan os.Signal, l *daemonLifetime) {
	sig, ok := <-sigs
	if !ok {
		return
	}
	code := 1
	if s, isSys := sig.(syscall.Signal); isSys {
		code = 128 + int(s)
	}
	l.shutdownWith("signal "+sig.String(), code)
}

func (l *daemonLifetime) run(every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-l.stopped:
			return
		case <-t.C:
			if l.tick() != "" {
				return
			}
		}
	}
}

// errOperatorState is refuseOperatorState's refusal.
var errOperatorState = errors.New("operator state")

// operatorPaths are the managed daemon's store and call log, from its
// record: a schema-2 record marked managed, or a schema-1 record at the
// managed address (before RT-166 only the managed daemon lived there).
func operatorPaths(recs []storedRecord, managedAddr string) (stores, callLogs []string) {
	for _, r := range recs {
		if !(r.Rec.Managed || (r.Rec.SchemaVersion == 1 && sameDaemonAddr(r.Rec.Addr, managedAddr))) {
			continue
		}
		if r.Rec.Store != "" {
			stores = append(stores, r.Rec.Store)
		}
		if r.Rec.CallLog != "" {
			callLogs = append(callLogs, r.Rec.CallLog)
		}
	}
	return stores, callLogs
}

// sameResolvedPath compares two paths after making them absolute and resolving
// symlinks where they exist.
func sameResolvedPath(a, b string) bool {
	norm := func(p string) string {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		// A file that does not exist yet: resolve its directory.
		if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
			return filepath.Join(r, filepath.Base(p))
		}
		return filepath.Clean(p)
	}
	return norm(a) == norm(b)
}

// refuseOperatorState refuses an unmanaged daemon's --store or --call-log
// that resolves to the managed daemon's (§10.1): a scratch daemon on the
// operator's store re-ingests real repositories and its calls land in the
// operator's log. It names the path, the record it came from and the flag.
func refuseOperatorState(storePath, callLog string, stores, callLogs []string) error {
	for _, s := range stores {
		if storePath != "" && sameResolvedPath(storePath, s) {
			return fmt.Errorf("%w: --store %s is the managed daemon's store; an unmanaged serve-http does not open it without --allow-operator-state (pass a scratch --store, or --managed if this IS the managed daemon)", errOperatorState, storePath)
		}
	}
	for _, c := range callLogs {
		if callLog != "" && sameResolvedPath(callLog, c) {
			return fmt.Errorf("%w: --call-log %s is the managed daemon's call log; an unmanaged serve-http does not write it without --allow-operator-state (pass --call-log \"\" or a scratch path)", errOperatorState, callLog)
		}
	}
	return nil
}
