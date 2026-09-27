//go:build darwin && cgo

package main

// host_darwin.go — the governor's macOS host reader (design §2; RT-32,
// RT-161). One cgo call per sample reads this process's phys_footprint and
// disk bytes written (proc_pid_rusage), walks the child-process tree
// (proc_listchildpids — `go list -export`, git and rg run as children and
// their memory is not in the daemon's phys_footprint) summing each child's
// phys_footprint, and reads kern.memorystatus_vm_pressure_level,
// vm.swapusage and statfs of the store's, TMPDIR's and the swap volume. All
// in C over stack buffers and C strings made once: no Go allocation per
// sample.

/*
#include <libproc.h>
#include <signal.h>
#include <stdint.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/resource.h>
#include <sys/sysctl.h>
#include <unistd.h>

#define FATHOM_MAX_TREE 512

typedef struct {
	int64_t footprint, disk_written;
	int64_t children, child_count;
	int64_t pressure;
	int64_t swap_used, swap_total;
	int64_t store_free, tmp_free, swap_vol_free;
} fathom_host_reading;

static int fathom_rusage(pid_t pid, uint64_t *phys, uint64_t *written) {
	struct rusage_info_v2 info;
	memset(&info, 0, sizeof(info));
	if (proc_pid_rusage(pid, RUSAGE_INFO_V2, (rusage_info_t *)&info) != 0) return -1;
	*phys = info.ri_phys_footprint;
	if (written) *written = info.ri_diskio_byteswritten;
	return 0;
}

// fathom_tree collects root's descendants breadth-first into out (at most
// max); returns the count, or -1 when the first listing failed.
static int fathom_tree(pid_t root, pid_t *out, int max) {
	pid_t buf[FATHOM_MAX_TREE];
	int n = 0, head = 0;
	int got = proc_listchildpids(root, buf, sizeof(buf));
	if (got < 0) return -1;
	for (int i = 0; i < got && n < max; i++) out[n++] = buf[i];
	while (head < n) {
		pid_t p = out[head++];
		got = proc_listchildpids(p, buf, sizeof(buf));
		for (int i = 0; got > 0 && i < got && n < max; i++) out[n++] = buf[i];
	}
	return n;
}

static int64_t fathom_children(pid_t root, int64_t *count) {
	pid_t pids[FATHOM_MAX_TREE];
	int n = fathom_tree(root, pids, FATHOM_MAX_TREE);
	if (n < 0) { *count = -1; return -1; }
	int64_t sum = 0, c = 0;
	for (int i = 0; i < n; i++) {
		uint64_t phys = 0;
		if (fathom_rusage(pids[i], &phys, NULL) == 0) { sum += (int64_t)phys; c++; }
	}
	*count = c;
	return sum;
}

// fathom_kill_tree SIGKILLs root's descendants, deepest first; returns how
// many were signalled.
static int fathom_kill_tree(pid_t root) {
	pid_t pids[FATHOM_MAX_TREE];
	int n = fathom_tree(root, pids, FATHOM_MAX_TREE);
	int killed = 0;
	for (int i = n - 1; i >= 0; i--) {
		if (kill(pids[i], SIGKILL) == 0) killed++;
	}
	return killed;
}

static int64_t fathom_free(const char *path) {
	if (path == NULL || path[0] == 0) return -1;
	struct statfs st;
	if (statfs(path, &st) != 0) return -1;
	return (int64_t)st.f_bavail * (int64_t)st.f_bsize;
}

static void fathom_read_host(pid_t self, const char *store, const char *tmp, const char *swapvol, fathom_host_reading *r) {
	uint64_t phys = 0, written = 0;
	if (fathom_rusage(self, &phys, &written) == 0) {
		r->footprint = (int64_t)phys;
		r->disk_written = (int64_t)written;
	} else {
		r->footprint = -1;
		r->disk_written = -1;
	}
	r->children = fathom_children(self, &r->child_count);

	int level = 0;
	size_t sz = sizeof(level);
	if (sysctlbyname("kern.memorystatus_vm_pressure_level", &level, &sz, NULL, 0) == 0) r->pressure = level;
	else r->pressure = 0;

	struct xsw_usage swap;
	sz = sizeof(swap);
	if (sysctlbyname("vm.swapusage", &swap, &sz, NULL, 0) == 0) {
		r->swap_used = (int64_t)swap.xsu_used;
		r->swap_total = (int64_t)swap.xsu_total;
	} else {
		r->swap_used = -1;
		r->swap_total = -1;
	}
	r->store_free = fathom_free(store);
	r->tmp_free = fathom_free(tmp);
	r->swap_vol_free = fathom_free(swapvol);
}

static int64_t fathom_memsize(void) {
	int64_t mem = 0;
	size_t sz = sizeof(mem);
	if (sysctlbyname("hw.memsize", &mem, &sz, NULL, 0) != 0) return 0;
	return mem;
}
*/
import "C"

import (
	"os"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// darwinSwapVolume is where macOS keeps its swap files.
const darwinSwapVolume = "/System/Volumes/VM"

// darwinHost is the macOS tenancy.Host. Its C strings are made once and
// live for the process (a daemon has one).
type darwinHost struct {
	self                C.pid_t
	store, tmp, swapVol *C.char
	reading             C.fathom_host_reading
}

// newTenancyHost returns the platform's host reader; storeDir is the
// store's directory (its volume's free space is sampled).
func newTenancyHost(storeDir string) tenancy.Host {
	return &darwinHost{
		self:    C.pid_t(os.Getpid()),
		store:   C.CString(storeDir),
		tmp:     C.CString(os.TempDir()),
		swapVol: C.CString(darwinSwapVolume),
	}
}

// ReadHost fills the host half of s with no Go allocation.
func (h *darwinHost) ReadHost(s *tenancy.Sample) {
	C.fathom_read_host(h.self, h.store, h.tmp, h.swapVol, &h.reading)
	r := &h.reading
	s.Footprint = int64(r.footprint)
	s.DiskWritten = int64(r.disk_written)
	s.Children, s.ChildCount = int64(r.children), int(r.child_count)
	if s.ChildCount < 0 {
		s.ChildCount = 0
	}
	switch r.pressure {
	case 1:
		s.Pressure = tenancy.PressureNormal
	case 2:
		s.Pressure = tenancy.PressureWarn
	case 4:
		s.Pressure = tenancy.PressureCritical
	default:
		s.Pressure = tenancy.PressureUnknown
	}
	s.SwapUsed, s.SwapTotal = int64(r.swap_used), int64(r.swap_total)
	s.StoreFree, s.TmpFree, s.SwapVolumeFree = int64(r.store_free), int64(r.tmp_free), int64(r.swap_vol_free)
}

// childTreeFootprint sums the phys_footprint of pid's child-process tree
// (RT-161): bytes, how many processes answered, and ok false when the tree
// could not be listed.
func childTreeFootprint(pid int) (bytes int64, count int, ok bool) {
	var c C.int64_t
	sum := C.fathom_children(C.pid_t(pid), &c)
	if sum < 0 {
		return tenancy.Unknown, 0, false
	}
	return int64(sum), int(c), true
}

// killChildTree SIGKILLs pid's descendants, deepest first (the exit rung
// kills the tree before it exits, §2).
func killChildTree(pid int) int {
	return int(C.fathom_kill_tree(C.pid_t(pid)))
}

// hostRAM is hw.memsize (0 when unknown).
func hostRAM() int64 { return int64(C.fathom_memsize()) }
