// Package guardsm1 holds the guards-m1 corpus's violation-probe proof
// (TestGuardsM1ViolationProbesFireAndClear, RC-50) and nothing else: it has
// no production code, only tests.
//
// It sits beside internal/scenariobench, not inside it, because `go test`
// overlaps packages, never the tests of one package: the probe runs the
// corpus's own detector commands (`go test`, `go build`) against a `git
// archive HEAD` copy of the repo for each of its six tasks, and was the
// longest single test in internal/scenariobench, running serially with the
// package's other tests.
// As its own package it is a second test binary that runs while
// internal/scenariobench's tests do. It reaches only scenariobench's exported
// corpus types, so nothing was exported to make the move.
package guardsm1
