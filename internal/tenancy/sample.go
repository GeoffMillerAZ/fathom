package tenancy

// sample.go — the host sampler (design §2, RT-32, RT-171). One sample every
// SampleEvery: the process's phys_footprint, its child-process tree, disk
// bytes written, runtime/metrics, the host's memory pressure, swap used and
// volume free space. The OS readers live behind Host (cmd/fathom wires the
// darwin cgo reader in; tests inject a fake), so the policy stays pure. No
// stop-the-world ReadMemStats and no allocation: the runtime half reads
// into a preallocated metrics.Sample slice.

import (
	"runtime/metrics"
	"time"
)

// SampleEvery is the sampler's cadence: the measured runaway grew ~7.5 GB a
// minute, so 2 s is ~250 MB of resolution.
const SampleEvery = 2 * time.Second

// Unknown is the value of a byte reading the host could not take.
const Unknown int64 = -1

// Sample is one reading. A byte field of Unknown (-1) or a PressureUnknown
// is disclosed (Unknowns) and never a rung trigger; an Unknown Footprint
// turns the ladder off.
type Sample struct {
	At   time.Time
	Late time.Duration // how late this sample ran against its tick

	Footprint   int64 // phys_footprint (darwin), RSS elsewhere
	Children    int64 // summed phys_footprint of the child-process tree
	ChildCount  int
	DiskWritten int64 // cumulative bytes written by this process
	// ChildDiskWritten is the running total of bytes the child-process tree
	// was seen writing (ChildWriteTracker: sampled while alive, a floor).
	ChildDiskWritten int64

	GoTotalMinusReleased int64 // /memory/classes/total − /memory/classes/heap/released
	HeapReleased         int64 // /memory/classes/heap/released: heap returned to the OS
	HeapLive             int64 // /gc/heap/live
	GCCPUFraction        float64

	Pressure            PressureLevel
	SwapUsed, SwapTotal int64 // growth over a window is a trigger; used/total never is

	StoreFree, TmpFree, SwapVolumeFree int64 // free bytes on the store's, TMPDIR's and the swap volume
}

// Host reads the host half of a sample into s. Implementations set a
// reading they cannot take to Unknown (or PressureUnknown) and must not
// allocate.
type Host interface {
	ReadHost(s *Sample)
}

// Unknowns names every reading this sample could not take, for /statusz,
// `resources` and doctor. Allocates; call it to render, never per sample.
func (s *Sample) Unknowns() []string {
	var out []string
	if s.Footprint < 0 {
		out = append(out, "footprint unknown (ladder off)")
	}
	if s.Children < 0 {
		out = append(out, "children unknown")
	}
	if s.DiskWritten < 0 {
		out = append(out, "disk written unknown")
	}
	if s.ChildDiskWritten < 0 {
		out = append(out, "children disk written unknown")
	}
	if s.Pressure == PressureUnknown {
		out = append(out, "pressure unknown")
	}
	if s.SwapUsed < 0 {
		out = append(out, "swap unknown")
	}
	if s.SwapVolumeFree < 0 {
		out = append(out, "swap volume free unknown")
	}
	if s.StoreFree < 0 {
		out = append(out, "store volume free unknown")
	}
	if s.TmpFree < 0 {
		out = append(out, "tmp volume free unknown")
	}
	return out
}

// runtime/metrics names the sampler reads.
const (
	mTotal    = "/memory/classes/total:bytes"
	mReleased = "/memory/classes/heap/released:bytes"
	mLive     = "/gc/heap/live:bytes"
	mGCCPU    = "/cpu/classes/gc/total:cpu-seconds"
	mCPU      = "/cpu/classes/total:cpu-seconds"
)

// RuntimeReader reads runtime/metrics into a preallocated slice and keeps
// the previous CPU readings for the GC CPU fraction.
type RuntimeReader struct {
	samples          [5]metrics.Sample
	prevGC, prevCPU  float64
	primed           bool
	haveGC, haveCPUs bool
}

// NewRuntimeReader allocates the metric slots once.
func NewRuntimeReader() *RuntimeReader {
	r := &RuntimeReader{}
	for i, name := range []string{mTotal, mReleased, mLive, mGCCPU, mCPU} {
		r.samples[i].Name = name
	}
	return r
}

// Read fills the runtime half of s.
func (r *RuntimeReader) Read(s *Sample) {
	metrics.Read(r.samples[:])
	u := func(i int) int64 {
		if r.samples[i].Value.Kind() == metrics.KindUint64 {
			return int64(r.samples[i].Value.Uint64())
		}
		return Unknown
	}
	f := func(i int) (float64, bool) {
		if r.samples[i].Value.Kind() == metrics.KindFloat64 {
			return r.samples[i].Value.Float64(), true
		}
		return 0, false
	}
	total, released := u(0), u(1)
	s.HeapReleased = released
	if total >= 0 && released >= 0 {
		s.GoTotalMinusReleased = total - released
	} else {
		s.GoTotalMinusReleased = Unknown
	}
	s.HeapLive = u(2)
	gc, okGC := f(3)
	cpu, okCPU := f(4)
	s.GCCPUFraction = 0
	if okGC && okCPU {
		if r.primed && cpu > r.prevCPU {
			s.GCCPUFraction = (gc - r.prevGC) / (cpu - r.prevCPU)
		}
		r.prevGC, r.prevCPU, r.primed = gc, cpu, true
	}
}

// SampleHost takes one sample into s: the host readings, then the
// runtime's. at and late come from the caller's ticker (injected clock).
// Zero allocation (TestLadder_LateSampleEscalatesAndSamplerZeroAlloc).
func SampleHost(h Host, rt *RuntimeReader, at time.Time, late time.Duration, s *Sample) {
	*s = Sample{
		At: at, Late: late,
		Footprint: Unknown, Children: Unknown, DiskWritten: Unknown, ChildDiskWritten: Unknown, HeapReleased: Unknown,
		SwapUsed: Unknown, SwapTotal: Unknown,
		StoreFree: Unknown, TmpFree: Unknown, SwapVolumeFree: Unknown,
	}
	if h != nil {
		h.ReadHost(s)
	}
	if rt != nil {
		rt.Read(s)
	}
}
