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
//
// Invalidation (critic finding F2, BLOCKING, phase-P fix round — the O15
// class: silent injection death on rename/reingest). The original design
// keyed ONLY on (repo root, daemon base URL), which is constant across a
// daemon restart, an index rebuild, or a re-ingest, so a stale entry could
// outlive the very event that invalidated it, for up to identCacheTTL —
// and because a NEGATIVE resolution (repo not indexed) was cached too, a
// freshly-onboarded repo could see its own first obligations silently
// starved for that whole window while doctor (reading through the same
// cache) reported healthy. Fixed two ways: (1) negative resolutions get a
// much shorter TTL (identCacheNegativeTTL) than positive ones — a repo
// that WAS unindexed a few seconds ago is re-checked promptly rather than
// trusted for the full 5 minutes; (2) every ingest — CLI `fathom ingest`
// (this file's caller in ingest.go), the MCP `ingest` tool, and
// auto-refresh's own re-ingest (internal/mcp/autorefresh.go,
// internal/mcp/extra_tools.go) — busts the ENTIRE cache via
// internal/hookcache.BustIdent(), so a stale entry can never survive the
// event that changed the answer, regardless of TTL.
//
// Folding a daemon boot-id/PID into the key itself (the design's other
// stated option) was assessed and NOT implemented: server_stats does
// expose started_at, but reading it to validate a key requires the exact
// daemon round trip this cache exists to avoid — checking it on every
// call (hit or miss) would erase most of the "~3,600 round trips -> tens"
// win the design measures success by. The bust-on-ingest mechanism above
// covers the concrete failure the critic named (a rename/reingest making
// a cached answer wrong) without that cost; a cheap LOCAL signal (e.g. a
// daemon-written pidfile a hook could stat without a network call) would
// close the remaining restart-without-reingest gap and is filed as a
// follow-up, not silently dropped.

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

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

// identCacheTTL bounds how long a POSITIVE cached resolution is honored —
// short enough that a service rename or a fresh ingest is discovered
// within one window, long enough to collapse the "one daemon round trip
// per edit" pattern this cache exists to fix.
const identCacheTTL = 5 * time.Minute

// identCacheNegativeTTL bounds a NEGATIVE resolution (repo not yet
// indexed) — critic finding F2: caching "not found" for the full positive
// TTL meant a repo's first few minutes after `fathom init`/first ingest
// could see obligations silently starved. Short enough that a same-session
// ingest is picked up within seconds, not minutes.
const identCacheNegativeTTL = 15 * time.Second

// identCacheDir is os.TempDir()/fathom-ident-cache — a package var (not a
// const) so tests can point it at a t.TempDir(), mirroring guardStampDir.
// Initialized from internal/hookcache.IdentDir so cross-process busting
// (internal/mcp's auto-refresh / ingest tool) targets the SAME default
// path without the two packages importing each other.
var identCacheDir = hookcache.IdentDir

// identCacheDisabled reports the --no-cache escape (see package doc).
func identCacheDisabled() bool {
	return os.Getenv("FATHOM_IDENT_NO_CACHE") != ""
}

// identCacheEntry is one cached (repo root, daemon base URL) resolution.
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
// hit=false on any absence/corruption/ownership mismatch, or expiry —
// Found=true entries expire after identCacheTTL, Found=false entries
// after the much shorter identCacheNegativeTTL (see package doc, critic
// finding F2). Mirrors readGuardStamp's exact hardening otherwise (see
// guardstamp.go's doc comment for the full TOCTOU reasoning; identical
// here).
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
	if err != nil {
		return serviceCandidate{}, false, false
	}
	ttl := identCacheTTL
	if !e.Found {
		ttl = identCacheNegativeTTL
	}
	if now.Sub(writtenAt) > ttl {
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
