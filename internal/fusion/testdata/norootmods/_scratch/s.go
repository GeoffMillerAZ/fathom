// Package scratch sits in a directory Go ignores ("_" prefix).
package scratch

// S is never loaded.
func S() int { return s() }

func s() int { return 0 }
