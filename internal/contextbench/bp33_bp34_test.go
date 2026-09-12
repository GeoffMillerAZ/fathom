package contextbench

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ─── BP-34: the primary model ──────────────────────────────────────────────

// TestPrimaryModelID: the model that generated the answers is the run's
// model; the harness's small model (tool search, summaries) is aux. A joined
// "a+b" id made the ledger row and the cell key read as if the bench had been
// measured on two models at once.
func TestPrimaryModelID(t *testing.T) {
	for _, tc := range []struct {
		name        string
		in          map[string]int64
		wantPrimary string
		wantAux     []string
	}{
		{"single model", map[string]int64{"claude-sonnet-4-6": 900}, "claude-sonnet-4-6", nil},
		{"harness small model is aux", map[string]int64{"claude-sonnet-4-6": 900, "claude-haiku-4-5": 40}, "claude-sonnet-4-6", []string{"claude-haiku-4-5"}},
		{"two aux", map[string]int64{"big": 900, "small": 40, "tiny": 1}, "big", []string{"small", "tiny"}},
		{"ties break lexicographically", map[string]int64{"b": 10, "a": 10}, "a", []string{"b"}},
		{"all zero (probe envelope)", map[string]int64{"claude-x": 0, "claude-y": 0}, "claude-x", []string{"claude-y"}},
		{"empty", map[string]int64{}, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			primary, aux := PrimaryModelID(tc.in)
			if primary != tc.wantPrimary {
				t.Errorf("primary = %q, want %q", primary, tc.wantPrimary)
			}
			if !reflect.DeepEqual(aux, tc.wantAux) {
				t.Errorf("aux = %v, want %v", aux, tc.wantAux)
			}
			if strings.ContainsAny(primary, "+,") {
				t.Errorf("primary %q is a joined blend — the cell key must be able to use it alone", primary)
			}
		})
	}
}

// TestArmResultFromClaudeJSON_PrimaryModelAndAux: a real envelope names more
// than one model because Claude Code drives a small model of its own. The
// scored cell records the model that answered, and the rest additively.
func TestArmResultFromClaudeJSON_PrimaryModelAndAux(t *testing.T) {
	raw := []byte(`{"result":"the answer is alpha","total_cost_usd":0.4,
	  "usage":{"input_tokens":10,"output_tokens":900},
	  "modelUsage":{"claude-sonnet-4-6":{"outputTokens":900},"claude-haiku-4-5":{"outputTokens":37}}}`)
	ar, err := ArmResultFromClaudeJSON(Task{ID: "T1", GroundTruth: []string{"alpha"}}, "fathom", raw)
	if err != nil {
		t.Fatalf("ArmResultFromClaudeJSON: %v", err)
	}
	if ar.ModelID != "claude-sonnet-4-6" {
		t.Errorf("ModelID = %q, want the primary claude-sonnet-4-6", ar.ModelID)
	}
	if !reflect.DeepEqual(ar.AuxModels, []string{"claude-haiku-4-5"}) {
		t.Errorf("AuxModels = %v, want [claude-haiku-4-5]", ar.AuxModels)
	}
}

// TestPrimaryModelIDKeysTheCell: the cell key is computed from the primary id
// alone. Keying on a joined id would refuse every borrow the moment the
// harness changed its helper model — a mismatch about something the bench
// never measured.
func TestPrimaryModelIDKeysTheCell(t *testing.T) {
	primary, aux := PrimaryModelID(map[string]int64{"claude-sonnet-4-6": 900, "claude-haiku-4-5": 40})
	base := CellInputs{CorpusSHA256: "c", TaskID: "T1", Arm: "fathom", HarnessVersion: "h", Scoring: ScoringSubstring, PromptSubject: "s"}
	primaryOnly := base
	primaryOnly.ModelID = primary
	joined := base
	joined.ModelID = primary + "+" + aux[0]
	if CellKey(primaryOnly) == CellKey(joined) {
		t.Fatal("the cell key must change when the model id changes — this test's premise is broken")
	}
	again := base
	again.ModelID = primary
	if CellKey(primaryOnly) != CellKey(again) {
		t.Error("the same primary id must reproduce the same cell key regardless of aux models")
	}
}

// ─── BP-33: targeted rows and honest cells_lost ────────────────────────────

// TestTargetedLedgerPath: docs/bench/ledger.jsonl is committed history. A
// diff-selected subset carries no verdict, so it must not grow that file
// unless the operator asked for it by name.
func TestTargetedLedgerPath(t *testing.T) {
	const committed = "docs/bench/ledger.jsonl"
	const artifacts = "workspace/artifacts/context-bench"
	targeted := filepath.Join(artifacts, "ledger-targeted.jsonl")
	for _, tc := range []struct {
		name     string
		explicit bool
		scope    string
		only     bool
		want     string
	}{
		{"full run keeps the committed ledger", false, ScopeFull, false, committed},
		{"scope targeted redirects", false, ScopeTargeted, false, targeted},
		{"--only redirects even at full scope", false, ScopeFull, true, targeted},
		{"explicit -ledger is always obeyed", true, ScopeTargeted, true, committed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := TargetedLedgerPath(committed, tc.explicit, tc.scope, tc.only, artifacts)
			if got != tc.want {
				t.Errorf("TargetedLedgerPath = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCellsLostCountsOnlySelectedTasks (BP-33): a smoke run with
// `--only H1-T01 --repeat 1 --arms fathom` reported cells_lost=12 — twelve
// cells nobody intended to buy. cells_lost is selected x arms x trials minus
// scored, never the unselected corpus.
func TestCellsLostCountsOnlySelectedTasks(t *testing.T) {
	tasks := []Task{{ID: "T1"}, {ID: "T2"}, {ID: "T3"}, {ID: "T4"}}
	results := []ArmResult{{TaskID: "T1", Arm: "fathom", Trial: 1}}

	if got := CellsLost(tasks, []string{"fathom"}, 1, results); got != 3 {
		t.Fatalf("premise: CellsLost over the whole corpus = %d, want 3", got)
	}
	selected := TasksSelected(tasks, []string{"T1"})
	if got := CellsLost(selected, []string{"fathom"}, 1, results); got != 0 {
		t.Errorf("cells_lost over the SELECTED task = %d, want 0 (the run bought every cell it intended)", got)
	}

	// A selected task whose cell really is missing is still counted.
	selected2 := TasksSelected(tasks, []string{"T1", "T2"})
	if got := CellsLost(selected2, []string{"fathom"}, 1, results); got != 1 {
		t.Errorf("cells_lost = %d, want 1 (T2 was selected and never scored)", got)
	}
	// An empty selection means the full run.
	if got := TasksSelected(tasks, nil); len(got) != 4 {
		t.Errorf("TasksSelected(nil) returned %d tasks, want the whole list", len(got))
	}
}
