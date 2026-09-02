// Package main is the second `package main` binary in the reachfix fixture
// (PC-32). Its `func run` has no test: TestRunA in cmd/a must never be
// credited to it, and it must never steal cmd/a's edges by being ingested
// last.
package main

func main() {
	run()
}

func run() {
	helperB()
}

func helperB() {}
