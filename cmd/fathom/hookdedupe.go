package main

// Hook-owned context bookkeeping, beside the existing session stamps. It
// never reaches the daemon store. A missed optimization always costs bytes,
// never guidance: even a duplicate must successfully write its state before
// it may be suppressed. Nonblocking flock serializes hook processes without
// another lock file or time budget. A contending process simply emits in full.

import (
	"bytes"
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
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

const (
	hookDedupeMaxBytes   = 256 << 10
	hookDedupeMaxEntries = 2048
	hookDedupeAge        = 7 * 24 * time.Hour
)

type hookDedupeState struct {
	Version int               `json:"version"`
	Epoch   string            `json:"epoch"`
	At      time.Time         `json:"at"`
	Digests map[string]string `json:"digests"`
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

// hookDedupeEnabled follows the existing pin-check config fast path. A bad
// config disables dedupe, preserving the full context instead of guessing.
func hookDedupeEnabled(root string) bool {
	user, project := config.DefaultUserPath(), projectdir.Config.Path(root)
	if !bytes.Contains(readConfigLayer(user), []byte("dedupe")) && !bytes.Contains(readConfigLayer(project), []byte("dedupe")) {
		return true
	}
	loaded, err := config.Load(user, project)
	return err == nil && loaded.Config.Hooks.Dedupe
}

// hookInjectionSeen compares the latest digest for a channel/target, not a
// historical set: A -> B -> A is a changed answer and must be served again.
// Keys and payloads are hashed; no rule text or repository paths are saved.
// now is injected for expiry; state and per-call work are size bounded.
func hookInjectionSeen(sessionID, channel, target, payload string, now time.Time, reset bool) (seen bool) {
	// An id without SessionStart evidence is an unprimed/replayed hook.
	stamp := readSessionStamp(sessionID)
	if !reset && (stamp.SessionID != sessionID || stamp.At == "") {
		return false
	}
	path := hookDedupePath(sessionID)
	if path == "" {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return false
	}
	defer func() {
		if err := f.Close(); err != nil {
			seen = false
		}
	}()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return false
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > hookDedupeMaxBytes {
		return false
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return false
	}
	if reset {
		// Empty is a fresh context, including a resume whose old stamp was
		// unavailable. The next injection writes its current epoch.
		if err := f.Truncate(0); err == nil {
			_ = f.Sync()
		}
		return false
	}
	epoch := stamp.At
	state := hookDedupeState{Version: 1, Epoch: epoch, At: now, Digests: map[string]string{}}
	if info.Size() > 0 {
		raw, err := io.ReadAll(io.LimitReader(f, hookDedupeMaxBytes+1))
		var prior hookDedupeState
		if err != nil || len(raw) > hookDedupeMaxBytes || json.Unmarshal(raw, &prior) != nil || prior.Version != 1 || prior.Epoch == "" || prior.Digests == nil || len(prior.Digests) > hookDedupeMaxEntries || prior.At.IsZero() {
			return false
		}
		state = prior
		if state.Epoch != epoch || now.Sub(state.At) > hookDedupeAge || state.At.After(now) {
			state = hookDedupeState{Version: 1, Epoch: epoch, At: now, Digests: map[string]string{}}
		}
	}
	duplicate := false
	{
		key, digest := hookDigest(channel+"\x00"+target), hookDigest(payload)
		previous, exists := state.Digests[key]
		if !exists && len(state.Digests) >= hookDedupeMaxEntries {
			return false
		}
		duplicate = exists && previous == digest
		state.Digests[key] = digest
	}
	state.At = now
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > hookDedupeMaxBytes {
		return false
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	if err = f.Truncate(0); err != nil {
		return false
	}
	if n, err := f.Write(raw); err != nil || n != len(raw) {
		return false
	}
	// Detect delayed write/close errors too. A corrupt/partial record is never
	// trusted next time; no failure path can authorize suppression.
	if err = f.Sync(); err != nil {
		return false
	}
	return duplicate
}

func resetHookDedupe(sessionID, source string, now time.Time) {
	if source == "compact" || source == "resume" {
		hookInjectionSeen(sessionID, "", "", "", now, true)
	}
}

func dedupeHookInjection(root, sessionID, channel, target, payload string, now time.Time) bool {
	if sessionID == "" || !hookDedupeEnabled(root) {
		return false
	}
	if !hookInjectionSeen(sessionID, channel, target, payload, now, false) {
		return false
	}
	if payload != "" {
		emitBeat("hook-dedupe", "suppressed", firingMeta(map[string]string{
			"channel": channel, "bytes": "0", "suppressed_bytes": strconv.Itoa(len(payload)), "emitted": "false",
		}, sessionID, ""))
	}
	return true
}
