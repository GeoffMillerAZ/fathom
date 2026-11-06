//go:build race

package hclbound

// raceSlowdown scales the time bounds of this package's tests: the race detector costs 2 to 20 times the CPU, and
// the Linux CI runner under -race read about 5 times slower than a developer machine.
const raceSlowdown = 10
