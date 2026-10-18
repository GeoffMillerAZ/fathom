package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sizingReplay mirrors the three sizing sections of `readout replay --json`,
// decoded by their JSON names so the test holds the contract, not the types.
type sizingReplay struct {
	Dedupe struct {
		Bash struct {
			Results int   `json:"results"`
			Bytes   int64 `json:"bytes"`
			Exact   struct {
				WithinBatch sizingTestCount `json:"within_batch"`
				AcrossTurns sizingTestCount `json:"across_turns"`
			} `json:"exact"`
			RecoveryExcluded sizingTestCount `json:"recovery_excluded"`
		} `json:"bash"`
		Read struct {
			Results int   `json:"results"`
			Bytes   int64 `json:"bytes"`
			Exact   struct {
				WithinBatch sizingTestCount `json:"within_batch"`
				AcrossTurns sizingTestCount `json:"across_turns"`
			} `json:"exact"`
			Contained struct {
				WithinBatch sizingTestCount `json:"within_batch"`
				AcrossTurns sizingTestCount `json:"across_turns"`
			} `json:"contained"`
		} `json:"read"`
		Bytes        int64           `json:"bash_read_bytes"`
		Repeat       sizingTestCount `json:"repeat"`
		WithinBatch  sizingTestCount `json:"within_batch"`
		AcrossTurns  sizingTestCount `json:"across_turns"`
		Share        float64         `json:"share_of_bash_read_bytes"`
		RepeatShapes []struct {
			Shape string `json:"shape"`
			Calls int    `json:"calls"`
			Bytes int64  `json:"bytes"`
		} `json:"repeat_shapes"`
		KeepBar           float64 `json:"keep_bar"`
		VerdictProjection string  `json:"verdict_projection"`
	} `json:"dedupe"`
	Spawns struct {
		Workflow          sizingTestSpawns `json:"workflow"`
		Other             sizingTestSpawns `json:"other"`
		FirstCalls        int              `json:"first_calls"`
		Bar               float64          `json:"bar"`
		VerdictProjection string           `json:"verdict_projection"`
	} `json:"spawns"`
	Compactions struct {
		Days     int     `json:"days"`
		Events   int     `json:"events"`
		PerDay   float64 `json:"per_day"`
		Complete int     `json:"complete"`
		Rows     []struct {
			PreSet    int  `json:"pre_set"`
			Reread    int  `json:"reread"`
			PostTurns int  `json:"post_turns"`
			Complete  bool `json:"complete"`
			Subagent  bool `json:"subagent"`
		} `json:"per_event"`
		MedianPreSet      float64 `json:"median_pre_set"`
		MedianReread      float64 `json:"median_reread"`
		PerDayBar         float64 `json:"per_day_bar"`
		MedianBar         float64 `json:"median_bar"`
		VerdictProjection string  `json:"verdict_projection"`
	} `json:"compactions"`
}

type sizingTestCount struct {
	Results int   `json:"results"`
	Bytes   int64 `json:"bytes"`
}

type sizingTestSpawns struct {
	Subagents      int     `json:"subagents"`
	DiscoveryFirst int     `json:"discovery_first"`
	Share          float64 `json:"share"`
	Fewer          int     `json:"fewer_than_first_calls"`
}

// runSizingReplay copies the fixture transcripts (fresh mtimes, inside any
// window) and runs `readout replay` over them with args.
func runSizingReplay(t *testing.T, args ...string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "projects")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "readout-sizing"))); err != nil {
		t.Fatal(err)
	}
	rules := writeReadoutRules(t, dir, readoutTestRules)
	useReadoutReplayEnv(t, root, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	var stdout, stderr bytes.Buffer
	if code := cmdReadoutReplay(append([]string{"--rules", rules}, args...), &stdout, &stderr); code != 0 {
		t.Fatalf("readout replay %v exit %d: %s", args, code, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// TestCmdReadoutReplaySizesDedupeSpawnsAndCompactions holds the three sizing
// instruments over a fixture project: a main session with a compaction
// boundary, an old session whose compaction is outside the latest 14 days,
// two workflow subagents (one discovery-first), a workflow journal that is
// no subagent, and one Explore subagent.
func TestCmdReadoutReplaySizesDedupeSpawnsAndCompactions(t *testing.T) {
	out, _ := runSizingReplay(t, "--json")
	var rep sizingReplay
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}

	d := rep.Dedupe
	// Bash: cat, the parallel pair, go test twice, echo, printf, then ls and
	// grep after the boundary: 9 results, 139 bytes. Read: 5 results, 308.
	if d.Bash.Results != 9 || d.Bash.Bytes != 139 || d.Read.Results != 5 || d.Read.Bytes != 308 || d.Bytes != 447 {
		t.Errorf("dedupe totals = bash %d/%d read %d/%d all %d, want 9/139, 5/308, 447",
			d.Bash.Results, d.Bash.Bytes, d.Read.Results, d.Read.Bytes, d.Bytes)
	}
	// The parallel batch's second ls repeats the first within its message.
	if d.Bash.Exact.WithinBatch != (sizingTestCount{1, 17}) {
		t.Errorf("bash exact within batch = %+v, want the second ls, 17 bytes", d.Bash.Exact.WithinBatch)
	}
	// printf repeats echo's output a turn later; the ls after the boundary
	// repeats nothing (the boundary resets the session's results).
	if d.Bash.Exact.AcrossTurns != (sizingTestCount{1, 5}) {
		t.Errorf("bash exact across turns = %+v, want printf's 5 bytes alone", d.Bash.Exact.AcrossTurns)
	}
	// The second go test repeats the first within five Bash calls: a recovery.
	if d.Bash.RecoveryExcluded != (sizingTestCount{1, 16}) {
		t.Errorf("recovery excluded = %+v, want the second go test, 16 bytes", d.Bash.RecoveryExcluded)
	}
	// The span 2-3 inside the whole read, and beta's line 3 inside the cat;
	// the span read after the Edit is not contained.
	if d.Read.Contained.AcrossTurns != (sizingTestCount{2, 82}) || d.Read.Contained.WithinBatch != (sizingTestCount{}) {
		t.Errorf("read contained = %+v / %+v, want 2 across turns, 82 bytes", d.Read.Contained.AcrossTurns, d.Read.Contained.WithinBatch)
	}
	if d.Read.Exact.AcrossTurns != (sizingTestCount{}) || d.Read.Exact.WithinBatch != (sizingTestCount{}) {
		t.Errorf("read exact = %+v, want none", d.Read.Exact)
	}
	if d.Repeat != (sizingTestCount{4, 104}) || d.WithinBatch != (sizingTestCount{1, 17}) || d.AcrossTurns != (sizingTestCount{3, 87}) {
		t.Errorf("dedupe repeat %+v within %+v across %+v, want 4/104, 1/17, 3/87", d.Repeat, d.WithinBatch, d.AcrossTurns)
	}
	if d.Share != 0.232662 || d.KeepBar != 0.01 || d.VerdictProjection != "KEEP" {
		t.Errorf("dedupe share %v bar %v projection %q, want 0.232662, 0.01, KEEP", d.Share, d.KeepBar, d.VerdictProjection)
	}
	if len(d.RepeatShapes) != 2 || d.RepeatShapes[0].Shape != "ls" || d.RepeatShapes[0].Bytes != 17 || d.RepeatShapes[1].Shape != "printf" {
		t.Errorf("repeat shapes = %+v, want ls 17 then printf 5", d.RepeatShapes)
	}

	s := rep.Spawns
	if s.Workflow != (sizingTestSpawns{2, 1, 0.5, 0}) {
		t.Errorf("workflow spawns = %+v, want 2 subagents, 1 discovery-first (the journal is no subagent)", s.Workflow)
	}
	if s.Other != (sizingTestSpawns{1, 1, 1, 0}) {
		t.Errorf("other spawns = %+v, want the Explore subagent, discovery-first", s.Other)
	}
	if s.FirstCalls != 3 || s.Bar != 0.3 || s.VerdictProjection != "BUILD" {
		t.Errorf("spawns first %d bar %v projection %q, want 3, 0.3, BUILD", s.FirstCalls, s.Bar, s.VerdictProjection)
	}

	c := rep.Compactions
	if c.Days != 14 || c.Events != 1 || c.PerDay != 0.071429 || c.Complete != 1 {
		t.Errorf("compactions = days %d events %d per day %v complete %d, want 14, 1 (the old one is outside), 0.071429, 1",
			c.Days, c.Events, c.PerDay, c.Complete)
	}
	if len(c.Rows) != 1 || c.Rows[0].PreSet != 2 || c.Rows[0].Reread != 2 || c.Rows[0].PostTurns != 20 || !c.Rows[0].Complete || c.Rows[0].Subagent {
		t.Errorf("per event = %+v, want one main-session event: pre set 2, re-read 2, 20 post turns, complete", c.Rows)
	}
	if c.MedianPreSet != 2 || c.MedianReread != 2 || c.PerDayBar != 2 || c.MedianBar != 2 || c.VerdictProjection != "STOP" {
		t.Errorf("compaction medians %v/%v bars %v/%v projection %q, want 2, 2, 2, 2, STOP",
			c.MedianPreSet, c.MedianReread, c.PerDayBar, c.MedianBar, c.VerdictProjection)
	}

	text, _ := runSizingReplay(t)
	for _, want := range []string{"dedupe: ", "spawns: ", "compactions over the latest 14 days: "} {
		if !strings.Contains(text, want) {
			t.Errorf("text view lacks %q:\n%s", want, text)
		}
	}
	for _, view := range []string{out, text} {
		for _, leak := range []string{"secret", "alpha", "beta", "/work", "work-proj", "sess-fixture", "fixture", "Checking",
			"cb-main", "toolu_", "msg_", "awf", "aex", "Explore", "general-purpose", "journal", "package "} {
			if strings.Contains(view, leak) {
				t.Errorf("output carries input text %q", leak)
			}
		}
	}

	// --days bounds every section: a week back holds the Explore subagent alone.
	week, _ := runSizingReplay(t, "--json", "--days", "7")
	var wk sizingReplay
	if err := json.Unmarshal([]byte(week), &wk); err != nil {
		t.Fatal(err)
	}
	if wk.Dedupe.Bytes != 0 || wk.Compactions.Days != 7 || wk.Compactions.Events != 0 || wk.Compactions.Rows == nil ||
		wk.Spawns.Workflow.Subagents != 0 || wk.Spawns.Other.Subagents != 1 || wk.Dedupe.VerdictProjection != "STOP" {
		t.Errorf("--days 7 = dedupe %d bytes, compactions %d days %d events, spawns %d/%d, want 0, 7, 0, 0/1",
			wk.Dedupe.Bytes, wk.Compactions.Days, wk.Compactions.Events, wk.Spawns.Workflow.Subagents, wk.Spawns.Other.Subagents)
	}
}

// TestReplaySizingVerdictProjections holds each bar at its edge.
func TestReplaySizingVerdictProjections(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{sizingDedupeProjection(0.01), "KEEP"},
		{sizingDedupeProjection(0.009999), "STOP"},
		{sizingSpawnProjection(0.3), "BUILD"},
		{sizingSpawnProjection(0.299999), "STOP"},
		{sizingCompactionProjection(2, 2), "BUILD"},
		{sizingCompactionProjection(2, 1.5), "STOP"},
		{sizingCompactionProjection(1.9, 3), "STOP"},
	} {
		if tc.got != tc.want {
			t.Errorf("projection = %q, want %q", tc.got, tc.want)
		}
	}
	if m := sizingMedian([]int{3, 0, 1, 4}); m != 2 {
		t.Errorf("median of 0,1,3,4 = %v, want 2", m)
	}
}
