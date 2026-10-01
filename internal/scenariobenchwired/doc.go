// Package scenariobenchwired holds the wired arm's end-to-end hook-survival
// proof (TestRenderArm_WiredKeepsRealFathomInitHooks) and nothing else: it
// has no production code, only tests.
//
// It sits beside internal/scenariobench, not inside it, because `go test`
// overlaps packages, never the tests of one package: the proof builds the
// real fathom binary (scenariobench.BuildFathomBinary, the build a cell
// runs) and runs `fathom init` with it, and was among the longest single
// tests in internal/scenariobench, running serially with the package's other
// tests. As its own package it is a second test binary that runs while
// internal/scenariobench's tests do.
//
// Two choices keep the test body token for token the one that moved, which
// is how fathom's integrity plane (internal/analysis/integrity) recognizes a
// moved test: the test file dot-imports scenariobench, so no qualifier
// appears in front of Arm, RenderArm or BuildFathomBinary; and this package
// is two directories below the repository root, as internal/scenariobench
// is, so the body's relative path to the root is unchanged.
package scenariobenchwired
