// Package main is the first of two `package main` binaries in the reachfix
// fixture (PC-32, docs/plans/proof-credibility.md; O21). Both declare a
// `func run`; only this one is reached by a test. Before symbol keys carried
// the package directory, the two collapsed to one node in callgraph.Build
// and whichever was ingested last decided the verdict for both.
package main

func main() {
	run()
}

func run() {
	helperA()
}

func helperA() {}
