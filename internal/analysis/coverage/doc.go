// Package coverage joins a parsed Go cover profile (ParseProfile, the
// `go test -coverprofile` file format) to indexed symbols (JoinSymbols),
// producing per-symbol statement coverage — RC-37/RC-38,
// docs/vision/28-receipts.md §5.
//
// The boundary this package keeps (docs/vision/28-receipts.md §5): fathom
// ASSERTS a fact — this symbol has N of M statements executed by SOME test
// run — never a judgment about whether that coverage is adequate, which is
// the review kit's job, not fathom's. Pure and deterministic, mirroring
// every other internal/analysis/* package: no I/O (the caller reads the
// profile file and hands ParseProfile an io.Reader), sorted output.
package coverage
