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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
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
	e, ok := readIdentCacheEntry(projDir, baseURL)
	if !ok {
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

// readIdentCacheEntry loads and validates the raw cache entry for (projDir,
// baseURL) with every hardening check readIdentCache always applied
// (O_NOFOLLOW, owner check, permission check, size cap, JSON shape), but
// WITHOUT judging its age — the shared parse step both the fresh
// (TTL-bounded) read above and the DG-12 stale-served read below build on.
func readIdentCacheEntry(projDir, baseURL string) (identCacheEntry, bool) {
	data, ok := readOwnedCacheFile(identCacheKey(projDir, baseURL))
	if !ok {
		return identCacheEntry{}, false
	}
	var e identCacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return identCacheEntry{}, false
	}
	return e, true
}

// identCacheStaleServeMax bounds how far past its normal TTL a POSITIVE
// cache entry may still be served as a DG-12 fallback when a fresh daemon
// round trip fails outright — never indefinitely: an identity that has
// been unconfirmable for this long is worse than none, and a long-dead
// daemon should eventually degrade to the honest "not resolved" state
// rather than replay an arbitrarily old answer forever.
const identCacheStaleServeMax = 24 * time.Hour

// readIdentCacheStale serves a POSITIVE resolution past its normal TTL
// (DG-12): "identity timeout survives load" — a daemon round trip that
// fails outright (a network error, or a timeout when list_services is
// slow under load, never a genuine "not indexed" answer, which already
// has its own short identCacheNegativeTTL and is handled entirely by
// readIdentCache) should degrade to the last KNOWN-good identity rather
// than the raw, unverified basename guess candidatesWithDaemon otherwise
// falls all the way back to. hit=false when there is no entry, the last
// cached answer was itself negative, or it is older than
// identCacheStaleServeMax.
func readIdentCacheStale(projDir, baseURL string, now time.Time) (cand serviceCandidate, hit bool) {
	e, ok := readIdentCacheEntry(projDir, baseURL)
	if !ok || !e.Found {
		return serviceCandidate{}, false
	}
	writtenAt, err := time.Parse(time.RFC3339, e.WrittenAt)
	if err != nil {
		return serviceCandidate{}, false
	}
	if now.Sub(writtenAt) > identCacheStaleServeMax {
		return serviceCandidate{}, false
	}
	return serviceCandidate{name: e.Name, root: e.Root}, true
}

// writeIdentCache persists the resolution for (projDir, baseURL). Failures
// are swallowed (this is a pure optimization — worst case, the next call
// pays a full daemon round trip again).
func writeIdentCache(projDir, baseURL string, cand serviceCandidate, found bool, now time.Time) {
	data, err := json.Marshal(identCacheEntry{
		Name: cand.name, Root: cand.root, Found: found,
		WrittenAt: now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	writeOwnedCacheFile(identCacheKey(projDir, baseURL), data)
}

// resolveIndexedCandidateCached wraps resolveIndexedCandidate with the
// cache: a fresh hit skips the daemon round trip entirely; a miss (or
// FATHOM_IDENT_NO_CACHE) falls through to the real call and populates the
// cache for next time. Every outcome emits an "ident-cache" beat
// (hit/miss/stale-served/miss-error) — research_report.go sums these into
// a hit rate.
//
// stale is true ONLY for the DG-12 fallback below — a caller (specifically
// candidatesWithDaemon) MUST treat that case as a lower trust tier than a
// genuinely fresh round trip or cache hit (finding [3]): a stale-served
// answer can be up to identCacheStaleServeMax old and may no longer be
// true after a remap, rename, or service removal, so it must never be
// stamped with the SAME "verified" trust DG-11's daemon-confirmed
// candidates carry.
//
// DG-12: when the fresh round trip fails OUTRIGHT (fetchServices' own
// error — a network error, or list_services timing out under load, never
// a resolved-but-empty answer), this serves the last known POSITIVE
// resolution past its normal TTL instead of returning ok=false — the
// caller (candidatesWithDaemon) would otherwise fall all the way back to
// an unverified basename guess on every miss that lands during a load
// spike, exactly when the daemon-verified identity matters most.
func resolveIndexedCandidateCached(hc *http.Client, baseURL, projDir string) (cand serviceCandidate, found bool, stale bool) {
	if !identCacheDisabled() {
		if cand, found, hit := readIdentCache(projDir, baseURL, time.Now()); hit {
			emitBeat("ident-cache", "hit", nil)
			return cand, found, false
		}
	}
	emitBeat("ident-cache", "miss", nil)
	resolved, found, err := resolveIndexedCandidateErr(hc, baseURL, projDir)
	if err != nil {
		if !identCacheDisabled() {
			if staleCand, hit := readIdentCacheStale(projDir, baseURL, time.Now()); hit {
				emitBeat("ident-cache", "stale-served", nil)
				return staleCand, true, true
			}
		}
		emitBeat("ident-cache", "miss-error", nil)
		return serviceCandidate{}, false, false
	}
	if !identCacheDisabled() {
		writeIdentCache(projDir, baseURL, resolved, found, time.Now())
	}
	return resolved, found, false
}

// ─── the full identity, cached (RT-133, §11.3) ───────────────────────────

// identityMemo is this process's identity answers by (daemon, directory):
// SessionStart's stamp and its primer, or a hook's two lookups of one
// directory, share one round trip.
var identityMemo sync.Map // string → identityResult

// resetIdentityMemo forgets this process's identity answers (tests).
func resetIdentityMemo() {
	identityMemo.Range(func(k, _ any) bool { identityMemo.Delete(k); return true })
}

// identityCacheEntry is lookupIdentityCached's on-disk entry.
type identityCacheEntry struct {
	Result    identityResult `json:"result"`
	WrittenAt string         `json:"written_at"`
}

func identityCacheKey(dir, baseURL string) string { return identCacheKey(dir+"\x1fidentity", baseURL) }

// lookupIdentityCached is lookupIdentity behind the in-process memo and the
// on-disk ident cache (identCacheTTL for a match, identCacheNegativeTTL
// for none; a match up to identCacheStaleServeMax old answers when the
// daemon cannot). errIdentityRouteMissing is returned, never cached, so a
// caller facing an older daemon falls back to the full list.
// FATHOM_IDENT_NO_CACHE disables both layers.
func lookupIdentityCached(hc *http.Client, baseURL, dir string, contexts ...context.Context) (identityResult, error) {
	dir = strings.TrimRight(dir, "/")
	key := baseURL + "\x1f" + dir
	noCache := identCacheDisabled()
	if !noCache {
		if v, ok := identityMemo.Load(key); ok {
			id := v.(identityResult)
			return refreshCloneIdentity(id, dir, contexts...)
		}
		if e, ok := readIdentityCacheEntry(dir, baseURL); ok {
			if at, err := time.Parse(time.RFC3339, e.WrittenAt); err == nil {
				ttl := identCacheTTL
				if e.Result.Service == "" {
					ttl = identCacheNegativeTTL
				}
				if time.Since(at) <= ttl {
					emitBeat("ident-cache", "hit", nil)
					identityMemo.Store(key, e.Result)
					return refreshCloneIdentity(e.Result, dir, contexts...)
				}
			}
		}
		emitBeat("ident-cache", "miss", nil)
	}
	id, err := lookupIdentity(hc, baseURL, dir, contexts...)
	if err != nil {
		if !noCache && !errors.Is(err, errIdentityRouteMissing) {
			if e, ok := readIdentityCacheEntry(dir, baseURL); ok && e.Result.Service != "" {
				if at, perr := time.Parse(time.RFC3339, e.WrittenAt); perr == nil && time.Since(at) <= identCacheStaleServeMax {
					emitBeat("ident-cache", "stale-served", nil)
					return refreshCloneIdentity(e.Result, dir, contexts...)
				}
			}
		}
		return identityResult{}, err
	}
	if !noCache {
		identityMemo.Store(key, id)
		writeIdentityCacheEntry(dir, baseURL, id, time.Now())
	}
	return id, nil
}

func readIdentityCacheEntry(dir, baseURL string) (identityCacheEntry, bool) {
	data, ok := readOwnedCacheFile(identityCacheKey(dir, baseURL))
	if !ok {
		return identityCacheEntry{}, false
	}
	var e identityCacheEntry
	if json.Unmarshal(data, &e) != nil || e.Result.Schema != 1 {
		return identityCacheEntry{}, false
	}
	return e, true
}

func writeIdentityCacheEntry(dir, baseURL string, id identityResult, now time.Time) {
	data, err := json.Marshal(identityCacheEntry{Result: id, WrittenAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return
	}
	writeOwnedCacheFile(identityCacheKey(dir, baseURL), data)
}

// readOwnedCacheFile reads a cache file only when it is a regular file this
// user owns and nobody else can write (never through a planted symlink).
func readOwnedCacheFile(path string) ([]byte, bool) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o022 != 0 {
		return nil, false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, 1<<16))
	return data, err == nil
}

// writeOwnedCacheFile replaces path with data (0600, O_EXCL after a remove,
// never writing through a pre-existing entry).
func writeOwnedCacheFile(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

// The cached service mapping survives edits and new commits. Only its
// freshness note depends on the clone's current HEAD; no registry/history
// lookup is repeated on a hook cache hit.
func refreshCloneIdentity(id identityResult, dir string, contexts ...context.Context) (identityResult, error) {
	if id.FreshnessNote == "" {
		return id, nil
	}
	ctx, done := testRunContext()
	defer done()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if hookDeadlineSpent() {
		return identityResult{}, context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	head, err := gitro.Output(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return identityResult{}, err
	}
	short := func(s string) string {
		if s == "" {
			return "unknown"
		}
		if len(s) > 7 {
			return s[:7]
		}
		return s
	}
	id.FreshnessNote = "clone at " + short(strings.TrimSpace(string(head))) + ", index at " + short(id.IndexedCommit)
	return id, nil
}
