package testlayer

import (
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

// Classify reports which test layer filePath (a repo-relative file path)
// belongs to, given buildTags already extracted from its `//go:build` (or
// legacy `// +build`) constraint line — Classify does no file I/O and
// parses no build directive itself, so a caller with a different tag
// source (a pre-parsed AST, an already-cached extraction) can call it
// directly. Uses DefaultLayers(); ClassifyWith takes an explicit LayerSet
// for a caller that loaded a repo-specific config via LoadLayers.
//
// Defaults, first match wins, in this order: a declared layer's
// build_tags, then its dirs, then its globs; a test file matching none of
// those is "unit"; a path that is not a test file is "" (not a test at all
// — coverage/review callers use "" to mean "no layer classification
// applies"). A test file is repofacts' convention (RF-2): Go's _test.go
// and every other ecosystem's pack pattern (test_*.py, *.test.ts,
// FooTest.java ...), never one under testdata or vendor.
func Classify(filePath string, buildTags []string) string {
	return ClassifyWith(DefaultLayers(), filePath, buildTags)
}

// ClassifyWith is Classify against an explicit LayerSet (typically loaded
// via LoadLayers) instead of the built-in DefaultLayers().
func ClassifyWith(spec LayerSet, filePath string, buildTags []string) string {
	slashPath := filepath.ToSlash(filePath)
	if !repofacts.IsTest(slashPath) {
		return ""
	}
	tagSet := make(map[string]bool, len(buildTags))
	for _, t := range buildTags {
		tagSet[t] = true
	}
	for _, l := range spec.Layers {
		for _, t := range l.BuildTags {
			if tagSet[t] {
				return l.Name
			}
		}
		for _, d := range l.Dirs {
			if dirMatches(slashPath, d) {
				return l.Name
			}
		}
		for _, g := range l.Globs {
			if pathglob.Match(g, slashPath) {
				return l.Name
			}
		}
	}
	return "unit"
}

// dirMatches reports whether d appears as a whole path SEGMENT of
// slashPath (e.g. dir "e2e" matches "test/e2e/foo_test.go" but not
// "test/e2eish/foo_test.go").
func dirMatches(slashPath, d string) bool {
	d = strings.Trim(d, "/")
	if d == "" {
		return false
	}
	for _, seg := range strings.Split(slashPath, "/") {
		if seg == d {
			return true
		}
	}
	return false
}
