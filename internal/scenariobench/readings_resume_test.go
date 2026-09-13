package scenariobench

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

func TestResumptionReadings_TokensAndDuplicates(t *testing.T) {
	sessions := []SessionResult{
		{Index: 0, Role: "single", TokensToFirstEdit: 100, BoxChecks: []BoxCheck{{RowID: "r1"}}},
		{Index: 1, Role: "single", TokensToFirstEdit: 50, BoxChecks: []BoxCheck{{RowID: "r1"}}},
		{Index: 2, Role: "single", TokensToFirstEdit: 70, BoxChecks: []BoxCheck{{RowID: "r2"}, {RowID: "r1"}}},
	}
	proven := map[string]bool{"r1": true, "r2": false}

	got := ResumptionReadings(sessions, proven)
	if got.Sessions != 3 {
		t.Fatalf("sessions = %d, want 3", got.Sessions)
	}
	if got.TokensToFirstEditS2 != 120 {
		t.Fatalf("tokens to first edit from session 2 = %d, want 120 (50+70)", got.TokensToFirstEditS2)
	}
	// r1 recurs in session 1 (proven -> +1) and again in session 2
	// (proven -> +1); r2 appears once, never a duplicate.
	if got.DuplicatedEdits != 2 {
		t.Fatalf("duplicated edits = %d, want 2", got.DuplicatedEdits)
	}
	if got.NotMeasurable != "" {
		t.Fatalf("unexpected not_measurable: %s", got.NotMeasurable)
	}
}

func TestResumptionReadings_FewerThanTwoSessions(t *testing.T) {
	got := ResumptionReadings([]SessionResult{{Index: 0, Role: "single"}}, nil)
	if got.NotMeasurable != "fewer than 2 sessions" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
	if got.TokensToFirstEditS2 != 0 {
		t.Fatalf("tokens to first edit from session 2 = %d, want 0", got.TokensToFirstEditS2)
	}
}

func TestContractDrift_VanishedAndReached(t *testing.T) {
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\n\nfunc Foo() int {\n\treturn 1\n}\n"
	file := filepath.Join(tree, "pkg", "foo.go")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	spans, err := publicloc.FuncSpans([]byte(src), file)
	if err != nil {
		t.Fatal(err)
	}
	foo := spans[0]

	cover := filepath.Join(tree, "cover.out")
	profile := "mode: set\n" + fmt.Sprintf("github.com/example/mod/pkg/foo.go:%d.1,%d.2 1 1\n", foo.Start, foo.End)
	if err := os.WriteFile(cover, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := []byte(`- [x] keeps Foo {id: R1, symbol: Foo, package: pkg}
- [x] used to have Ghost {id: R2, symbol: Ghost, package: pkg}
`)
	got := ContractDrift(plan, tree, cover)
	if got.Rows != 2 {
		t.Fatalf("rows = %d, want 2", got.Rows)
	}
	if len(got.Drifted) != 1 || got.Drifted[0] != "R2: symbol vanished" {
		t.Fatalf("drifted = %v", got.Drifted)
	}
}

func TestContractDrift_NoCoverprofileStillChecksVanished(t *testing.T) {
	tree := t.TempDir()
	plan := []byte("- [x] {id: R1, symbol: Ghost, package: pkg}\n")
	got := ContractDrift(plan, tree, "")
	if got.NotMeasurable == "" {
		t.Fatalf("expected a not_measurable note about the missing coverprofile")
	}
	if len(got.Drifted) != 1 || got.Drifted[0] != "R1: symbol vanished" {
		t.Fatalf("drifted = %v", got.Drifted)
	}
}

func TestContractDrift_NoSymbolRows(t *testing.T) {
	got := ContractDrift([]byte("- [ ] just prose\n"), t.TempDir(), "")
	if got.NotMeasurable != "no symbol rows found in plan" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
}
