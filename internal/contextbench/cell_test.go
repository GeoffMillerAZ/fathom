package contextbench

import (
	"path/filepath"
	"strings"
	"testing"
)

func baseInputs() CellInputs {
	return CellInputs{
		CorpusSHA256:   "c0ffee",
		TaskID:         "H1-T05",
		Arm:            "grep",
		ModelID:        "claude-sonnet-4-5-20250929",
		HarnessVersion: "2.1.258 (Claude Code)",
		Scoring:        ScoringSubstring,
		PromptSubject:  "the fathom codebase",
	}
}

// TestCellKeyDependsOnEveryInvariant is the contract that makes borrowing
// safe: a cell key changes when any invariant changes, so a stale reading
// cannot be reused under new conditions — it simply has a different
// address and is never found.
func TestCellKeyDependsOnEveryInvariant(t *testing.T) {
	base := baseInputs()
	key := CellKey(base)
	if key != CellKey(baseInputs()) {
		t.Fatal("CellKey is not a pure function of its inputs")
	}
	if len(key) != 64 {
		t.Fatalf("want a hex sha256, got %d chars", len(key))
	}

	mutations := map[string]func(*CellInputs){
		"corpus":         func(in *CellInputs) { in.CorpusSHA256 = "deadbeef" },
		"task":           func(in *CellInputs) { in.TaskID = "H1-T06" },
		"arm":            func(in *CellInputs) { in.Arm = "fathom" },
		"model":          func(in *CellInputs) { in.ModelID = "claude-opus-4-1" },
		"harness":        func(in *CellInputs) { in.HarnessVersion = "2.2.0 (Claude Code)" },
		"scoring":        func(in *CellInputs) { in.Scoring = ScoringPrecision },
		"prompt_subject": func(in *CellInputs) { in.PromptSubject = "the acme codebase" },
		"arm_surface":    func(in *CellInputs) { in.ArmSurface = "1b8765d" },
	}
	seen := map[string]string{key: "base"}
	for name, mutate := range mutations {
		in := baseInputs()
		mutate(&in)
		got := CellKey(in)
		if got == key {
			t.Errorf("changing %s did not change the cell key — a cell bought under different %s would be borrowed", name, name)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("changing %s collides with %s", name, prev)
		}
		seen[got] = name
	}

	// The labeling guard: two different field splits must not hash alike.
	a, b := baseInputs(), baseInputs()
	a.TaskID, a.Arm = "H1", "T05grep"
	b.TaskID, b.Arm = "H1T05", "grep"
	if CellKey(a) == CellKey(b) {
		t.Error("adjacent fields concatenate: the key is ambiguous")
	}

	if err := (CellInputs{}).Validate(); err == nil {
		t.Error("empty inputs must not produce a usable key")
	}
	if err := base.Validate(); err != nil {
		t.Errorf("complete inputs: %v", err)
	}
}

// TestCellStoreBorrowRoundTrip pins BP-4: a scored cell is stored
// content-addressed and a later run borrows it with provenance, while a
// cell that was never bought is a miss, not an error.
func TestCellStoreBorrowRoundTrip(t *testing.T) {
	dir := t.TempDir()
	key := CellKey(baseInputs())
	cell := ArmResult{
		TaskID: "H1-T05", Arm: "grep", Trial: 2, Success: true,
		TotalTokens: 41000, CellKey: key, SourceRunID: "20260902T211103Z",
	}
	if err := StoreCell(dir, cell); err != nil {
		t.Fatalf("StoreCell: %v", err)
	}
	if want := CellPath(dir, key, 2); !strings.HasSuffix(want, filepath.Join(key, "t2.json")) {
		t.Errorf("cell path is not content-addressed: %s", want)
	}

	got, ok, err := BorrowCell(dir, key, 2)
	if err != nil || !ok {
		t.Fatalf("BorrowCell: ok=%v err=%v", ok, err)
	}
	if !got.Borrowed {
		t.Error("a borrowed cell must say so")
	}
	if got.SourceRunID != "20260902T211103Z" {
		t.Errorf("provenance lost: %q", got.SourceRunID)
	}
	if got.TotalTokens != 41000 || !got.Success {
		t.Error("the borrowed reading is not the stored reading")
	}

	if _, ok, err := BorrowCell(dir, key, 3); ok || err != nil {
		t.Errorf("a trial nobody bought must be a miss, not an error: ok=%v err=%v", ok, err)
	}
	if _, ok, err := BorrowCell(dir, strings.Repeat("a", 64), 1); ok || err != nil {
		t.Errorf("an unknown key must be a miss: ok=%v err=%v", ok, err)
	}

	// First writer owns provenance: re-storing must not rewrite it.
	cell.SourceRunID, cell.TotalTokens = "later-run", 1
	if err := StoreCell(dir, cell); err != nil {
		t.Fatalf("StoreCell (second): %v", err)
	}
	again, _, _ := BorrowCell(dir, key, 2)
	if again.SourceRunID != "20260902T211103Z" || again.TotalTokens != 41000 {
		t.Error("re-storing a cell rewrote the original reading")
	}
}

// TestAcceptBorrowedRefusesForeignKey is BP-5's contract: a borrowed cell
// is admitted by key equality and nothing else, and a refusal names the
// cell it refused.
func TestAcceptBorrowedRefusesForeignKey(t *testing.T) {
	want := CellKey(baseInputs())
	other := baseInputs()
	other.ArmSurface = "a-different-fathom"
	foreign := ArmResult{TaskID: "H1-T05", Arm: "grep", Trial: 1, Borrowed: true, CellKey: CellKey(other), SourceRunID: "20260901T000000Z"}

	err := AcceptBorrowed(want, foreign)
	if err == nil {
		t.Fatal("a borrowed cell with a different key must be refused")
	}
	for _, frag := range []string{"H1-T05", "grep", "20260901T000000Z", "refused"} {
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("refusal does not name %q: %v", frag, err)
		}
	}
	if err := AcceptBorrowed(want, ArmResult{TaskID: "x", Borrowed: true}); err == nil {
		t.Error("a borrowed cell with no key must be refused")
	}
	ok := foreign
	ok.CellKey = want
	if err := AcceptBorrowed(want, ok); err != nil {
		t.Errorf("a key-equal borrowed cell must be accepted: %v", err)
	}
	fresh := ArmResult{TaskID: "H1-T05", Arm: "grep", CellKey: "whatever"}
	if err := AcceptBorrowed(want, fresh); err != nil {
		t.Errorf("a cell this run paid for is not subject to the borrow rule: %v", err)
	}
}

func TestRefuseMismatchedBorrowsDropsTheCell(t *testing.T) {
	base := baseInputs()
	base.TaskID, base.Arm, base.ArmSurface = "", "", ""
	surfaces := map[string]string{"fathom": "1b8765d"}

	good := base
	good.TaskID, good.Arm = "H1-T05", "grep"
	bad := base
	bad.TaskID, bad.Arm, bad.ArmSurface = "H1-T05", "fathom", "an-older-fathom"

	results := []ArmResult{
		{TaskID: "H1-T05", Arm: "grep", Trial: 1, Borrowed: true, CellKey: CellKey(good), SourceRunID: "r0"},
		{TaskID: "H1-T05", Arm: "fathom", Trial: 1, Borrowed: true, CellKey: CellKey(bad), SourceRunID: "r0"},
		{TaskID: "H1-T05", Arm: "fathom", Trial: 2},
	}
	kept, refused := RefuseMismatchedBorrows(results, base, surfaces)
	if len(kept) != 2 {
		t.Fatalf("want 2 kept cells, got %d", len(kept))
	}
	if len(refused) != 1 || !strings.Contains(refused[0], "H1-T05/fathom") {
		t.Fatalf("want one named refusal, got %v", refused)
	}

	prov := BorrowedCellsOf(kept)
	if len(prov) != 1 || prov[0].Arm != "grep" || prov[0].SourceRunID != "r0" {
		t.Errorf("borrowed provenance: %+v", prov)
	}
}

// TestEvaluateRefusesTargetedAndAllBorrowed pins the two rows the
// evaluator may not turn into a verdict (BP-5, BP-8).
func TestEvaluateRefusesTargetedAndAllBorrowed(t *testing.T) {
	c := Criterion{ID: "H2-1", RequireStrictlyMoreSuccesses: true, MaxTokenRatio: 1, MinTrials: 1}
	win := PairedSummary{
		Tasks: 2, MinTrials: 1, TreatmentSuccesses: 2, ControlSuccesses: 1,
		TreatmentTrials: 2, ControlTrials: 2, AggregateTokenRatio: 0.5,
	}
	if ev := c.Evaluate(win); ev.Verdict != VerdictPass {
		t.Fatalf("baseline row should pass, got %s: %s", ev.Verdict, ev.Reason)
	}

	targeted := win
	targeted.Scope = ScopeTargeted
	ev := c.Evaluate(targeted)
	if ev.Verdict != VerdictIndeterminate || !strings.Contains(ev.Reason, "targeted") {
		t.Errorf("a targeted run must be indeterminate with the reason named, got %s: %s", ev.Verdict, ev.Reason)
	}

	borrowed := win
	borrowed.Cells, borrowed.BorrowedCells = 4, 4
	ev = c.Evaluate(borrowed)
	if ev.Verdict != VerdictIndeterminate || !strings.Contains(ev.Reason, "borrowed") {
		t.Errorf("an all-borrowed row must be indeterminate, got %s: %s", ev.Verdict, ev.Reason)
	}

	partial := win
	partial.Cells, partial.BorrowedCells = 4, 2
	ev = c.Evaluate(partial)
	if ev.Verdict != VerdictPass || !strings.Contains(ev.Reason, "2 of 4 cell(s) borrowed") {
		t.Errorf("a partly borrowed row stands, with provenance in the reason: %s / %s", ev.Verdict, ev.Reason)
	}

	full := win
	full.Scope = ScopeFull
	if ev := c.Evaluate(full); ev.Verdict != VerdictPass {
		t.Errorf("scope full is a verdict-bearing run: %s", ev.Reason)
	}
}

// TestPairCountsBorrowedCells pins that the provenance the evaluator reads
// is computed from the cells, never passed in beside them.
func TestPairCountsBorrowedCells(t *testing.T) {
	results := []ArmResult{
		{TaskID: "T1", Arm: "fathom", Trial: 1, Success: true, TotalTokens: 100},
		{TaskID: "T1", Arm: "grep", Trial: 1, Success: true, TotalTokens: 200, Borrowed: true, SourceRunID: "r0"},
	}
	p := Pair(results, "fathom", "grep")
	if p.Cells != 2 || p.BorrowedCells != 1 {
		t.Fatalf("want 2 cells / 1 borrowed, got %d / %d", p.Cells, p.BorrowedCells)
	}
}
