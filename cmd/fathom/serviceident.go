package main

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// serviceCandidate is one (service name, corpus root) identity guess for a
// Claude Code project directory. verified is true only for the candidate
// resolveIndexedCandidate(Cached) returned — i.e. one the daemon itself
// records a live ServiceMeta.Path for (DG-11) — never for a bare basename/
// git-toplevel guess, which carries no such backing at all. It is set by
// candidatesWithDaemon on the struct it hands to mergeCandidates, not by
// mergeCandidates itself, so a caller building its own serviceCandidate
// literals (as the O1 tests do) is unaffected: the zero value is false.
type serviceCandidate struct {
	name     string
	root     string
	verified bool
}

// serviceCandidates returns identity guesses for projDir, in preference order:
//
//  1. the directory's own basename (root = projDir) — the common case where
//     the Claude project IS the repo root;
//  2. the enclosing git work tree's basename (root = toplevel) — for projects
//     rooted in a subdirectory (e.g. <repo>/deploy), whose sessions would
//     otherwise resolve to a meaningless service like "deploy".
//
// Both hooks iterate these against the index, so either layout finds its
// service without configuration.
//
// Each root whose fathom stanza names a service other than its basename
// contributes that stanza-named identity FIRST (stanzaNamedService, the same
// fallback reconcile's stanzaServiceID uses): a checkout at …/sextant
// registered as "fathom" otherwise sent every daemon-unresolved lookup
// (spec_refs, memories) to a service that does not exist — 170 of 306
// live spec_refs calls failed that way.
func serviceCandidates(projDir string, parents ...context.Context) []serviceCandidate {
	projDir = strings.TrimRight(projDir, "/")
	var out []serviceCandidate
	add := func(root string) {
		base := filepath.Base(root)
		if named := stanzaNamedService(root); named != "" && !strings.EqualFold(named, base) {
			out = append(out, serviceCandidate{name: named, root: root})
		}
		out = append(out, serviceCandidate{name: base, root: root})
	}
	add(projDir)
	top := ""
	if len(parents) > 0 {
		top = gitToplevelContext(parents[0], projDir)
	} else {
		top = gitToplevel(projDir)
	}
	if top != "" && top != projDir {
		add(top)
	}
	return out
}

// gitToplevel returns the enclosing git work-tree root for dir, or "" when
// dir is not inside a repo (or git is unavailable). Memoized in process
// and cached per cwd across processes (gitToplevelCached, RT-130): a hook
// fires on every tool call, and a git fork per hook per call was a fixed
// cost of every edit.
func gitToplevel(dir string) string { return gitToplevelCached(dir) }

// gitToplevelUncached forks `git rev-parse --show-toplevel` in dir.
func gitToplevelUncached(dir string) string {
	return gitToplevelContext(context.Background(), dir)
}

func gitToplevelContext(ctx context.Context, dir string) string {
	out, err := gitro.Output(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resolveIndexedCandidate asks the daemon which indexed service records this
// project's directory (or its git toplevel) as its corpus root — the
// AUTHORITATIVE repo↔service identity, replacing the basename guess when
// they diverge (dogfood O15: the sextant→fathom rename left this repo's dir
// basename pointing at a service that does not exist, silently killing
// obligations injection for six days once the name-alias shim was removed).
// Best-effort and fail-open: any error, a daemon without the path field
// (pre-O15 binary), or no match returns ok=false and callers proceed with
// the basename candidates exactly as before. Symlinks are resolved on both
// sides so /var vs /private/var style aliases cannot defeat the match.
func resolveIndexedCandidate(hc *http.Client, baseURL, projDir string) (serviceCandidate, bool) {
	cand, ok, _ := resolveIndexedCandidateErr(hc, baseURL, projDir)
	return cand, ok
}

// resolveIndexedCandidateErr is resolveIndexedCandidate plus the raw
// fetchServices error (DG-12): resolveIndexedCandidate's every other
// caller wants the fail-open (cand, ok) pair exactly as before and stays
// on it unchanged; only the cache layer (identcache.go) needs to tell "the
// round trip itself failed" (network error, timeout under load) apart
// from "the round trip succeeded and genuinely found no match" — the two
// collapse to the same ok=false here otherwise, but only the FIRST should
// ever trigger a stale-cache fallback.
func resolveIndexedCandidateErr(hc *http.Client, baseURL, projDir string) (serviceCandidate, bool, error) {
	// RT-133 (§11.3): GET /identity answers from the daemon's registry
	// without listing every service over list_services; a daemon that
	// predates it falls back to the full list.
	if id, err := lookupIdentity(hc, baseURL, projDir); !errors.Is(err, errIdentityRouteMissing) {
		if err != nil {
			return serviceCandidate{}, false, err
		}
		if id.MatchedBy != identityByRoot && id.MatchedBy != destClone && id.MatchedBy != destWorktree {
			return serviceCandidate{}, false, nil
		}
		return serviceCandidate{name: id.Service, root: id.Root}, true, nil
	}
	svcs, err := fetchServices(hc, baseURL)
	cand, ok := resolveFromServices(svcs, projDir)
	return cand, ok, err
}

// resolveFromServices is resolveIndexedCandidate over an already-fetched
// services map (the session hook has one in hand — no second fetch).
//
// Resolution is DETERMINISTIC (O1): when more than one indexed service
// records the same corpus root — the duplicate-corpus-root class that once
// zeroed obligations on 36% of edits by winning or losing a Go map-order
// coin flip — the winner is chosen by richness (symbol count), then
// basename affinity (a name matching the dir/toplevel basename outranks a
// stranger), then lexicographic order. Never iteration order.
func resolveFromServices(svcs map[string]svcInfo, projDir string) (serviceCandidate, bool) {
	return resolveFromServicesCands(svcs, serviceCandidates(projDir))
}

// resolveFromServicesCands is resolveFromServices over explicit candidates
// (GET /identity receives them from the hook).
func resolveFromServicesCands(svcs map[string]svcInfo, base []serviceCandidate) (serviceCandidate, bool) {
	roots := map[string]string{} // resolved path → original root
	baseNames := map[string]bool{}
	for _, cand := range base {
		if r, err := filepath.EvalSymlinks(cand.root); err == nil {
			roots[r] = cand.root
		}
		baseNames[strings.ToLower(cand.name)] = true
	}
	if len(roots) == 0 {
		return serviceCandidate{}, false
	}
	type match struct {
		cand    serviceCandidate
		symbols int
		affine  bool
	}
	var best *match
	better := func(a, b match) bool {
		if a.symbols != b.symbols {
			return a.symbols > b.symbols
		}
		if a.affine != b.affine {
			return a.affine
		}
		return a.cand.name < b.cand.name
	}
	for name, info := range svcs {
		if info.path == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(info.path)
		if err != nil {
			continue
		}
		root, hit := roots[resolved]
		if !hit {
			continue
		}
		m := match{
			cand:    serviceCandidate{name: name, root: root},
			symbols: info.symbols,
			affine:  baseNames[strings.ToLower(name)],
		}
		if best == nil || better(m, *best) {
			best = &m
		}
	}
	if best == nil {
		return serviceCandidate{}, false
	}
	return best.cand, true
}

// candidatesWithDaemon returns serviceCandidates(projDir) with the
// daemon-resolved identity (when one exists) PREPENDED and deduplicated —
// the shared resolution every hook uses so injection cannot silently die on
// a dir-name/service-ID mismatch again.
//
// The daemon round trip (resolveIndexedCandidate) is the fleet's single
// most-called operation (list_services, 3,624 calls/week measured
// 2026-07-30 — docs/design/production-tokenomics.md §7): identcache.go
// caches it per (projDir, baseURL) with a short TTL, so the overwhelming
// majority of calls in a session resolve locally instead of round-tripping
// the daemon again.
func candidatesWithDaemon(hc *http.Client, baseURL, projDir string) []serviceCandidate {
	base := serviceCandidates(projDir)
	resolved, ok, stale := resolveIndexedCandidateCached(hc, baseURL, projDir)
	if !ok {
		return base
	}
	// DG-11: mark this candidate as daemon-verified BEFORE merging — the
	// only caller that stamps this flag, so a test constructing its own
	// serviceCandidate literals (TestMergeCandidatesNeverCollapses) or
	// calling resolveFromServices directly never sees it set implicitly.
	//
	// finding [3]/DG-12: verified must mean a genuinely FRESH daemon
	// confirmation, never a stale-served replay (up to identCacheStaleServeMax
	// old, and possibly wrong after a remap/rename/service removal in the
	// meantime) — stamping a stale answer as verified let
	// renderPostEdit's DG-11 short-circuit (`if cand.verified { break }`)
	// lock onto a since-invalidated identity instead of trying the next,
	// possibly-correct candidate: exactly the O15-class regression DG-11
	// exists to prevent, reintroduced through this client-side cache.
	resolved.verified = !stale
	return mergeCandidates(resolved, base)
}

// mergeCandidates prepends the daemon-resolved identity and drops only EXACT
// (name AND root) duplicates. Dedup must never shrink the fallback list on a
// name collision alone (O1 fix c): when the resolved answer is wrong — a
// stale twin claiming the same name at a different root — the basename
// candidates must survive so injection degrades instead of dying outright.
func mergeCandidates(resolved serviceCandidate, base []serviceCandidate) []serviceCandidate {
	out := []serviceCandidate{resolved}
	for _, c := range base {
		if strings.EqualFold(c.name, resolved.name) && c.root == resolved.root {
			continue
		}
		out = append(out, c)
	}
	return out
}
