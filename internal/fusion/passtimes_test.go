package fusion

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// stepClock is a deterministic PassClock: every Now reading is one wall
// step later than the last, every CPU reading one CPU step later.
type stepClock struct {
	mu        sync.Mutex
	wall, cpu time.Duration
	wStep     time.Duration
	cStep     time.Duration
}

func (c *stepClock) clock() PassClock {
	base := time.Unix(1_700_000_000, 0)
	return PassClock{
		Now: func() time.Time {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.wall += c.wStep
			return base.Add(c.wall)
		},
		CPU: func() time.Duration {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.cpu += c.cStep
			return c.cpu
		},
	}
}

// scriptClock returns its readings in order (wall and CPU separately).
func scriptClock(walls, cpus []int64) PassClock {
	base := time.Unix(1_700_000_000, 0)
	wi, ci := 0, 0
	return PassClock{
		Now: func() time.Time { v := walls[wi]; wi++; return base.Add(time.Duration(v)) },
		CPU: func() time.Duration { v := cpus[ci]; ci++; return time.Duration(v) },
	}
}

// TestPassRecorderLapsSpansAndSubs pins the arithmetic: a lap is the time
// since the previous mark less the spans recorded inside it, a span is its
// own start to stop, and a sub-pass accumulates every span of it, follows
// its parent and is included in the parent's time.
func TestPassRecorderLapsSpansAndSubs(t *testing.T) {
	// readings, in call order: open, sub a start/stop, sub a start/stop,
	// mark one, span start/stop, mark two
	walls := []int64{0, 10, 13, 20, 24, 30, 40, 47, 50}
	cpus := []int64{0, 5, 7, 9, 12, 20, 22, 26, 30}
	ctx, rec := startPassTimes(WithPassClock(context.Background(), scriptClock(walls, cpus)))
	if passTimesFrom(ctx) != rec || rec == nil {
		t.Fatal("startPassTimes did not attach its recorder")
	}
	stop := rec.sub("a")
	stop()
	stop = rec.sub("a")
	stop()
	rec.mark("one")
	stop = rec.span("nested")
	stop()
	rec.mark("two")
	want := []PassTime{
		{Name: "one", WallNS: 30, CPUNS: 20},
		{Name: "a", Parent: "one", WallNS: 3 + 4, CPUNS: 2 + 3},
		{Name: "nested", WallNS: 7, CPUNS: 4},
		{Name: "two", WallNS: 50 - 30 - 7, CPUNS: 30 - 20 - 4},
	}
	if got := rec.times(); !reflect.DeepEqual(got, want) {
		t.Fatalf("times = %+v\nwant    %+v", got, want)
	}

	// A zero clock is off: no recorder, nil entries, no-op stops.
	ctx, rec = startPassTimes(WithPassClock(context.Background(), PassClock{}))
	if rec != nil || passTimesFrom(ctx) != nil {
		t.Fatal("a zero PassClock must turn the timer off")
	}
	rec.sub("x")()
	rec.span("y")()
	rec.mark("z")
	if rec.times() != nil {
		t.Fatal("an off recorder returned entries")
	}
}

// TestIngestReportsPassTimes is EW-48: an ingest's Report carries one entry
// per pass in execution order — the top-level passes with their sub-passes
// right after them — each with wall and CPU time from the injected clock;
// a zero clock leaves PassTimes nil.
func TestIngestReportsPassTimes(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"app/main.py":        "from app import util\n\n\ndef main():\n    return util.helper()\n",
		"app/util.py":        "def helper():\n    return 1\n",
		"app/__init__.py":    "",
		"docs/design.md":     "# Design\n\n## Flow\n\n`helper` returns one; see `app/util.py`.\n",
		"Dockerfile":         "FROM alpine:3.19\nCOPY . /app\n",
		"docker-compose.yml": "services:\n  web:\n    build: .\n    image: example/web:1\n",
	})
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "passes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	p := New(connector.NewLocalConnector(), sq, Config{})
	t.Cleanup(p.Close)
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })

	clk := &stepClock{wStep: time.Millisecond, cStep: time.Microsecond}
	ctx := WithPassClock(context.Background(), clk.clock())
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "passes", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	type nameParent struct{ name, parent string }
	var got []nameParent
	for _, pt := range rep.PassTimes {
		got = append(got, nameParent{pt.Name, pt.Parent})
		if pt.WallNS <= 0 || pt.CPUNS <= 0 {
			t.Errorf("%s: wall %d ns, cpu %d ns, want both > 0 under a stepping clock", pt.Name, pt.WallNS, pt.CPUNS)
		}
	}
	want := []nameParent{
		{"discover", ""},
		{"extract", ""}, {"store_writes", "extract"},
		{"typed_go", ""},
		{"cue", ""},
		{"file_resolved", ""}, {"resolve", "file_resolved"},
		{"doc_xref", ""}, {"image_ref", "doc_xref"},
		{"route_ref", ""},
		{"policy_ref", ""},
		{"doc_index", ""}, {"doc_links", "doc_index"},
		{"centrality", ""},
		{"commit", ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pass entries =\n%v\nwant\n%v", got, want)
	}

	// A zero clock: no entries (the field is omitted downstream).
	writeCorpus(t, dir, map[string]string{"app/util.py": "def helper():\n    return 2\n"})
	rep, err = p.Ingest(WithPassClock(context.Background(), PassClock{}), core.ServiceRef{ID: "passes", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.PassTimes != nil {
		t.Fatalf("PassTimes = %+v under a zero clock, want nil", rep.PassTimes)
	}
}
