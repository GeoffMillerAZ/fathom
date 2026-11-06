//go:build !race

package iacfacts

// raceSlowdown scales the time bounds of this package's tests (race_test.go).
const raceSlowdown = 1
