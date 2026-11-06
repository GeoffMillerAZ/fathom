//go:build !race

package hclbound

// raceSlowdown scales the time bounds of this package's tests (race_test.go).
const raceSlowdown = 1
