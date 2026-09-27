package tenancy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRing_FixedSizeAndWrap (RT-47, design §13.1): the ring is one
// fixed-size file — a minute sample written a day and a half long, plus
// more transitions than it holds, never grows it — and a reader gets back
// the last 24 h of minutes and the newest transitions, in time order, with
// every field it was given. A torn slot is skipped and counted, never
// rendered as a zero sample (I38).
func TestRing_FixedSizeAndWrap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.ring")
	r, err := OpenRing(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != RingFileSize {
		t.Fatalf("a fresh ring is %d B, want the fixed %d B", fi.Size(), RingFileSize)
	}
	if RingFileSize > 300*1024 {
		t.Fatalf("ring is %d B; §13.1 sizes it at a few hundred KB", RingFileSize)
	}

	base := time.Date(2026, 9, 26, 0, 0, 30, 0, time.UTC)
	const minutes = 36 * 60
	for i := 0; i < minutes; i++ {
		rec := RingRecord{
			At: base.Add(time.Duration(i) * time.Minute), Level: LevelNormal,
			Footprint: int64(i) * MiB, Children: 7 * MiB, HeapLive: 3 * MiB, GoTotalMinusReleased: 5 * MiB,
			GCCPUFraction: 0.01, Pressure: PressureNormal, SwapUsed: 3 * GiB, StoreFree: 40 * GiB,
			TmpFree: 41 * GiB, SwapVolumeFree: 42 * GiB, DiskWritten: int64(i) * 1000,
			OldestTool: "find_symbol", OldestClass: ClassPoint, OldestAge: 1500 * time.Millisecond,
			Abandoned: 2,
		}
		rec.InFlight[ClassPoint] = 3
		rec.InFlight[ClassTree] = 1
		if err := r.WriteRingSample(rec); err != nil {
			t.Fatal(err)
		}
	}
	const transitions = RingTransitionSlots + 40
	for i := 0; i < transitions; i++ {
		lv := LevelShed
		if i%2 == 1 {
			lv = LevelNormal
		}
		at := base.Add(time.Duration(i)*time.Minute + 17*time.Second)
		if err := r.WriteRingSample(RingRecord{At: at, Transition: true, Level: lv, Footprint: 3 * GiB, SafeMode: i == transitions-1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if fi, _ = os.Stat(path); fi.Size() != RingFileSize {
		t.Fatalf("after %d minutes and %d transitions the ring is %d B, want the fixed %d B", minutes, transitions, fi.Size(), RingFileSize)
	}

	rd, err := ReadRingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if rd.Torn != 0 {
		t.Fatalf("clean ring read %d torn slots", rd.Torn)
	}
	var mins, trans []RingRecord
	for i, rec := range rd.Records {
		if i > 0 && rec.At.Before(rd.Records[i-1].At) {
			t.Fatalf("records out of time order at %d: %s before %s", i, rec.At, rd.Records[i-1].At)
		}
		if rec.Transition {
			trans = append(trans, rec)
		} else {
			mins = append(mins, rec)
		}
	}
	if len(mins) != RingMinuteSlots {
		t.Fatalf("got %d minute samples, want the last %d (24 h)", len(mins), RingMinuteSlots)
	}
	if first, want := mins[0].At, base.Add(time.Duration(minutes-RingMinuteSlots)*time.Minute); !first.Equal(want) {
		t.Fatalf("oldest minute sample %s, want %s (the last 24 h)", first, want)
	}
	if len(trans) != RingTransitionSlots {
		t.Fatalf("got %d transitions, want the newest %d", len(trans), RingTransitionSlots)
	}
	if !trans[len(trans)-1].SafeMode || trans[len(trans)-1].Level != LevelNormal {
		t.Fatalf("newest transition lost its fields: %+v", trans[len(trans)-1])
	}
	last := mins[len(mins)-1]
	want := RingRecord{
		At: base.Add((minutes - 1) * time.Minute), Level: LevelNormal,
		Footprint: (minutes - 1) * MiB, Children: 7 * MiB, HeapLive: 3 * MiB, GoTotalMinusReleased: 5 * MiB,
		GCCPUFraction: 0.01, Pressure: PressureNormal, SwapUsed: 3 * GiB, StoreFree: 40 * GiB,
		TmpFree: 41 * GiB, SwapVolumeFree: 42 * GiB, DiskWritten: (minutes - 1) * 1000,
		OldestTool: "find_symbol", OldestClass: ClassPoint, OldestAge: 1500 * time.Millisecond,
		Abandoned: 2,
	}
	want.InFlight[ClassPoint] = 3
	want.InFlight[ClassTree] = 1
	if last != want {
		t.Fatalf("round trip lost fields:\n got %+v\nwant %+v", last, want)
	}

	// Reopening finds the next transition slot: a new transition replaces
	// the oldest, never the newest.
	r2, err := OpenRing(path)
	if err != nil {
		t.Fatal(err)
	}
	newest := base.Add(48 * time.Hour)
	if err := r2.WriteRingSample(RingRecord{At: newest, Transition: true, Level: LevelRefuse}); err != nil {
		t.Fatal(err)
	}
	_ = r2.Close()
	rd, _ = ReadRingFile(path)
	lt, ok := rd.LastTransition()
	if !ok || !lt.At.Equal(newest) || lt.Level != LevelRefuse {
		t.Fatalf("last transition after reopen = %+v, %v", lt, ok)
	}
	n := 0
	for _, rec := range rd.Records {
		if rec.Transition {
			n++
		}
	}
	if n != RingTransitionSlots {
		t.Fatalf("after reopen %d transitions, want %d", n, RingTransitionSlots)
	}

	// A torn slot (a kill mid-write) is skipped and counted.
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("garbage!"), ringHeaderSize+5*RingRecordSize+20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	rd, err = ReadRingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if rd.Torn != 1 {
		t.Fatalf("torn slots = %d, want 1", rd.Torn)
	}

	// A file that is not a ring is an error naming it, never an empty ring.
	bad := filepath.Join(t.TempDir(), "resources.ring")
	if err := os.WriteFile(bad, []byte("not a ring"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRingFile(bad); err == nil || !strings.Contains(err.Error(), "ring") {
		t.Fatalf("a non-ring file read without a named error: %v", err)
	}
	// A missing ring is its own, recognizable error.
	if _, err := ReadRingFile(filepath.Join(t.TempDir(), "absent.ring")); !os.IsNotExist(err) {
		t.Fatalf("missing ring: %v, want a not-exist error", err)
	}
}

// TestRing_SinceAndMax: the reader's window helpers pick the samples in
// the window and its peak (self + children), for `resources`' 24 h line.
func TestRing_SinceAndMax(t *testing.T) {
	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	rd := RingRead{Records: []RingRecord{
		{At: base, Footprint: 9 * GiB},
		{At: base.Add(2 * time.Hour), Footprint: 1 * GiB, Children: 1 * GiB},
		{At: base.Add(3 * time.Hour), Footprint: 3 * GiB, Children: Unknown, Level: LevelShed, Transition: true},
		{At: base.Add(4 * time.Hour), Footprint: Unknown},
	}}
	w := rd.Since(base.Add(time.Hour))
	if len(w.Records) != 3 {
		t.Fatalf("Since kept %d, want 3", len(w.Records))
	}
	peak, ok := w.Peak()
	if !ok || peak.Footprint != 3*GiB {
		t.Fatalf("peak = %+v, %v; want the 3 GiB sample (unknown children count as zero, unknown footprint never peaks)", peak, ok)
	}
	last, ok := w.Last()
	if !ok || !last.At.Equal(base.Add(4*time.Hour)) {
		t.Fatalf("last = %+v", last)
	}
}
