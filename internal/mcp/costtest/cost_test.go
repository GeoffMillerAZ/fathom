package costtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const rowsPath = "testdata/costrows.json"

var measured struct {
	once     sync.Once
	err      error
	rows     Rows
	one, ten *Fleet
	byScale  map[int]map[string]Measurement
	dirs     []string // the fleets' roots, removed by TestMain
}

// TestMain removes the fleets every test shares.
func TestMain(m *testing.M) {
	code := m.Run()
	for _, d := range measured.dirs {
		_ = os.RemoveAll(d)
	}
	os.Exit(code)
}

// measureAll builds the 1× and 10× fleets and measures every row at both,
// once per test binary.
func measureAll(t *testing.T) {
	t.Helper()
	measured.once.Do(func() {
		ctx := context.Background()
		if measured.rows, measured.err = LoadRows(rowsPath); measured.err != nil {
			return
		}
		measured.byScale = map[int]map[string]Measurement{}
		for _, scale := range []int{1, 10} {
			dir, err := os.MkdirTemp("", fmt.Sprintf("costtest-%dx-", scale))
			if err != nil {
				measured.err = err
				return
			}
			measured.dirs = append(measured.dirs, dir)
			f, err := BuildFleet(ctx, dir, scale)
			if err != nil {
				measured.err = err
				return
			}
			if scale == 1 {
				measured.one = f
			} else {
				measured.ten = f
			}
			ms, err := MeasureFleet(ctx, f, measured.rows)
			if err != nil {
				measured.err = err
				return
			}
			byTool := map[string]Measurement{}
			for _, m := range ms {
				byTool[m.Tool] = m
			}
			measured.byScale[scale] = byTool
		}
	})
	if measured.err != nil {
		t.Fatal(measured.err)
	}
}

func table(ms map[int]map[string]Measurement) string {
	var names []string
	for n := range ms[1] {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s %-6s %10s %10s %10s %10s %9s\n", "tool", "class", "cold 1x", "warm 1x", "cold 10x", "warm 10x", "read 10x")
	for _, n := range names {
		o, x := ms[1][n], ms[10][n]
		fmt.Fprintf(&b, "%-24s %-6s %9.2fM %9.2fM %9.2fM %9.2fM %8dB\n", n, o.Class,
			float64(o.Cold.Allocs)/MiB, float64(o.Warm.Allocs)/MiB, float64(x.Cold.Allocs)/MiB, float64(x.Warm.Allocs)/MiB, x.Cold.BytesRead)
	}
	return b.String()
}

// TestEveryToolHasCostRow (RT-93): every registered tool has a committed
// row, and at 1× and 10×, cold and warm, it stays within its class's
// allocation and bytes-read ceilings (design §4.8).
func TestEveryToolHasCostRow(t *testing.T) {
	measureAll(t)
	srv := NewServer(measured.one)
	registered := srv.RegisteredTools()
	var bad []string
	for _, name := range registered {
		if _, ok := measured.rows.Tools[name]; !ok {
			bad = append(bad, name+": registered, but has no cost row in "+rowsPath)
		}
	}
	reg := map[string]bool{}
	for _, n := range registered {
		reg[n] = true
	}
	for name := range measured.rows.Tools {
		if !reg[name] {
			bad = append(bad, name+": a cost row for a tool that is not registered")
		}
	}
	for _, scale := range []int{1, 10} {
		for name, m := range measured.byScale[scale] {
			bad = append(bad, Check(m, measured.rows.Tools[name])...)
		}
	}
	sort.Strings(bad)
	t.Logf("measured (GOMAXPROCS=1 per call):\n%s", table(measured.byScale))
	if len(bad) > 0 {
		t.Fatalf("cost rows:\n%s", strings.Join(bad, "\n"))
	}
}

// TestOversizeAndBinaryReadNothing: the fixture's 12 MiB file costs a
// point call no byte to refuse, and its binary blob no more than the sniff.
func TestOversizeAndBinaryReadNothing(t *testing.T) {
	measureAll(t)
	srv := NewServer(measured.one)
	for _, tc := range []struct {
		file string
		max  int64
	}{{"big.txt", 0}, {"blob.bin", corpus.SniffSize}} {
		args, _ := json.Marshal(map[string]any{"service": QueriedService, "file": tc.file})
		s, err := MeasureToolCost(context.Background(), srv, "read_source", args)
		if err != nil {
			t.Fatal(err)
		}
		if s.Err == "" {
			t.Errorf("read_source %s answered; want a refusal", tc.file)
		}
		if s.BytesRead > tc.max || s.Allocs > Ceilings[tenancy.ClassPoint].Allocs {
			t.Errorf("read_source %s: read %d bytes and allocated %d; want at most %d bytes within the point ceiling", tc.file, s.BytesRead, s.Allocs, tc.max)
		}
	}
}

// TestPointIndexToolsScaleSublinearly (RT-94, I48): a point or index
// call's allocations grow at most ScaleRatio when the fleet grows 10×
// around the service it asks about.
func TestPointIndexToolsScaleSublinearly(t *testing.T) {
	measureAll(t)
	var bad []string
	checked := 0
	for name, one := range measured.byScale[1] {
		ten, ok := measured.byScale[10][name]
		if !ok {
			bad = append(bad, name+": no 10× measurement")
			continue
		}
		if one.Class == tenancy.ClassPoint || one.Class == tenancy.ClassIndex {
			checked++
		}
		bad = append(bad, ScaleBreaks(one, ten)...)
	}
	if checked == 0 {
		t.Fatal("no point or index tool was measured")
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("point/index tools not sublinear in the fleet (I48):\n%s", strings.Join(bad, "\n"))
	}
}

// stampedeCases are one tool per memo group it builds and per class that
// builds through memo: the index navigation (snapshot, typed edges,
// graphs), the tree obligations walk (layer coverage paths, and the hot
// reads it single-flights), symbol_context's graph, and the fleet's
// cross-service blast radius (a snapshot and routes per service).
var stampedeCases = []struct {
	tool string
	args string
}{
	{"find_callers", `{"service":"ledger-service","symbol":"validateEntry"}`},
	{"obligations", `{"service":"ledger-service","file":"ledger.go"}`},
	{"symbol_context", `{"service":"ledger-service","symbol":"Post"}`},
	{"cross_repo_blast_radius", `{"symbol":"Post"}`},
}

// TestColdStampedeOneBuildPerKey (RT-95): 32 concurrent cold callers start
// exactly the builds one cold caller starts (one per key) and read one
// walk's bytes; 32 callers cancelled mid-build leave the process's
// goroutines at their baseline within 1 s, the held flight cancelled.
func TestColdStampedeOneBuildPerKey(t *testing.T) {
	measureAll(t)
	ctx := context.Background()
	f := measured.ten
	for _, tc := range stampedeCases {
		t.Run(tc.tool, func(t *testing.T) {
			args := json.RawMessage(tc.args)
			// One cold caller: the builds and bytes per key; then its
			// warm repeat: the bytes a call reads with every build cached.
			single := NewServer(f)
			one, err := MeasureStampede(ctx, single, tc.tool, args, 1)
			if err != nil {
				t.Fatal(err)
			}
			warm, err := MeasureStampede(ctx, single, tc.tool, args, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(one.Errors) > 0 {
				t.Fatalf("single cold call failed: %v", one.Errors)
			}
			if len(one.Flights) == 0 {
				t.Fatalf("a cold %s started no memo build: it is not a stampede case", tc.tool)
			}
			const callers = 32
			many, err := MeasureStampede(ctx, NewServer(f), tc.tool, args, callers)
			if err != nil {
				t.Fatal(err)
			}
			if len(many.Errors) > 0 {
				t.Fatalf("%d of %d callers failed: %v", len(many.Errors), callers, many.Errors[0])
			}
			for group, n := range one.Flights {
				if many.Flights[group] != n {
					t.Errorf("group %s: %d cold callers started %d builds; one cold caller starts %d (one per key)", group, callers, many.Flights[group], n)
				}
			}
			for group, n := range many.Flights {
				if _, ok := one.Flights[group]; !ok {
					t.Errorf("group %s: the stampede started %d builds one caller never starts", group, n)
				}
			}
			walk := one.BytesRead - warm.BytesRead
			if want := walk + callers*warm.BytesRead; many.BytesRead != want {
				t.Errorf("%d callers read %d bytes; want one walk (%d) plus %d × the warm call's %d = %d", callers, many.BytesRead, walk, callers, warm.BytesRead, want)
			}
			t.Logf("%s: builds per group %v, walk %d B, warm %d B, stampede %d B", tc.tool, one.Flights, walk, warm.BytesRead, many.BytesRead)
		})
	}

	t.Run("cancel", func(t *testing.T) {
		gated := NewGatedBackend(f.NewClient())
		srv := mcp.NewServerWithFleet(gated, client.NewFleetAdapter(gated.Client, dlog.New(discard{}, dlog.LevelWarn, time.Now)))
		srv.SetGate(tenancy.NewGate(tenancy.GateConfig{Mode: tenancy.ModeObserve, Level: func() tenancy.Level { return tenancy.LevelNormal }}))
		defer close(gated.Release)
		res, err := MeasureStampedeCancel(srv, gated, "find_callers", json.RawMessage(stampedeCases[0].args), 32, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if res.Leaked > 0 {
			t.Fatalf("%d goroutine(s) still above the baseline %v after the 32 callers were cancelled", res.Leaked, res.Settle)
		}
		if res.Cancelled["snapshot"] < 1 {
			t.Errorf("the held snapshot flight was not cancelled when its last caller left: cancelled %v", res.Cancelled)
		}
		t.Logf("goroutines back at baseline %v after the cancel; cancelled %v", res.Settle, res.Cancelled)
	})
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// TestChecksCatchWhatTheyGuard: the ceiling and scale checks fail a
// measurement that breaks them — the harness is not vacuous.
func TestChecksCatchWhatTheyGuard(t *testing.T) {
	point := Measurement{Tool: "p", Class: tenancy.ClassPoint, Scale: 1,
		Cold: Sample{Allocs: 9 * MiB, BytesRead: 3 * MiB}, Warm: Sample{Allocs: MiB}}
	if bad := Check(point, Row{}); len(bad) != 2 {
		t.Errorf("a point call over both ceilings: %v", bad)
	}
	if bad := Check(Measurement{Tool: "e", Class: tenancy.ClassIndex, Cold: Sample{Err: "boom"}}, Row{}); len(bad) != 1 {
		t.Errorf("an unexpected error is a finding: %v", bad)
	}
	if bad := Check(Measurement{Tool: "e", Class: tenancy.ClassIndex}, Row{ExpectError: "why"}); len(bad) != 2 {
		t.Errorf("an answer where the row expects an error is a finding: %v", bad)
	}
	if bad := Check(Measurement{Tool: "i", Class: tenancy.ClassIngest, Cold: Sample{Allocs: 4096 * MiB}}, Row{}); len(bad) != 0 {
		t.Errorf("ingest has no per-call ceiling: %v", bad)
	}
	one := Measurement{Tool: "idx", Class: tenancy.ClassIndex, Cold: Sample{Allocs: 2 * MiB}, Warm: Sample{Allocs: 100 << 10}}
	fleetLoad := Measurement{Tool: "idx", Class: tenancy.ClassIndex, Cold: Sample{Allocs: 20 * MiB}, Warm: Sample{Allocs: 300 << 10}}
	if bad := ScaleBreaks(one, fleetLoad); len(bad) != 1 {
		t.Errorf("a 10× cold growth breaks the ratio; warm noise under the floor does not: %v", bad)
	}
	tree := one
	tree.Class = tenancy.ClassTree
	if bad := ScaleBreaks(tree, fleetLoad); len(bad) != 0 {
		t.Errorf("tree tools are not held to the ratio: %v", bad)
	}
}
