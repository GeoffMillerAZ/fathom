//go:build darwin && cgo

package main

// rss_darwin.go — DG-16 (diagnostics sweep), OWNER AMENDMENT: the plan's
// original proposal was getrusage's ru_maxrss, but Maxrss is a PEAK
// resident size that never decreases for the life of the process — this
// program is a 24/7 daemon, so exposing that as "current RSS" would name
// last month's worst moment as today's number forever. proc_pid_rusage
// (RUSAGE_INFO_V2) reports LIVE occupancy instead: ri_resident_size is the
// same figure `top`'s RES column and Activity Monitor's "Memory" column
// read for this process, and ri_phys_footprint is Activity Monitor's own
// "Memory" figure on modern macOS (it accounts for compressed pages the
// way ri_resident_size does not) — both are exported so a reader can
// compare against whichever tool they already trust.

/*
#include <libproc.h>
#include <sys/resource.h>
#include <string.h>

static int fathom_proc_rusage_v2(pid_t pid, uint64_t *resident_size, uint64_t *phys_footprint) {
	struct rusage_info_v2 info;
	memset(&info, 0, sizeof(info));
	int rc = proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&info);
	if (rc != 0) {
		return rc;
	}
	*resident_size = info.ri_resident_size;
	*phys_footprint = info.ri_phys_footprint;
	return 0;
}
*/
import "C"

import "os"

// procRusageV2 is the one cgo call both exported readers share, so a
// single proc_pid_rusage syscall answers both — cheap enough to still call
// twice per /metrics scrape (each reader below does its own), but kept as
// one function so the C-side field mapping exists in exactly one place.
func procRusageV2() (residentSize, physFootprint uint64, ok bool) {
	var res, phys C.uint64_t
	if rc := C.fathom_proc_rusage_v2(C.pid_t(os.Getpid()), &res, &phys); rc != 0 {
		return 0, 0, false
	}
	return uint64(res), uint64(phys), true
}

// processResidentMemoryBytes returns this process's current resident set
// size (ri_resident_size) — proc_pid_rusage(RUSAGE_INFO_V2)'s answer to
// "how much of this process is in RAM right now," the same figure `top`'s
// RES column shows. ok is false only when the syscall itself failed; never
// a fabricated zero.
func processResidentMemoryBytes() (int64, bool) {
	res, _, ok := procRusageV2()
	if !ok {
		return 0, false
	}
	return int64(res), true
}

// processPhysFootprintBytes returns ri_phys_footprint — the figure
// Activity Monitor's own "Memory" column reads on modern macOS, which
// additionally accounts for compressed pages ri_resident_size does not.
// darwin-only: Linux has no equivalent single-number "footprint" concept
// distinct from RSS.
func processPhysFootprintBytes() (int64, bool) {
	_, phys, ok := procRusageV2()
	if !ok {
		return 0, false
	}
	return int64(phys), true
}
