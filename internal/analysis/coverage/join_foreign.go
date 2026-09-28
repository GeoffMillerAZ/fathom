package coverage

// join_foreign.go — joinForeignBlocks (TQ-14, docs/design/test-intelligence.md
// §3 T3): joins ForeignBlock (foreign.go) to indexed symbols. Unlike
// joinBlocks (join.go), which always has Go's AST-derived EndLine to work
// with and synthesizes one for the rare symbol that lacks it, a foreign
// (non-Go) language's symbols may carry NO end-line ranges at all — LB-F3
// is the tags-query engine's own upgrade, not yet universal across every
// extractor — so this join has two granularities: per-symbol where a
// file's function/method symbols all carry a real range, per-file
// (rolled up, named by its own path) where they do not.

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// foreignFunctionLike reports whether k is a function/method-shaped
// SourceKind in any language the extractor set produces, by suffix
// (Func/Method/Fn/Function) — the one naming pattern every language's
// callable Kind constant shares (internal/core/model.go) — rather than an
// enumerated per-language list, so a newly added language needs no edit
// here to join.
func foreignFunctionLike(k core.SourceKind) bool {
	s := string(k)
	return strings.HasSuffix(s, "Func") || strings.HasSuffix(s, "Method") ||
		strings.HasSuffix(s, "Fn") || strings.HasSuffix(s, "Function")
}

// ForeignFileCoverage is one file's rolled-up statement coverage — used
// when joinForeignBlocks falls back to file granularity because the
// language extracting that file's symbols carries no real end-line
// ranges to join against individually.
type ForeignFileCoverage struct {
	File       string
	Statements int
	Covered    int
	Percent    float64
	Tests      []string
}

// ForeignJoinResult is joinForeignBlocks' output: BySymbol for every file
// whose function/method symbols ALL carry a real (unsynthesized) EndLine,
// joined the same way JoinSymbols joins a Go profile; ByFile for every
// other file the blocks touched, named by its own path rather than
// guessed at symbol granularity. Both are sorted by File (then Line for
// BySymbol): deterministic given identical inputs.
type ForeignJoinResult struct {
	BySymbol []SymbolCoverage
	ByFile   []ForeignFileCoverage
}

// joinForeignBlocks joins foreign-format coverage blocks (ParseLcov,
// ParseCoberturaJaCoCo, ParseCoveragePyIstanbul, ParseLLVMCov — TQ-10 to
// TQ-13) to symbols: "Foreign blocks join to symbols by line range where
// the language has L1 ranges (LB-F3), else at file level, named" (design
// doc §3 T3). A file resolves at RANGE granularity only when every
// function/method symbol indexed for it (foreignFunctionLike) carries a
// real EndLine > 0 — never joinBlocks' own end-line SYNTHESIS, which
// would silently misattribute an L1-less language's blocks to the wrong
// function's guessed boundary; a file with zero such symbols, or any
// symbol missing a real range, rolls up to one ForeignFileCoverage entry
// instead.
//
// blocks' File is resolved to a repo-relative symbol FilePath the same
// way joinBlocks resolves a Go Block's ImportPath: an exact match, or a
// "/"-boundary suffix match, against the file paths symbols itself names
// — longest path preferred. A block matching no known file is omitted,
// never guessed.
func joinForeignBlocks(blocks []ForeignBlock, symbols []core.SymbolNode) ForeignJoinResult {
	fileSyms := make(map[string][]core.SymbolNode)
	var repoFiles []string
	for _, s := range symbols {
		if !foreignFunctionLike(s.Kind) {
			continue
		}
		if _, ok := fileSyms[s.FilePath]; !ok {
			repoFiles = append(repoFiles, s.FilePath)
		}
		fileSyms[s.FilePath] = append(fileSyms[s.FilePath], s)
	}
	// A file is range-eligible only when EVERY function/method symbol
	// indexed for it carries a real (unsynthesized) EndLine — a single
	// unranged symbol in that file makes the whole file's boundaries
	// untrustworthy, so it rolls up rather than mixing synthesized and
	// real spans.
	rangeEligible := make(map[string]bool, len(fileSyms))
	for f, syms := range fileSyms {
		sort.Slice(syms, func(i, j int) bool { return syms[i].Line < syms[j].Line })
		fileSyms[f] = syms
		eligible := true
		for _, s := range syms {
			if s.EndLine <= 0 {
				eligible = false
				break
			}
		}
		rangeEligible[f] = eligible
	}
	sort.Slice(repoFiles, func(i, j int) bool { return len(repoFiles[i]) > len(repoFiles[j]) })

	// Every file a symbol names is a candidate resolution target, not
	// only the ones with function-like symbols — a file with, say, only
	// top-level statements in a language with no per-function extraction
	// still needs to resolve so its blocks roll up rather than being
	// silently dropped as unmatched.
	knownFileSet := make(map[string]bool)
	for _, s := range symbols {
		knownFileSet[s.FilePath] = true
	}
	var knownFiles []string
	for f := range knownFileSet {
		knownFiles = append(knownFiles, f)
	}
	sort.Slice(knownFiles, func(i, j int) bool { return len(knownFiles[i]) > len(knownFiles[j]) })

	resolved := make(map[string]string)
	resolve := func(blockFile string) string {
		if fp, ok := resolved[blockFile]; ok {
			return fp
		}
		fp := ""
		for _, rf := range knownFiles {
			if blockFile == rf || strings.HasSuffix(blockFile, "/"+rf) {
				fp = rf
				break
			}
		}
		resolved[blockFile] = fp
		return fp
	}

	type symAccum struct {
		sym        core.SymbolNode
		statements int
		covered    int
		tests      map[string]bool
	}
	type fileAccum struct {
		statements int
		covered    int
		tests      map[string]bool
	}
	symAccums := make(map[string]*symAccum)
	fileAccums := make(map[string]*fileAccum)

	for _, b := range blocks {
		fp := resolve(b.File)
		if fp == "" {
			continue
		}
		if rangeEligible[fp] {
			// A block whose span falls outside every known symbol
			// (module-level code, exactly JoinSymbols' own
			// "unmatched block omitted" convention) is dropped here
			// too, never rolled up to file level — mixing the two
			// granularities for one file would make BySymbol's own
			// totals for that file uninterpretable against ByFile's.
			for _, s := range fileSyms[fp] {
				if b.StartLine < s.Line || b.EndLine > s.EndLine {
					continue
				}
				key := s.Key()
				a, ok := symAccums[key]
				if !ok {
					a = &symAccum{sym: s, tests: map[string]bool{}}
					symAccums[key] = a
				}
				a.statements += b.Statements
				a.covered += b.Covered
				for _, t := range b.Tests {
					a.tests[t] = true
				}
			}
			continue
		}
		fa, ok := fileAccums[fp]
		if !ok {
			fa = &fileAccum{tests: map[string]bool{}}
			fileAccums[fp] = fa
		}
		fa.statements += b.Statements
		fa.covered += b.Covered
		for _, t := range b.Tests {
			fa.tests[t] = true
		}
	}

	var bySymbol []SymbolCoverage
	for key, a := range symAccums {
		bySymbol = append(bySymbol, SymbolCoverage{
			Symbol:     key,
			File:       a.sym.FilePath,
			Line:       a.sym.Line,
			EndLine:    a.sym.EndLine,
			Statements: a.statements,
			Covered:    a.covered,
			Percent:    percentOf(a.statements, a.covered),
			Tests:      sortedTestKeys(a.tests),
		})
	}
	sort.Slice(bySymbol, func(i, j int) bool {
		if bySymbol[i].File != bySymbol[j].File {
			return bySymbol[i].File < bySymbol[j].File
		}
		return bySymbol[i].Line < bySymbol[j].Line
	})

	var byFile []ForeignFileCoverage
	for file, fa := range fileAccums {
		byFile = append(byFile, ForeignFileCoverage{
			File:       file,
			Statements: fa.statements,
			Covered:    fa.covered,
			Percent:    percentOf(fa.statements, fa.covered),
			Tests:      sortedTestKeys(fa.tests),
		})
	}
	sort.Slice(byFile, func(i, j int) bool { return byFile[i].File < byFile[j].File })

	return ForeignJoinResult{BySymbol: bySymbol, ByFile: byFile}
}

func sortedTestKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
