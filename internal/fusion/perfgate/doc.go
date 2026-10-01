// Package perfgate holds internal/fusion's whole-repo typed-edge performance
// gate (TestTypedEdges_PerformanceGate, CT-5 in docs/plans/callgraph-types.md)
// and nothing else: it has no production code, only tests.
//
// It sits beside internal/fusion, not inside it, because `go test` overlaps
// packages, never the tests of one package: the gate loads the whole module
// twice and was the longest single test in internal/fusion, running serially
// with the package's other tests. As its own package it is a second test
// binary that runs while internal/fusion's tests do. It reaches only exported
// API (fusion.New, fusion.EnrichTypedEdges), so nothing was exported to make
// the move.
package perfgate
