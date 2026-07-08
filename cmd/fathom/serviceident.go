package main

import (
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

// withNameAliases expands a service name with its pre/post-rename twin, so
// stores ingested under either brand era resolve (mirrors the connector's
// legacy-ignore-file treatment).
func withNameAliases(name string) []string {
	switch strings.ToLower(name) {
	case "sextant":
		return []string{name, "fathom"}
	case "fathom":
		return []string{name, "sextant"}
	}
	return []string{name}
}
