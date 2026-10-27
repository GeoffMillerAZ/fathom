package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

// TestCmdLangRegradeKeepsMeasuredRequiredKinds (owner ruling E2-R3): a reading scored under
// a manifest other than its embedded dialect's — astro's, whose L1 is the hosted TypeScript
// ranges' kinds on top of a manifest that requires none of its own — keeps the required_kinds
// it recorded. Regrading reads no new data, so it never empties a measured list.
func TestCmdLangRegradeKeepsMeasuredRequiredKinds(t *testing.T) {
	// astro's committed manifest requires no kinds; the reading records the measured three.
	t.Chdir(filepath.Join("..", ".."))
	spec, _, err := resolveBenchSpec("astro", "")
	if err != nil {
		t.Fatalf("resolve astro: %v", err)
	}
	if got := spec.Manifest.RequiredCanonical(); len(got) != 0 {
		t.Skipf("the astro manifest now requires %v, so this case no longer bites", got)
	}
	r := langbench.NewReading(spec.Manifest, "L3")
	r.RequiredKinds = []string{"constant", "function", "interface"}
	langbench.MergeSubject(&r, langbench.Subject{Name: "s", Files: 10, Bytes: 4 << 20,
		Counts:           map[string]int{langbench.CntOracleDefs: 100, langbench.CntFathomDefs: 100, langbench.CntMatched: 100, langbench.CntKindEqual: 100},
		Families:         map[string]string{langbench.FamilyL1: "B"},
		KindIntersection: []string{"constant", "function", "interface"},
		Tenancy:          &langbench.SubjectTenancy{ExtractMS: 100, MaxFileMS: 10}})
	r.RequiredKinds = []string{"constant", "function", "interface"}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "astro.json")
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
	if want := []string{"constant", "function", "interface"}; !slices.Equal(got.RequiredKinds, want) {
		t.Errorf("required_kinds %v, want the measured %v", got.RequiredKinds, want)
	}
}

// TestCmdLangRegradeAdoptsTheManifestsKinds: when the embedded manifest does require kinds,
// regrading still refreshes a reading's list from it — that is the required-kind mapping's
// own regrade path.
func TestCmdLangRegradeAdoptsTheManifestsKinds(t *testing.T) {
	m := langbench.Manifest{ID: "html", Kind: "language", RequiredKinds: []string{"label"},
		Kinds: map[string]langbench.KindSpec{"label": {SourceKind: "HTMLId", Canonical: "label"}}}
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "extractor", "dialects", "html")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mb, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dialect.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	r := langbench.NewReading(m, "L3")
	langbench.MergeSubject(&r, langbench.Subject{Name: "s", Files: 10, Bytes: 4 << 20,
		Counts:   map[string]int{langbench.CntOracleDefs: 100, langbench.CntFathomDefs: 100, langbench.CntMatched: 100, langbench.CntKindEqual: 100},
		Families: map[string]string{langbench.FamilyL1: "B"}, KindIntersection: []string{"label"},
		Tenancy: &langbench.SubjectTenancy{ExtractMS: 100, MaxFileMS: 10}})
	r.RequiredKinds = []string{"stale"}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "html.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	if code := cmdLangRegrade([]string{"-commit", "new5678", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := loadReading(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"label"}; !slices.Equal(got.RequiredKinds, want) {
		t.Errorf("required_kinds %v, want the manifest's %v", got.RequiredKinds, want)
	}
}
