//go:build linux

package main

// rss_linux.go — DG-16's Linux leg: /proc/self/status's VmRSS line is the
// kernel's own live resident-set-size accounting (what `top`'s RES column
// reads), reported in kB. No cgo, no syscall — a plain text read, so this
// file carries no build constraint beyond GOOS.

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// processResidentMemoryBytes reads VmRSS from /proc/self/status. ok is
// false only when the file could not be read or parsed; never a
// fabricated zero.
func processResidentMemoryBytes() (int64, bool) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		// "VmRSS:", "<N>", "kB"
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

// processPhysFootprintBytes has no Linux equivalent — VmRSS already IS the
// figure Linux's own `top`/`ps` report, so there is no separate
// "footprint" concept to distinguish it from (darwin's ri_phys_footprint
// additionally accounts for compressed pages; Linux's RSS accounting has
// no analogous compressed-memory carve-out to name separately). ok is
// always false here — /metrics omits the line entirely rather than
// printing a duplicate of process_resident_memory_bytes under a different
// name.
func processPhysFootprintBytes() (int64, bool) {
	return 0, false
}
