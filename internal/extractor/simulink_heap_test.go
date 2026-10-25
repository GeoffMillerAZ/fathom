//go:build unix

package extractor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// simulinkHeapEnv names the model a child of TestReadSimulinkModelHeapBound reads.
const simulinkHeapEnv = "FATHOM_SIMULINK_HEAP_INPUT"

// simulinkHeapLimit is the most a model may raise the reader's peak resident set over an
// empty model's: a small multiple of the inflated budget.
const simulinkHeapLimit = 8 * SimulinkMaxInflated

// TestReadSimulinkModelHeapBound is S3-R3's memory bound, S4-R3's with the extractor: each
// adversarial shape, built to its cap (or past it), is read and extracted in a child process
// (this test binary run again), and the child's peak resident set, from the kernel's own high-water mark, stays within
// simulinkHeapLimit of an empty model's (4x that under -race). A shape past a cap must end in
// ErrSimulinkCap.
func TestReadSimulinkModelHeapBound(t *testing.T) {
	if in := os.Getenv(simulinkHeapEnv); in != "" {
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		// ExtractDoc reads the model and emits its rows, as an ingest does: the peak and the CPU
		// are its; the reader's own counts are taken after.
		r, xerr := NewSimulinkExtractor().ExtractDoc("svc", filepath.Base(in), src)
		rss, cpu := simulinkPeakRSS(t), simulinkCPU(t)
		calls := 0
		for _, n := range r.Nodes {
			for _, c := range n.Calls {
				calls += len(c)
			}
		}
		nodes := len(r.Nodes)
		r = DocResult{}
		m, err := ReadSimulinkModel(filepath.Base(in), src)
		fmt.Printf("heap-child: rss=%d cpu_ms=%d blocks=%d signals=%d nodes=%d call_bytes=%d xerr=%v cap=%v err=%v\n",
			rss, cpu.Milliseconds(), len(m.Blocks), len(m.Signals), nodes, calls, xerr, errors.Is(err, ErrSimulinkCap), err)
		return
	}
	if testing.Short() {
		t.Skip("reads every shape at its cap")
	}
	limit := uint64(simulinkHeapLimit)
	if raceEnabled { // the race detector's shadow memory is not the reader's: about 3x more
		limit *= 4
	}
	root := "simulink/systems/system_root.xml"
	slx := func(part string) []byte { return slxPackage(t, root, part) }
	name16k := strings.Repeat("n", 16<<10)
	name2k := strings.Repeat("n", 2500)
	parent16k := simulinkLetters(16_000, 3)
	var wide strings.Builder // 2,040 children of a 16,000-byte name: the path bytes at their cap
	wide.WriteString(`<Block BlockType="SubSystem" Name="` + parent16k + `"><System>`)
	for i := 0; i < 2040; i++ {
		fmt.Fprintf(&wide, `<Block Name="c%d"/>`, i)
	}
	wide.WriteString(`</System></Block>`)
	// 100,000 branches to 64 destinations, picked pseudo-randomly so the part compresses as a
	// real one does.
	var fanSLX strings.Builder
	fanSLX.WriteString(`<Block Name="s" SID="1"/>`)
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&fanSLX, `<Block Name="d%d"/>`, i)
	}
	fanSLX.WriteString(`<Line><P Name="SrcBlock">s</P>`)
	for i, seed := 0, uint32(7); i < 100_000; i++ {
		seed = seed*1664525 + 1013904223
		fmt.Fprintf(&fanSLX, `<Branch><P Name="DstBlock">d%d</P></Branch>`, seed>>26)
	}
	fanSLX.WriteString(`</Line>`)
	fanMDL := "Block {\nName \"s\"\n}\nBlock {\nName \"d\"\n}\nLine {\nSrcBlock \"s\"\n" +
		strings.Repeat("Branch {\nDstBlock \"d\"\n}\n", 100_000) + "}\n"
	var mdlBlocks strings.Builder
	for i := 0; i < SimulinkMaxBlocks; i++ {
		fmt.Fprintf(&mdlBlocks, "Block {\nBlockType Gain\nName \"b%d\"\nSID \"%d\"\n}\n", i, i)
	}
	cases := []struct {
		name string
		src  []byte
		cap  bool // the shape is past a cap
	}{
		{"element flood slx", slx(slxNoise(SimulinkMaxElements-2, `<P Name="Dst">%s</P>`, false)), false},
		{"block flood slx", slx(slxNoise(SimulinkMaxBlocks, `<Block Name="%s" SID="1" BlockType="G" Ref="r"/>`, false)), false},
		// F1: one start tag of about 3.9M attributes, inside the ratio cap.
		{"attribute flood slx", slx("<System><Block" + strings.TrimSuffix(strings.TrimPrefix(slxNoise((SimulinkMaxInflated-64)/8, ` %s="1"`, false), "<System>"), "</System>") + "/></System>"), true},
		{"attributes at the cap slx", slx(slxNoise(15_000, `<Block Name="%s"`+strings.Repeat(` a="1"`, SimulinkMaxAttrs-1)+`/>`, false)), false},
		// F2: 200 nested blocks, each named by the one 16 KiB name.
		{"deep long names slx", slx(simulinkNested(200, name16k, "")), true},
		{"long names at the cap slx", slx("<System>" + wide.String() + "</System>"), false},
		{"endpoint fan-out slx", slx(simulinkNested(6, name2k, fanSLX.String())), false},
		// S4-R1 (F1): 700 children of a long-named parent, every pair joined; at 32 children
		// the signals' endpoint paths sit at their budget.
		{"fan pairs slx", slx(simulinkFanPairs(700, parent16k, false)), true},
		{"fan pairs 40k parent slx", slx(simulinkFanPairs(700, simulinkLetters(40_000, 3), false)), true},
		{"fan pairs at the cap slx", slx(simulinkFanPairs(32, parent16k, false)), false},
		// S4-R2 (F2): one long path, 500,000 destinations of it; the 983 KB path of the
		// finding is past the path cap now.
		{"hashburn slx", slx(simulinkHashburn(15, simulinkLetters(SimulinkMaxValue, 7), 500_000, false)), true},
		{"hashburn at the cap slx", slx(simulinkHashburn(1, simulinkLetters(16_000, 7), 500_000, false)), false},
		{"element flood mdl", []byte("Model {\n" + strings.Repeat("A {\n}\n", SimulinkMaxElements-2) + "}\n"), false},
		{"block flood mdl", []byte("Model {\nSystem {\n" + mdlBlocks.String() + "}\n}\n"), false},
		{"property flood mdl", []byte("Model {\nSystem {\nBlock {\nName \"b\"\n" + strings.Repeat("Note 1\nName \"c\"\n", 2<<20) + "}\n}\n}\n"), false},
		{"deep long names mdl", []byte(mdlNested(200, name16k, "")), true},
		{"long names at the cap mdl", []byte(mdlNested(1, parent16k, strings.Repeat("Block {\nName \"c\"\n}\n", 2040))), false},
		{"endpoint fan-out mdl", []byte(mdlNested(6, name2k, fanMDL)), false},
		{"fan pairs mdl", []byte(simulinkFanPairs(700, parent16k, true)), true},
		{"fan pairs at the cap mdl", []byte(simulinkFanPairs(32, parent16k, true)), false},
		{"hashburn mdl", []byte(simulinkHashburn(15, simulinkLetters(SimulinkMaxValue, 7), 500_000, true)), true},
		{"hashburn at the cap mdl", []byte(simulinkHashburn(1, simulinkLetters(16_000, 7), 500_000, true)), false},
	}
	dir := t.TempDir()
	base, _ := simulinkChild(t, dir, "empty.mdl", []byte("Model {\n}\n"), false)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := "m.slx"
			if strings.HasSuffix(c.name, "mdl") {
				rel = "m.mdl"
			}
			peak, _ := simulinkChild(t, dir, rel, c.src, c.cap)
			grew := max(peak, base) - base
			t.Logf("%d-byte input: peak RSS %d MiB, %d MiB over an empty model's", len(c.src), peak>>20, grew>>20)
			if grew > limit {
				t.Errorf("peak RSS grew %d MiB, want at most %d MiB", grew>>20, limit>>20)
			}
		})
	}
}

// simulinkChild reads and extracts src in a child process and returns the child's peak
// resident set in bytes and its CPU time, failing unless the read ends in ErrSimulinkCap
// exactly when wantCap.
func simulinkChild(t *testing.T, dir, rel string, src []byte, wantCap bool) (uint64, time.Duration) {
	t.Helper()
	in := filepath.Join(dir, rel)
	if err := os.WriteFile(in, src, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestReadSimulinkModelHeapBound$", "-test.count=1")
	cmd.Env = append(os.Environ(), simulinkHeapEnv+"="+in)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	line := ""
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "heap-child: ") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("child printed no result:\n%s", out)
	}
	if got := strings.Contains(line, " cap=true "); got != wantCap {
		t.Errorf("%s; want ErrSimulinkCap %v", line, wantCap)
	} else if wantCap && !strings.Contains(line, " nodes=0 ") {
		t.Errorf("%s; want the refused model to yield no symbols", line)
	} else if !wantCap && !strings.HasSuffix(line, " err=<nil>") {
		t.Errorf("%s; want no error", line)
	}
	t.Log(strings.TrimPrefix(line, "heap-child: "))
	var rss, cpu uint64
	if _, err := fmt.Sscanf(line, "heap-child: rss=%d cpu_ms=%d", &rss, &cpu); err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	return rss, time.Duration(cpu) * time.Millisecond
}

// simulinkMaxFileMS is the max_file_ms bar a model must read and extract within.
const simulinkMaxFileMS = 2000 * time.Millisecond

// TestSimulinkHashburnCPU is S4-R2's CPU bound: the hashburn shape (one long path, 500,000
// destinations of it), as the 1 MB package of the finding and at the path cap, for .slx and
// .mdl, reads and extracts in a child process within simulinkMaxFileMS of process CPU.
func TestSimulinkHashburnCPU(t *testing.T) {
	if os.Getenv(simulinkHeapEnv) != "" {
		t.Skip("a heap child")
	}
	if testing.Short() || raceEnabled {
		t.Skip("CPU time is measured without the race detector, at full size")
	}
	root := "simulink/systems/system_root.xml"
	long, cap16k := simulinkLetters(SimulinkMaxValue, 7), simulinkLetters(16_000, 7)
	cases := []struct {
		name string
		src  []byte
		cap  bool
	}{
		{"hashburn slx", slxPackage(t, root, simulinkHashburn(15, long, 500_000, false)), true},
		{"hashburn at the cap slx", slxPackage(t, root, simulinkHashburn(1, cap16k, 500_000, false)), false},
		{"hashburn mdl", []byte(simulinkHashburn(15, long, 500_000, true)), true},
		{"hashburn at the cap mdl", []byte(simulinkHashburn(1, cap16k, 500_000, true)), false},
	}
	dir := t.TempDir()
	_, base := simulinkChild(t, dir, "empty.mdl", []byte("Model {\n}\n"), false)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := "m.slx"
			if strings.HasSuffix(c.name, "mdl") {
				rel = "m.mdl"
			}
			_, cpu := simulinkChild(t, dir, rel, c.src, c.cap)
			t.Logf("%d-byte input: %v of CPU (an empty model's %v)", len(c.src), cpu, base)
			if cpu > simulinkMaxFileMS {
				t.Errorf("read and extract took %v of CPU, want at most %v", cpu, simulinkMaxFileMS)
			}
		})
	}
}

// simulinkCPU is this process's user and system CPU time.
func simulinkCPU(t *testing.T) time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatal(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// simulinkPeakRSS is this process's peak resident set in bytes, the kernel's high-water mark:
// on Linux the mm's own (VmHWM), since getrusage's carries the parent's across a vfork and
// exec; elsewhere getrusage's.
func simulinkPeakRSS(t *testing.T) uint64 {
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile("/proc/self/status")
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(l, "VmHWM:"); ok {
				var kib uint64
				if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d kB", &kib); err != nil {
					t.Fatal(err)
				}
				return kib << 10
			}
		}
		t.Fatal("no VmHWM in /proc/self/status")
	}
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" { // darwin reports bytes, the others KiB
		return uint64(ru.Maxrss)
	}
	return uint64(ru.Maxrss) << 10
}
