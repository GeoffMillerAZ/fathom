package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

func withTempIdentCacheDir(t *testing.T) {
	t.Helper()
	orig := identCacheDir
	identCacheDir = filepath.Join(t.TempDir(), "fathom-ident-cache")
	t.Cleanup(func() { identCacheDir = orig })
}

func TestIdentCache_WriteThenRead(t *testing.T) {
	withTempIdentCacheDir(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeIdentCache("/repo", "http://localhost:7575", serviceCandidate{name: "fathom", root: "/repo"}, true, now)

	cand, found, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(1*time.Minute))
	if !hit {
		t.Fatal("expected a cache hit shortly after write")
	}
	if !found || cand.name != "fathom" || cand.root != "/repo" {
		t.Errorf("cand=%+v found=%v", cand, found)
	}
}

func TestIdentCache_NotFoundCachedToo(t *testing.T) {
	withTempIdentCacheDir(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeIdentCache("/repo", "http://localhost:7575", serviceCandidate{}, false, now)

	_, found, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(5*time.Second))
	if !hit {
		t.Fatal("expected a cache hit for a cached negative resolution within the negative TTL")
	}
	if found {
		t.Error("expected found=false to be preserved")
	}
}

// TestIdentCache_NegativeTTLShorterThanPositive is critic finding F2's own
// proof (BLOCKING, phase-P fix round): a NEGATIVE resolution (repo not yet
// indexed) must expire far sooner than a positive one — caching "not
// found" for the full positive TTL was the O15-class bug (a freshly
// ingested repo's first obligations silently starved for minutes).
func TestIdentCache_NegativeTTLShorterThanPositive(t *testing.T) {
	withTempIdentCacheDir(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeIdentCache("/repo", "http://localhost:7575", serviceCandidate{}, false, now)

	if _, _, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(identCacheNegativeTTL+time.Second)); hit {
		t.Fatal("expected a negative resolution to expire shortly after identCacheNegativeTTL")
	}
	if identCacheNegativeTTL >= identCacheTTL {
		t.Fatalf("identCacheNegativeTTL (%s) must be shorter than identCacheTTL (%s)", identCacheNegativeTTL, identCacheTTL)
	}

	// A POSITIVE resolution written at the same instant must still be
	// live at that same elapsed time — the two TTLs are genuinely
	// independent, not one shared clock.
	writeIdentCache("/repo2", "http://localhost:7575", serviceCandidate{name: "svc", root: "/repo2"}, true, now)
	if _, found, hit := readIdentCache("/repo2", "http://localhost:7575", now.Add(identCacheNegativeTTL+time.Second)); !hit || !found {
		t.Fatal("expected the positive resolution to still be live past the negative TTL")
	}
}

func TestIdentCache_AbsentWhenNeverWritten(t *testing.T) {
	withTempIdentCacheDir(t)
	if _, _, hit := readIdentCache("/repo", "http://localhost:7575", time.Now()); hit {
		t.Fatal("expected no cache entry for a key that was never written")
	}
}

func TestIdentCache_ExpiresAfterTTL(t *testing.T) {
	withTempIdentCacheDir(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeIdentCache("/repo", "http://localhost:7575", serviceCandidate{name: "fathom", root: "/repo"}, true, now)

	if _, _, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(identCacheTTL+time.Minute)); hit {
		t.Fatal("expected the entry to be expired past the TTL")
	}
	if _, _, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(identCacheTTL-time.Minute)); !hit {
		t.Fatal("expected the entry to still be valid just under the TTL")
	}
}

func TestIdentCache_KeyedByBothRootAndURL(t *testing.T) {
	withTempIdentCacheDir(t)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	writeIdentCache("/repo-a", "http://localhost:7575", serviceCandidate{name: "a", root: "/repo-a"}, true, now)

	if _, _, hit := readIdentCache("/repo-b", "http://localhost:7575", now); hit {
		t.Fatal("a different repo root must not share a cache entry")
	}
	if _, _, hit := readIdentCache("/repo-a", "http://localhost:9999", now); hit {
		t.Fatal("a different daemon URL must not share a cache entry")
	}
}

// TestResolveIndexedCandidateCached_ServesStaleOnTimeoutOverBasenameFallback
// is the DG-12 contract test: "identity timeout survives load." A fresh
// daemon round trip that fails outright (simulated here with a closed
// listener — connection-refused and a client timeout share the exact same
// code path, fetchServices returning a non-nil error, so either
// reproduces it) must serve the last known-good, past-TTL resolution
// rather than reporting ok=false and sending the caller
// (candidatesWithDaemon) all the way back to an unverified basename
// guess.
func TestResolveIndexedCandidateCached_ServesStaleOnTimeoutOverBasenameFallback(t *testing.T) {
	withTempIdentCacheDir(t)
	dir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := srv.URL
	srv.Close() // nothing listens on baseURL anymore: every round trip fails

	// Seed a POSITIVE resolution already past the normal positive TTL.
	staleWrittenAt := time.Now().Add(-identCacheTTL - time.Minute)
	writeIdentCache(dir, baseURL, serviceCandidate{name: "realname", root: dir}, true, staleWrittenAt)

	// Sanity: the fresh-TTL read must already report it expired.
	if _, _, hit := readIdentCache(dir, baseURL, time.Now()); hit {
		t.Fatal("setup: expected the seeded entry to already be expired under the normal TTL")
	}

	hc := &http.Client{Timeout: 200 * time.Millisecond}
	cand, ok, stale := resolveIndexedCandidateCached(hc, baseURL, dir)
	if !ok {
		t.Fatal("resolveIndexedCandidateCached ok = false, want true (served from the stale cache entry, not the raw basename fallback)")
	}
	if cand.name != "realname" || cand.root != dir {
		t.Errorf("cand = %+v, want the stale-served known-good identity {realname %s}", cand, dir)
	}
	// finding [3]/DG-12: the stale-served case MUST be reported distinctly
	// from a fresh hit/round-trip — candidatesWithDaemon depends on this to
	// avoid stamping a possibly-stale identity as daemon-verified.
	if !stale {
		t.Error("stale = false, want true: a DG-12 stale-served answer must be reported as stale, not indistinguishable from a fresh one")
	}
}

// TestResolveIndexedCandidateCached_StaleFallbackNeverServesNegative proves
// the stale-serve path never resurrects a cached NEGATIVE ("not indexed")
// resolution as though it were a usable identity — only a past POSITIVE
// resolution is ever worth serving stale.
func TestResolveIndexedCandidateCached_StaleFallbackNeverServesNegative(t *testing.T) {
	withTempIdentCacheDir(t)
	dir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	staleWrittenAt := time.Now().Add(-identCacheTTL - time.Minute)
	writeIdentCache(dir, baseURL, serviceCandidate{}, false, staleWrittenAt)

	hc := &http.Client{Timeout: 200 * time.Millisecond}
	if _, ok, _ := resolveIndexedCandidateCached(hc, baseURL, dir); ok {
		t.Fatal("expected ok = false: a cached NEGATIVE resolution must never be served as a stale fallback identity")
	}
}

// TestCandidatesWithDaemon_CachesAcrossCalls is the load-bearing proof: two
// calls to candidatesWithDaemon for the same (root, URL) must hit the
// daemon only ONCE — the second is served entirely from the local cache
// (docs/design/production-tokenomics.md §7's "~3,600 round trips -> tens").
func TestCandidatesWithDaemon_CachesAcrossCalls(t *testing.T) {
	withTempIdentCacheDir(t)
	dir := t.TempDir()

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"services": []map[string]any{
					{"service": strings.ToLower(filepath.Base(dir)), "symbol_count": 3, "path": dir},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	hc := &http.Client{Timeout: 2 * time.Second}
	first := candidatesWithDaemon(hc, srv.URL, dir)
	second := candidatesWithDaemon(hc, srv.URL, dir)

	if n := *calls; n != 1 {
		t.Fatalf("daemon queried %d time(s) across two candidatesWithDaemon calls, want exactly 1 (second should be a cache hit)", n)
	}
	if len(first) == 0 || len(second) == 0 || first[0].name != second[0].name {
		t.Errorf("first=%v second=%v — expected the same resolved identity both times", first, second)
	}
}

// TestCandidatesWithDaemon_StaleServedNeverMarksVerified is the finding [3]
// regression test: a DG-12 stale-served answer (the daemon round trip
// failed outright; the cache replayed a past-TTL POSITIVE resolution) must
// NOT be stamped verified=true the way a genuinely fresh daemon
// confirmation is. Before this fix, resolveIndexedCandidateCached's ok=true
// covered both cases identically, so candidatesWithDaemon always set
// verified=true — letting renderPostEdit's DG-11 short-circuit
// (`if cand.verified { break }`) lock onto a stale identity that may have
// gone wrong (remap/rename/removal) during the staleness window, instead
// of trying the next, possibly-correct candidate.
func TestCandidatesWithDaemon_StaleServedNeverMarksVerified(t *testing.T) {
	withTempIdentCacheDir(t)
	dir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL := srv.URL
	srv.Close() // every fresh round trip fails from here on

	staleWrittenAt := time.Now().Add(-identCacheTTL - time.Minute)
	writeIdentCache(dir, baseURL, serviceCandidate{name: "realname", root: dir}, true, staleWrittenAt)

	hc := &http.Client{Timeout: 200 * time.Millisecond}
	cands := candidatesWithDaemon(hc, baseURL, dir)
	if len(cands) == 0 {
		t.Fatal("candidatesWithDaemon returned no candidates at all")
	}
	if cands[0].name != "realname" {
		t.Fatalf("cands[0] = %+v, want the stale-served identity to still be tried first", cands[0])
	}
	if cands[0].verified {
		t.Error("cands[0].verified = true, want false: a stale-served (possibly outdated) identity must never carry the same trust as a fresh daemon confirmation")
	}
}

// TestCandidatesWithDaemon_NoCacheEscape proves FATHOM_IDENT_NO_CACHE
// disables the cache entirely — every call round-trips the daemon again.
func TestCandidatesWithDaemon_NoCacheEscape(t *testing.T) {
	withTempIdentCacheDir(t)
	t.Setenv("FATHOM_IDENT_NO_CACHE", "1")
	dir := t.TempDir()

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0", "id": 1,
			"result": map[string]any{
				"services": []map[string]any{
					{"service": strings.ToLower(filepath.Base(dir)), "symbol_count": 3, "path": dir},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	hc := &http.Client{Timeout: 2 * time.Second}
	candidatesWithDaemon(hc, srv.URL, dir)
	candidatesWithDaemon(hc, srv.URL, dir)

	if n := *calls; n != 2 {
		t.Fatalf("daemon queried %d time(s) with FATHOM_IDENT_NO_CACHE set, want exactly 2 (cache must be fully bypassed)", n)
	}
}

// TestCmdIngest_BustsIdentCache is critic finding F2's end-to-end proof
// (BLOCKING, phase-P fix round): `fathom ingest` must remove every cached
// service-identity resolution, never letting a hook see a pre-ingest
// answer past the cache's TTL. identCacheDir (cmd/fathom's own package
// var) and hookcache.IdentDir (the cross-process bust target) are pointed
// at the SAME temp directory so this test can observe both the write
// path and the bust path operating on one location, exactly as they do
// in production (both default to the same os.TempDir() path).
func TestCmdIngest_BustsIdentCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fathom-ident-cache")
	origIdentCacheDir, origHookcacheDir := identCacheDir, hookcache.IdentDir
	identCacheDir, hookcache.IdentDir = dir, dir
	t.Cleanup(func() { identCacheDir, hookcache.IdentDir = origIdentCacheDir, origHookcacheDir })

	writeIdentCache("/some/repo", "http://localhost:7575", serviceCandidate{name: "svc", root: "/some/repo"}, true, time.Now())
	if _, _, hit := readIdentCache("/some/repo", "http://localhost:7575", time.Now()); !hit {
		t.Fatal("setup: expected the seeded cache entry to be readable before ingest")
	}

	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	var out, errBuf bytes.Buffer
	code := cmdIngest([]string{"--service", "payment-service", "--mode", "faux", "--store", storePath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("cmdIngest exit = %d, stderr = %s", code, errBuf.String())
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected the ident cache directory to be removed by ingest, stat err = %v", err)
	}
	if _, _, hit := readIdentCache("/some/repo", "http://localhost:7575", time.Now()); hit {
		t.Fatal("expected the seeded cache entry to be gone after ingest busts the cache")
	}
}
