package scenariobench

import (
	"path/filepath"
	"reflect"
	"testing"
)

const lcaFixturePath = "testdata/lca/fixture.jsonl"

func TestLoadLCARecords_ParsesFixture(t *testing.T) {
	records, err := LoadLCARecords(lcaFixturePath)
	if err != nil {
		t.Fatalf("LoadLCARecords: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	want := LCARecord{
		TextID:       "fauxtown__ledger-service-1",
		ID:           1,
		RepoOwner:    "fauxtown",
		RepoName:     "ledger-service",
		RepoLanguage: "Go",
		BaseSHA:      "d1e951aa499500523bfd6f600c967afdea0bd24c",
		IssueTitle:   "Balance returns nil error for an empty account ID",
		IssueBody:    "Ledger.Balance(ctx, \"\") should refuse an empty accountID the same way Post does via validateEntry, but it currently returns (0, nil). The check belongs next to the other sentinel-error validation.",
		ChangedFiles: []string{"ledger.go", "errors.go"},
	}
	if !reflect.DeepEqual(records[0], want) {
		t.Errorf("record 0 = %+v, want %+v", records[0], want)
	}
	if records[1].RepoName != "payment-service" || records[2].RepoName != "worktree-service" {
		t.Errorf("unexpected repo order: %s, %s", records[1].RepoName, records[2].RepoName)
	}
}

func TestLoadLCARecords_MissingFile(t *testing.T) {
	if _, err := LoadLCARecords(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Error("expected an error for a missing dataset file")
	}
}

func TestLoadLCARecords_MalformedLineNamesLineNumber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	writeTestFile(t, path, "{\"repo_owner\":\"fauxtown\"}\nnot json at all\n")
	_, err := LoadLCARecords(path)
	if err == nil {
		t.Fatal("expected an error for a malformed line")
	}
	if got := err.Error(); !containsAll(got, "line 2") {
		t.Errorf("error %q does not name the malformed line", got)
	}
}

func TestLoadLCARecords_BlankLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blank.jsonl")
	rec := `{"repo_owner":"fauxtown","repo_name":"ledger-service","base_sha":"d1e951aa499500523bfd6f600c967afdea0bd24c","issue_title":"x","changed_files":["ledger.go"]}`
	writeTestFile(t, path, "\n"+rec+"\n\n"+rec+"\n\n")
	records, err := LoadLCARecords(path)
	if err != nil {
		t.Fatalf("LoadLCARecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (blank lines skipped), got %d", len(records))
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestLCATaskFromRecord(t *testing.T) {
	rec := LCARecord{
		TextID:       "fauxtown__ledger-service-1",
		RepoOwner:    "fauxtown",
		RepoName:     "ledger-service",
		BaseSHA:      "d1e951aa499500523bfd6f600c967afdea0bd24c",
		IssueTitle:   "Balance returns nil error for an empty account ID",
		IssueBody:    "See validateEntry.",
		ChangedFiles: []string{"errors.go", "ledger.go"},
	}
	task, err := LCATaskFromRecord(rec, lcaFixturePath, 0)
	if err != nil {
		t.Fatalf("LCATaskFromRecord: %v", err)
	}
	if task.ID != "fauxtown__ledger-service-1" {
		t.Errorf("ID = %q", task.ID)
	}
	if task.Category != CategoryLocalization {
		t.Errorf("Category = %q, want %q", task.Category, CategoryLocalization)
	}
	if task.Subject.RepoURL != "https://github.com/fauxtown/ledger-service" {
		t.Errorf("RepoURL = %q", task.Subject.RepoURL)
	}
	if task.Subject.BaseCommit != rec.BaseSHA {
		t.Errorf("BaseCommit = %q", task.Subject.BaseCommit)
	}
	if !task.Oracle.Localization {
		t.Error("expected Oracle.Localization to be set")
	}
	if !reflect.DeepEqual(task.Gold.Files, []string{"errors.go", "ledger.go"}) {
		t.Errorf("Gold.Files = %v (want sorted, deduplicated)", task.Gold.Files)
	}
	if !containsAll(task.Prompt, rec.IssueTitle, rec.IssueBody, rec.RepoOwner, rec.RepoName, rec.BaseSHA) {
		t.Errorf("Prompt missing expected content: %s", task.Prompt)
	}
	if task.Predictions[LCAArmFathom] == "" || task.Predictions[LCAArmControl] == "" {
		t.Error("expected a registered prediction for both arms")
	}
	if task.Derivation == "" {
		t.Error("expected a non-empty derivation")
	}
	if task.Sessions.Mode != ModeSingle || task.Sessions.Count != 1 {
		t.Errorf("Sessions = %+v, want a single cold session", task.Sessions)
	}
	// A localization task must never be change-shaped (no plan-first).
	if isChangeShaped(task) {
		t.Error("expected a localization task to NOT be change-shaped")
	}
}

func TestLCATaskFromRecord_MissingFieldsRefused(t *testing.T) {
	cases := []struct {
		name string
		rec  LCARecord
	}{
		{"no repo_owner", LCARecord{RepoName: "x", BaseSHA: "a", ChangedFiles: []string{"f.go"}}},
		{"no repo_name", LCARecord{RepoOwner: "x", BaseSHA: "a", ChangedFiles: []string{"f.go"}}},
		{"no base_sha", LCARecord{RepoOwner: "x", RepoName: "y", ChangedFiles: []string{"f.go"}}},
		{"no changed_files", LCARecord{RepoOwner: "x", RepoName: "y", BaseSHA: "a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := LCATaskFromRecord(c.rec, "test.jsonl", 0); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestLCATaskFromRecord_SynthesizesIDWhenTextIDEmpty(t *testing.T) {
	rec := LCARecord{RepoOwner: "fauxtown", RepoName: "ledger-service", BaseSHA: "abc", ChangedFiles: []string{"ledger.go"}}
	task, err := LCATaskFromRecord(rec, "test.jsonl", 7)
	if err != nil {
		t.Fatalf("LCATaskFromRecord: %v", err)
	}
	if task.ID != "fauxtown__ledger-service-7" {
		t.Errorf("ID = %q, want a synthesized id naming the record index", task.ID)
	}
}

func TestBuildLCACorpus(t *testing.T) {
	records, err := LoadLCARecords(lcaFixturePath)
	if err != nil {
		t.Fatalf("LoadLCARecords: %v", err)
	}
	corpus, err := BuildLCACorpus(records, lcaFixturePath)
	if err != nil {
		t.Fatalf("BuildLCACorpus: %v", err)
	}
	if len(corpus.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(corpus.Tasks))
	}
	if _, ok := corpus.Arms[LCAArmFathom]; !ok {
		t.Error("expected a fathom arm")
	}
	if _, ok := corpus.Arms[LCAArmControl]; !ok {
		t.Error("expected a control arm")
	}
	if !corpus.Arms[LCAArmFathom].Wired {
		t.Error("expected the fathom arm to be wired")
	}
	if corpus.Arms[LCAArmControl].Wired {
		t.Error("expected the control arm to be unwired")
	}
	if corpus.Criterion.ID != LCACriterionID {
		t.Errorf("Criterion.ID = %q, want %q", corpus.Criterion.ID, LCACriterionID)
	}
	if corpus.Criterion.Treatment != LCAArmFathom || corpus.Criterion.Control != LCAArmControl {
		t.Errorf("Criterion arms = %q/%q", corpus.Criterion.Treatment, corpus.Criterion.Control)
	}
	if len(corpus.Sources) != 1 || corpus.Sources[0].URL == "" || corpus.Sources[0].License == "" {
		t.Errorf("expected one Source pin with a URL and license, got %+v", corpus.Sources)
	}
}

func TestBuildLCACorpus_DuplicateIDRefused(t *testing.T) {
	rec := LCARecord{TextID: "dup", RepoOwner: "fauxtown", RepoName: "ledger-service", BaseSHA: "abc", ChangedFiles: []string{"ledger.go"}}
	if _, err := BuildLCACorpus([]LCARecord{rec, rec}, "test.jsonl"); err == nil {
		t.Error("expected an error for a duplicate task id")
	}
}

func TestBuildLCACorpus_NoRecords(t *testing.T) {
	if _, err := BuildLCACorpus(nil, "test.jsonl"); err == nil {
		t.Error("expected an error for an empty record set")
	}
}

func TestCanonicalCorpusSHA256_DeterministicAndSensitive(t *testing.T) {
	records, err := LoadLCARecords(lcaFixturePath)
	if err != nil {
		t.Fatalf("LoadLCARecords: %v", err)
	}
	c1, err := BuildLCACorpus(records, lcaFixturePath)
	if err != nil {
		t.Fatalf("BuildLCACorpus: %v", err)
	}
	c2, err := BuildLCACorpus(records, lcaFixturePath)
	if err != nil {
		t.Fatalf("BuildLCACorpus: %v", err)
	}
	sha1, err := CanonicalCorpusSHA256(c1)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256: %v", err)
	}
	sha2, err := CanonicalCorpusSHA256(c2)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256: %v", err)
	}
	if sha1 != sha2 {
		t.Errorf("identical corpora hashed differently: %s vs %s", sha1, sha2)
	}

	c3, err := BuildLCACorpus(records[:2], lcaFixturePath)
	if err != nil {
		t.Fatalf("BuildLCACorpus: %v", err)
	}
	sha3, err := CanonicalCorpusSHA256(c3)
	if err != nil {
		t.Fatalf("CanonicalCorpusSHA256: %v", err)
	}
	if sha3 == sha1 {
		t.Error("a different corpus (fewer tasks) hashed the same as the full one")
	}
}

// TestExternalCorpusCriteria_RegistersLCA is wave N15 item 1's own proof:
// the render-lookup fix needs LCACriterionID resolvable outside of a
// loaded corpus (criteriaBySHA256 can never glob an in-memory external
// corpus's own sha), and this is the map that supplies it.
func TestExternalCorpusCriteria_RegistersLCA(t *testing.T) {
	criteria := ExternalCorpusCriteria()
	crit, ok := criteria[LCACriterionID]
	if !ok {
		t.Fatalf("expected ExternalCorpusCriteria to register %q, got keys %v", LCACriterionID, criteria)
	}
	if crit.ID != LCACriterionID {
		t.Errorf("crit.ID = %q, want %q", crit.ID, LCACriterionID)
	}
	if crit.Treatment != LCAArmFathom || crit.Control != LCAArmControl {
		t.Errorf("Treatment/Control = %q/%q, want %q/%q", crit.Treatment, crit.Control, LCAArmFathom, LCAArmControl)
	}
	if crit.MinTrials != 3 {
		t.Errorf("MinTrials = %d, want 3 (VB-27's own floor)", crit.MinTrials)
	}
}
