package specdialect

// ghtasklist_test.go — SI-13 (docs/design/spec-interop.md §3): the GitHub
// task-list adapter over an exported issue/PR body, foreign file
// untouched; and the never-auto-claims contract that sets this dialect
// apart from every other one.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGitHubTaskListAdapter(t *testing.T) {
	root := filepath.Join("testdata", "ghtasklist")
	rel := "export/issue-142.md"
	before, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	plan, spans, read, err := ParseFile(GHTaskList(), root, rel)
	if err != nil {
		t.Fatal(err)
	}
	if direct, directSpans := ghTaskListParse(rel, before); !reflect.DeepEqual(direct, plan) || !reflect.DeepEqual(directSpans, spans) {
		t.Fatal("ParseFile and ghTaskListParse disagree")
	}
	after, _ := os.ReadFile(filepath.Join(root, rel))
	if !bytes.Equal(before, after) || !bytes.Equal(before, read) {
		t.Fatal("the foreign file changed")
	}

	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}
	byID := map[string]bool{}
	for i, tk := range plan.Tasks {
		byID[tk.ID] = true
		if tk.Line == 0 {
			t.Fatalf("task %d has no foreign line", i)
		}
	}
	for _, want := range []string{"t-001", "t-002", "t-003", "t-004"} {
		if !byID[want] {
			t.Fatalf("missing positional id %q among %+v", want, plan.Tasks)
		}
	}

	t1, t2, t3, t4 := plan.Tasks[0], plan.Tasks[1], plan.Tasks[2], plan.Tasks[3]
	if !t1.Claimed || !t2.Claimed || t3.Claimed || t4.Claimed {
		t.Fatalf("claimed states wrong: %v %v %v %v", t1.Claimed, t2.Claimed, t3.Claimed, t4.Claimed)
	}
	if t1.Oracle != "lexical" || t1.OraclePath != "src/payments/client.go" {
		t.Errorf("t1 anchor from its named path: %+v", t1)
	}
	if t2.Expect.Symbol != "Retry" || t2.Expect.Package != "src/payments" || t2.Oracle != "" {
		t.Errorf("t2 inline annotation: %+v", t2)
	}
	if t3.OraclePath != "src/audit/log.go" || !reflect.DeepEqual(t3.Ctx, []string{"docs/payments.md"}) {
		t.Errorf("t3 first named path is the contract, the rest ctx: %+v", t3)
	}
	if hasContract(t4) {
		t.Errorf("t4 names nothing and must carry no contract: %+v", t4)
	}

	wantSpans := []Span{{"t-001", 8, 3}, {"t-002", 9, 3}, {"t-003", 10, 3}, {"t-004", 11, 3}}
	if !reflect.DeepEqual(spans, wantSpans) {
		t.Fatalf("spans = %v, want %v", spans, wantSpans)
	}
}

// TestGitHubTaskListNeverAutoClaims (SI-13, design §3): unlike every
// other dialect, an exported GitHub issue/PR body carries no directory
// convention or content shape fathom can trust — "the user passes" it —
// so Detect and Claims never fire on their own; the only way in is an
// explicit --dialect gh-tasklist (dialectForPlan bypasses them). A
// registry holding it must therefore still fall back a plain checkbox
// file to the native grammar, exactly as if gh-tasklist were absent.
func TestGitHubTaskListNeverAutoClaims(t *testing.T) {
	g := GHTaskList()
	if got := g.Detect("testdata", []string{"export/issue-142.md", "docs/plans/x.md"}); got != nil {
		t.Fatalf("Detect must never auto-claim anything, got %v", got)
	}
	body := mustRead(t, filepath.Join("testdata", "ghtasklist", "export", "issue-142.md"))
	if g.Claims("export/issue-142.md", body) {
		t.Fatal("Claims must never auto-claim, even a real export, by path or shape alone")
	}

	r := NewRegistry()
	for _, d := range []SpecDialect{Native(), GHTaskList()} {
		if err := RegisterSpecDialect(r, d); err != nil {
			t.Fatalf("register %s: %v", d.ID(), err)
		}
	}
	d, ok := r.ForFile("export/issue-142.md", body)
	if !ok || d.ID() != NativeID {
		t.Fatalf("ForFile with gh-tasklist registered = %v, want the native fallback unchanged", d)
	}
	// The explicit override still reaches it directly (dialectForPlan's
	// own contract, exercised at the cmd/fathom layer) — Lookup, not
	// Claims/Detect.
	if got, ok := r.Lookup("gh-tasklist"); !ok || got.ID() != "gh-tasklist" {
		t.Fatalf("Lookup(gh-tasklist) = %v, %v", got, ok)
	}
}
