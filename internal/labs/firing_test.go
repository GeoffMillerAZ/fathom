package labs

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func firingLab(id string, surface ...string) Lab {
	lab := Lab{ID: id, Surface: surface}
	lab.Firing.Should, lab.Firing.ShouldNot, lab.Firing.Grader = "the rule", "the near miss", "tool_used"
	return lab
}

func naLab(id string, surface ...string) Lab {
	lab := Lab{ID: id, Surface: surface}
	lab.Firing.Mode, lab.Firing.Reason = "n/a", "always loaded"
	return lab
}

func firingPoints() []Point {
	return []Point{
		// A should-fire point the surface missed, served by Read.
		{Ordinal: 0, Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{number,number,string}", ToolUseID: "a", Result: CallResult{Present: true, Bytes: 10}}},
		// The surface fired at an eligible point and delivered.
		{Ordinal: 1, Call: Call{Kind: "tool_use", Name: "mcp__fathom__read_source", Shape: "object{number,number,string,string}", ToolUseID: "b", Result: CallResult{Present: true, Bytes: 20}}},
		// It fired again and errored: fired, not delivered.
		{Ordinal: 2, Call: Call{Kind: "tool_use", Name: "mcp__fathom__read_source", Shape: "object{number,number,string,string}", ToolUseID: "c", Result: CallResult{Present: true, IsError: true, Bytes: 5}}},
		// A should-not point: no fathom surface competes with it.
		{Ordinal: 3, Call: Call{Kind: "tool_use", Name: "TodoWrite", Shape: "object{string}", ToolUseID: "d"}},
		// Another missed should-fire point, served by Read.
		{Ordinal: 4, Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{number,number,string}", ToolUseID: "e", Result: CallResult{Present: true, Bytes: 7}}},
	}
}

func readingFor(t *testing.T, readings []FiringReadingRow, lab string) FiringReadingRow {
	t.Helper()
	for _, r := range readings {
		if r.Lab == lab {
			return r
		}
	}
	t.Fatalf("no reading for %s: %+v", lab, readings)
	return FiringReadingRow{}
}

func TestFiringReadingCountsTheFunnel(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-read-source", "mcp:read_source"), firingLab("lab-search", "mcp:search")})
	if len(readings) != 2 || readings[0].Lab != "lab-read-source" || readings[1].Lab != "lab-search" {
		t.Fatalf("readings are not sorted by lab: %+v", readings)
	}
	got := readingFor(t, readings, "lab-read-source")
	if got.Eligible != 4 || got.Fired != 2 || got.FiredAtEligible != 2 {
		t.Fatalf("eligible/fired: %+v", got)
	}
	if got.ShouldNot != 1 || got.FalseFires != 0 {
		t.Fatalf("should-not margin: %+v", got)
	}
	if got.Delivered != 1 || got.DeliveredOf != 2 {
		t.Fatalf("delivered share: %+v", got)
	}
	// It fired at two of the four eligible points: recall 0.5 with a
	// Wilson interval, never a bare count.
	if got.Rates.TPR.Value != 0.5 || math.IsNaN(got.Rates.TPR.Hi) {
		t.Fatalf("recall: %+v", got.Rates.TPR)
	}
	// No false fire at one should-not point: specificity is 1.
	if got.Rates.TNR.Value != 1 {
		t.Fatalf("specificity: %+v", got.Rates.TNR)
	}
}

func TestFiringReadingNamesMissedSubstitutes(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-read-source", "mcp:read_source")})
	got := readingFor(t, readings, "lab-read-source")
	want := []SubstituteCount{{Tool: "Read", Count: 2}}
	if len(got.Missed) != len(want) || got.Missed[0] != want[0] {
		t.Fatalf("missed substitutes: %+v, want %+v", got.Missed, want)
	}
}

// Stages are a #Stage or a named "not measured", never a bare zero.
func TestFiringReadingStagesAreNeverAMeasuredZero(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-read-source", "mcp:read_source"), firingLab("lab-search", "mcp:search"), naLab("lab-stanza", "stanza:agents-md"), firingLab("lab-obligations", "hook:edit-hook [post-tool/obligations-post-edit PostToolUse matcher=Edit|Write|MultiEdit]")})
	for _, r := range readings {
		for name, stage := range map[string]any{"eligible": r.Stages.Eligible, "fired": r.Stages.Fired, "delivered": r.Stages.Delivered, "acted": r.Stages.Acted, "helped": r.Stages.Helped} {
			switch s := stage.(type) {
			case string:
				if !strings.HasPrefix(s, "not measured: ") {
					t.Fatalf("%s %s stage: %q", r.Lab, name, s)
				}
			case Stage:
				if s.N <= 0 {
					t.Fatalf("%s %s stage over no points: %+v", r.Lab, name, s)
				}
			default:
				t.Fatalf("%s %s stage: %T", r.Lab, name, stage)
			}
		}
		if got := r.Stages.Acted; got != "not measured: AD-21/RO-M4 not built" {
			t.Fatalf("%s acted: %v", r.Lab, got)
		}
	}
	// A hook surface reads AD-20's eligible log, which is not built.
	hook := readingFor(t, readings, "lab-obligations")
	if got := hook.Stages.Eligible; got != "not measured: AD-20 eligible log not built" {
		t.Fatalf("hook eligible: %v", got)
	}
	if got := hook.Stages.Fired; got != "not measured: AD-20 eligible log not built" {
		t.Fatalf("hook fired: %v", got)
	}
	// A lab with no firing question reads n/a, not a zero.
	na := readingFor(t, readings, "lab-stanza")
	if got := na.Stages.Fired; got != "not measured: firing is n/a: always loaded" {
		t.Fatalf("n/a fired: %v", got)
	}
	// A surface no point touched has nothing to divide by.
	quiet := readingFor(t, readings, "lab-search")
	if got := quiet.Stages.Fired; got != "not measured: no point names mcp:search" {
		t.Fatalf("untouched fired: %v", got)
	}
}

func TestFiringReadingRowsAreLedgerRows(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-read-source", "mcp:read_source")})
	row := readings[0].ResultRow("run-1", "firing", strings.Repeat("a", 64), ReplayProvenance{CorpusSHA: strings.Repeat("b", 64), FathomCommit: "0123456789ab"})
	if row["arm"] != "replay" || row["spend_usd"] != 0.0 || row["spend_label"] != "replay" || row["task_id"] != "mcp:read_source" {
		t.Fatalf("replay row: %+v", row)
	}
	if _, ok := row["pins"]; ok {
		t.Fatalf("a replay row cannot pin what it does not know: %+v", row)
	}
	if row["replay"].(ReplayProvenance).CorpusSHA == "" {
		t.Fatalf("replay provenance lost: %+v", row)
	}
	evaluation := row["evaluation"].(map[string]any)
	if evaluation["verdict"] != "indeterminate" || evaluation["criterion_id"] != "firing" {
		t.Fatalf("evaluation: %+v", evaluation)
	}
}

// A rendered row is a #Result the ledger accepts, under the arm whose
// provenance replay can prove.
func TestFiringReadingRowAppendsToTheLedger(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-read-source", "mcp:read_source")})
	readings[0].Feature = "F-08"
	row := readings[0].ResultRow("run-1", "firing", strings.Repeat("a", 64), ReplayProvenance{CorpusSHA: strings.Repeat("b", 64), FathomCommit: "0123456789ab"})
	path := filepath.Join(t.TempDir(), "docs", "bench", "labs-ledger.jsonl")
	if err := AppendLabResult(path, row); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := AppendLabResult(path, row); err == nil {
		t.Fatal("a second row under one key was accepted")
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(written), `{"run_id":"run-1","task_id":"mcp:read_source","arm":"replay"`) {
		t.Fatalf("ledger keys not first: %s", written)
	}
}

// One row per (run, surface): a lab with two surfaces writes two rows.
func TestFiringReadingRowPerSurface(t *testing.T) {
	lab := firingLab("lab-verify-plan", "mcp:verify_plan", "hook:stop-hook [stop/stop-verify Stop matcher=]")
	readings := FiringReading(firingPoints(), []Lab{lab})
	if len(readings) != 2 {
		t.Fatalf("one reading per surface: %+v", readings)
	}
	if readings[0].Surface != "hook:stop-hook [stop/stop-verify Stop matcher=]" || readings[1].Surface != "mcp:verify_plan" {
		t.Fatalf("surfaces are not sorted: %+v", readings)
	}
}

// M3: a surface that fired where v0 classifies no point should-fire has no
// eligible margin: eligible and fired read "not measured", never 0 of n.
func TestFiringReadingNoEligiblePointIsNotAMeasuredZero(t *testing.T) {
	points := []Point{{Call: Call{Kind: "tool_use", Name: "mcp__fathom__find_symbol", Shape: "object{string}", ToolUseID: "a", Result: CallResult{Present: true, Bytes: 4}}}}
	got := readingFor(t, FiringReading(points, []Lab{firingLab("lab-find", "mcp:find_symbol")}), "lab-find")
	if got.Eligible != 0 || got.Fired != 1 {
		t.Fatalf("setup: %+v", got)
	}
	want := "not measured: v0 classifies no point should-fire for mcp:find_symbol"
	if got.Stages.Eligible != want || got.Stages.Fired != want {
		t.Fatalf("eligible %v, fired %v; want %q", got.Stages.Eligible, got.Stages.Fired, want)
	}
}

// B2: the reading is JSON-encodable even where a rate has no data (NaN).
func TestFiringReadingEncodesAsJSON(t *testing.T) {
	readings := FiringReading(firingPoints(), []Lab{firingLab("lab-search", "mcp:search")})
	if _, err := json.Marshal(readings); err != nil {
		t.Fatalf("a reading with an empty rate cannot be encoded: %v", err)
	}
}

// M4: an unbounded read_source call is open discovery, a should-not the
// surface fired at: a false fire, and specificity below 1.
func TestFiringReadingCountsAFalseFire(t *testing.T) {
	points := []Point{
		{Call: Call{Kind: "tool_use", Name: "mcp__fathom__read_source", Shape: "object{string,string}", ToolUseID: "a", Result: CallResult{Present: true, Bytes: 9}}},
		{Call: Call{Kind: "tool_use", Name: "Read", Shape: "object{number,number,string}", ToolUseID: "b", Result: CallResult{Present: true, Bytes: 9}}},
	}
	got := readingFor(t, FiringReading(points, []Lab{firingLab("lab-read-source", "mcp:read_source")}), "lab-read-source")
	if got.FalseFires != 1 || got.ShouldNot != 1 || got.Rates.TNR.Value >= 1 {
		t.Fatalf("false fire not counted: %+v", got)
	}
}
