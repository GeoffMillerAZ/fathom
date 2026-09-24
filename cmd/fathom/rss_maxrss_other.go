//go:build !darwin && !linux

package main

// rss_maxrss_other.go — syscall.Rusage/RUSAGE_SELF's shape is unix-specific;
// any other GOOS gets no peak gauge rather than a build failure.
func processMaxResidentMemoryBytes() (int64, bool) { return 0, false }
