package checkcatalog_test

import (
	"errors"
	"strings"
	"testing"

	checkcatalog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/check-catalog"
)

// input is the consumer-shaped thing a check reads; the package is generic
// over it.
type input struct {
	Text string
}

func row(id string, sev checkcatalog.Severity, means checkcatalog.Means) checkcatalog.Row {
	return checkcatalog.Row{ID: id, Name: "check " + id, Severity: sev, Means: means, Rejects: "something about " + id}
}

func check(r checkcatalog.Row, fn func(input) []checkcatalog.Finding) checkcatalog.Check[input] {
	return checkcatalog.Check[input]{Row: r, Fn: fn}
}

func clean(input) []checkcatalog.Finding { return nil }

func catalog() []checkcatalog.Row {
	return []checkcatalog.Row{
		row("INT-01", checkcatalog.SeverityFatal, checkcatalog.MeansDeterministic),
		row("STR-01", checkcatalog.SeverityMajor, checkcatalog.MeansDeterministic),
		row("STY-01", checkcatalog.SeverityImprovement, checkcatalog.MeansDeterministic),
		row("HON-01", checkcatalog.SeverityData, checkcatalog.MeansModel),
	}
}

func impls() []checkcatalog.Check[input] {
	var out []checkcatalog.Check[input]
	for _, r := range catalog() {
		out = append(out, check(r, clean))
	}
	return out
}

// ─── vocabulary ─────────────────────────────────────────────────────────────

func TestSeverityBlockingIsDecidedOnce(t *testing.T) {
	want := map[checkcatalog.Severity]bool{
		checkcatalog.SeverityFatal:       true,
		checkcatalog.SeverityMajor:       true,
		checkcatalog.SeverityData:        false,
		checkcatalog.SeverityImprovement: false,
	}
	for _, s := range checkcatalog.Severities {
		if got := s.Blocking(); got != want[s] {
			t.Errorf("%s.Blocking() = %v, want %v", s, got, want[s])
		}
		if !s.Valid() {
			t.Errorf("%s is in Severities but not Valid", s)
		}
	}
	if checkcatalog.Severity("critical").Valid() {
		t.Error("an invented severity validated")
	}
	if checkcatalog.Means("heuristic").Valid() {
		t.Error("an invented means validated")
	}
}

// TestRowValidate mirrors testdata/invalid.cue: every commented-out negative
// fixture there is rejected here in Go.
func TestRowValidate(t *testing.T) {
	ok := row("X-01", checkcatalog.SeverityData, checkcatalog.MeansDeterministic)
	tests := []struct {
		name    string
		mutate  func(*checkcatalog.Row)
		wantErr string
	}{
		{name: "valid", mutate: func(*checkcatalog.Row) {}},
		{name: "with source", mutate: func(r *checkcatalog.Row) { r.Source = "judgement" }},
		{name: "empty id", mutate: func(r *checkcatalog.Row) { r.ID = "  " }, wantErr: "id must not be empty"},
		{name: "empty name", mutate: func(r *checkcatalog.Row) { r.Name = "" }, wantErr: "name must not be empty"},
		{name: "bad severity", mutate: func(r *checkcatalog.Row) { r.Severity = "critical" }, wantErr: `severity "critical"`},
		{name: "bad means", mutate: func(r *checkcatalog.Row) { r.Means = "heuristic" }, wantErr: `means "heuristic"`},
		{name: "empty rejects", mutate: func(r *checkcatalog.Row) { r.Rejects = "" }, wantErr: "rejects must say"},
		{name: "fatal on a model", mutate: func(r *checkcatalog.Row) {
			r.Severity = checkcatalog.SeverityFatal
			r.Means = checkcatalog.MeansModel
		}, wantErr: "a fatal check must be deterministic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ok
			tt.mutate(&r)
			err := r.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, checkcatalog.ErrInvalidRow) {
				t.Fatalf("Validate() = %v, want ErrInvalidRow", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

// ─── Reconcile: the bidirectional test ──────────────────────────────────────

// TestReconcileCleanIsThreeLines is the consumer's whole catalog test.
func TestReconcileCleanIsThreeLines(t *testing.T) {
	if err := checkcatalog.Reconcile(catalog(), impls()).Err(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileRowWithNoImplementation(t *testing.T) {
	d := checkcatalog.Reconcile(catalog(), impls()[1:]) // INT-01 declared, not implemented
	if d.Clean() {
		t.Fatal("a promised check with no implementation reconciled clean")
	}
	if len(d.Unimplemented) != 1 || d.Unimplemented[0] != "INT-01" {
		t.Errorf("Unimplemented = %v, want [INT-01]", d.Unimplemented)
	}
	if len(d.Undeclared) != 0 {
		t.Errorf("Undeclared = %v, want none", d.Undeclared)
	}
	if err := d.Err(); err == nil || !strings.Contains(err.Error(), "INT-01 has NO implementation") {
		t.Errorf("Err() = %v, want it to name INT-01", err)
	}
}

func TestReconcileCheckWithNoRow(t *testing.T) {
	d := checkcatalog.Reconcile(catalog()[1:], impls()) // INT-01 runs, undeclared
	if len(d.Undeclared) != 1 || d.Undeclared[0] != "INT-01" {
		t.Errorf("Undeclared = %v, want [INT-01]", d.Undeclared)
	}
	if len(d.Unimplemented) != 0 {
		t.Errorf("Unimplemented = %v, want none", d.Unimplemented)
	}
	if !strings.Contains(d.String(), "undocumented gate") {
		t.Errorf("String() = %q, want the undocumented-gate line", d.String())
	}
}

func TestReconcileFieldMismatch(t *testing.T) {
	cs := impls()
	cs[1].Name = "renamed"
	cs[1].Severity = checkcatalog.SeverityData
	cs[1].Means = checkcatalog.MeansModel
	cs[1].Rejects = "different text"
	d := checkcatalog.Reconcile(catalog(), cs)
	if len(d.Mismatched) != 4 {
		t.Fatalf("Mismatched = %v, want four fields for STR-01", d.Mismatched)
	}
	fields := []string{}
	for _, m := range d.Mismatched {
		if m.ID != "STR-01" {
			t.Errorf("mismatch on %s, want only STR-01", m.ID)
		}
		fields = append(fields, m.Field)
	}
	if got := strings.Join(fields, ","); got != "means,name,rejects,severity" {
		t.Errorf("mismatch fields = %s, want sorted means,name,rejects,severity", got)
	}
}

func TestReconcileDuplicatesAndNilFn(t *testing.T) {
	rows := append(catalog(), row("INT-01", checkcatalog.SeverityFatal, checkcatalog.MeansDeterministic))
	cs := append(impls(), check(row("STR-01", checkcatalog.SeverityMajor, checkcatalog.MeansDeterministic), clean))
	cs[2].Fn = nil
	d := checkcatalog.Reconcile(rows, cs)
	if got := strings.Join(d.DuplicateRows, ","); got != "INT-01" {
		t.Errorf("DuplicateRows = %q, want INT-01", got)
	}
	if got := strings.Join(d.DuplicateChecks, ","); got != "STR-01" {
		t.Errorf("DuplicateChecks = %q, want STR-01", got)
	}
	if got := strings.Join(d.NoFn, ","); got != "STY-01" {
		t.Errorf("NoFn = %q, want STY-01", got)
	}
}

func TestReconcileReportsInvalidRowsOnBothSides(t *testing.T) {
	rows := catalog()
	rows[0].Means = checkcatalog.MeansModel // fatal on a model, in the catalog
	cs := impls()
	cs[3].Rejects = "" // empty rejects, in code
	d := checkcatalog.Reconcile(rows, cs)
	if len(d.Invalid) != 2 {
		t.Fatalf("Invalid = %v, want one per side", d.Invalid)
	}
	if !strings.HasPrefix(d.Invalid[0], "catalog: ") || !strings.HasPrefix(d.Invalid[1], "code: ") {
		t.Errorf("Invalid = %v, want the side named", d.Invalid)
	}
}

func TestReconcileOutputIsOrdered(t *testing.T) {
	first := checkcatalog.Reconcile(catalog()[2:], impls()[:2]).String()
	for range 5 {
		if again := checkcatalog.Reconcile(catalog()[2:], impls()[:2]).String(); again != first {
			t.Fatalf("Reconcile output varies between runs:\n%s\nvs\n%s", first, again)
		}
	}
}

// ─── Run: skipped is not passed ─────────────────────────────────────────────

func TestRunOrdersByIDAndStampsFindings(t *testing.T) {
	cs := impls()
	cs[1].Fn = func(in input) []checkcatalog.Finding {
		return []checkcatalog.Finding{{Message: "unlabelled", Subject: "rec-7"}, {Message: "lowered", Severity: checkcatalog.SeverityData}}
	}
	rep := checkcatalog.Run("fixture", input{Text: "x"}, cs)
	if got := strings.Join(rep.Ran, ","); got != "HON-01,INT-01,STR-01,STY-01" {
		t.Errorf("Ran = %s, want id order", got)
	}
	if len(rep.Findings) != 2 {
		t.Fatalf("Findings = %v, want two", rep.Findings)
	}
	if f := rep.Findings[0]; f.CheckID != "STR-01" || f.Severity != checkcatalog.SeverityMajor || f.Subject != "rec-7" {
		t.Errorf("finding not stamped with its check: %+v", f)
	}
	if f := rep.Findings[1]; f.Severity != checkcatalog.SeverityData {
		t.Errorf("a check's own severity was overwritten: %+v", f)
	}
	if rep.Passed() {
		t.Error("a major finding passed")
	}
	if !strings.Contains(rep.String(), "MAJOR (1)") || !strings.Contains(rep.String(), "DATA (1)") {
		t.Errorf("String() = %q, want severity groups", rep.String())
	}
}

func TestRunSkippedBlockingCheckDoesNotPass(t *testing.T) {
	cs := impls()
	cs[0].Fn = func(input) []checkcatalog.Finding { return checkcatalog.Skipped("reader unavailable") }
	rep := checkcatalog.Run("fixture", input{}, cs)
	if rep.Passed() {
		t.Fatal("a skipped FATAL check passed — a check that cannot run must never look clean")
	}
	if got := strings.Join(rep.Skipped, ","); got != "INT-01" {
		t.Errorf("Skipped = %q, want INT-01", got)
	}
	if len(rep.Findings) != 1 || !rep.Findings[0].Skipped || rep.Findings[0].Severity != checkcatalog.SeverityFatal {
		t.Errorf("skipped finding = %+v, want a fatal Skipped finding", rep.Findings)
	}
	if !strings.Contains(rep.Findings[0].String(), "skipped: reader unavailable") {
		t.Errorf("finding = %q, want the skip reason", rep.Findings[0])
	}
}

func TestRunNilFnIsSkippedNotSilent(t *testing.T) {
	cs := impls()
	cs[2].Fn = nil // STY-01, an improvement: skipped but not blocking
	rep := checkcatalog.Run("fixture", input{}, cs)
	if got := strings.Join(rep.Skipped, ","); got != "STY-01" {
		t.Errorf("Skipped = %q, want STY-01", got)
	}
	if !rep.Passed() {
		t.Error("a skipped non-blocking check blocked")
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Message != "skipped: no implementation" {
		t.Errorf("Findings = %v, want one skipped finding", rep.Findings)
	}
}

func TestRunCleanReport(t *testing.T) {
	rep := checkcatalog.Run("fixture", input{}, impls())
	if !rep.Passed() || len(rep.Skipped) != 0 || len(rep.Findings) != 0 {
		t.Errorf("clean run = %+v", rep)
	}
	if !strings.Contains(rep.String(), "clean") {
		t.Errorf("String() = %q, want clean", rep.String())
	}
}

// ─── the rest of the surface ────────────────────────────────────────────────

func TestDeterministicShareAndRows(t *testing.T) {
	det, total := checkcatalog.DeterministicShare(checkcatalog.Rows(impls()))
	if det != 3 || total != 4 {
		t.Errorf("DeterministicShare = %d/%d, want 3/4", det, total)
	}
}

func TestFindRoundTrips(t *testing.T) {
	for _, c := range impls() {
		got, ok := checkcatalog.Find(impls(), c.ID)
		if !ok || got.ID != c.ID {
			t.Errorf("Find(%q) did not round-trip", c.ID)
		}
	}
	if _, ok := checkcatalog.Find(impls(), "NOPE-99"); ok {
		t.Error("Find invented a check that does not exist")
	}
}
