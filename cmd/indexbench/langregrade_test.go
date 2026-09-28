package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangRegrade: a reading measured before range_agreement could be
// not_measured is recomputed from its recorded counts — no extraction —
// and gains the level the current rules give it, with a note saying so.
func TestCmdLangRegrade(t *testing.T) {
	m := langbench.Manifest{ID: "html", Kind: "language", RequiredKinds: []string{"label"},
		Kinds: map[string]langbench.KindSpec{"label": {SourceKind: "HTMLId", Canonical: "label"}}}
	r := langbench.NewReading(m, "L3")
	r.FathomCommit = "old1234"
	langbench.MergeSubject(&r, langbench.Subject{Name: "s", Files: 10, Bytes: 4 << 20,
		Counts:   map[string]int{langbench.CntOracleDefs: 100, langbench.CntFathomDefs: 100, langbench.CntMatched: 100, langbench.CntKindEqual: 100},
		Families: map[string]string{langbench.FamilyL1: "B"}, KindIntersection: []string{"label"},
		Tenancy: &langbench.SubjectTenancy{ExtractMS: 100, MaxFileMS: 10}})
	// Simulate the old harness's verdict on the same counts.
	r.NotMeasured, r.Notes = nil, []string{}
	r.LevelMet, r.Grade.L1 = langbench.LevelNone, langbench.GradeUnmet
	b, err := langbench.MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "html.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdLangRegrade([]string{"-commit", "new5678", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := loadReading(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.LevelMet != "L1" || got.Grade.L1 != langbench.GradeGood || got.FathomCommit != "old1234" {
		t.Fatalf("regraded: level %s grade %+v commit %s", got.LevelMet, got.Grade, got.FathomCommit)
	}
	if !strings.Contains(strings.Join(got.Notes, "\n"), "regraded at new5678") {
		t.Errorf("notes lack the regrade: %v", got.Notes)
	}
	if code := cmdLangRegrade(nil, &stdout, &stderr); code != 2 {
		t.Errorf("no args exit %d, want 2", code)
	}
}
