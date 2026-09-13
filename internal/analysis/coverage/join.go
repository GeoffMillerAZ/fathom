package coverage

import (
	"math"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SymbolCoverage is one symbol's joined statement coverage from a SINGLE
// profile. JoinLayeredSymbols (layered.go) is the multi-profile,
// per-test-layer wrapper most review/CLI callers want; JoinSymbols stays
// the single-profile primitive it is built on.
type SymbolCoverage struct {
	Symbol     string
	File       string
	Line       int
	EndLine    int
	Statements int
	Covered    int
	Percent    float64
	// Tests names the tests that covered this symbol — empty in v1: the
	// standard `go test -coverprofile` format carries no per-test
	// attribution, only aggregate statement hit counts across the whole
	// run. A future per-test profile source can populate this without
	// changing SymbolCoverage's shape.
	Tests []string
}

// coverageEligible reports whether kind is a Go symbol kind that can carry
// executable statements a cover-profile block might fall within — functions
// and methods only; types/vars/consts have no statement body of their own.
func coverageEligible(k core.SourceKind) bool {
	return k == core.KindGoFunc || k == core.KindGoMethod
}

// blockAccum is joinBlocks' per-symbol running total.
type blockAccum struct {
	sym        core.SymbolNode
	statements int
	covered    int
}

// joinBlocks is the block-to-symbol join shared by JoinSymbols (one
// profile) and JoinLayeredSymbols (layered.go; N labeled profiles, and the
// union-across-layers "Any" view) — every symbol/span/resolution rule
// documented on JoinSymbols applies here, since JoinSymbols is now a thin
// wrapper over this function.
func joinBlocks(blocks []Block, symbols []core.SymbolNode) map[string]*blockAccum {
	// fileSyms / repoFiles: the coverage-eligible symbols indexed by their
	// repo-relative file (EndLine synthesized where unknown — see
	// JoinSymbols' doc comment), and the distinct file set — longest path
	// first so a more specific suffix match is preferred over a shorter,
	// coincidental one (e.g. a vendored copy at a shorter relative depth).
	fileSyms := make(map[string][]core.SymbolNode)
	var repoFiles []string
	for _, s := range symbols {
		if !coverageEligible(s.Kind) {
			continue
		}
		if _, ok := fileSyms[s.FilePath]; !ok {
			repoFiles = append(repoFiles, s.FilePath)
		}
		fileSyms[s.FilePath] = append(fileSyms[s.FilePath], s)
	}
	for fp, syms := range fileSyms {
		sort.Slice(syms, func(i, j int) bool { return syms[i].Line < syms[j].Line })
		for i := range syms {
			if syms[i].EndLine > 0 {
				continue
			}
			if i+1 < len(syms) {
				syms[i].EndLine = syms[i+1].Line - 1
			} else {
				syms[i].EndLine = math.MaxInt32
			}
		}
		fileSyms[fp] = syms
	}
	sort.Slice(repoFiles, func(i, j int) bool { return len(repoFiles[i]) > len(repoFiles[j]) })

	resolved := make(map[string]string) // block ImportPath -> repo-relative FilePath ("" = no match)
	resolve := func(importPath string) string {
		if fp, ok := resolved[importPath]; ok {
			return fp
		}
		fp := ""
		for _, rf := range repoFiles {
			if importPath == rf || strings.HasSuffix(importPath, "/"+rf) {
				fp = rf
				break
			}
		}
		resolved[importPath] = fp
		return fp
	}

	byKey := make(map[string]*blockAccum)
	for _, b := range blocks {
		fp := resolve(b.ImportPath)
		if fp == "" {
			continue // no indexed symbol's file matches this block's path — nothing to join it to
		}
		for _, s := range fileSyms[fp] {
			if b.StartLine < s.Line || b.EndLine > s.EndLine {
				continue // block falls outside this symbol's span
			}
			key := s.Key()
			a, ok := byKey[key]
			if !ok {
				a = &blockAccum{sym: s}
				byKey[key] = a
			}
			a.statements += b.NumStmt
			if b.Count > 0 {
				a.covered += b.NumStmt
			}
		}
	}
	return byKey
}

func percentOf(statements, covered int) float64 {
	if statements <= 0 {
		return 0
	}
	return float64(covered) / float64(statements) * 100
}

// JoinSymbols joins profile's statement blocks to symbols: for every
// function/method symbol whose [Line, EndLine] span fully contains a
// profiled block, the block's NumStmt is added to Statements, and — when
// the block's Count > 0 — to Covered too. Percent is Covered/Statements*100
// (0 when Statements is 0, which cannot occur for an emitted entry: a
// symbol with no profiled block inside it is omitted entirely rather than
// reported as a fabricated 0% — JoinSymbols only speaks for symbols the
// profile actually has data for).
//
// EndLine is set by the Go extractor for every func/method symbol (from the
// declaration's own ast.Node.End()); a symbol with a positive EndLine keeps
// it untouched. It can still be 0 (unknown) for a symbol produced by an
// older, unrefreshed index, or by an extractor that does not compute it.
// Rather than making the whole join dead-on-arrival in that case, a symbol
// with EndLine<=0 gets a SYNTHESIZED end line: the line immediately before
// the next coverage-eligible symbol's own Line in the same file (sorted by
// Line), or "the rest of the file" (math.MaxInt32) for the last one —
// exactly the technique internal/extractor/mdextract.go already uses for a
// Markdown heading's own EndLine.
//
// A profile Block's ImportPath is import-path-relative (module-path-
// prefixed); it is resolved to a symbol's repo-relative FilePath by suffix
// match against the symbols slice itself — the join partner already
// carries the ground-truth repo-relative paths, so JoinSymbols needs no
// separate go.mod read to strip the module prefix. Output is sorted by
// (File, Line): deterministic given identical inputs.
//
// For multiple profiles (one per test layer — unit/integration/e2e), use
// JoinLayeredSymbols (layered.go) instead: it calls the SAME underlying
// join once per labeled profile plus once over their union, so "which
// layer(s) cover this symbol" is an assertion grounded in which profile
// actually recorded the coverage, never a guess from a symbol's own name
// or file.
func JoinSymbols(profile Profile, symbols []core.SymbolNode) []SymbolCoverage {
	byKey := joinBlocks(profile.Blocks, symbols)

	out := make([]SymbolCoverage, 0, len(byKey))
	for key, a := range byKey {
		out = append(out, SymbolCoverage{
			Symbol:     key,
			File:       a.sym.FilePath,
			Line:       a.sym.Line,
			EndLine:    a.sym.EndLine,
			Statements: a.statements,
			Covered:    a.covered,
			Percent:    percentOf(a.statements, a.covered),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
