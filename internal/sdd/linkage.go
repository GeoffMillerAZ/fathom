// Package sdd provides spec-driven design linkage analysis for sextant.
// It recognises spec ID tokens (e.g. [US-042], [EP-013], [FR-7], [NFR-1]),
// extracts references from source text, and builds a coverage matrix that
// classifies which specs are implemented, which are uncovered, and which
// referenced IDs are not in the declared spec set (orphans).
//
// All operations are pure, deterministic, and require no external I/O.
package sdd

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// defaultPrefixes is the set of spec-ID prefixes matched by ExtractRefs when no
// explicit prefix list is provided.
var defaultPrefixes = []string{"US", "EP", "FR", "NFR"}

// buildPattern compiles a regexp that matches bracketed spec IDs whose prefix is
// one of the supplied prefixes.  Each prefix must consist of two or more
// uppercase ASCII letters; prefixes that are shorter than two characters are
// silently dropped to honour the "2+ uppercase letters" design rule.
//
// Example for prefixes ["US","EP"]:
//
//	\[(?:US|EP)-\d+\]
func buildPattern(prefixes []string) *regexp.Regexp {
	if len(prefixes) == 0 {
		// Nothing can match an empty alternation — use a pattern that never matches.
		return regexp.MustCompile(`\[\x00\]`)
	}
	// Filter to prefixes that are at least 2 characters long.
	valid := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		if len(p) >= 2 {
			valid = append(valid, regexp.QuoteMeta(p))
		}
	}
	if len(valid) == 0 {
		return regexp.MustCompile(`\[\x00\]`)
	}
	// Sort for deterministic pattern text (aids caching and readability).
	sort.Strings(valid)
	pat := fmt.Sprintf(`\[(?:%s)-\d+\]`, strings.Join(valid, "|"))
	return regexp.MustCompile(pat)
}

// defaultPattern is the compiled default-prefix pattern, built once at init.
var defaultPattern = buildPattern(defaultPrefixes)

// ExtractRefs returns all spec IDs referenced in text using the default prefix
// set (US, EP, FR, NFR).  The result is sorted and deduplicated.
func ExtractRefs(text string) []string {
	return extractWithPattern(defaultPattern, text)
}

// ExtractRefsWithPrefixes returns all spec IDs referenced in text whose prefix
// is one of the supplied prefixes.  An empty prefix list returns nil.  The
// result is sorted and deduplicated.
func ExtractRefsWithPrefixes(text string, prefixes []string) []string {
	return extractWithPattern(buildPattern(prefixes), text)
}

// extractWithPattern is the shared extraction implementation.
func extractWithPattern(pat *regexp.Regexp, text string) []string {
	raw := pat.FindAllString(text, -1)
	if len(raw) == 0 {
		return nil
	}
	// Deduplicate via a set then sort.
	seen := make(map[string]struct{}, len(raw))
	for _, id := range raw {
		seen[id] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Link records a single relationship: one spec ID implemented by one symbol in
// one file.  Multiple Links may share the same Spec (many symbols implement it)
// or the same Symbol (one symbol implements many specs).
type Link struct {
	Spec   string // e.g. "[US-042]"
	Symbol string // e.g. "HandleLogin"
	File   string // e.g. "internal/auth/login.go"
}

// Matrix is the computed linkage matrix.  It is immutable after construction
// via BuildMatrix; all methods are safe for concurrent reads.
type Matrix struct {
	// specToSymbols maps spec ID → sorted slice of symbols implementing it.
	specToSymbols map[string][]string
	// specSet is the declared spec universe (for Uncovered / Orphans lookups).
	specSet map[string]struct{}
	// referencedSpecs is the set of all spec IDs seen in codeRefs values.
	referencedSpecs map[string]struct{}
}

// BuildMatrix constructs a coverage Matrix from:
//   - specs: the declared spec universe (e.g. from a backlog or requirements doc).
//   - codeRefs: a mapping of symbol/file identifier → spec IDs that it references.
//     The key may be a symbol name, a "symbol@file" key, or any stable string.
//
// BuildMatrix is deterministic: identical inputs always produce identical output
// regardless of Go map iteration order.
func BuildMatrix(specs []string, codeRefs map[string][]string) Matrix {
	specSet := make(map[string]struct{}, len(specs))
	for _, s := range specs {
		specSet[s] = struct{}{}
	}

	// Collect all spec IDs referenced by code and build specToSymbols.
	specToSymbols := make(map[string][]string)
	referencedSpecs := make(map[string]struct{})

	// Iterate codeRefs in a deterministic order: sort keys first.
	symbols := make([]string, 0, len(codeRefs))
	for sym := range codeRefs {
		symbols = append(symbols, sym)
	}
	sort.Strings(symbols)

	for _, sym := range symbols {
		for _, specID := range codeRefs[sym] {
			referencedSpecs[specID] = struct{}{}
			specToSymbols[specID] = append(specToSymbols[specID], sym)
		}
	}

	// Sort each symbol list so Coverage output is deterministic.
	for specID := range specToSymbols {
		sort.Strings(specToSymbols[specID])
	}

	return Matrix{
		specToSymbols:   specToSymbols,
		specSet:         specSet,
		referencedSpecs: referencedSpecs,
	}
}

// Coverage returns the sorted list of symbols (or file references) that
// implement the given spec ID.  Returns nil if the spec is unknown or
// uncovered.
func (m Matrix) Coverage(spec string) []string {
	syms := m.specToSymbols[spec]
	if len(syms) == 0 {
		return nil
	}
	// Return a copy to prevent callers from mutating internal state.
	out := make([]string, len(syms))
	copy(out, syms)
	return out
}

// Uncovered returns the sorted list of spec IDs (from specs) that have no
// implementing symbols in codeRefs.
func (m Matrix) Uncovered(specs []string) []string {
	var out []string
	seen := make(map[string]bool, len(specs))
	for _, s := range specs {
		if seen[s] {
			continue
		}
		seen[s] = true
		if len(m.specToSymbols[s]) == 0 {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// Orphans returns the sorted list of spec IDs that are referenced in codeRefs
// but are not in the declared spec universe used to build this matrix.
func (m Matrix) Orphans(codeRefs map[string][]string) []string {
	// Collect all referenced spec IDs from the supplied codeRefs, then filter
	// to those not in m.specSet.  We re-derive from codeRefs (rather than using
	// m.referencedSpecs) so that callers can pass a different codeRefs without
	// surprising results, but for the common case they match.
	refSet := make(map[string]struct{})
	for _, specIDs := range codeRefs {
		for _, id := range specIDs {
			refSet[id] = struct{}{}
		}
	}

	var out []string
	for id := range refSet {
		if _, declared := m.specSet[id]; !declared {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// WhatImplements returns the sorted list of symbols that implement the given
// spec ID.  It is a convenience wrapper around Matrix.Coverage.
func WhatImplements(m Matrix, spec string) []string {
	return m.Coverage(spec)
}

// SpecStatus returns "covered" if the spec ID has at least one implementing
// symbol, and "uncovered" otherwise (including unknown spec IDs).
func SpecStatus(m Matrix, spec string) string {
	if len(m.Coverage(spec)) > 0 {
		return "covered"
	}
	return "uncovered"
}
