// Package fakenested is a fixture nested-module package for the
// doc-coverage test — it has its own go.mod and must be skipped.
package fakenested

func Undocumented() int {
	return 1
}
