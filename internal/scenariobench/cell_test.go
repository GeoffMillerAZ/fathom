package scenariobench

import (
	"os"
	"path/filepath"
	"strconv"
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
