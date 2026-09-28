package specdialect

// kiro_test.go — SI-10 (docs/design/spec-interop.md §3): the Kiro adapter,
// numbered checkbox tasks with _Requirements:_ references resolved
// against requirements.md (EARS), foreign file untouched.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestKiroAdapterRequirements(t *testing.T) {
	root := filepath.Join("testdata", "kiro")
	rel := ".kiro/specs/payments/tasks.md"
	before, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	plan, spans, read, err := ParseFile(Kiro(), root, rel)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, rel))
	if !bytes.Equal(before, after) || !bytes.Equal(before, read) {
		t.Fatal("the foreign file changed")
	}

	if len(plan.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d: %+v", len(plan.Tasks), plan.Tasks)
	}
	byID := map[string]verify.Task{}
	for _, tk := range plan.Tasks {
		byID[tk.ID] = tk
	}
	t1, t2, t3, t4 := byID["1"], byID["2"], byID["3"], byID["4"]
	if !t1.Claimed || !t2.Claimed || t3.Claimed || t4.Claimed {
		t.Fatalf("claimed states wrong: %v %v %v %v", t1.Claimed, t2.Claimed, t3.Claimed, t4.Claimed)
	}

	if t1.Oracle != "lexical" || t1.OraclePath != "src/payments/client.go" {
		t.Errorf("t1 anchor from its named path: %+v", t1)
	}
	wantT1Accept := "1.1: WHEN a payment times out THEN the system SHALL retry up to three times; " +
		"1.2: WHEN all retries fail THEN the system SHALL mark the payment failed"
	if t1.Accept != wantT1Accept {
		t.Errorf("t1 Accept = %q, want %q", t1.Accept, wantT1Accept)
	}

	if t2.Expect.Symbol != "Retry" || t2.Expect.Package != "src/payments" {
		t.Errorf("t2 inline annotation: %+v", t2)
	}
	if want := "1.1: WHEN a payment times out THEN the system SHALL retry up to three times"; t2.Accept != want {
		t.Errorf("t2 Accept = %q, want %q", t2.Accept, want)
	}

	if t3.OraclePath != "src/audit/log.go" {
		t.Errorf("t3 anchor from its named path: %+v", t3)
	}
	if want := "2.1: WHEN a retry occurs THEN the system SHALL write an audit log entry"; t3.Accept != want {
		t.Errorf("t3 Accept = %q, want %q", t3.Accept, want)
	}

	if hasContract(t4) || t4.Accept != "" {
		t.Errorf("t4 names nothing and has no _Requirements:_ line: %+v", t4)
	}

	wantSpans := []Span{{"1", 3, 3}, {"2", 6, 3}, {"3", 9, 3}, {"4", 12, 3}}
	if !reflect.DeepEqual(spans, wantSpans) {
		t.Fatalf("spans = %v, want %v", spans, wantSpans)
	}

	// A stale requirement reference (nothing in requirements.md resolves
	// it) is kept verbatim, never silently dropped.
	p, _ := kiroParse("tasks.md", []byte("- [ ] 1. x\n  - _Requirements: 9.9_\n"), []byte("## Requirement 1\n\n1. WHEN x THEN y\n"))
	if p.Tasks[0].Accept != "9.9" {
		t.Fatalf("unresolved requirement ref: Accept = %q, want %q", p.Tasks[0].Accept, "9.9")
	}

	// Without a requirements.md (the pure Parse), no requirement is
	// linked — same guarantee OpenSpec's own adapter gives.
	p2, _, _ := Kiro().Parse(rel, before)
	for _, tk := range p2.Tasks {
		if tk.Accept != "" {
			t.Errorf("%s linked a requirement without requirements.md: %q", tk.ID, tk.Accept)
		}
	}

	reqs := kiroRequirements(mustRead(t, filepath.Join(root, ".kiro/specs/payments/requirements.md")))
	if len(reqs) != 3 {
		t.Fatalf("requirements = %+v, want 3", reqs)
	}
	if reqs[0].ID != "1.1" || reqs[2].ID != "2.1" {
		t.Fatalf("requirement ids = %+v", reqs)
	}
}
