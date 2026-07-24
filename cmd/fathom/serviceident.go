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
	return resolveFromServices(fetchServices(hc, baseURL), projDir)
}

// resolveFromServices is resolveIndexedCandidate over an already-fetched
// services map (the session hook has one in hand — no second fetch).
func resolveFromServices(svcs map[string]svcInfo, projDir string) (serviceCandidate, bool) {
	roots := map[string]string{} // resolved path → original root
	for _, cand := range serviceCandidates(projDir) {
		if r, err := filepath.EvalSymlinks(cand.root); err == nil {
			roots[r] = cand.root
		}
	}
	if len(roots) == 0 {
		return serviceCandidate{}, false
	}
	for name, info := range svcs {
		if info.path == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(info.path)
		if err != nil {
			continue
		}
		if root, hit := roots[resolved]; hit {
			return serviceCandidate{name: name, root: root}, true
		}
	}
	return serviceCandidate{}, false
}

// candidatesWithDaemon returns serviceCandidates(projDir) with the
// daemon-resolved identity (when one exists) PREPENDED and deduplicated —
// the shared resolution every hook uses so injection cannot silently die on
// a dir-name/service-ID mismatch again.
func candidatesWithDaemon(hc *http.Client, baseURL, projDir string) []serviceCandidate {
	base := serviceCandidates(projDir)
	resolved, ok := resolveIndexedCandidate(hc, baseURL, projDir)
	if !ok {
		return base
	}
	out := []serviceCandidate{resolved}
	for _, c := range base {
		if !strings.EqualFold(c.name, resolved.name) {
			out = append(out, c)
		}
	}
	return out
}
