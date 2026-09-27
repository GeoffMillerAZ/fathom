package tenancy

// lifecycle_state.go — the serving process's lifecycle modes, read on the
// request path (design §2.1, §2.3; RT-39, RT-40, RT-41): safe mode (the
// breaker's, or paused.json's), the warm-up window after a start, and store
// distress. serve-http sets them; auto-refresh's ensure(), the RPC gate and
// the feeders read them. Process-wide, like the published ladder level.

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SafeModeInfo describes the breaker's safe mode.
type SafeModeInfo struct {
	Since  time.Time
	Reason string // "3 counted exits in 30m0s (over_budget ×2, unclean ×1)", or paused.json's
	Paused bool   // paused.json (`fathom daemon pause`), not the breaker
}

// Message is the refusal sentence (§2.3): "fathom safe mode since 21:07:
// <reason> — run fathom resources".
func (s SafeModeInfo) Message() string {
	return fmt.Sprintf("fathom safe mode since %s: %s — run fathom resources", s.Since.Local().Format("15:04"), s.Reason)
}

// StoreDistressHold is how long a store error (SQLITE_FULL, SQLITE_IOERR)
// holds distress after the last one.
const StoreDistressHold = 5 * time.Minute

var (
	lcMu          sync.Mutex
	lcSafe        *SafeModeInfo
	lcPaused      bool
	lcPausedSince time.Time
	lcWarmStart   time.Time
	lcWarmFor     time.Duration
	lcErr         string
	lcErrAt       time.Time
	lcVolLow      string
)

// SetSafeMode sets (or, with nil, clears) the breaker's safe mode.
func SetSafeMode(info *SafeModeInfo) {
	lcMu.Lock()
	defer lcMu.Unlock()
	if info == nil {
		lcSafe = nil
		return
	}
	cp := *info
	lcSafe = &cp
}

// SetPausedSafeMode mirrors paused.json (`fathom daemon pause`): the
// managed daemon stays up in safe mode while it holds.
func SetPausedSafeMode(p bool) {
	lcMu.Lock()
	defer lcMu.Unlock()
	if p && !lcPaused {
		lcPausedSince = time.Now()
	}
	lcPaused = p
}

// SafeModeNow reports safe mode, the breaker's first, else paused.json's.
func SafeModeNow() (SafeModeInfo, bool) {
	lcMu.Lock()
	defer lcMu.Unlock()
	if lcSafe != nil {
		return *lcSafe, true
	}
	if lcPaused {
		return SafeModeInfo{Since: lcPausedSince, Reason: "paused by the operator (paused.json; fathom daemon resume)", Paused: true}, true
	}
	return SafeModeInfo{}, false
}

// SetWarmup starts the warm-up window: d (DefaultWarmup when zero) from
// start.
func SetWarmup(start time.Time, d time.Duration) {
	if d <= 0 {
		d = DefaultWarmup
	}
	lcMu.Lock()
	defer lcMu.Unlock()
	lcWarmStart, lcWarmFor = start, d
}

// Warmup returns the warm-up window (zero start when none was set).
func Warmup() (start time.Time, d time.Duration) {
	lcMu.Lock()
	defer lcMu.Unlock()
	return lcWarmStart, lcWarmFor
}

// InWarmup reports whether now is inside the warm-up window.
func InWarmup(now time.Time) bool {
	start, d := Warmup()
	return !start.IsZero() && now.Sub(start) < d
}

// NoteStoreDistress records a store error that is distress (SQLITE_FULL,
// SQLITE_IOERR): ingest and writes are refused for StoreDistressHold.
func NoteStoreDistress(reason string, at time.Time) {
	lcMu.Lock()
	defer lcMu.Unlock()
	lcErr, lcErrAt = reason, at
}

// StoreDistressKind names a store error that is distress ("disk full
// (SQLITE_FULL)", "disk I/O error (SQLITE_IOERR)"), else "" (§2.1).
// modernc.org/sqlite's *Error carries the extended code (Code() int); the
// message is the fallback, since the ingest pipeline wraps its errors.
func StoreDistressKind(err error) string {
	if err == nil {
		return ""
	}
	var coded interface{ Code() int }
	if errors.As(err, &coded) {
		switch coded.Code() & 0xff {
		case sqliteFull:
			return "disk full (SQLITE_FULL)"
		case sqliteIOErr:
			return "disk I/O error (SQLITE_IOERR)"
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "database or disk is full") || strings.Contains(msg, "sqlite_full") || strings.Contains(msg, "no space left on device"):
		return "disk full (SQLITE_FULL)"
	case strings.Contains(msg, "disk i/o error") || strings.Contains(msg, "sqlite_ioerr"):
		return "disk I/O error (SQLITE_IOERR)"
	}
	return ""
}

// SQLite primary result codes that are store distress (§2.1).
const (
	sqliteIOErr = 10
	sqliteFull  = 13
)

// NoteStoreError records err as store distress when it is one — a read's,
// or a write's (an ingest, a /refresh-file, an auto-refresh ingest: on a
// full disk the writes fail first) — and returns its kind, else "".
func NoteStoreError(err error, at time.Time) string {
	kind := StoreDistressKind(err)
	if kind != "" {
		NoteStoreDistress(kind, at)
	}
	return kind
}

// NoteStoreVolumeFree records the store volume's free bytes against its
// floor; below it is distress. An Unknown reading is never a trigger.
func NoteStoreVolumeFree(free, min int64, _ time.Time) {
	lcMu.Lock()
	defer lcMu.Unlock()
	if free >= 0 && min > 0 && free < min {
		lcVolLow = fmt.Sprintf("store volume has %s free, under the %s floor", FormatBytes(free), FormatBytes(min))
		return
	}
	lcVolLow = ""
}

// StoreDistressNow reports store distress and its named reason.
func StoreDistressNow(now time.Time) (string, bool) {
	lcMu.Lock()
	defer lcMu.Unlock()
	if lcVolLow != "" {
		return "store distress: " + lcVolLow, true
	}
	if lcErr != "" && now.Sub(lcErrAt) < StoreDistressHold {
		return "store distress: " + lcErr + " at " + lcErrAt.UTC().Format(time.RFC3339), true
	}
	return "", false
}

// InlineIngestAllowed reports whether auto-refresh may ingest inline now:
// never in safe mode, during warm-up or in store distress (§2.3, §2.1).
func InlineIngestAllowed(now time.Time) (bool, string) {
	if s, on := SafeModeNow(); on {
		return false, s.Message()
	}
	if InWarmup(now) {
		start, d := Warmup()
		return false, fmt.Sprintf("warm-up: no inline ingest for %s after the start at %s", d, start.UTC().Format(time.RFC3339))
	}
	if why, on := StoreDistressNow(now); on {
		return false, why
	}
	return true, ""
}

// resetLifecycleState clears every mode (tests).
func resetLifecycleState() {
	lcMu.Lock()
	defer lcMu.Unlock()
	lcSafe, lcPaused, lcPausedSince = nil, false, time.Time{}
	lcWarmStart, lcWarmFor = time.Time{}, 0
	lcErr, lcErrAt, lcVolLow = "", time.Time{}, ""
}

// ResetLifecycleState clears every mode: serve-http defers it, so an
// in-process caller (a test) never inherits a daemon's modes.
func ResetLifecycleState() { resetLifecycleState() }

// ResetLifecycleStateForTest is ResetLifecycleState, named for tests.
func ResetLifecycleStateForTest() { resetLifecycleState() }
