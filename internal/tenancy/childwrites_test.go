package tenancy

import "testing"

// TestChildWriteTrackerCountsSampledChildren pins review finding 5 on
// RT-162 (design §12.4: bytes written by "self and children"): rusage has
// no children's disk-I/O field, so the sampler reads each live child's own
// cumulative bytes written and the tracker turns them into one monotonic
// total — a new child counts from zero, a known child counts its growth, a
// child that exited keeps what it was seen writing, and a pid reused by a
// new process (its counter below the old one) counts from zero again. No
// allocation: it runs on the sampler's path.
func TestChildWriteTrackerCountsSampledChildren(t *testing.T) {
	var tr ChildWriteTracker
	steps := []struct {
		pids    []int32
		written []int64
		want    int64
	}{
		{[]int32{10, 11}, []int64{100, 50}, 150},     // two new children
		{[]int32{10, 11}, []int64{400, 50}, 450},     // 10 wrote 300 more
		{[]int32{11, 12}, []int64{70, 5}, 475},       // 10 exited (kept), 11 +20, 12 new
		{[]int32{12, 11}, []int64{5, 3}, 478},        // pid 11 reused (counter fell): counts from zero
		{[]int32{12, 10}, []int64{9, 30}, 512},       // 12 +4; 10 is a new process again
		{nil, nil, 512},                              // no children: total holds
		{[]int32{13}, []int64{1 << 30}, 512 + 1<<30}, // a large new child
	}
	for i, s := range steps {
		if got := tr.Observe(s.pids, s.written); got != s.want {
			t.Fatalf("step %d: total = %d, want %d", i, got, s.want)
		}
	}
	pids, written := []int32{1, 2, 3}, []int64{1, 2, 3}
	if n := testing.AllocsPerRun(100, func() { tr.Observe(pids, written) }); n != 0 {
		t.Fatalf("Observe allocates %v times per call, want 0", n)
	}
}
