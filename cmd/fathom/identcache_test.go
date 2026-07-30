package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	_, found, hit := readIdentCache("/repo", "http://localhost:7575", now.Add(1*time.Minute))
	if !hit {
		t.Fatal("expected a cache hit for a cached negative resolution")
	}
	if found {
		t.Error("expected found=false to be preserved")
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
