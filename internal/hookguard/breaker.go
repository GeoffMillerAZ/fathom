package hookguard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

// Verdict is BreakerAllows' answer for one prospective RPC.
type Verdict string

const (
	// Allow: send the RPC.
	Allow Verdict = "allow"
	// HalfOpen: the breaker's backoff lapsed; one hook may probe.
	HalfOpen Verdict = "half_open"
	// Open: recent failures; fail open without an RPC.
	Open Verdict = "breaker_open"
	// Busy: the daemon answered 503 with Retry-After; not a failure.
	Busy Verdict = "busy"
	// Disabled: disabled.json names the managed daemon off (§9).
	Disabled Verdict = "disabled"
	// Paused: paused.json holds the managed daemon in safe mode (§9).
	Paused Verdict = "paused"
)

// Breaker tuning (§11.2).
const (
	failureWindow  = 10 * time.Second
	failuresToOpen = 2
	backoffBase    = 5 * time.Second
	backoffCap     = 300 * time.Second
	retryAfterCap  = 60 * time.Second
	stateLockWait  = 100 * time.Millisecond
	// ProbeTimeout bounds the half-open GET /healthz.
	ProbeTimeout  = 300 * time.Millisecond
	switchReadCap = 4 << 10
)

// DefaultManagedURL is the managed daemon's address when disabled.json /
// paused.json do not name one.
const DefaultManagedURL = "http://127.0.0.1:7575"

// Decision is BreakerAllows' result.
type Decision struct {
	Verdict Verdict
	Until   time.Time // when Open/Busy lapses
	Reason  string    // the switch's reason, or the breaker's last error
}

// Guard is one hook invocation's view of the machine-shared hook state.
// The zero value of every field has a working default.
type Guard struct {
	// Deadline is the invocation's monotonic deadline (HookDeadline).
	Deadline Deadline
	// HealthDir holds the per-URL breaker files; "" is hookcache.HealthDir.
	HealthDir string
	// DaemonDir holds disabled.json / paused.json; "" is ~/.fathom/daemon.
	DaemonDir string
	// ManagedURL is the managed daemon's URL when the switch file names
	// none; "" is DefaultManagedURL.
	ManagedURL string
	// Now is the wall clock the breaker file is written in; nil is time.Now.
	Now func() time.Time

	// Skips counts RPCs this guard refused, by verdict (for the beat).
	skips map[Verdict]int
}

func (g *Guard) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *Guard) healthDir() string {
	if g.HealthDir != "" {
		return g.HealthDir
	}
	return defaultHealthDir()
}

func (g *Guard) daemonDir() string {
	if g.DaemonDir != "" {
		return g.DaemonDir
	}
	return fathomhome.Path("daemon")
}

// NormalizeURL folds a daemon address to scheme://host:port: a bare
// host:port gains http://, an empty/unspecified/localhost host is
// 127.0.0.1, and any path is dropped — so every spelling of one daemon
// shares one breaker file and one kill-switch match.
func NormalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return strings.TrimRight(s, "/")
	}
	host, port := u.Hostname(), u.Port()
	switch host {
	case "", "localhost", "0.0.0.0", "::", "::1":
		host = "127.0.0.1"
	}
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return u.Scheme + "://" + host + ":" + port
}

func urlKey(base string) string {
	sum := sha256.Sum256([]byte(NormalizeURL(base)))
	return hex.EncodeToString(sum[:4])
}

func (g *Guard) statePath(base string) string {
	return filepath.Join(g.healthDir(), "daemon-health-"+urlKey(base)+".json")
}

func (g *Guard) leasePath(base string) string {
	return g.statePath(base) + ".lease"
}

// breakerState is the shared file, written by rename.
type breakerState struct {
	URL       string    `json:"url"`
	State     string    `json:"state"` // closed | open | busy
	Until     time.Time `json:"until,omitempty"`
	Failures  []int64   `json:"failures,omitempty"`  // unix ms within the window
	Opens     int       `json:"opens,omitempty"`     // consecutive opens since the last real RPC success
	OpenedAt  int64     `json:"opened_at,omitempty"` // unix ms of the latest open
	LastError string    `json:"last_error,omitempty"`
}

func (g *Guard) readState(base string) breakerState {
	raw, err := readCapped(g.statePath(base), switchReadCap)
	if err != nil {
		return breakerState{State: "closed"}
	}
	var s breakerState
	if json.Unmarshal(raw, &s) != nil || s.State == "" {
		return breakerState{State: "closed"} // fail open
	}
	return s
}

func (g *Guard) writeState(base string, s breakerState) {
	dir := g.healthDir()
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	s.URL = NormalizeURL(base)
	raw, err := json.Marshal(s)
	if err != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".daemon-health-*")
	if err != nil {
		return
	}
	_, werr := f.Write(raw)
	cerr := f.Close()
	if werr != nil || cerr != nil || os.Rename(f.Name(), g.statePath(base)) != nil {
		_ = os.Remove(f.Name())
	}
}

func readCapped(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// switchFile is disabled.json / paused.json. Every field is optional: an
// empty file reads "disabled, reason unknown" (§9).
type switchFile struct {
	Reason string `json:"reason"`
	Until  string `json:"until"`
	Addr   string `json:"addr"`
	URL    string `json:"url"`
}

// killSwitch stats disabled.json then paused.json. Either governs only
// the managed daemon (§2.3): a hook addressing any other URL — a proof
// agent's scratch daemon — is never silenced by it.
func (g *Guard) killSwitch(base string) (Verdict, string, bool) {
	dir := g.daemonDir()
	if dir == "" {
		return "", "", false
	}
	for _, sw := range []struct {
		name    string
		verdict Verdict
	}{{"disabled.json", Disabled}, {"paused.json", Paused}} {
		p := filepath.Join(dir, sw.name)
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue // absent; any other stat failure holds, reason unknown (I38)
		}
		var f switchFile
		if raw, err := readCapped(p, switchReadCap); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
			_ = json.Unmarshal(raw, &f) // unparseable still means "switched, reason unknown"
		}
		if f.Until != "" {
			if until, err := time.Parse(time.RFC3339, f.Until); err == nil && !g.now().Before(until) {
				continue // a timed stop that has lapsed
			}
		}
		managed := firstNonEmpty(f.URL, f.Addr, g.ManagedURL, DefaultManagedURL)
		if NormalizeURL(base) != NormalizeURL(managed) {
			continue
		}
		reason := f.Reason
		if reason == "" {
			reason = "reason unknown"
		}
		return sw.verdict, reason, true
	}
	return "", "", false
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// BreakerAllows is the gate every hook RPC passes first, in cost order:
// a stat of the managed daemon's disabled.json / paused.json (§9), then
// the shared per-URL breaker file (§11.2). It never blocks and never
// acquires anything — a HalfOpen verdict only says the caller may try
// for the probe lease (Transport does).
func BreakerAllows(g *Guard, base string) Decision {
	if v, reason, ok := g.killSwitch(base); ok {
		return Decision{Verdict: v, Reason: reason}
	}
	s := g.readState(base)
	now := g.now()
	switch s.State {
	case "open":
		if now.Before(s.Until) {
			return Decision{Verdict: Open, Until: s.Until, Reason: s.LastError}
		}
		return Decision{Verdict: HalfOpen, Until: s.Until, Reason: s.LastError}
	case "busy":
		if now.Before(s.Until) {
			return Decision{Verdict: Busy, Until: s.Until, Reason: s.LastError}
		}
	}
	return Decision{Verdict: Allow}
}

func backoff(opens int) time.Duration {
	d := backoffBase
	for i := 1; i < opens; i++ {
		d *= 2
		if d >= backoffCap {
			return backoffCap
		}
	}
	return d
}

// withStateLock serializes one read-modify-write of the shared breaker
// file across hook processes with flock(2) on a sibling lock file: two
// hooks failing at the same moment must both count, or "2 failures in
// 10 s" never holds under the very concurrency the breaker exists for.
// The critical section is a small read and a rename; a lock not had
// within stateLockWait runs fn unlocked (fail open, I9) rather than stall
// a hook.
func (g *Guard) withStateLock(base string, fn func()) {
	if os.MkdirAll(g.healthDir(), 0o755) != nil {
		fn()
		return
	}
	f, err := os.OpenFile(g.statePath(base)+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		fn()
		return
	}
	defer f.Close()
	fd := int(f.Fd())
	locked := false
	for wait := time.Duration(0); wait < stateLockWait; wait += time.Millisecond {
		if syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			locked = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if locked {
		defer syscall.Flock(fd, syscall.LOCK_UN)
	}
	fn()
}

// RecordSuccess closes the breaker and resets the backoff ladder after a
// real RPC succeeded (not a probe: see RecordProbeSuccess).
func (g *Guard) RecordSuccess(base string) { g.RecordSuccessSince(base, time.Time{}) }

// RecordSuccessSince is RecordSuccess for an RPC sent at started: an RPC
// that was already in flight when other hooks opened the breaker, and
// answered late, does not close that fresher "open". It writes only when
// there is state to reset, so the healthy path costs one small read and
// no lock.
func (g *Guard) RecordSuccessSince(base string, started time.Time) {
	clean := func(s breakerState) bool { return s.State == "closed" && len(s.Failures) == 0 && s.Opens == 0 }
	if clean(g.readState(base)) {
		return
	}
	g.withStateLock(base, func() {
		s := g.readState(base)
		if clean(s) {
			return
		}
		if s.State == "open" && !started.IsZero() && s.OpenedAt > started.UnixMilli() {
			return // this success predates the open other hooks just wrote
		}
		g.writeState(base, breakerState{State: "closed"})
	})
}

// RecordProbeSuccess closes a half-open breaker after a good GET /healthz
// but keeps the backoff ladder: /healthz is cheap and can answer while
// real RPCs still hang (the incident shape), so only a real RPC success
// (RecordSuccessSince) resets it. Otherwise every cycle would reopen at
// 5 s and cost two more hooks their whole deadline.
func (g *Guard) RecordProbeSuccess(base string) {
	g.withStateLock(base, func() {
		s := g.readState(base)
		g.writeState(base, breakerState{State: "closed", Opens: s.Opens})
	})
}

// RecordFailure counts a timeout, 5xx or refused connection; the second
// within 10 s opens the breaker with the next backoff.
func (g *Guard) RecordFailure(base string, err error) {
	g.withStateLock(base, func() {
		s := g.readState(base)
		now := g.now()
		if s.State == "open" && now.Before(s.Until) {
			return // already open: another hook got there first
		}
		cutoff := now.Add(-failureWindow).UnixMilli()
		kept := s.Failures[:0:0]
		for _, f := range s.Failures {
			if f > cutoff {
				kept = append(kept, f)
			}
		}
		kept = append(kept, now.UnixMilli())
		s.LastError = errString(err)
		if len(kept) >= failuresToOpen {
			g.open(&s, now)
		} else {
			if s.State != "open" {
				s.State = "closed"
			}
			s.Failures = kept
		}
		g.writeState(base, s)
	})
}

// open advances the backoff ladder one rung from now.
func (g *Guard) open(s *breakerState, now time.Time) {
	s.Opens++
	s.State, s.Until, s.Failures, s.OpenedAt = "open", now.Add(backoff(s.Opens)), nil, now.UnixMilli()
}

// RecordProbeFailure re-opens a half-open breaker with the doubled backoff.
func (g *Guard) RecordProbeFailure(base string, err error) {
	g.withStateLock(base, func() {
		s := g.readState(base)
		g.open(&s, g.now())
		s.LastError = errString(err)
		g.writeState(base, s)
	})
}

// RecordBusy holds hooks off for Retry-After (capped at 60 s). Busy is not
// a failure: the backoff ladder does not advance.
func (g *Guard) RecordBusy(base string, retryAfter time.Duration) {
	if retryAfter > retryAfterCap {
		retryAfter = retryAfterCap
	}
	if retryAfter <= 0 {
		return
	}
	g.withStateLock(base, func() {
		s := g.readState(base)
		s.State, s.Until, s.Failures, s.LastError = "busy", g.now().Add(retryAfter), nil, "daemon busy (503 Retry-After)"
		g.writeState(base, s)
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// acquireLease takes the half-open probe lease: a non-blocking flock(2)
// on the lease file, held for the probe. The kernel drops it when its
// holder exits, so a hook that died mid-probe leaves nothing to break —
// the old O_EXCL-plus-TTL lease had to delete stale files, and two hooks
// breaking one stale lease could each delete the other's fresh one and
// both probe. The file itself stays (unlinking a flocked file races the
// next opener onto a different inode).
func (g *Guard) acquireLease(base string) (release func(), ok bool) {
	if os.MkdirAll(g.healthDir(), 0o755) != nil {
		return nil, false
	}
	f, err := os.OpenFile(g.leasePath(base), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		_ = f.Close()
		return nil, false
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, true
}

func defaultHealthDir() string { return hookcache.HealthDir }
