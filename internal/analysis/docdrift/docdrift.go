// Package docdrift detects documentation drift: references in Markdown text
// to symbols or file paths that no longer exist in the codebase.
//
// Detection approach:
//   - Inline code spans (`symbol`) whose content matches a known-symbol or
//     known-file pattern are checked against the provided sets.
//   - Fenced code blocks are skipped.
//   - Markdown link targets [text](target) are checked against knownFiles.
//   - Plain words matching a known symbol exactly are NOT checked (to avoid
//     false positives in prose) — only backtick spans and link targets are
//     checked.
//
// The caller supplies:
//   - knownSymbols: set of symbol names that currently exist.
//   - knownFiles:   set of file paths (repo-relative) that currently exist.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package docdrift

import (
	"regexp"
	"sort"
	"strings"
)

// Drift is a single stale reference found in a markdown document.
type Drift struct {
	File string // markdown file name
	Line int    // 1-based line number
	Ref  string // the reference that could not be resolved
}

var (
	// backtickRe matches inline code spans: `content`
	backtickRe = regexp.MustCompile("`([^`]+)`")

	// linkTargetRe matches markdown link targets: [text](target)
	linkTargetRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	// fenceRe matches fenced code block delimiters (``` or ~~~)
	fenceRe = regexp.MustCompile("^\\s*(`{3,}|~{3,})")
)

// Analyze scans the markdown text in content (filename used for Drift.File)
// and returns Drift entries for any inline-code references or link targets
// that are not present in knownSymbols or knownFiles.
// Results are sorted by File, Line, Ref for determinism.
func Analyze(filename, content string, knownSymbols, knownFiles map[string]bool) []Drift {
	lines := strings.Split(content, "\n")
	var drifts []Drift

	inFence := false
	for i, line := range lines {
		lineNum := i + 1

		// Toggle fenced code block state.
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		// Check markdown link targets.
		for _, m := range linkTargetRe.FindAllStringSubmatch(line, -1) {
			target := m[2]
			// Strip anchors from file links.
			targetBase := strings.SplitN(target, "#", 2)[0]
			if targetBase == "" {
				continue
			}
			// Skip external URLs.
			if strings.HasPrefix(targetBase, "http://") || strings.HasPrefix(targetBase, "https://") {
				continue
			}
			if !knownFiles[targetBase] {
				drifts = append(drifts, Drift{File: filename, Line: lineNum, Ref: target})
			}
		}

		// Check inline code spans (backtick) — checked against both symbols
		// and files.
		for _, m := range backtickRe.FindAllStringSubmatch(line, -1) {
			ref := strings.TrimSpace(m[1])
			if ref == "" {
				continue
			}
			// If it looks like a file path, check knownFiles.
			if looksLikePath(ref) {
				if !knownFiles[ref] {
					drifts = append(drifts, Drift{File: filename, Line: lineNum, Ref: ref})
				}
				continue
			}
			// Otherwise check knownSymbols.
			if !knownSymbols[ref] {
				drifts = append(drifts, Drift{File: filename, Line: lineNum, Ref: ref})
			}
		}
	}

	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].File != drifts[j].File {
			return drifts[i].File < drifts[j].File
		}
		if drifts[i].Line != drifts[j].Line {
			return drifts[i].Line < drifts[j].Line
		}
		return drifts[i].Ref < drifts[j].Ref
	})
	return drifts
}

// looksLikePath returns true if ref contains a path separator or file
// extension, suggesting it refers to a file rather than a symbol.
func looksLikePath(ref string) bool {
	return strings.Contains(ref, "/") ||
		strings.Contains(ref, "\\") ||
		strings.Contains(ref, ".go") ||
		strings.Contains(ref, ".md") ||
		strings.Contains(ref, ".yaml") ||
		strings.Contains(ref, ".json") ||
		strings.Contains(ref, ".proto")
}
