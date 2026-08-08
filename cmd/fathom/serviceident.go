package main

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
)

// serviceCandidate is one (service name, corpus root) identity guess for a
// Claude Code project directory.
type serviceCandidate struct {
	name string
	root string
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
func serviceCandidates(projDir string) []serviceCandidate {
	projDir = strings.TrimRight(projDir, "/")
	out := []serviceCandidate{{name: filepath.Base(projDir), root: projDir}}
	if top := gitToplevel(projDir); top != "" && top != projDir {
		out = append(out, serviceCandidate{name: filepath.Base(top), root: top})
	}
	return out
}

// gitToplevel returns the enclosing git work-tree root for dir, or "" when
// dir is not inside a repo (or git is unavailable).
func gitToplevel(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
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
	svcs, _ := fetchServices(hc, baseURL) // fail-open: resolution has its own fallbacks
	return resolveFromServices(svcs, projDir)
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
	base := serviceCandidates(projDir)
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
	resolved, ok := resolveIndexedCandidateCached(hc, baseURL, projDir)
	if !ok {
		return base
	}
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
