package costtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/metrics"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// MiB is a mebibyte.
const MiB = 1 << 20

// Ceiling is one class's per-call ceiling (design §4.8): heap allocations,
// and bytes read. Ingest has none: it is bounded by the one ingest slot
// and the in-flight byte window (I50), not a per-call class ceiling.
type Ceiling struct {
	Allocs, BytesRead uint64
	None              bool
}

// Ceilings are §4.8's class ceilings. Only point states its own byte
// ceiling (one capped file); the others hold bytes read to their
// allocation ceiling, since every byte read is at least one byte allocated.
var Ceilings = map[tenancy.Class]Ceiling{
	tenancy.ClassPoint:  {Allocs: 8 * MiB, BytesRead: 2 * MiB},
	tenancy.ClassIndex:  {Allocs: 64 * MiB, BytesRead: 64 * MiB},
	tenancy.ClassTree:   {Allocs: 256 * MiB, BytesRead: 256 * MiB},
	tenancy.ClassFleet:  {Allocs: 512 * MiB, BytesRead: 512 * MiB},
	tenancy.ClassIngest: {None: true},
}

// ScaleRatio is I48's bound: a point or index call's allocations at 10×
// the fleet are at most this multiple of its allocations at 1×.
const ScaleRatio = 1.5

// ScaleFloor is the allocation below which a 1× measurement is taken as
// ScaleFloor for the ratio: under it, a few map growths or a longer
// service list in an envelope are noise, not a fleet-proportional load.
// A whole-fleet load of the generated fleet is several MiB at 10×.
const ScaleFloor = 256 << 10

// Sample is one measured call.
type Sample struct {
	Allocs    uint64 `json:"allocs"`
	BytesRead int64  `json:"bytes_read"`
	// Err is the call's error text (a JSON-RPC error or an isError
	// result), "" when it answered.
	Err string `json:"err,omitempty"`
}

// Row is one tool's committed cost row: the arguments the harness calls it
// with ("$service" is QueriedService), and, when the call is expected to
// fail (a tool that needs what the fixture cannot give it), why.
type Row struct {
	Args        json.RawMessage `json:"args"`
	ExpectError string          `json:"expect_error,omitempty"`
	Note        string          `json:"note,omitempty"`
}

// Rows is the committed table (testdata/costrows.json).
type Rows struct {
	Comment string         `json:"_comment"`
	Tools   map[string]Row `json:"tools"`
}

// LoadRows reads the committed rows.
func LoadRows(path string) (Rows, error) {
	var r Rows
	raw, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("costtest: %s: %w", path, err)
	}
	return r, nil
}

// ArgsFor is a row's arguments with $service filled in.
func (r Row) ArgsFor(service string) json.RawMessage {
	if len(r.Args) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(strings.ReplaceAll(string(r.Args), `"$service"`, strconvQuote(service)))
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// NewServer is a server over a fresh client of f (cold memo keys) and the
// daemon's own fleet adapter over it, with an
// observe-mode admission gate at a normal ladder level so every call is
// admitted and metered.
func NewServer(f *Fleet) *mcp.Server {
	c := f.NewClient()
	srv := mcp.NewServerWithFleet(c, client.NewFleetAdapter(c, dlog.New(io.Discard, dlog.LevelWarn, time.Now)))
	srv.SetGate(tenancy.NewGate(tenancy.GateConfig{
		Mode:  tenancy.ModeObserve,
		Level: func() tenancy.Level { return tenancy.LevelNormal },
	}))
	return srv
}

var allocsMetric = []metrics.Sample{{Name: "/gc/heap/allocs:bytes"}}

func heapAllocs() uint64 {
	metrics.Read(allocsMetric)
	return allocsMetric[0].Value.Uint64()
}

// MeasureToolCost makes one tools/call of tool on srv with GOMAXPROCS=1
// and returns its heap allocations (a runtime/metrics
// /gc/heap/allocs:bytes delta) and the bytes its producers charged to the
// admission meter (fathom_bytes_read_total{tool}). The caller keeps
// anything else from running meanwhile.
func MeasureToolCost(ctx context.Context, srv *mcp.Server, tool string, args json.RawMessage) (Sample, error) {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return Sample{}, err
	}
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)
	runtime.GC()
	bytesBefore := tenancy.BytesReadTotal(tool)
	before := heapAllocs()
	raw, ok := srv.HandleRequest(ctx, req)
	// A GC flushes every P's cached allocation counts into the metric, so
	// the delta holds the call's small allocations too.
	runtime.GC()
	after := heapAllocs()
	s := Sample{Allocs: after - before, BytesRead: tenancy.BytesReadTotal(tool) - bytesBefore}
	if !ok {
		return s, fmt.Errorf("costtest: %s: no response", tool)
	}
	s.Err = errorText(raw)
	return s, nil
}

// errorText is a response's JSON-RPC error or isError text ("" for an
// answer), cut to 200 bytes.
func errorText(raw []byte) string {
	var resp struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "unparseable response: " + err.Error()
	}
	msg := ""
	switch {
	case resp.Error != nil:
		msg = resp.Error.Message
		if msg == "" {
			msg = "error"
		}
	case resp.Result.IsError:
		msg = "isError"
		if len(resp.Result.Content) > 0 {
			msg = resp.Result.Content[0].Text
		}
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// Measurement is one tool's cold and warm samples at one scale.
type Measurement struct {
	Tool  string        `json:"tool"`
	Class tenancy.Class `json:"-"`
	Scale int           `json:"scale"`
	Cold  Sample        `json:"cold"`
	Warm  Sample        `json:"warm"`
}

// MeasureFleet measures every tool in rows against f: per tool, a fresh
// server (cold), one call, then the same call again (warm). Tools run in
// name order; the memo registry is dropped between tools so each cold call
// builds what it reads.
func MeasureFleet(ctx context.Context, f *Fleet, rows Rows) ([]Measurement, error) {
	names := make([]string, 0, len(rows.Tools))
	for n := range rows.Tools {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Measurement
	for _, name := range names {
		m, err := measureTool(ctx, f, name, rows.Tools[name])
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// measureTool is one tool's cold call on a fresh server with the memo
// registry dropped, then its warm repeat.
func measureTool(ctx context.Context, f *Fleet, name string, row Row) (Measurement, error) {
	srv := NewServer(f)
	srv.DropCaches()
	args := row.ArgsFor(QueriedService)
	class, ok := srv.ToolCostClass(name, args)
	if !ok {
		return Measurement{}, fmt.Errorf("costtest: row %s names no registered tool", name)
	}
	m := Measurement{Tool: name, Class: class, Scale: f.Scale}
	var err error
	if m.Cold, err = MeasureToolCost(ctx, srv, name, args); err != nil {
		return m, err
	}
	if m.Warm, err = MeasureToolCost(ctx, srv, name, args); err != nil {
		return m, err
	}
	return m, nil
}

// RemeasureRuns is how many times RemeasureMin repeats a tool's cold and
// warm pair. Under -race, sync.Pool drops a quarter of its Puts at random,
// so a call that reuses a pooled encode buffer (encoding/json's
// encodeState) sometimes regrows it: a tool's allocations are bimodal
// (capabilities: 0.18 or ~0.43 MiB at either scale), and one sample of
// each scale can land in opposite modes. The noise only ever adds, so the
// minimum of 16 runs sits in the low mode (0.55^16 ≈ 7e-5 to miss it).
const RemeasureRuns = 16

// RemeasureMin re-measures one tool RemeasureRuns times against f and
// returns the first run with each phase's Allocs lowered to the minimum
// seen. A fleet-proportional load is in every run, so the minimum removes
// only additive noise, never real scaling.
func RemeasureMin(ctx context.Context, f *Fleet, name string, row Row) (Measurement, error) {
	var best Measurement
	for i := range RemeasureRuns {
		m, err := measureTool(ctx, f, name, row)
		if err != nil {
			return best, err
		}
		if i == 0 {
			best = m
			continue
		}
		best.Cold.Allocs = min(best.Cold.Allocs, m.Cold.Allocs)
		best.Warm.Allocs = min(best.Warm.Allocs, m.Warm.Allocs)
	}
	return best, nil
}

// Check returns every way m breaks its class ceiling or its row's
// expectation.
func Check(m Measurement, row Row) []string {
	var bad []string
	for _, s := range []struct {
		phase string
		s     Sample
	}{{"cold", m.Cold}, {"warm", m.Warm}} {
		if row.ExpectError == "" && s.s.Err != "" {
			bad = append(bad, fmt.Sprintf("%s ×%d %s: failed: %s", m.Tool, m.Scale, s.phase, s.s.Err))
		}
		if row.ExpectError != "" && s.s.Err == "" {
			bad = append(bad, fmt.Sprintf("%s ×%d %s: answered, but its row expects an error (%s): tighten the row", m.Tool, m.Scale, s.phase, row.ExpectError))
		}
		c := Ceilings[m.Class]
		if c.None {
			continue
		}
		if s.s.Allocs > c.Allocs {
			bad = append(bad, fmt.Sprintf("%s ×%d %s: %s class allocated %.1f MiB, over its %d MiB ceiling", m.Tool, m.Scale, s.phase, m.Class, float64(s.s.Allocs)/MiB, c.Allocs/MiB))
		}
		if uint64(max(s.s.BytesRead, 0)) > c.BytesRead {
			bad = append(bad, fmt.Sprintf("%s ×%d %s: %s class read %.1f MiB, over its %d MiB ceiling", m.Tool, m.Scale, s.phase, m.Class, float64(s.s.BytesRead)/MiB, c.BytesRead/MiB))
		}
	}
	return bad
}

// ScaleBreaks returns how a point or index tool's allocations at 10× break
// ScaleRatio against 1× (cold against cold, warm against warm).
func ScaleBreaks(one, ten Measurement) []string {
	if one.Class != tenancy.ClassPoint && one.Class != tenancy.ClassIndex {
		return nil
	}
	var bad []string
	for _, p := range []struct {
		phase    string
		one, ten uint64
	}{{"cold", one.Cold.Allocs, ten.Cold.Allocs}, {"warm", one.Warm.Allocs, ten.Warm.Allocs}} {
		base := max(p.one, ScaleFloor)
		if float64(p.ten) > ScaleRatio*float64(base) {
			bad = append(bad, fmt.Sprintf("%s (%s) %s: %.2f MiB at 10× vs %.2f MiB at 1× (%.1f×, bound %.1f× over a %d KiB floor)",
				one.Tool, one.Class, p.phase, float64(p.ten)/MiB, float64(p.one)/MiB, float64(p.ten)/float64(base), ScaleRatio, ScaleFloor>>10))
		}
	}
	return bad
}
