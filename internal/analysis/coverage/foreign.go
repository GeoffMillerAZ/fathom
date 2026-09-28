package coverage

// foreign.go — the one block model T3 (docs/design/test-intelligence.md
// §3 T3, ADR-051 decision 2) normalizes every non-Go coverage report
// into, and the format parsers that fill it: ParseLcov (TQ-10),
// ParseCoberturaJaCoCo (TQ-11), ParseCoveragePyIstanbul (TQ-12),
// ParseLLVMCov (TQ-13). Foreign formats report at LINE granularity (no
// column-precise block boundaries the way `go test -coverprofile` does),
// so ForeignBlock drops StartCol/EndCol and speaks in "statements"
// (countable units — one line, for every format this package parses
// today) and "covered" rather than Block's single NumStmt/Count pair.
//
// Kept pure and deterministic like every internal/analysis/* package: no
// I/O here — each parser takes an io.Reader and returns a plain slice, in
// the order its blocks appeared in the report. The CLI (cmd/fathom's
// cmdCoverageIngest, TQ-9) owns the file open, size cap and streaming.

// ForeignBlock is one coverage block from a non-Go coverage report,
// already normalized to the shared shape every T3 parser produces: a
// source span (File + an inclusive [StartLine, EndLine] range — equal for
// every parser here, since none of the six formats reports a multi-line
// block), how many statements it counts (Statements) and how many of them
// ran (Covered), and — only for a format that itself names which test
// exercised the block — the covering test IDs. Tests is nil for every
// parser in this package today: none of lcov, Cobertura, JaCoCo,
// coverage.py, Istanbul or the llvm-cov JSON export attributes a line to
// a specific test in its standard form. lcov's optional `TN:` record name
// is the one partial exception (ParseLcov's own doc comment).
type ForeignBlock struct {
	// File is the path exactly as the report wrote it — never resolved
	// against the indexed repository here; joinForeignBlocks (TQ-14) does
	// that resolution the same way joinBlocks resolves a Go Block's
	// ImportPath, by suffix match against the repo-relative symbol paths
	// it is handed.
	File       string
	StartLine  int
	EndLine    int
	Statements int
	Covered    int
	Tests      []string
}
