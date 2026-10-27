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

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// simulinkHeapEnv names the model a child of TestReadSimulinkModelHeapBound reads.
const simulinkHeapEnv = "FATHOM_SIMULINK_HEAP_INPUT"

// simulinkHeapLimit is the most a model may raise the reader's peak resident set over an
// empty model's: a small multiple of the inflated budget.
const simulinkHeapLimit = 8 * SimulinkMaxInflated

// TestReadSimulinkModelHeapBound is S3-R3's memory bound, S4-R3's with the extractor: each
// adversarial shape, built to its cap (or past it), is read and extracted in a child process
// (this test binary run again), and the child's peak resident set, from the kernel's own
// high-water mark, stays within simulinkHeapLimit of an empty model's. A shape past a cap must
// end in ErrSimulinkCap from the cap it is named for, and a shape that models ingestible
// input fits the fetch cap (connector.MaxFetchSize). It runs without the race detector, in
// validate's simulink-gate step (S5-R4): under -race the peak is the detector's.
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
	if testing.Short() || raceEnabled {
		t.Skip("peak memory is measured without the race detector, at full size (task simulink-gate)")
	}
	limit := uint64(simulinkHeapLimit)
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
		strings.Repeat("Branch {\nDstBlock \"d\"\n}\n", 75_000) + "}\n"
	var mdlBlocks strings.Builder
	for i := 0; i < SimulinkMaxBlocks; i++ {
		fmt.Fprintf(&mdlBlocks, "Block {\nName \"b%d\"\n}\n", i)
	}
	// S5-R2: 878,537 model references in graphicalInterface.json (31.6 MB inflated), and one
	// 30 MiB BlockPath, each compressing about as the finding's did.
	var refs strings.Builder
	refs.WriteString(`{"ModelReferences":[`)
	for i, seed := 0, uint32(17); i < 878_537; i++ {
		if i > 0 {
			refs.WriteString(",")
		}
		seed = seed*1664525 + 1013904223
		fmt.Fprintf(&refs, `{"BlockPath":"a","ModelName":"m%c"}`, 'a'+seed>>29)
	}
	refs.WriteString(`]}`)
	huge := []byte(simulinkLetters(30<<20, 19))
	for i := range huge {
		if huge[i] == 'a' { // one letter in 26 a b, the rest a: it compresses inside the ratio cap
			huge[i] = 'b'
		} else {
			huge[i] = 'a'
		}
	}
	block := `<System><Block Name="K" SID="1"/></System>`
	gi := "simulink/graphicalInterface.json"
	const (
		pathCap   = "a block path is longer than"
		signalCap = "signals hold past"
	)
	cases := []struct {
		name   string
		src    []byte
		cap    string // the cap that must end it, by its message; "" the shape reads
		reader bool   // past the fetch cap: only a direct caller of the reader sees it
	}{
		{"refflood slx", slxPackage(t, root, block, gi, refs.String()), "past 4096 model references", false},
		{"hugeref slx", slxPackage(t, root, block, gi, `{"ModelReferences":{"BlockPath":"`+string(huge)+`","ModelName":"m"}}`), "a string is longer than", false},
		{"element flood slx", slx(slxNoise(SimulinkMaxElements-2, `<P Name="Dst">%s</P>`, false)), "", false},
		{"block flood slx", slx(slxNoise(SimulinkMaxBlocks, `<Block Name="%s" SID="1" BlockType="G" Ref="r"/>`, false)), "", false},
		// F1: one start tag of about 3.9M attributes, inside the ratio cap.
		{"attribute flood slx", slx("<System><Block" + strings.TrimSuffix(strings.TrimPrefix(slxNoise((SimulinkMaxInflated-64)/8, ` %s="1"`, false), "<System>"), "</System>") + "/></System>"), "a start tag carries past", true},
		{"attributes at the cap slx", slx(slxNoise(15_000, `<Block Name="%s"`+strings.Repeat(` a="1"`, SimulinkMaxAttrs-1)+`/>`, false)), "", false},
		// F2: 200 nested blocks, each named by the one 16 KiB name: the second's path is past it.
		{"deep long names past the path cap slx", slx(simulinkNested(200, name16k, "")), pathCap, false},
		{"long names at the cap slx", slx("<System>" + wide.String() + "</System>"), "", false},
		{"endpoint fan-out slx", slx(simulinkNested(6, name2k, fanSLX.String())), "", false},
		// S4-R1 (F1): 700 children of a long-named parent, every pair joined; at 32 children
		// the signals' endpoint paths sit at their budget. A 40,000-byte parent is past the path
		// cap before any signal is read.
		{"fan pairs slx", slx(simulinkFanPairs(700, parent16k, false)), signalCap, false},
		{"40k parent past the path cap slx", slx(simulinkFanPairs(700, simulinkLetters(40_000, 3), false)), pathCap, false},
		{"fan pairs at the cap slx", slx(simulinkFanPairs(32, parent16k, false)), "", false},
		// S4-R2 (F2): one long path, 500,000 destinations of it; the 983 KB path of the
		// finding is past the path cap.
		{"hashburn past the path cap slx", slx(simulinkHashburn(15, simulinkLetters(SimulinkMaxValue, 7), 500_000, false)), pathCap, false},
		{"hashburn at the cap slx", slx(simulinkHashburn(1, simulinkLetters(16_000, 7), 500_000, false)), "", false},
		// S5-R1: short names pass the byte caps. 700 one-letter children, every pair joined, are
		// 490,000 signals; 8,300 blocks named A with one line to 1,000 others emit 8.3M calls.
		// The count caps end both; the .mdl fan pairs are 260 children, 1.7 MB.
		{"short fan pairs slx", slx(simulinkFanPairs(700, "p", false)), "past 65536 signals", false},
		{"duplicate sources slx", slx(simulinkDupSources(8300, 1000, false)), "past 65536 emitted calls", false},
		{"short fan pairs mdl", []byte(simulinkFanPairs(260, "p", true)), "past 65536 signals", false},
		{"duplicate sources mdl", []byte(simulinkDupSources(8300, 1000, true)), "past 65536 emitted calls", false},
		{"element flood mdl", []byte("Model {\n" + strings.Repeat("A {\n}\n", SimulinkMaxElements-2) + "}\n"), "", true},
		{"block flood mdl", []byte("Model {\nSystem {\n" + mdlBlocks.String() + "}\n}\n"), "", false},
		{"property flood mdl", []byte("Model {\nSystem {\nBlock {\nName \"b\"\n" + strings.Repeat("Note 1\nName \"c\"\n", 125_000) + "}\n}\n}\n"), "", false},
		{"deep long names past the path cap mdl", []byte(mdlNested(120, name16k, "")), pathCap, false},
		{"long names at the cap mdl", []byte(mdlNested(1, parent16k, strings.Repeat("Block {\nName \"c\"\n}\n", 2040))), "", false},
		{"endpoint fan-out mdl", []byte(mdlNested(6, name2k, fanMDL)), "", false},
		{"fan pairs mdl", []byte(simulinkFanPairs(64, parent16k, true)), signalCap, false},
		{"fan pairs at the cap mdl", []byte(simulinkFanPairs(32, parent16k, true)), "", false},
		{"hashburn past the path cap mdl", []byte(simulinkHashburn(15, simulinkLetters(SimulinkMaxValue, 7), 40_000, true)), pathCap, false},
		{"hashburn at the cap mdl", []byte(simulinkHashburn(1, simulinkLetters(16_000, 7), 80_000, true)), "", false},
	}
	dir := t.TempDir()
	base, _ := simulinkChild(t, dir, "empty.mdl", []byte("Model {\n}\n"), "")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel := "m.slx"
			if strings.HasSuffix(c.name, "mdl") {
				rel = "m.mdl"
			}
			if !c.reader && len(c.src) > connector.MaxFetchSize {
				t.Errorf("a %d-byte input is past the %d-byte fetch cap: ingest never reads it", len(c.src), connector.MaxFetchSize)
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
// resident set in bytes and its CPU time, failing unless the read ends in an ErrSimulinkCap
// whose message holds wantCap, or reads without error when wantCap is "".
func simulinkChild(t *testing.T, dir, rel string, src []byte, wantCap string) (uint64, time.Duration) {
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
	_, msg, _ := strings.Cut(line, " err=")
	if got := strings.Contains(line, " cap=true "); got != (wantCap != "") {
		t.Errorf("%s; want ErrSimulinkCap %v", line, wantCap != "")
	} else if wantCap != "" && !strings.Contains(msg, wantCap) {
		t.Errorf("%s; want the cap %q to end it", line, wantCap)
	} else if wantCap != "" && !strings.Contains(line, " nodes=0 ") {
		t.Errorf("%s; want the refused model to yield no symbols", line)
	} else if wantCap == "" && msg != "<nil>" {
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

// TestSimulinkHashburnCPU is S4-R2's CPU bound: the hashburn shape (one long path, its block
// each destination of a line), past the path cap and at it, reads and extracts in a child
// process within simulinkMaxFileMS of process CPU, from inputs inside the fetch cap.
// TestSimulinkResolveHashWork is the check a path-keyed dedupe fails. Both run without the
// race detector, in validate's simulink-gate step.
func TestSimulinkHashburnCPU(t *testing.T) {
	if os.Getenv(simulinkHeapEnv) != "" {
		t.Skip("a heap child")
	}
	if testing.Short() || raceEnabled {
		t.Skip("CPU time is measured without the race detector, at full size (task simulink-gate)")
	}
	root := "simulink/systems/system_root.xml"
	long, cap16k := simulinkLetters(SimulinkMaxValue, 7), simulinkLetters(16_000, 7)
	dir := t.TempDir()
	_, base := simulinkChild(t, dir, "empty.mdl", []byte("Model {\n}\n"), "")
	cpuOf := func(t *testing.T, rel string, src []byte, wantCap string) time.Duration {
		if len(src) > connector.MaxFetchSize {
			t.Errorf("a %d-byte input is past the %d-byte fetch cap", len(src), connector.MaxFetchSize)
		}
		_, cpu := simulinkChild(t, dir, rel, src, wantCap)
		t.Logf("%d-byte input: %v of CPU (an empty model's %v)", len(src), cpu, base)
		if cpu > simulinkMaxFileMS {
			t.Errorf("read and extract took %v of CPU, want at most %v", cpu, simulinkMaxFileMS)
		}
		return cpu
	}
	t.Run("hashburn past the path cap slx", func(t *testing.T) {
		cpuOf(t, "m.slx", slxPackage(t, root, simulinkHashburn(15, long, 500_000, false)), "a block path is longer than")
	})
	t.Run("hashburn past the path cap mdl", func(t *testing.T) {
		cpuOf(t, "m.mdl", []byte(simulinkHashburn(15, long, 40_000, true)), "a block path is longer than")
	})
	t.Run("hashburn at the cap slx", func(t *testing.T) {
		cpuOf(t, "m.slx", slxPackage(t, root, simulinkHashburn(1, cap16k, 500_000, false)), "")
	})
	t.Run("hashburn at the cap mdl", func(t *testing.T) {
		cpuOf(t, "m.mdl", []byte(simulinkHashburn(1, cap16k, 80_000, true)), "")
	})
}

// TestSimulinkResolveHashWork is S5-R4's bite: signal resolution hashes no path per
// endpoint. One line from a block to itself, 1,000,000 times over, resolves within twice (plus
// 50 ms) the CPU of the same line when the block's path is one byte, not 16 KiB (the least of
// 3 runs each). A dedupe keyed on the endpoints' paths hashes 32 GB here and fails by
// seconds; keyed on block indexes, path length costs nothing. Resolution runs in this process,
// past the parser, so the parse's CPU and its noise do not mask it.
func TestSimulinkResolveHashWork(t *testing.T) {
	if os.Getenv(simulinkHeapEnv) != "" {
		t.Skip("a heap child")
	}
	if testing.Short() || raceEnabled {
		t.Skip("CPU time is measured without the race detector, at full size (task simulink-gate)")
	}
	const dsts = 1_000_000
	resolve := func(name string) time.Duration {
		best := time.Duration(1 << 62)
		for i := 0; i < 3; i++ {
			parent := "m/" + name
			m := SimulinkModel{Name: "m", SIDPath: map[string]string{},
				Blocks: []SimulinkBlock{{Path: parent, Parent: "m", Name: name, Subsystem: true},
					{Path: parent + "/x", Parent: parent, Name: "x", SID: "1"}},
				owners: []int{-1, 0}}
			e := simulinkEnds{owner: 0, src: simulinkEnd{sid: "1"}, dsts: make([]simulinkEnd, dsts)}
			for k := range e.dsts {
				e.dsts[k] = simulinkEnd{sid: "1"}
			}
			m.ends = []simulinkEnds{e}
			before := simulinkCPU(t)
			if err := m.resolveSignals(); err != nil || len(m.Signals) != 1 {
				t.Fatalf("%d signals, %v; want the one", len(m.Signals), err)
			}
			best = min(best, simulinkCPU(t)-before)
		}
		return best
	}
	long, short := resolve(simulinkLetters(16_000, 7)), resolve("n")
	t.Logf("%d destinations: %v of CPU for a 16 KiB path, %v for a one-byte one", dsts, long, short)
	if bound := 2*short + 50*time.Millisecond; long > bound {
		t.Errorf("a 16 KiB path took %v of CPU to resolve, a one-byte one %v: want at most %v, so path length does not reach the dedupe", long, short, bound)
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
