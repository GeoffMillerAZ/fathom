package main

// Hook-owned bookkeeping beside session stamps, never the daemon store.
// A full emit is recorded only after stdout succeeds. Nonblocking flock
// holds the check through delivery; contention and every state error emit
// in full. A killed hook leaves no new claim that context was delivered.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

const (
	hookDedupeMaxBytes        = 256 << 10
	hookDedupeMaxEntries      = 2048
	hookDedupeAge             = 7 * 24 * time.Hour
	hookDedupeRefresh         = 30 * time.Minute
	hookDedupeMaxSuppressions = 10
)

type hookDedupeEntry struct {
	Digest       string    `json:"digest"`
	EmittedAt    time.Time `json:"emitted_at"`
	Suppressions int       `json:"suppressions"`
}

type hookDedupeState struct {
	Version int                        `json:"version"`
	Epoch   string                     `json:"epoch"`
	At      time.Time                  `json:"at"`
	Digests map[string]hookDedupeEntry `json:"digests"`
}

// A pending full injection owns the lock until commit or close. Merely
// checking a payload never records it as delivered.
type hookDedupeCheck struct {
	file                               *os.File
	state                              hookDedupeState
	key, digest, logKey                string
	ruleKeys                           []string
	ruleMode                           bool
	suppressed, firstTouch, stateError bool
}

func hookDedupePath(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	return fathomhome.Path("hook-dedupe", hookDigest(sessionID)+".json")
}

func hookDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Even config without a dedupe key must parse and validate. A bad config
// cannot authorize withholding guidance.
func hookDedupeEnabled(root string) bool {
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	return err == nil && loaded.Config.Hooks.Dedupe
}

func openHookDedupe(sessionID string) (*os.File, os.FileInfo, error) {
	path := hookDedupePath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, nil, err
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > hookDedupeMaxBytes) {
		err = os.ErrInvalid
	}
	if err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
			err = os.ErrPermission
		}
	}
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, info, nil
}

func dedupeHookInjection(id, root, sessionID, channel, target, payload string, now time.Time) *hookDedupeCheck {
	if id != harness.ClaudeCode || sessionID == "" || payload == "" || !hookDedupeEnabled(root) {
		return newHookDedupeCheck(channel, target, payload)
	}
	return checkHookInjection(sessionID, channel, target, payload, now)
}

func newHookDedupeCheck(channel, target, payload string) *hookDedupeCheck {
	d := &hookDedupeCheck{firstTouch: true}
	d.key, d.digest = hookDigest(channel+"\x00"+target), hookDigest(payload)
	if channel == "cwd-changed" {
		// One slot per session: only the last repository AND digest count.
		// A -> B -> A must re-emit A even if its text did not change.
		d.key, d.digest = hookDigest(channel), hookDigest(target+"\x00"+payload)
	}
	d.logKey = hookDigest(channel + "\x00" + target)
	return d
}

func checkHookInjection(sessionID, channel, target, payload string, now time.Time) *hookDedupeCheck {
	return checkHookInjectionMode(sessionID, channel, target, payload, now, false)
}

// once records a changing claim set for the session, without CB-1's timed
// refresh or suppression-count backstops. An empty set clears a past report.
func checkHookInjectionMode(sessionID, channel, target, payload string, now time.Time, once bool) *hookDedupeCheck {
	d := loadHookDedupe(sessionID, channel, target, payload, now, once)
	if d.file == nil {
		return d
	}
	previous, exists := d.state.Digests[d.key]
	d.firstTouch = !exists || (channel == "cwd-changed" && previous.Digest != d.digest)
	if !exists && len(d.state.Digests) >= hookDedupeMaxEntries {
		d.close()
		return d
	}
	if exists && previous.Digest == d.digest && once {
		ok := d.persist() // a state write error must report the set in full
		d.close()
		d.suppressed = ok && !d.stateError
	} else if exists && previous.Digest == d.digest && !now.Before(previous.EmittedAt) && now.Sub(previous.EmittedAt) < hookDedupeRefresh && previous.Suppressions < hookDedupeMaxSuppressions {
		previous.Suppressions++
		d.state.Digests[d.key] = previous
		d.state.At = now
		ok := d.persist()
		d.close()
		d.suppressed = ok && !d.stateError
	}
	return d
}

// loadHookDedupe opens the existing per-session record without deciding what
// to withhold. All hook channels share its lock, validation and size bounds.
func loadHookDedupe(sessionID, channel, target, payload string, now time.Time, allowEmpty bool) *hookDedupeCheck {
	d := newHookDedupeCheck(channel, target, payload)
	stamp := readSessionStamp(sessionID)
	if sessionID == "" || (payload == "" && !allowEmpty) || stamp.SessionID != sessionID || stamp.At == "" {
		return d
	}
	f, info, err := openHookDedupe(sessionID)
	if err != nil {
		d.stateError = true
		return d
	}
	d.file = f
	fresh := func() hookDedupeState {
		return hookDedupeState{Version: 2, Epoch: stamp.At, At: now, Digests: map[string]hookDedupeEntry{}}
	}
	d.state = fresh()
	if info.Size() > 0 {
		raw, err := io.ReadAll(io.LimitReader(f, hookDedupeMaxBytes+1))
		// v1 has no delivery timestamps or counters. Discard its claims and
		// start with a full emit when upgrading; no old digest can suppress.
		var header struct {
			Version int `json:"version"`
		}
		if err != nil || len(raw) > hookDedupeMaxBytes || json.Unmarshal(raw, &header) != nil || (header.Version != 1 && header.Version != 2) {
			d.stateError = true
			d.close()
			return d
		}
		if header.Version == 2 {
			var prior hookDedupeState
			if json.Unmarshal(raw, &prior) != nil || prior.Epoch == "" || prior.Digests == nil || len(prior.Digests) > hookDedupeMaxEntries || prior.At.IsZero() {
				d.stateError = true
				d.close()
				return d
			}
			d.state = prior
			for key, entry := range d.state.Digests {
				if len(key) != 64 || len(entry.Digest) != 64 || entry.EmittedAt.IsZero() || entry.Suppressions < 0 || entry.Suppressions > hookDedupeMaxSuppressions {
					d.stateError = true
					d.close()
					return d
				}
			}
			if d.state.Epoch != stamp.At || now.Sub(d.state.At) > hookDedupeAge || d.state.At.After(now) {
				d.state = fresh()
			}
		}
	}
	return d
}

func (d *hookDedupeCheck) persist() bool {
	raw, err := json.Marshal(d.state)
	if err != nil || len(raw) > hookDedupeMaxBytes {
		d.stateError = true
		return false
	}
	if _, err = d.file.Seek(0, io.SeekStart); err != nil {
		d.stateError = true
		return false
	}
	if err = d.file.Truncate(0); err != nil {
		d.stateError = true
		return false
	}
	if n, err := d.file.Write(raw); err != nil || n != len(raw) {
		d.stateError = true
		return false
	}
	if err = d.file.Sync(); err != nil {
		d.stateError = true
		return false
	}
	return true
}

func (d *hookDedupeCheck) fits() bool {
	raw, err := json.Marshal(d.state)
	return err == nil && len(raw) <= hookDedupeMaxBytes
}

func (d *hookDedupeCheck) commit(now time.Time) {
	if d.file == nil {
		return
	}
	if d.ruleMode {
		for _, key := range d.ruleKeys {
			d.state.Digests[key] = hookDedupeEntry{Digest: key, EmittedAt: now}
		}
	} else {
		d.state.Digests[d.key] = hookDedupeEntry{Digest: d.digest, EmittedAt: now}
	}
	d.state.At = now
	d.persist()
	d.close()
}

func (d *hookDedupeCheck) close() {
	if d.file != nil {
		// Close releases flock too. A delayed close error cannot authorize
		// suppression, even if the preceding write appeared to succeed.
		if err := d.file.Close(); err != nil {
			d.stateError = true
		}
		d.file = nil
	}
}

func (d *hookDedupeCheck) record(id, sessionID, channel, payload string) {
	if payload == "" {
		return
	}
	kind, emittedBytes, suppressedBytes := "emitted", len(payload), 0
	if d.suppressed {
		kind, emittedBytes, suppressedBytes = "suppressed", 0, len(payload)
	}
	emitBeat("hook-dedupe", kind, firingMeta(map[string]string{
		"channel": channel, "harness": id, "emitted": boolStr(!d.suppressed), "bytes": strconv.Itoa(emittedBytes), "suppressed_bytes": strconv.Itoa(suppressedBytes),
		"key": d.logKey, "first_touch": boolStr(d.firstTouch), "state_error": boolStr(d.stateError),
	}, sessionID, ""))
}

func resetHookDedupe(sessionID, source string, _ time.Time) {
	if sessionID == "" || (source != "compact" && source != "resume") {
		return
	}
	f, _, err := openHookDedupe(sessionID)
	if err != nil {
		return
	}
	defer f.Close()
	if err := f.Truncate(0); err == nil {
		_ = f.Sync()
	}
}
