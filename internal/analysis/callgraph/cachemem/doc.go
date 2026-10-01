// Package cachemem holds the call-graph memo's heap-leak guard (O108,
// DF-6 in docs/plans/daemon-footprint.md): the two tests that churn
// hundreds of 10k-node graphs through callgraph.CachedCallGraph and read
// process heap_inuse. It has no production code, only tests.
//
// They sit beside internal/analysis/callgraph, not inside it, because
// `go test` overlaps packages, never the tests of one package: they were
// the longest tests in callgraph, running serially with its other tests.
// As their own package they run in a second test binary while callgraph's
// tests do, and in their own process, so no sibling test's allocations
// reach the heap they measure. They reach only exported API
// (NewCachedCallGraph, Graph, Stats, Build), so nothing was exported to
// make the move. Both read process heap: never t.Parallel them.
//
// The test file dot-imports callgraph, so each test body is token for
// token the one that moved: fathom's integrity plane
// (internal/analysis/integrity) recognizes a moved test by its normalized
// body, and a qualifier added in front of every callgraph name would read
// as a deleted test.
package cachemem
