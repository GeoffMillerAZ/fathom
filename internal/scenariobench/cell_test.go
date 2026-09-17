package scenariobench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func testInvariants() CellInvariants {
	return CellInvariants{
		CorpusSHA256:   "abc123",
		TaskID:         "T1",
		Arm:            "full",
		ModelID:        "sonnet",
		HarnessVersion: "2.1.0",
		Scoring:        "oracle-v1",
		PromptSubject:  "the fathom codebase",
		ArmSurface:     "deadbeef",
	}
}

func TestScenarioCellKey_Deterministic(t *testing.T) {
	inv := testInvariants()
	k1 := ScenarioCellKey(inv)
	k2 := ScenarioCellKey(inv)
	if k1 != k2 {
		t.Fatalf("expected a stable key, got %s then %s", k1, k2)
	}
	if len(k1) != 64 {
		t.Fatalf("expected a hex sha256 key, got len %d: %s", len(k1), k1)
	}
}

func TestScenarioCellKey_DiffersOnEachInvariant(t *testing.T) {
	base := testInvariants()
	baseKey := ScenarioCellKey(base)

	variants := []func(*CellInvariants){
		func(c *CellInvariants) { c.CorpusSHA256 = "different" },
		func(c *CellInvariants) { c.TaskID = "T2" },
		func(c *CellInvariants) { c.Arm = "control" },
		func(c *CellInvariants) { c.ModelID = "opus" },
		func(c *CellInvariants) { c.HarnessVersion = "2.2.0" },
		func(c *CellInvariants) { c.Scoring = "oracle-v2" },
		func(c *CellInvariants) { c.PromptSubject = "a different codebase" },
		func(c *CellInvariants) { c.ArmSurface = "cafef00d" },
	}
	for i, mutate := range variants {
		v := base
		mutate(&v)
		if ScenarioCellKey(v) == baseKey {
			t.Errorf("variant %d: expected a different key when an invariant changes", i)
		}
	}
}

func TestCellInvariants_ValidateRefusesEmptyField(t *testing.T) {
	inv := testInvariants()
	inv.ModelID = ""
	if err := inv.Validate(); err == nil {
		t.Fatal("expected Validate to refuse an empty model_id")
	}
	// ArmSurface may be empty (a pure control) — Validate must not refuse it.
	inv2 := testInvariants()
	inv2.ArmSurface = ""
	if err := inv2.Validate(); err != nil {
		t.Errorf("Validate refused an empty ArmSurface (a valid control): %v", err)
	}
}

func TestResolveArmSurface(t *testing.T) {
	cases := []struct {
		name    string
		surface string
		want    string
	}{
		{"control", "", ""},
		{"wired", "fathom", "deadbeef"},
		{"competitor", "graphify-v1.2.3", "graphify-v1.2.3"},
	}
	for _, c := range cases {
		got := ResolveArmSurface(Arm{Surface: c.surface}, "deadbeef")
		if got != c.want {
			t.Errorf("%s: ResolveArmSurface = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestStoreAndBorrowScenarioCell(t *testing.T) {
	dir := t.TempDir()
	inv := testInvariants()
	key := ScenarioCellKey(inv)
	cell := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1", Success: true}

	if err := StoreScenarioCell(dir, cell); err != nil {
		t.Fatalf("StoreScenarioCell: %v", err)
	}

	got, ok, err := BorrowScenarioCell(dir, key, 1)
	if err != nil {
		t.Fatalf("BorrowScenarioCell: %v", err)
	}
	if !ok {
		t.Fatal("expected the stored cell to be found")
	}
	if !got.Borrowed {
		t.Error("expected Borrowed=true on a returned cell")
	}
	if got.SourceRunID != "run-1" {
		t.Errorf("SourceRunID = %q, want run-1", got.SourceRunID)
	}
	if got.TaskID != "T1" || !got.Success {
		t.Errorf("unexpected cell content: %+v", got)
	}

	_, ok, err = BorrowScenarioCell(dir, key, 2)
	if err != nil {
		t.Fatalf("BorrowScenarioCell trial 2: %v", err)
	}
	if ok {
		t.Error("expected no cell for an un-stored trial")
	}
}

func TestStoreScenarioCell_RequiresKey(t *testing.T) {
	dir := t.TempDir()
	if err := StoreScenarioCell(dir, CellResult{TaskID: "T1", Arm: "full"}); err == nil {
		t.Fatal("expected an error for a cell with no CellKey")
	}
}

func TestStoreScenarioCell_FirstWriterWins(t *testing.T) {
	dir := t.TempDir()
	key := ScenarioCellKey(testInvariants())
	first := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1"}
	second := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-2"}

	if err := StoreScenarioCell(dir, first); err != nil {
		t.Fatalf("storing first: %v", err)
	}
	if err := StoreScenarioCell(dir, second); err != nil {
		t.Fatalf("storing second: %v", err)
	}
	got, ok, err := BorrowScenarioCell(dir, key, 1)
	if err != nil || !ok {
		t.Fatalf("borrowing: ok=%v err=%v", ok, err)
	}
	if got.SourceRunID != "run-1" {
		t.Errorf("expected the FIRST writer's run to win, got %q", got.SourceRunID)
	}
}

func TestBorrowScenarioCell_ReadErrorIsNotAMiss(t *testing.T) {
	dir := t.TempDir()
	key := "somekey"
	path := ScenarioCellPath(dir, key, 1)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "not json")
	if _, _, err := BorrowScenarioCell(dir, key, 1); err == nil {
		t.Fatal("expected an error for an unparseable cell file, not a miss")
	}
}

func TestAcceptBorrowedScenario(t *testing.T) {
	cell := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: "want-key", Borrowed: true, SourceRunID: "run-1"}
	if err := AcceptBorrowedScenario("want-key", cell); err != nil {
		t.Errorf("expected acceptance on matching keys: %v", err)
	}
	if err := AcceptBorrowedScenario("other-key", cell); err == nil {
		t.Error("expected refusal on mismatched keys")
	}

	notBorrowed := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: "anything"}
	if err := AcceptBorrowedScenario("want-key", notBorrowed); err != nil {
		t.Errorf("a non-borrowed cell should always be accepted: %v", err)
	}
}

func TestAggregateReadings_SumsToolCalls(t *testing.T) {
	sessions := []SessionResult{
		{ToolCalls: map[string]int{"Read": 2, "mcp__fathom__find_symbol": 1}},
		{ToolCalls: map[string]int{"Read": 1, "mcp__fathom__references": 3}},
	}
	got := AggregateReadings(sessions)
	want := map[string]int{"Read": 3, "mcp__fathom__find_symbol": 1, "mcp__fathom__references": 3}
	if len(got.ToolCalls) != len(want) {
		t.Fatalf("ToolCalls = %v, want %v", got.ToolCalls, want)
	}
	for name, n := range want {
		if got.ToolCalls[name] != n {
			t.Errorf("ToolCalls[%q] = %d, want %d", name, got.ToolCalls[name], n)
		}
	}
}

func TestAggregateReadings_SumsHooksFired(t *testing.T) {
	sessions := []SessionResult{
		{HooksFired: map[string]int{"SessionStart": 1}},
		{HooksFired: map[string]int{"SessionStart": 1, "PostToolUse": 2}},
		{}, // no hook_response at all — must not panic or introduce a false 0
	}
	got := AggregateReadings(sessions)
	want := map[string]int{"SessionStart": 2, "PostToolUse": 2}
	if len(got.HooksFired) != len(want) {
		t.Fatalf("HooksFired = %v, want %v", got.HooksFired, want)
	}
	for name, n := range want {
		if got.HooksFired[name] != n {
			t.Errorf("HooksFired[%q] = %d, want %d", name, got.HooksFired[name], n)
		}
	}
}

func TestReadingsCompileCost_NilWhenWorktreeNeverCompiled(t *testing.T) {
	if got := readingsCompileCost(Worktree{}); got != nil {
		t.Errorf("readingsCompileCost(no CompileCost) = %+v, want nil", got)
	}
}

func TestReadingsCompileCost_CopiesWorktreeCompileCost(t *testing.T) {
	wt := Worktree{CompileCost: &CompileCost{TokensIn: 100, TokensOut: 50, CostUSD: 0.02, Cached: true}}
	got := readingsCompileCost(wt)
	if got == nil {
		t.Fatal("expected a non-nil CompileCostReading")
	}
	if got.TokensIn != 100 || got.TokensOut != 50 || got.CostUSD != 0.02 || !got.Cached {
		t.Errorf("readingsCompileCost = %+v, want {100 50 0.02 true}", got)
	}
}

func TestApplyCompileCost_RollsUSDIntoTotalCostUSD(t *testing.T) {
	wt := Worktree{CompileCost: &CompileCost{CostUSD: 0.5}}
	r := Readings{TotalCostUSD: 1.0}
	ApplyCompileCost(&r, wt)
	if r.CompileCost == nil || r.CompileCost.CostUSD != 0.5 {
		t.Errorf("expected r.CompileCost.CostUSD = 0.5, got %+v", r.CompileCost)
	}
	if r.CompileCostUSD != 0.5 {
		t.Errorf("r.CompileCostUSD = %v, want 0.5", r.CompileCostUSD)
	}
	if r.TotalCostUSD != 1.5 {
		t.Errorf("r.TotalCostUSD = %v, want 1.5 (1.0 sessions + 0.5 compile)", r.TotalCostUSD)
	}
}

func TestApplyCompileCost_NoOpWhenNeverCompiled(t *testing.T) {
	r := Readings{TotalCostUSD: 1.0}
	ApplyCompileCost(&r, Worktree{})
	if r.CompileCost != nil {
		t.Errorf("expected CompileCost to stay nil, got %+v", r.CompileCost)
	}
	if r.TotalCostUSD != 1.0 {
		t.Errorf("expected TotalCostUSD unchanged, got %v", r.TotalCostUSD)
	}
}

func TestSurfaceToolPrefix(t *testing.T) {
	cases := []struct{ surface, want string }{
		{"", ""},
		{"fathom", "mcp__fathom__"},
		{"graphify@v0.9.61", "mcp__graphify__"},
		{"serena@v1.7.0", "mcp__serena__"},
	}
	for _, c := range cases {
		if got := SurfaceToolPrefix(c.surface); got != c.want {
			t.Errorf("SurfaceToolPrefix(%q) = %q, want %q", c.surface, got, c.want)
		}
	}
}

func TestSurfaceCallCount(t *testing.T) {
	toolCalls := map[string]int{
		"Read":                     5,
		"mcp__graphify__query":     2,
		"mcp__graphify__neighbors": 1,
		"mcp__fathom__find_symbol": 7,
	}
	cases := []struct {
		surface string
		want    int
	}{
		{"", 0},
		{"graphify@v0.9.61", 3},
		{"fathom", 7},
		{"serena@v1.7.0", 0},
	}
	for _, c := range cases {
		if got := SurfaceCallCount(toolCalls, c.surface); got != c.want {
			t.Errorf("SurfaceCallCount(surface=%q) = %d, want %d", c.surface, got, c.want)
		}
	}
}

// intPtr/boolPtr are tiny test helpers for the *int/*bool reading fields.
func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }

// TestAggregateRowReadings_TwoTrialsSurviveEveryField is item 1's own
// proof: a ledger row built from two trials whose per-cell readings
// differ in every field must carry every one of them — counts summed,
// ratios averaged with n, and the full per-trial detail preserved
// verbatim — never the pre-fix defect where only total_tokens/turns/
// cost/payload_bytes/tool_calls/surface_calls survived.
func TestAggregateRowReadings_TwoTrialsSurviveEveryField(t *testing.T) {
	cells := []CellResult{
		{
			Trial: 1,
			Readings: Readings{
				TotalTokens: 100, TotalTurns: 5, TotalCostUSD: 1.0, PayloadBytes: 500,
				CompileCostUSD:       0.10,
				ToolCalls:            map[string]int{"Read": 2},
				HooksFired:           map[string]int{"SessionStart": 1},
				FactsReused:          intPtr(3),
				Violation:            boolPtr(true),
				RuleServedBeforeEdit: boolPtr(true),
				Overlap:              &OverlapReading{Workers: 2, Ratio: 0.4},
				Resumption:           &ResumptionReading{TokensToFirstEditAll: 200},
				PlanCoverage:         &PlanCoverageReading{Fraction: 0.5},
				FalseDone:            &FalseDoneReading{Claimed: 2, Refuted: 1},
				Drift:                &DriftReading{Rows: 3},
				GoldSourceTool:       "Read",
			},
		},
		{
			Trial: 2,
			Readings: Readings{
				TotalTokens: 50, TotalTurns: 3, TotalCostUSD: 0.5, PayloadBytes: 250,
				CompileCostUSD:       0.05,
				ToolCalls:            map[string]int{"Read": 1, "Grep": 4},
				HooksFired:           map[string]int{"SessionStart": 1, "PostToolUse": 2},
				FactsReused:          intPtr(1),
				Violation:            boolPtr(false),
				RuleServedBeforeEdit: boolPtr(false),
				Overlap:              &OverlapReading{Workers: 3, Ratio: 0.6},
				Resumption:           &ResumptionReading{TokensToFirstEditAll: 400},
				PlanCoverage:         &PlanCoverageReading{Fraction: 0.9},
				FalseDone:            &FalseDoneReading{Claimed: 1, Refuted: 0},
				Drift:                &DriftReading{Rows: 1},
				GoldSourceTool:       GoldSourceNone,
			},
		},
	}

	got, trials := AggregateRowReadings(cells)

	if got.TotalTokens != 150 || got.TotalTurns != 8 || got.TotalCostUSD != 1.5 || got.PayloadBytes != 750 {
		t.Errorf("basic sums = %+v, want tokens=150 turns=8 cost=1.5 payload=750", got)
	}
	if diff := got.CompileCostUSD - 0.15; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CompileCostUSD = %v, want ~0.15", got.CompileCostUSD)
	}
	if got.ToolCalls["Read"] != 3 || got.ToolCalls["Grep"] != 4 {
		t.Errorf("ToolCalls = %v, want Read=3 Grep=4", got.ToolCalls)
	}
	if got.HooksFired["SessionStart"] != 2 || got.HooksFired["PostToolUse"] != 2 {
		t.Errorf("HooksFired = %v, want SessionStart=2 PostToolUse=2", got.HooksFired)
	}
	if got.FactsReused == nil || *got.FactsReused != 4 {
		t.Errorf("FactsReused = %v, want 4", got.FactsReused)
	}
	if got.ViolationTrials == nil || *got.ViolationTrials != 1 {
		t.Errorf("ViolationTrials = %v, want 1 (one of two trials violated)", got.ViolationTrials)
	}
	if got.RuleServedBeforeEditTrials == nil || *got.RuleServedBeforeEditTrials != 1 {
		t.Errorf("RuleServedBeforeEditTrials = %v, want 1", got.RuleServedBeforeEditTrials)
	}
	if got.OverlapRatioN != 2 || got.OverlapRatioMean != 0.5 {
		t.Errorf("Overlap ratio mean/n = %v/%v, want 0.5/2", got.OverlapRatioMean, got.OverlapRatioN)
	}
	if got.ResumptionTokensToFirstEditN != 2 || got.ResumptionTokensToFirstEditMean != 300 {
		t.Errorf("Resumption mean/n = %v/%v, want 300/2", got.ResumptionTokensToFirstEditMean, got.ResumptionTokensToFirstEditN)
	}

	if len(trials) != 2 {
		t.Fatalf("expected 2 per-trial entries, got %d", len(trials))
	}
	if trials[0].Trial != 1 || trials[1].Trial != 2 {
		t.Fatalf("expected per-trial entries sorted by trial, got %+v", trials)
	}
	if trials[0].GoldSourceTool != "Read" || trials[1].GoldSourceTool != GoldSourceNone {
		t.Errorf("per-trial GoldSourceTool = %q/%q, want Read/none", trials[0].GoldSourceTool, trials[1].GoldSourceTool)
	}
	if trials[0].PlanCoverage == nil || trials[0].PlanCoverage.Fraction != 0.5 {
		t.Errorf("trial 1 PlanCoverage = %+v, want Fraction=0.5", trials[0].PlanCoverage)
	}
	if trials[1].FalseDone == nil || trials[1].FalseDone.Claimed != 1 {
		t.Errorf("trial 2 FalseDone = %+v, want Claimed=1", trials[1].FalseDone)
	}
	if trials[0].Drift == nil || trials[0].Drift.Rows != 3 {
		t.Errorf("trial 1 Drift = %+v, want Rows=3", trials[0].Drift)
	}
	if trials[1].Overlap == nil || trials[1].Overlap.Workers != 3 {
		t.Errorf("trial 2 Overlap = %+v, want Workers=3", trials[1].Overlap)
	}
	if trials[0].Resumption == nil || trials[0].Resumption.TokensToFirstEditAll != 200 {
		t.Errorf("trial 1 Resumption = %+v, want TokensToFirstEditAll=200", trials[0].Resumption)
	}
}

// TestAggregateRowReadings_NeverMeasuredStaysNil covers a row whose
// trials never measured a reading at all (a plain public-corpus task with
// no Rule, no ModePlannerWorkers): the corresponding row-level field must
// stay nil/zero, never a false 0 that would read as "measured, zero".
func TestAggregateRowReadings_NeverMeasuredStaysNil(t *testing.T) {
	cells := []CellResult{
		{Trial: 1, Readings: Readings{TotalTokens: 10}},
		{Trial: 2, Readings: Readings{TotalTokens: 20}},
	}
	got, trials := AggregateRowReadings(cells)
	if got.ViolationTrials != nil {
		t.Errorf("ViolationTrials = %v, want nil", got.ViolationTrials)
	}
	if got.RuleServedBeforeEditTrials != nil {
		t.Errorf("RuleServedBeforeEditTrials = %v, want nil", got.RuleServedBeforeEditTrials)
	}
	if got.FactsReused != nil {
		t.Errorf("FactsReused = %v, want nil", got.FactsReused)
	}
	if got.OverlapRatioN != 0 {
		t.Errorf("OverlapRatioN = %d, want 0", got.OverlapRatioN)
	}
	if got.ResumptionTokensToFirstEditN != 0 {
		t.Errorf("ResumptionTokensToFirstEditN = %d, want 0", got.ResumptionTokensToFirstEditN)
	}
	if len(trials) != 2 {
		t.Fatalf("expected 2 per-trial entries even with nothing structured measured, got %d", len(trials))
	}
}

func TestStoredScenarioTrials_ListsAndSorts(t *testing.T) {
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "somekey")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"t1.json", "t5.json", "t2.json", "not-a-trial.json"} {
		if err := os.WriteFile(filepath.Join(keyDir, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := StoredScenarioTrials(dir, "somekey")
	if err != nil {
		t.Fatalf("StoredScenarioTrials: %v", err)
	}
	want := []int{1, 2, 5}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %d, want %d", i, got[i], w)
		}
	}
}

func TestStoredScenarioTrials_MissingKeyIsEmptyNotError(t *testing.T) {
	got, err := StoredScenarioTrials(t.TempDir(), "does-not-exist")
	if err != nil {
		t.Fatalf("expected no error for a missing cell dir, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil trials, got %v", got)
	}
}

func TestBorrowedScenarioCellsOf_SortedByTaskArmTrial(t *testing.T) {
	results := []CellResult{
		{TaskID: "T2", Arm: "full", Trial: 1, Borrowed: true, CellKey: "k1", SourceRunID: "r1"},
		{TaskID: "T1", Arm: "full", Trial: 2, Borrowed: true, CellKey: "k2", SourceRunID: "r1"},
		{TaskID: "T1", Arm: "control", Trial: 1, Borrowed: true, CellKey: "k3", SourceRunID: "r1"},
		{TaskID: "T1", Arm: "control", Trial: 1, Borrowed: false, CellKey: "k4"},
	}
	got := BorrowedScenarioCellsOf(results)
	if len(got) != 3 {
		t.Fatalf("expected 3 borrowed entries (non-borrowed excluded), got %d", len(got))
	}
	want := []string{"T1/control/1", "T1/full/2", "T2/full/1"}
	for i, w := range want {
		gotID := got[i].TaskID + "/" + got[i].Arm + "/" + strconv.Itoa(got[i].Trial)
		if gotID != w {
			t.Errorf("index %d: got %s, want %s", i, gotID, w)
		}
	}
}

// TestOverwriteScenarioCell_RewritesInPlace is O53 defect 7's own proof
// at the cell-store layer: an EXISTING cell record can be rewritten
// (unlike StoreScenarioCell's write-once contract).
func TestOverwriteScenarioCell_RewritesInPlace(t *testing.T) {
	dir := t.TempDir()
	key := ScenarioCellKey(testInvariants())
	cell := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1", Success: true}
	if err := StoreScenarioCell(dir, cell); err != nil {
		t.Fatalf("StoreScenarioCell: %v", err)
	}

	cell.InstrumentDefect = "the operator daemon was reachable"
	if err := OverwriteScenarioCell(dir, cell); err != nil {
		t.Fatalf("OverwriteScenarioCell: %v", err)
	}

	got, ok, err := BorrowScenarioCell(dir, key, 1)
	if err != nil || !ok {
		t.Fatalf("borrowing after overwrite: ok=%v err=%v", ok, err)
	}
	if got.InstrumentDefect != "the operator daemon was reachable" {
		t.Errorf("InstrumentDefect = %q, want it set", got.InstrumentDefect)
	}
	if !got.Success {
		t.Error("expected every other field (Success) to survive the overwrite untouched")
	}
}

// TestOverwriteScenarioCell_RefusesWhenAbsent: unlike StoreScenarioCell,
// there is nothing to silently create — a cell that was never stored
// cannot be overwritten.
func TestOverwriteScenarioCell_RefusesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	key := ScenarioCellKey(testInvariants())
	cell := CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1"}
	if err := OverwriteScenarioCell(dir, cell); err == nil {
		t.Fatal("expected an error overwriting a cell that was never stored")
	}
}

// TestOverwriteScenarioCell_RequiresKey mirrors StoreScenarioCell's own
// requirement.
func TestOverwriteScenarioCell_RequiresKey(t *testing.T) {
	dir := t.TempDir()
	if err := OverwriteScenarioCell(dir, CellResult{TaskID: "T1", Arm: "full"}); err == nil {
		t.Fatal("expected an error for a cell with no CellKey")
	}
}

// TestFindScenarioCellsByRun_MatchesExactTriple is O53 defect 7's own
// lookup proof: only cells matching (task_id, arm, run_id) EXACTLY come
// back, sorted by trial — a same-task different-arm cell, a same-arm
// different-task cell, and a same-(task,arm) different-run cell are all
// excluded.
func TestFindScenarioCellsByRun_MatchesExactTriple(t *testing.T) {
	dir := t.TempDir()
	key := ScenarioCellKey(testInvariants())
	otherArmKey := ScenarioCellKey(CellInvariants{CorpusSHA256: "abc123", TaskID: "T1", Arm: "control", ModelID: "sonnet", HarnessVersion: "2.1.0", Scoring: "oracle-v1", PromptSubject: "x"})
	otherTaskKey := ScenarioCellKey(CellInvariants{CorpusSHA256: "abc123", TaskID: "T2", Arm: "full", ModelID: "sonnet", HarnessVersion: "2.1.0", Scoring: "oracle-v1", PromptSubject: "x"})

	for _, c := range []CellResult{
		{TaskID: "T1", Arm: "full", Trial: 2, CellKey: key, RunID: "run-1"},
		{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-1"},
		{TaskID: "T1", Arm: "control", Trial: 1, CellKey: otherArmKey, RunID: "run-1"},
		{TaskID: "T2", Arm: "full", Trial: 1, CellKey: otherTaskKey, RunID: "run-1"},
		{TaskID: "T1", Arm: "full", Trial: 1, CellKey: key, RunID: "run-2"},
	} {
		if err := StoreScenarioCell(dir, c); err != nil {
			t.Fatalf("StoreScenarioCell %+v: %v", c, err)
		}
	}

	got, err := FindScenarioCellsByRun(dir, "T1", "full", "run-1")
	if err != nil {
		t.Fatalf("FindScenarioCellsByRun: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matching cells, got %d: %+v", len(got), got)
	}
	if got[0].Trial != 1 || got[1].Trial != 2 {
		t.Errorf("expected trial order [1,2], got [%d,%d]", got[0].Trial, got[1].Trial)
	}
}

// TestFindScenarioCellsByRun_NoMatchIsEmptyNotError covers the miss case.
func TestFindScenarioCellsByRun_NoMatchIsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	got, err := FindScenarioCellsByRun(dir, "T1", "full", "run-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

// TestFindScenarioCellsByRun_MissingCellsDirIsEmptyNotError: a cellsDir
// that does not exist yet (no cell has ever been stored) is zero
// results, not an error.
func TestFindScenarioCellsByRun_MissingCellsDirIsEmptyNotError(t *testing.T) {
	got, err := FindScenarioCellsByRun(filepath.Join(t.TempDir(), "does-not-exist"), "T1", "full", "run-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

// TestHarnessVersionsForRun_DistinctSortedAcrossTaskArm is O65 (wave N12)
// item 3's own proof: a cell key is built over HarnessVersion, so a
// mismatched -harness-version at filing time borrows nothing under ANY
// (task, arm) — this is the diagnostic that tells that case apart from a
// run-id with genuinely no cells: every DISTINCT HarnessVersion recorded
// for the run, across every task/arm, sorted, duplicates collapsed.
func TestHarnessVersionsForRun_DistinctSortedAcrossTaskArm(t *testing.T) {
	dir := t.TempDir()
	for i, c := range []CellResult{
		{TaskID: "T1", Arm: "full", Trial: 1, CellKey: "k1", RunID: "run-1", HarnessVersion: "2.1.270 (Claude Code)"},
		{TaskID: "T1", Arm: "control", Trial: 1, CellKey: "k2", RunID: "run-1", HarnessVersion: "2.1.270 (Claude Code)"},
		{TaskID: "T2", Arm: "full", Trial: 1, CellKey: "k3", RunID: "run-1", HarnessVersion: "2.1.269 (Claude Code)"},
		{TaskID: "T1", Arm: "full", Trial: 1, CellKey: "k4", RunID: "run-2", HarnessVersion: "9.9.9 (Claude Code)"},
	} {
		c.Trial = i + 1 // distinct trial per record so the store never collides on one path
		if err := StoreScenarioCell(dir, c); err != nil {
			t.Fatalf("StoreScenarioCell %+v: %v", c, err)
		}
	}
	got, err := HarnessVersionsForRun(dir, "run-1")
	if err != nil {
		t.Fatalf("HarnessVersionsForRun: %v", err)
	}
	want := []string{"2.1.269 (Claude Code)", "2.1.270 (Claude Code)"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("HarnessVersionsForRun(run-1) = %v, want %v", got, want)
	}
}

// TestHarnessVersionsForRun_NoMatchIsEmptyNotError covers the genuine
// "this run-id has no cells at all" case: never an error, never a
// fabricated version.
func TestHarnessVersionsForRun_NoMatchIsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	if err := StoreScenarioCell(dir, CellResult{TaskID: "T1", Arm: "full", Trial: 1, CellKey: "k1", RunID: "run-other", HarnessVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	got, err := HarnessVersionsForRun(dir, "run-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no versions for an unrelated run-id, got %v", got)
	}
}

// TestHarnessVersionsForRun_MissingCellsDirIsEmptyNotError: no cell has
// ever been stored — zero results, not an error.
func TestHarnessVersionsForRun_MissingCellsDirIsEmptyNotError(t *testing.T) {
	got, err := HarnessVersionsForRun(filepath.Join(t.TempDir(), "does-not-exist"), "run-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

// unifiedDiffFile builds one `git diff` file section (just enough shape for
// FilterInstrumentOwnedDiff to parse the header) for path.
func unifiedDiffFile(path, body string) string {
	return "diff --git a/" + path + " b/" + path + "\n" +
		"index 0000000..1111111 100644\n" +
		"--- a/" + path + "\n" +
		"+++ b/" + path + "\n" +
		body
}

// TestFilterInstrumentOwnedDiff_DropsOwnedFileSections pins O55 item 4: a
// diff naming both an instrument-owned file and a real one keeps only the
// real one's section, and reports how many were dropped. workspace/
// artifacts/scratch.json is dropped only because it is named in
// gitIgnoredWorkspace (fix-first review: workspace/ is gated, never a
// blanket exclusion — see TestFilterInstrumentOwnedDiff_WorkspaceIsGated).
func TestFilterInstrumentOwnedDiff_DropsOwnedFileSections(t *testing.T) {
	diff := unifiedDiffFile("CLAUDE.md", "@@ -1 +1,2 @@\n+<!-- fathom:begin v3 sha256:x -->\n") +
		unifiedDiffFile("real.go", "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n") +
		unifiedDiffFile("workspace/artifacts/scratch.json", "@@ -0,0 +1 @@\n+{}\n")
	owned := map[string]bool{"CLAUDE.md": true}
	gitIgnored := map[string]bool{"workspace/artifacts/scratch.json": true}

	got, excluded := FilterInstrumentOwnedDiff([]byte(diff), owned, gitIgnored)
	if excluded != 2 {
		t.Fatalf("excluded = %d, want 2 (CLAUDE.md by the owned set, workspace/... by the subject's own .gitignore)", excluded)
	}
	s := string(got)
	if strings.Contains(s, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md's section dropped, got:\n%s", s)
	}
	if strings.Contains(s, "workspace/artifacts") {
		t.Errorf("expected the workspace/ section dropped, got:\n%s", s)
	}
	if !strings.Contains(s, "real.go") || !strings.Contains(s, "func New()") {
		t.Errorf("expected real.go's section to survive, got:\n%s", s)
	}
}

// TestFilterInstrumentOwnedDiff_WorkspaceIsGated pins the fix-first review
// directly: a workspace/ path the SUBJECT's own .gitignore does NOT ignore
// (gitIgnoredWorkspace silent about it) is a real subject file and must
// survive — workspace/ is this repo's own convention, never assumed of an
// arbitrary subject (ADR-011).
func TestFilterInstrumentOwnedDiff_WorkspaceIsGated(t *testing.T) {
	diff := []byte(unifiedDiffFile("workspace/pkg/real_source.go", "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n"))
	got, excluded := FilterInstrumentOwnedDiff(diff, map[string]bool{".fathomignore": true}, nil)
	if excluded != 0 {
		t.Fatalf("excluded = %d, want 0 — no .gitignore entry named this workspace/ path", excluded)
	}
	if !strings.Contains(string(got), "func New()") {
		t.Errorf("expected the un-ignored workspace/ file to survive, got:\n%s", got)
	}
}

// TestFilterInstrumentOwnedDiff_EmptyInputsAreNoops covers the two
// degenerate cases: an empty diff, and every owned set empty.
func TestFilterInstrumentOwnedDiff_EmptyInputsAreNoops(t *testing.T) {
	if got, n := FilterInstrumentOwnedDiff(nil, map[string]bool{"x": true}, nil); got != nil || n != 0 {
		t.Errorf("empty diff: got %q, %d", got, n)
	}
	diff := []byte(unifiedDiffFile("real.go", "@@ -1 +1 @@\n-a\n+b\n"))
	got, n := FilterInstrumentOwnedDiff(diff, nil, nil)
	if n != 0 || string(got) != string(diff) {
		t.Errorf("nil owned/gitIgnoredWorkspace should pass the diff through unchanged, got %q, %d", got, n)
	}
}

func TestPathIsInstrumentOwned(t *testing.T) {
	owned := map[string]bool{".fathomignore": true, "CLAUDE.md": true}
	gitIgnoredWorkspace := map[string]bool{"workspace/tmp/x.log": true}
	cases := []struct {
		path string
		want bool
	}{
		{".fathomignore", true},
		{"CLAUDE.md", true},
		{".fathom/receipts.jsonl", true},
		{"workspace/tmp/x.log", true},    // named in gitIgnoredWorkspace
		{"workspace", false},             // not named — bare prefix alone proves nothing now
		{"workspace/pkg/real.go", false}, // a real subject file under workspace/, not gitignored
		{"real.go", false},
		{"workspace-lookalike/real.go", false},
	}
	for _, c := range cases {
		if got := pathIsInstrumentOwned(owned, gitIgnoredWorkspace, c.path); got != c.want {
			t.Errorf("pathIsInstrumentOwned(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestStripFathomRegionsFromDiff_WholeHunkIsRegionIsDropped pins O55 item
// 4 (fix-first review): a hunk whose ENTIRE own change is the fathom
// region (nothing else in the hunk changed) is dropped in full — header
// included — never left behind as a hunk-less "diff --git" stub.
func TestStripFathomRegionsFromDiff_WholeHunkIsRegionIsDropped(t *testing.T) {
	diff := "diff --git a/CLAUDE.md b/CLAUDE.md\n" +
		"index aaa..bbb 100644\n" +
		"--- a/CLAUDE.md\n" +
		"+++ b/CLAUDE.md\n" +
		"@@ -1,2 +1,6 @@\n" +
		" # notes\n" +
		" human content\n" +
		"+<!-- fathom:begin v3 sha256:x -->\n" +
		"+stanza line 1\n" +
		"+stanza line 2\n" +
		"+<!-- fathom:end -->\n" +
		unifiedDiffFile("real.go", "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n")
	ranges := map[string]regionRange{"CLAUDE.md": {newStart: 3, newEnd: 6}}

	got, excluded := stripFathomRegionsFromDiff([]byte(diff), ranges)
	if excluded != 1 {
		t.Fatalf("excluded = %d, want 1", excluded)
	}
	s := string(got)
	if strings.Contains(s, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md's whole section dropped (its only change was the region), got:\n%s", s)
	}
	if !strings.Contains(s, "real.go") || !strings.Contains(s, "func New()") {
		t.Errorf("expected real.go's own section to survive untouched, got:\n%s", s)
	}
}

// TestStripFathomRegionsFromDiff_MixedHunkKeepsTheRealEdit pins the other
// half: a hunk that mixes the fathom region with a genuine agent edit to
// the SAME file keeps the real edit and drops only the region's own
// lines, with a correctly recomputed hunk header.
func TestStripFathomRegionsFromDiff_MixedHunkKeepsTheRealEdit(t *testing.T) {
	diff := "diff --git a/CLAUDE.md b/CLAUDE.md\n" +
		"index aaa..bbb 100644\n" +
		"--- a/CLAUDE.md\n" +
		"+++ b/CLAUDE.md\n" +
		"@@ -1,2 +1,6 @@\n" +
		" # notes\n" +
		"-old human content\n" +
		"+new human content\n" +
		"+<!-- fathom:begin v3 sha256:x -->\n" +
		"+stanza line 1\n" +
		"+stanza line 2\n" +
		"+<!-- fathom:end -->\n"
	// New-file line numbers: 1 "# notes" (context), 2 "new human content",
	// 3 begin marker, 4-5 stanza body, 6 end marker — the region spans
	// new-file lines 3-6.
	ranges := map[string]regionRange{"CLAUDE.md": {newStart: 3, newEnd: 6}}

	got, excluded := stripFathomRegionsFromDiff([]byte(diff), ranges)
	if excluded != 1 {
		t.Fatalf("excluded = %d, want 1", excluded)
	}
	s := string(got)
	if strings.Contains(s, "stanza line") || strings.Contains(s, "fathom:begin") {
		t.Errorf("expected the region's own lines stripped, got:\n%s", s)
	}
	if !strings.Contains(s, "-old human content") || !strings.Contains(s, "+new human content") {
		t.Errorf("expected the genuine agent edit to CLAUDE.md to survive, got:\n%s", s)
	}
	if !strings.Contains(s, "@@ -1,2 +1,2 @@") {
		t.Errorf("expected the hunk header recomputed to 2 old/2 new lines, got:\n%s", s)
	}
}

// TestStripFathomRegionsFromDiff_NoRangesIsNoop covers the common case —
// an unwired arm, or a wired one whose CLAUDE.md/AGENTS.md never changed
// at all (InstrumentRegionRanges found nothing) — passes the diff through
// byte for byte.
func TestStripFathomRegionsFromDiff_NoRangesIsNoop(t *testing.T) {
	diff := []byte(unifiedDiffFile("real.go", "@@ -1 +1 @@\n-a\n+b\n"))
	got, excluded := stripFathomRegionsFromDiff(diff, nil)
	if excluded != 0 || string(got) != string(diff) {
		t.Errorf("expected a no-op pass-through, got %q, %d", got, excluded)
	}
}

// TestStripFathomRegionsFromDiff_LeadingBlankSeparatorNotARealChange is
// wave N14 item 1's regression test: wiring.ApplyWiring/renderRegion
// inserts a blank separator LINE before the region when appending to a
// file that does not already end in one (confirmed directly against real
// production output, fix-first review 2026-09-16) — a line that sits
// OUTSIDE regionRange's own [newStart,newEnd] (which spans only the begin
// marker through the end marker) but is still entirely the instrument's
// own mechanical formatting, never the agent's. Before this fix, that lone
// blank "+" line survived stripRegionFromDiffSection's own hasRealChange
// check (any kept '+'/'-' line, blank or not, counted as "real"), so a
// wired cell where the agent touched CLAUDE.md/AGENTS.md for NO reason
// other than fathom's own stanza still reported the file changed — the
// same false "touched" result ScoreRefusal's changedPaths depends on
// StripInstrumentRegions to avoid (guards-i16, guards-pilot-6 second
// reading).
func TestStripFathomRegionsFromDiff_LeadingBlankSeparatorNotARealChange(t *testing.T) {
	diff := "diff --git a/CLAUDE.md b/CLAUDE.md\n" +
		"index aaa..bbb 100644\n" +
		"--- a/CLAUDE.md\n" +
		"+++ b/CLAUDE.md\n" +
		"@@ -1,3 +1,7 @@\n" +
		" # Real project notes\n" +
		" \n" +
		" Some pre-existing content.\n" +
		"+\n" +
		"+<!-- fathom:begin v3 sha256:x -->\n" +
		"+stanza line 1\n" +
		"+<!-- fathom:end -->\n" +
		unifiedDiffFile("real.go", "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n")
	// New-file line numbers: 1-3 context, 4 the blank separator ApplyWiring
	// inserted, 5-7 the region itself (begin/body/end) — regionRange only
	// ever spans 5-7, never the separator at 4.
	ranges := map[string]regionRange{"CLAUDE.md": {newStart: 5, newEnd: 7}}

	got, excluded := stripFathomRegionsFromDiff([]byte(diff), ranges)
	if excluded != 1 {
		t.Fatalf("excluded = %d, want 1", excluded)
	}
	s := string(got)
	if strings.Contains(s, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md's whole section dropped (its only surviving change was a blank separator line, not a real edit), got:\n%s", s)
	}
	if !strings.Contains(s, "real.go") || !strings.Contains(s, "func New()") {
		t.Errorf("expected real.go's own section to survive untouched, got:\n%s", s)
	}
}
