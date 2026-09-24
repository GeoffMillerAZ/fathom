//go:build darwin || linux

package main

// rss_maxrss_unix.go — DG-16's optional peak gauge: getrusage's ru_maxrss
// IS a genuine peak (it never decreases for the process's life), which is
// exactly why the owner amendment forbids exposing it as
// process_resident_memory_bytes ("current RSS"). Named honestly instead —
// process_max_resident_memory_bytes — it is legitimately useful ALONGSIDE
// the current-RSS gauges above: the existing go_memstats_heap_sys_bytes
// line is a Go-runtime-level peak that never decreases either, and O108's
// whole finding was the divergence between a peak and a live reading.
// Having an OS-level peak next to an OS-level live reading lets a reader
// see that divergence without cross-referencing two different measurement
// layers (Go's allocator vs the kernel's).
//
// syscall.Getrusage/syscall.Rusage are available on both darwin and linux
// without cgo; the unit differs (bytes on darwin, kilobytes on linux) and
// is normalized to bytes here.

import (
	"runtime"
	"syscall"
)

// processMaxResidentMemoryBytes returns this process's peak resident set
// size since it started (ru_maxrss), normalized to bytes. ok is false only
// when the syscall itself failed; never a fabricated zero.
func processMaxResidentMemoryBytes() (int64, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	maxrss := int64(ru.Maxrss)
	if runtime.GOOS == "linux" {
		maxrss *= 1024 // linux reports ru_maxrss in kB; darwin already reports bytes
	}
	return maxrss, true
}
