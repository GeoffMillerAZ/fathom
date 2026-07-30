package main

// identcache.go — service-identity resolution caching
// (docs/design/production-tokenomics.md §7): `list_services` is the
// fleet's most-called tool (3,624 calls/week measured 2026-07-30), and
// every one of them is hook identity resolution via candidatesWithDaemon's
// daemon round trip (resolveIndexedCandidate). This caches that round trip
// per (repo root, daemon base URL) in a short-lived local stamp, reusing
// guardstamp.go's exact hardening conventions: 0700 dir, 0600 file,
// O_CREATE|O_EXCL|O_NOFOLLOW on write (never through a planted path entry),
// O_NOFOLLOW plus an owner/permission/regular-file check on the OPEN FD on
// read (no TOCTOU gap between check and trust).
//
// This is a LATENCY/LOAD win, never a token win (production-tokenomics.md
// §7's own label) — the daemon round trip costs wall time and daemon CPU,
// not context tokens, so it is reported separately from every "what fathom
// SPENT" token figure, never folded into it.
//
// Escape hatch: FATHOM_IDENT_NO_CACHE=1 disables both read and write —
// deliberately an env var (identCacheDisabled), not a per-command
// --no-cache flag, because the dominant callers are Claude Code hooks
// invoked via a fixed JSON command line, not an interactive CLI session a
// human can add a flag to.
//
// Hit/miss is reported as beats (channel "ident-cache") rather than an
// in-process counter — each hook invocation is its own short-lived
// process making at most a couple of these calls, so a per-process
// counter would be nearly meaningless; the beat log lets research_report.go
// aggregate a real hit rate across sessions and time, exactly like every
// other hook instrument in this program.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// identCacheTTL bounds how long a cached resolution is honored — short
// enough that a service rename or a fresh ingest is discovered within one
// window, long enough to collapse the "one daemon round trip per edit"
// pattern this cache exists to fix.
const identCacheTTL = 5 * time.Minute

// identCacheDir is os.TempDir()/fathom-ident-cache — a package var (not a
// const) so tests can point it at a t.TempDir(), mirroring guardStampDir.
var identCacheDir = filepath.Join(os.TempDir(), "fathom-ident-cache")

// identCacheDisabled reports the --no-cache escape (see package doc).
func identCacheDisabled() bool {
	return os.Getenv("FATHOM_IDENT_NO_CACHE") != ""
}

// identCacheEntry is one cached (repo root, daemon base URL) resolution.
// Found=false is cached too (a repo not yet indexed) so a hook on an
// unindexed project doesn't re-probe the daemon on every single edit
// either — it simply retries after identCacheTTL, which is short enough
// that a same-session `fathom init`/ingest is picked up promptly.
type identCacheEntry struct {
	Name      string `json:"name"`
	Root      string `json:"root"`
	Found     bool   `json:"found"`
	WrittenAt string `json:"written_at"`
}

// identCacheKey returns the stamp path for (projDir, baseURL).
func identCacheKey(projDir, baseURL string) string {
	h := sha256.Sum256([]byte(projDir + "\x1f" + baseURL))
	return filepath.Join(identCacheDir, hex.EncodeToString(h[:16])+".json")
}

// readIdentCache returns the cached candidate for (projDir, baseURL),
// hit=false on any absence/corruption/expiry/ownership mismatch — mirrors
// readGuardStamp's exact hardening (see guardstamp.go's doc comment for the
// full TOCTOU reasoning; identical here).
func readIdentCache(projDir, baseURL string, now time.Time) (cand serviceCandidate, found, hit bool) {
	path := identCacheKey(projDir, baseURL)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return serviceCandidate{}, false, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return serviceCandidate{}, false, false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Getuid() {
			return serviceCandidate{}, false, false
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return serviceCandidate{}, false, false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<16))
	if err != nil {
		return serviceCandidate{}, false, false
	}
	var e identCacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return serviceCandidate{}, false, false
	}
	writtenAt, err := time.Parse(time.RFC3339, e.WrittenAt)
	if err != nil || now.Sub(writtenAt) > identCacheTTL {
		return serviceCandidate{}, false, false
	}
	return serviceCandidate{name: e.Name, root: e.Root}, e.Found, true
}

// writeIdentCache persists the resolution for (projDir, baseURL). Failures
// are swallowed (this is a pure optimization — worst case, the next call
// pays a full daemon round trip again).
func writeIdentCache(projDir, baseURL string, cand serviceCandidate, found bool, now time.Time) {
	if err := os.MkdirAll(identCacheDir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(identCacheEntry{
		Name: cand.name, Root: cand.root, Found: found,
		WrittenAt: now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	path := identCacheKey(projDir, baseURL)
	_ = os.Remove(path) // never write through a pre-existing entry (planted symlink or stale stamp)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// resolveIndexedCandidateCached wraps resolveIndexedCandidate with the
// cache: a fresh hit skips the daemon round trip entirely; a miss (or
// FATHOM_IDENT_NO_CACHE) falls through to the real call and populates the
// cache for next time. Every outcome emits an "ident-cache" beat
// (hit/miss) — research_report.go sums these into a hit rate.
func resolveIndexedCandidateCached(hc *http.Client, baseURL, projDir string) (serviceCandidate, bool) {
	if !identCacheDisabled() {
		if cand, found, hit := readIdentCache(projDir, baseURL, time.Now()); hit {
			emitBeat("ident-cache", "hit", nil)
			return cand, found
		}
	}
	emitBeat("ident-cache", "miss", nil)
	cand, found := resolveIndexedCandidate(hc, baseURL, projDir)
	if !identCacheDisabled() {
		writeIdentCache(projDir, baseURL, cand, found, time.Now())
	}
	return cand, found
}
