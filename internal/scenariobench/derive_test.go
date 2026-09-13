package scenariobench

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multiSWEFixtureRow writes one Multi-SWE-bench-shaped JSONL row. Kept
// tiny and hand-built so this test never depends on the network or on
// workspace/bench/sources (which TestPublicScenarioCorpusIsDerived alone
// reaches for).
func multiSWEFixtureRow(t *testing.T, id, org, repo, baseSha, patch string, extra map[string]any) string {
	t.Helper()
	row := map[string]any{
		"org": org, "repo": repo, "number": 1, "title": "t", "body": "b",
		"base":            map[string]any{"sha": baseSha},
		"resolved_issues": []any{map[string]any{"number": 1, "title": "issue", "body": "problem statement"}},
		"fix_patch":       patch,
	}
	if id != "" {
		row["instance_id"] = id
	}
	for k, v := range extra {
		row[k] = v
	}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal fixture row: %v", err)
	}
	return string(raw)
}

func writeSnapshot(t *testing.T, dir, name string, rows []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("writing fixture snapshot: %v", err)
	}
	return path
}

const singleFilePatch = `diff --git a/pkg/a.go b/pkg/a.go
index 1111111..2222222 100644
--- a/pkg/a.go
+++ b/pkg/a.go
@@ -1,3 +1,3 @@
 package pkg

-func F() {}
+func F() { println("fixed") }
`

// goModPlusCodePatch bumps go.mod AND touches a real gold-eligible Go file,
// so the instance clears selection (>=1 non-test Go file) and rule 1
// (go.mod touched) can be observed winning over rule 3/4 end to end.
const goModPlusCodePatch = `diff --git a/go.mod b/go.mod
index 1111111..2222222 100644
--- a/go.mod
+++ b/go.mod
@@ -1,1 +1,1 @@
-require y v1.0.0
+require y v1.1.0
diff --git a/pkg/a.go b/pkg/a.go
index 1111111..2222222 100644
--- a/pkg/a.go
+++ b/pkg/a.go
@@ -1,1 +1,1 @@
-func F() {}
+func F() { println("fixed") }
`

const twoPackagePatch = `diff --git a/pkg/a/a.go b/pkg/a/a.go
index 1111111..2222222 100644
--- a/pkg/a/a.go
+++ b/pkg/a/a.go
@@ -1,1 +1,2 @@
 package a
+var X = 1
diff --git a/pkg/b/b.go b/pkg/b/b.go
index 1111111..2222222 100644
--- a/pkg/b/b.go
+++ b/pkg/b/b.go
@@ -1,1 +1,2 @@
 package b
+var Y = 1
`

func TestDeriveScenarioTasks_RequiresAtLeastOneMultiSWEPin(t *testing.T) {
	if _, err := DeriveScenarioTasks(DeriveOptions{}); err == nil {
		t.Fatalf("want an error with no MultiSWE pins")
	}
}

func TestDeriveScenarioTasks_NoCloneFallback(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-1", "org", "repo", "deadbeef01", singleFilePatch, nil),
	}
	path := writeSnapshot(t, dir, "multi-swe-fixture.jsonl", rows)

	corpus, err := DeriveScenarioTasks(DeriveOptions{
		MultiSWE:  []SnapshotPin{{Path: path, Revision: "test-rev"}},
		SkipClone: true,
	})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	if len(corpus.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(corpus.Tasks))
	}
	task := corpus.Tasks[0]
	if len(task.Requires) != 1 || task.Requires[0] != "subject-clone" {
		t.Fatalf("Requires = %v, want [subject-clone] when cloning is disabled", task.Requires)
	}
	if len(task.Gold.Files) != 1 || task.Gold.Files[0] != "pkg/a.go" {
		t.Fatalf("Gold.Files = %v, want [pkg/a.go] from the diff paths", task.Gold.Files)
	}
	if len(task.Gold.Functions) != 0 {
		t.Fatalf("Gold.Functions = %v, want empty without a clone", task.Gold.Functions)
	}
	if !strings.Contains(task.Derivation, "subject clone unavailable") {
		t.Fatalf("Derivation = %q, want it to say why functions are empty", task.Derivation)
	}
	if task.Category != CategoryBugFix {
		t.Fatalf("Category = %q, want %q for a single-file change", task.Category, CategoryBugFix)
	}
	if task.Predictions["full"] == "" || task.Predictions["control"] == "" {
		t.Fatalf("Predictions = %v, want an entry per arm", task.Predictions)
	}
	if task.Sessions.Count != 2 || task.Sessions.MaxTurns != 40 || task.Sessions.Mode != ModeSingle {
		t.Fatalf("Sessions = %+v, want {2 40 single ...}", task.Sessions)
	}
	if corpus.Criterion.ID != "M2-1" || corpus.Criterion.MinTrials != 2 || corpus.Criterion.MaxTokensPerSuccessRatio != 1.0 {
		t.Fatalf("Criterion = %+v, want the registered M2-1 bar", corpus.Criterion)
	}
	if _, ok := corpus.Arms["full"]; !ok {
		t.Fatalf("Arms missing full")
	}
	if _, ok := corpus.Arms["control"]; !ok {
		t.Fatalf("Arms missing control")
	}
	if corpus.Arms["full"].Surface != "fathom" || !corpus.Arms["full"].Wired {
		t.Fatalf("Arms[full] = %+v, want wired:true surface:fathom", corpus.Arms["full"])
	}
	if corpus.Arms["control"].Wired || !corpus.Arms["control"].NotesFile {
		t.Fatalf("Arms[control] = %+v, want wired:false notes_file:true", corpus.Arms["control"])
	}
}

func TestDeriveScenarioTasks_GoModTouchedIsDependencyUpgrade(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-2", "org", "repo", "deadbeef02", goModPlusCodePatch, nil),
	}
	path := writeSnapshot(t, dir, "gomod.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	if len(corpus.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(corpus.Tasks))
	}
	if corpus.Tasks[0].Category != CategoryUpgrade {
		t.Fatalf("Category = %q, want %q (rule 1 wins end to end)", corpus.Tasks[0].Category, CategoryUpgrade)
	}
	if got := corpus.Tasks[0].Predictions["full"]; !strings.Contains(got, "no effect") {
		t.Fatalf("Predictions[full] = %q, want a no-effect prediction for dependency_upgrade", got)
	}
}

func TestDeriveScenarioTasks_GoModAloneIsNotGoldEligible(t *testing.T) {
	dir := t.TempDir()
	goModOnly := `diff --git a/go.mod b/go.mod
index 1111111..2222222 100644
--- a/go.mod
+++ b/go.mod
@@ -1,1 +1,1 @@
-require y v1.0.0
+require y v1.1.0
`
	rows := []string{multiSWEFixtureRow(t, "org__repo-2b", "org", "repo", "deadbeef02b", goModOnly, nil)}
	path := writeSnapshot(t, dir, "gomod-only.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	if len(corpus.Tasks) != 0 {
		t.Fatalf("len(Tasks) = %d, want 0 (a go.mod-only patch touches zero non-test Go source files, so ADR-031's selection rule excludes it)", len(corpus.Tasks))
	}
}

func TestDeriveScenarioTasks_CrossPackagePredictsWin(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-3", "org", "repo", "deadbeef03", twoPackagePatch, nil),
	}
	path := writeSnapshot(t, dir, "twopkg.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	if len(corpus.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(corpus.Tasks))
	}
	task := corpus.Tasks[0]
	if task.Category != CategoryFeature {
		t.Fatalf("Category = %q, want %q", task.Category, CategoryFeature)
	}
	if !strings.Contains(task.Predictions["full"], "wins") {
		t.Fatalf("Predictions[full] = %q, want it to predict a win", task.Predictions["full"])
	}
}

func TestDeriveScenarioTasks_TestNamesFromSourceFields(t *testing.T) {
	dir := t.TempDir()
	extra := map[string]any{
		"n2p_tests": map[string]any{"TestFoo": map[string]any{"fix": "PASS"}},
		"p2p_tests": map[string]any{"TestBar": map[string]any{"fix": "PASS"}},
	}
	rows := []string{multiSWEFixtureRow(t, "org__repo-4", "org", "repo", "deadbeef04", singleFilePatch, extra)}
	path := writeSnapshot(t, dir, "tests.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	task := corpus.Tasks[0]
	if len(task.Oracle.FailToPass) != 1 || task.Oracle.FailToPass[0] != "TestFoo" {
		t.Fatalf("Oracle.FailToPass = %v, want [TestFoo]", task.Oracle.FailToPass)
	}
	if len(task.Oracle.PassToPass) != 1 || task.Oracle.PassToPass[0] != "TestBar" {
		t.Fatalf("Oracle.PassToPass = %v, want [TestBar]", task.Oracle.PassToPass)
	}
}

func TestDeriveScenarioTasks_MergedAtKeptWhenPresent(t *testing.T) {
	dir := t.TempDir()
	extra := map[string]any{"merged_at": "2026-07-01T00:00:00Z"}
	rows := []string{multiSWEFixtureRow(t, "org__repo-5", "org", "repo", "deadbeef05", singleFilePatch, extra)}
	path := writeSnapshot(t, dir, "merged.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	task := corpus.Tasks[0]
	if task.MergedAt != "2026-07-01T00:00:00Z" {
		t.Fatalf("MergedAt = %q, want the source's merged_at kept", task.MergedAt)
	}
	if v, _ := corpus.Notes["merged_at_after_cutoff"].(int); v != 1 {
		t.Fatalf("Notes[merged_at_after_cutoff] = %v, want 1", corpus.Notes["merged_at_after_cutoff"])
	}
}

func TestDeriveScenarioTasks_PreferPostCutoffOrderingUnderNCap(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-zzz-old", "org", "repo", "deadbeef06", singleFilePatch, map[string]any{"merged_at": "2026-01-01T00:00:00Z"}),
		multiSWEFixtureRow(t, "org__repo-aaa-new", "org", "repo", "deadbeef07", singleFilePatch, map[string]any{"merged_at": "2026-08-01T00:00:00Z"}),
	}
	path := writeSnapshot(t, dir, "cutoff.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true, N: 1})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	if len(corpus.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1 (N=1 cap)", len(corpus.Tasks))
	}
	// Sorted by instance_id alone, "org__repo-aaa-new" would win; the
	// post-cutoff preference must pick it regardless of id order here
	// because it is ALSO the alphabetically-first id, so make the
	// pre-cutoff one sort first by id to prove preference, not luck.
	if corpus.Tasks[0].MergedAt != "2026-08-01T00:00:00Z" {
		t.Fatalf("MergedAt = %q, want the post-cutoff instance preferred over the pre-cutoff one", corpus.Tasks[0].MergedAt)
	}
}

func TestDeriveScenarioTasks_Deterministic(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-10", "org", "repo", "deadbeef10", singleFilePatch, nil),
		multiSWEFixtureRow(t, "org__repo-11", "org", "repo", "deadbeef11", twoPackagePatch, nil),
	}
	path := writeSnapshot(t, dir, "det.jsonl", rows)
	opts := DeriveOptions{MultiSWE: []SnapshotPin{{Path: path, Revision: "rev1"}}, SkipClone: true}

	c1, err := DeriveScenarioTasks(opts)
	if err != nil {
		t.Fatalf("DeriveScenarioTasks (1): %v", err)
	}
	c2, err := DeriveScenarioTasks(opts)
	if err != nil {
		t.Fatalf("DeriveScenarioTasks (2): %v", err)
	}
	b1, err := canonicalMarshal(c1)
	if err != nil {
		t.Fatalf("marshal (1): %v", err)
	}
	b2, err := canonicalMarshal(c2)
	if err != nil {
		t.Fatalf("marshal (2): %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("two derivations from the same snapshot were not byte-identical")
	}
}

// TestPublicScenarioCorpusIsDerived re-derives testdata/scenariobench/
// public-go.json from the real pinned snapshots when they are present
// under workspace/bench/sources/ (docs/bench/scenarios.md step 1) and a
// subject clone cache under workspace/bench/subjects/, and compares the
// result byte-for-byte against the committed file. It skips — never
// fails — when a snapshot is missing, naming which one, because most
// environments (including a fresh clone or CI) never fetch them
// (workspace/ is gitignored by design).
func TestPublicScenarioCorpusIsDerived(t *testing.T) {
	// go test's cwd is this package's directory; the pinned snapshots and
	// subject clones live under the repo-root workspace/ (gitignored).
	const root = "../.."
	pins := []struct {
		path     string
		revision string
	}{
		{root + "/workspace/bench/sources/multi-swe-cli__cli_dataset.jsonl", "56ff018c04a38e27ada1e9d0a6d5839a51f88f0d"},
		{root + "/workspace/bench/sources/multi-swe-grpc__grpc-go_dataset.jsonl", "56ff018c04a38e27ada1e9d0a6d5839a51f88f0d"},
		{root + "/workspace/bench/sources/multi-swe-zeromicro__go-zero_dataset.jsonl", "56ff018c04a38e27ada1e9d0a6d5839a51f88f0d"},
	}
	var opts DeriveOptions
	for _, p := range pins {
		if _, err := os.Stat(p.path); err != nil {
			t.Skipf("missing pinned snapshot %s (docs/bench/scenarios.md step 1 fetches it): %v", p.path, err)
		}
		opts.MultiSWE = append(opts.MultiSWE, SnapshotPin{Path: p.path, Revision: p.revision})
	}
	if _, err := os.Stat(root + "/workspace/bench/sources/swe-explore-bench.final.public.jsonl"); err == nil {
		opts.SWEExplore = append(opts.SWEExplore, SnapshotPin{
			Path: root + "/workspace/bench/sources/swe-explore-bench.final.public.jsonl", Revision: "bdb0ae45d7c337d9e1dc3ebfe2a0af6bc7c1fbd9",
		})
	}
	opts.SubjectsDir = root + "/workspace/bench/subjects"

	got, err := DeriveScenarioTasks(opts)
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	gotBytes, err := canonicalMarshal(got)
	if err != nil {
		t.Fatalf("marshal derived corpus: %v", err)
	}

	committed := root + "/testdata/scenariobench/public-go.json"
	wantBytes, err := os.ReadFile(committed)
	if err != nil {
		t.Skipf("committed corpus %s not present yet: %v", committed, err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Errorf("re-derivation does not match the committed corpus — regenerate testdata/scenariobench/public-go.json (docs/bench/scenarios.md)")
	}
}

// TestDeriveScenarioTasks_TestPatchCarriedVerbatim proves the source's
// hidden test_patch field (not read by publicloc.ReadMultiSWEBench) is
// carried verbatim into Gold.TestPatch, with a matching TestPatchSHA256,
// and that the derivation note says so when the field is absent.
func TestDeriveScenarioTasks_TestPatchCarriedVerbatim(t *testing.T) {
	dir := t.TempDir()
	const testPatch = `diff --git a/pkg/a_test.go b/pkg/a_test.go
new file mode 100644
--- /dev/null
+++ b/pkg/a_test.go
@@ -0,0 +1,3 @@
+package pkg
+
+func TestF(t *testing.T) {}
`
	rows := []string{
		multiSWEFixtureRow(t, "org__repo-tp", "org", "repo", "deadbeeftp", singleFilePatch, map[string]any{"test_patch": testPatch}),
	}
	path := writeSnapshot(t, dir, "testpatch.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	task := corpus.Tasks[0]
	if task.Gold.TestPatch != testPatch {
		t.Fatalf("Gold.TestPatch = %q, want the source's test_patch verbatim", task.Gold.TestPatch)
	}
	sum := sha256.Sum256([]byte(testPatch))
	wantSHA := hex.EncodeToString(sum[:])
	if task.Gold.TestPatchSHA256 != wantSHA {
		t.Fatalf("Gold.TestPatchSHA256 = %q, want %q", task.Gold.TestPatchSHA256, wantSHA)
	}
	if strings.Contains(task.Derivation, "no test patch") {
		t.Fatalf("Derivation = %q, should not claim no test patch when one was supplied", task.Derivation)
	}
}

// TestDeriveScenarioTasks_NoTestPatchIsNoted proves the absence of a
// test_patch field is empty (never a placeholder) and noted in Derivation.
func TestDeriveScenarioTasks_NoTestPatchIsNoted(t *testing.T) {
	dir := t.TempDir()
	rows := []string{multiSWEFixtureRow(t, "org__repo-notp", "org", "repo", "deadbeefnotp", singleFilePatch, nil)}
	path := writeSnapshot(t, dir, "notestpatch.jsonl", rows)
	corpus, err := DeriveScenarioTasks(DeriveOptions{MultiSWE: []SnapshotPin{{Path: path}}, SkipClone: true})
	if err != nil {
		t.Fatalf("DeriveScenarioTasks: %v", err)
	}
	task := corpus.Tasks[0]
	if task.Gold.TestPatch != "" || task.Gold.TestPatchSHA256 != "" {
		t.Fatalf("Gold.TestPatch(SHA256) = %q/%q, want both empty", task.Gold.TestPatch, task.Gold.TestPatchSHA256)
	}
	if !strings.Contains(task.Derivation, "no test patch") {
		t.Fatalf("Derivation = %q, want it to note the missing test patch", task.Derivation)
	}
}

// TestReuseGoCorpusIsDerived re-derives testdata/scenariobench/
// reuse-go.json from the committed public-go.json (DeriveReuseTasks with
// the registered n=3, workers=3) and compares byte-for-byte — both files
// are committed, so unlike TestPublicScenarioCorpusIsDerived this never
// skips.
func TestReuseGoCorpusIsDerived(t *testing.T) {
	const root = "../.."
	pub, err := LoadScenarioCorpus(root + "/testdata/scenariobench/public-go.json")
	if err != nil {
		t.Fatalf("loading public-go.json: %v", err)
	}
	got := DeriveReuseTasks(pub, 3, 3)
	gotBytes, err := canonicalMarshal(got)
	if err != nil {
		t.Fatalf("marshal derived reuse corpus: %v", err)
	}
	wantBytes, err := os.ReadFile(root + "/testdata/scenariobench/reuse-go.json")
	if err != nil {
		t.Skipf("committed reuse corpus %s not present yet: %v", root+"/testdata/scenariobench/reuse-go.json", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Errorf("re-derivation does not match the committed reuse-go.json — regenerate it with DeriveReuseTasks(pub, 3, 3)")
	}
}
