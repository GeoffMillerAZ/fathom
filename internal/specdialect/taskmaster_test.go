package specdialect

// taskmaster_test.go — SI-12 (docs/design/spec-interop.md §3): the Task
// Master adapter over .taskmaster/tasks/tasks.json, tasks and subtasks,
// status: done as claimed, anchors from details/testStrategy text, foreign
// file untouched.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTaskMasterAdapterJSON(t *testing.T) {
	root := filepath.Join("testdata", "taskmaster")
	rel := ".taskmaster/tasks/tasks.json"
	before, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	plan, spans, read, err := ParseFile(TaskMaster(), root, rel)
	if err != nil {
		t.Fatal(err)
	}
	direct, directSpans, derr := taskMasterParse(rel, before)
	if derr != nil || !reflect.DeepEqual(direct, plan) || !reflect.DeepEqual(directSpans, spans) {
		t.Fatalf("ParseFile and taskMasterParse disagree (err=%v)", derr)
	}
	after, _ := os.ReadFile(filepath.Join(root, rel))
	if !bytes.Equal(before, after) || !bytes.Equal(before, read) {
		t.Fatal("the foreign file changed")
	}

	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 tasks (1 top-level with 2 subtasks, plus 1 more top-level), got %d: %+v", len(plan.Tasks), plan.Tasks)
	}
	byID := map[string]int{}
	for i, tk := range plan.Tasks {
		byID[tk.ID] = i
	}
	for _, want := range []string{"1", "1.1", "1.2", "2"} {
		if _, ok := byID[want]; !ok {
			t.Fatalf("missing id %q among %+v", want, plan.Tasks)
		}
	}

	t1 := plan.Tasks[byID["1"]]
	t11 := plan.Tasks[byID["1.1"]]
	t12 := plan.Tasks[byID["1.2"]]
	t2 := plan.Tasks[byID["2"]]

	if !t1.Claimed || !t11.Claimed || t12.Claimed || t2.Claimed {
		t.Fatalf("claimed states wrong (status: done only): %v %v %v %v", t1.Claimed, t11.Claimed, t12.Claimed, t2.Claimed)
	}

	// t1: no inline annotation; anchors come from details + testStrategy,
	// never the title — the first non-Markdown path is the oracle, the
	// rest ctx.
	if t1.Oracle != "lexical" || t1.OraclePath != "src/payments/client.go" {
		t.Errorf("t1 anchor from details: %+v", t1)
	}
	if !reflect.DeepEqual(t1.Ctx, []string{"internal/payments/retry_test.go"}) {
		t.Errorf("t1 ctx from testStrategy: %+v", t1)
	}
	if t1.Line != 4 {
		t.Errorf("t1 Line = %d, want 4", t1.Line)
	}

	// t1.1: inline annotation on the subtask's own title, markers
	// stripped from the title, id stays the dotted "1.1".
	if t11.Expect.Symbol != "Retry" || t11.Expect.Package != "src/payments" {
		t.Errorf("t1.1 inline annotation: %+v", t11)
	}
	if t11.Title != "Write the retry loop" {
		t.Errorf("t1.1 Title = %q, annotation must be stripped", t11.Title)
	}
	if t11.Line != 13 {
		t.Errorf("t1.1 Line = %d, want 13", t11.Line)
	}

	// t1.2: details names a Markdown path only — never made a contract,
	// so this row carries none, exactly like every other adapter's
	// md-only-path row.
	if hasContract(t12) {
		t.Errorf("t1.2 details name only a Markdown path and must carry no contract: %+v", t12)
	}
	if !reflect.DeepEqual(t12.Ctx, []string{"docs/payments.md"}) {
		t.Errorf("t1.2 ctx: %+v", t12)
	}
	if t12.Line != 20 {
		t.Errorf("t1.2 Line = %d, want 20", t12.Line)
	}

	// t2: names nothing at all.
	if hasContract(t2) || len(t2.Ctx) != 0 {
		t.Errorf("t2 names nothing and must carry no contract or ctx: %+v", t2)
	}
	if t2.Line != 29 {
		t.Errorf("t2 Line = %d, want 29", t2.Line)
	}

	wantSpans := []Span{{"1", 4, 1}, {"1.1", 13, 1}, {"1.2", 20, 1}, {"2", 29, 1}}
	if !reflect.DeepEqual(spans, wantSpans) {
		t.Fatalf("spans = %v, want %v", spans, wantSpans)
	}

	// Malformed JSON is refused, not silently read as zero tasks.
	if _, _, err := taskMasterParse("x.json", []byte("{not json")); err == nil {
		t.Fatal("malformed JSON must return an error")
	}
}

// TestTaskMasterDetectRequiresTasksArray: the path alone is not enough —
// a .taskmaster/tasks/tasks.json without a top-level "tasks" array (a
// tag-scoped export, or garbage) is not claimed.
func TestTaskMasterDetectRequiresTasksArray(t *testing.T) {
	d := TaskMaster()
	if d.Claims(".taskmaster/tasks/tasks.json", []byte(`{"master": {"tasks": []}}`)) {
		t.Fatal("a tag-scoped export (no top-level tasks array) must not be claimed")
	}
	if !d.Claims(".taskmaster/tasks/tasks.json", []byte(`{"tasks": []}`)) {
		t.Fatal("an empty but present top-level tasks array must be claimed")
	}
	if d.Claims("notes/tasks.json", []byte(`{"tasks": []}`)) {
		t.Fatal("the wrong path must not be claimed")
	}
}
