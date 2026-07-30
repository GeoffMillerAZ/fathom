package main

// editset.go — instrument B's edit-set exclusion (docs/design/production-
// tokenomics.md §4: "Read when the target is outside the current edit
// set"). A Read of a file THIS session just edited is verification, not
// discovery — critic finding F8b (phase-P fix round): counting EVERY Read
// inflates the discovery-share denominator against the design's own
// scope. markFileEdited is called from the POST edit-hook (the point
// where an edit is confirmed to have actually landed); fileInEditSet is
// consulted by the discovery observer's Read branch before it counts a
// beat.
//
// Mechanism mirrors guardstamp.go/identcache.go exactly: 0700 dir, 0600
// file, O_CREATE|O_EXCL|O_NOFOLLOW write (never through a planted path
// entry), O_NOFOLLOW + owner/permission/regular-file check on the OPEN FD
// on read (no TOCTOU gap). A short TTL bounds "recently" — this is a
// counting refinement, not a correctness mechanism, so a missed exclusion
// (TTL expired, no session id) just means a verification read counts as
// discovery, never the reverse.

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// editSetTTL bounds how long a file counts as "currently being edited"
// for the purposes of excluding a same-session verification Read from the
// discovery-share denominator.
const editSetTTL = 10 * time.Minute

// editSetDir is os.TempDir()/fathom-edit-set — a package var (not a
// const) so tests can point it at a t.TempDir().
var editSetDir = filepath.Join(os.TempDir(), "fathom-edit-set")

func editSetPath(sessionID, repoRelFile string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + repoRelFile))
	return filepath.Join(editSetDir, hex.EncodeToString(h[:16])+".stamp")
}

// markFileEdited records that repoRelFile was just edited in sessionID.
// A no-op with no session id or file (mirrors guardstamp.go's own
// leakage-bound doctrine: no session, no dedup-adjacent bookkeeping).
// Best-effort: every failure is swallowed, exactly like emitBeat.
func markFileEdited(sessionID, repoRelFile string, now time.Time) {
	if sessionID == "" || repoRelFile == "" {
		return
	}
	if err := os.MkdirAll(editSetDir, 0o700); err != nil {
		return
	}
	path := editSetPath(sessionID, repoRelFile)
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte(now.UTC().Format(time.RFC3339)))
}

// fileRecentlyEdited reports whether repoRelFile was marked edited by
// sessionID within editSetTTL. false on any absence/corruption/expiry/
// ownership mismatch — a fail-toward-"count it as discovery" default.
func fileRecentlyEdited(sessionID, repoRelFile string, now time.Time) bool {
	if sessionID == "" || repoRelFile == "" {
		return false
	}
	path := editSetPath(sessionID, repoRelFile)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil {
		return false
	}
	writtenAt, err := time.Parse(time.RFC3339, string(data))
	if err != nil || now.Sub(writtenAt) > editSetTTL {
		return false
	}
	return true
}

// fileInEditSet resolves filePath (as sent by a Read PreToolUse call)
// against the local, daemon-free service-candidate roots and checks
// whether the resolved repo-relative path is in sessionID's recent edit
// set. Zero daemon calls, mirrors cmdEditHookPreFile's own local
// resolution exactly. false (never excluded) on any resolution failure —
// counting the Read is always the safe default.
func fileInEditSet(filePath, cwd, sessionID string) bool {
	if filePath == "" || sessionID == "" {
		return false
	}
	if !filepath.IsAbs(filePath) {
		a, err := filepath.Abs(filePath)
		if err != nil {
			return false
		}
		filePath = a
	}
	projDir := resolveProjDir(cwd)
	now := time.Now()
	for _, cand := range serviceCandidates(projDir) {
		if rel, ok := relativeToRoot(cand.root, filePath); ok {
			if fileRecentlyEdited(sessionID, rel, now) {
				return true
			}
		}
	}
	return false
}
