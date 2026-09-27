package tenancy

// childwrites.go — RT-162's children half (design §12.4: bytes written by
// "self and children"). rusage carries no children's disk I/O, so the
// sampler reads each live child's own cumulative bytes written
// (proc_pid_rusage per child, the same walk RT-161's footprint uses) and
// ChildWriteTracker turns those per-child counters into one monotonic
// total. What a child writes after the last sample that saw it — or a
// child that lives less than one sample (2 s) — is not seen: the total is a
// floor, and /metrics and the docs say so.

// maxTrackedChildren bounds the per-child table (the host's tree walk
// stops at the same size).
const maxTrackedChildren = 512

// ChildWriteTracker keeps the previous sample's per-child counters. Not safe
// for concurrent use (the sampler owns it). Zero allocation.
type ChildWriteTracker struct {
	pids    [maxTrackedChildren]int32
	written [maxTrackedChildren]int64
	n       int
	total   int64

	nextPIDs    [maxTrackedChildren]int32
	nextWritten [maxTrackedChildren]int64
}

// Observe takes one sample's live children (pids[i] wrote written[i] bytes
// since it started) and returns the running total: a child seen before
// adds its growth, a new child — or a pid whose counter fell, a reused pid
// — adds its whole counter, and a child no longer listed keeps what it was
// seen writing.
func (t *ChildWriteTracker) Observe(pids []int32, written []int64) int64 {
	n := len(pids)
	if len(written) < n {
		n = len(written)
	}
	if n > maxTrackedChildren {
		n = maxTrackedChildren
	}
	for i := 0; i < n; i++ {
		w := written[i]
		if w < 0 {
			w = 0
		}
		delta := w
		for j := 0; j < t.n; j++ {
			if t.pids[j] == pids[i] {
				if w >= t.written[j] {
					delta = w - t.written[j]
				}
				break
			}
		}
		t.total += delta
		t.nextPIDs[i], t.nextWritten[i] = pids[i], w
	}
	t.pids, t.written, t.n = t.nextPIDs, t.nextWritten, n
	return t.total
}
