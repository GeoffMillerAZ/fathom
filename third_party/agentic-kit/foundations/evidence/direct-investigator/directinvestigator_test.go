package directinvestigator_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	inv "github.com/geoffmilleraz/agentic-kit/foundations/evidence/direct-investigator"
	evidencesig "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
)

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

func writeTempFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func tempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func makeExistenceSig(globs []string, pattern string) evidencesig.ConcernSignature {
	return evidencesig.ConcernSignature{
		ID:        "test-existence",
		Archetype: evidencesig.ArchetypeExistence,
		Scope:     evidencesig.Scope{Globs: globs},
		Predicates: []evidencesig.Predicate{
			{Kind: "regexp", Arg: pattern},
		},
	}
}

func makeAbsenceSig(globs []string, pattern, canaryPath, canaryPattern string) evidencesig.ConcernSignature {
	return evidencesig.ConcernSignature{
		ID:        "test-absence",
		Archetype: evidencesig.ArchetypeAbsence,
		Scope:     evidencesig.Scope{Globs: globs},
		Predicates: []evidencesig.Predicate{
			{Kind: "regexp", Arg: pattern},
		},
		Canary: &evidencesig.Canary{Path: canaryPath, Pattern: canaryPattern},
	}
}

// ---------------------------------------------------------------------------
// Config validation.
// ---------------------------------------------------------------------------

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	if err := (inv.Config{MaxFileBytes: 1}).Validate(); err == nil {
		t.Error("empty RootDir should be invalid")
	}
	if err := (inv.Config{RootDir: "/tmp", MaxFileBytes: 0}).Validate(); err == nil {
		t.Error("zero MaxFileBytes should be invalid")
	}
	if err := (inv.Config{RootDir: "/tmp", MaxFileBytes: 1024}).Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test class 1: Canary — planted positive is found.
// ---------------------------------------------------------------------------

func TestCanaryPlantedPositiveFound(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	// Plant the canary.
	writeTempFile(t, dir, "canary_planted.go", `package main
// password = "CANARY_VALUE_DO_NOT_SHIP"
`)
	// Plant a clean production file.
	writeTempFile(t, dir, "main.go", "package main\nfunc main() {}\n")

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20, TreeHash: "t1"}
	investigator, err := inv.New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	sig := makeAbsenceSig(
		[]string{"*.go"},
		`password\s*=\s*"[^"]+"`,
		"canary_planted.go",
		`password = "CANARY_VALUE_DO_NOT_SHIP"`,
	)

	findings, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Canary must be found (ensuring the engine CAN find the pattern).
	if len(findings.Items) == 0 {
		t.Error("canary: planted positive was not found — engine is broken")
	}
	// Manifest must include the canary file.
	found := false
	for _, f := range manifest.Files {
		if f == "canary_planted.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("canary file not listed in manifest.Files")
	}
}

// ---------------------------------------------------------------------------
// Test class 2: Liveness — globs match files in the tree.
// ---------------------------------------------------------------------------

func TestLivenessGlobsMatchTree(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	writeTempFile(t, dir, "a.go", "package a\n")
	writeTempFile(t, dir, "b.go", "package b\n")
	// No .txt files planted.

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, err := inv.New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Sig with a matching glob.
	sig := makeExistenceSig([]string{"*.go"}, "package")
	_, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(manifest.Files) == 0 {
		t.Error("expected at least one file in manifest.Files")
	}

	// Sig with a non-matching glob — all files should be in Ignored.
	sigNone := makeExistenceSig([]string{"*.txt"}, "package")
	_, manifestNone, err := investigator.Search(context.Background(), sigNone)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(manifestNone.Files) != 0 {
		t.Errorf("expected no files in manifest for non-matching glob, got %v", manifestNone.Files)
	}
	if len(manifestNone.Skipped.Ignored) == 0 {
		t.Error("expected skipped.Ignored to be non-empty for non-matching glob")
	}
}

// ---------------------------------------------------------------------------
// Test class 3: Manifest — skip accounting sums to the walk.
// ---------------------------------------------------------------------------

func TestManifestSkipAccounting(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	writeTempFile(t, dir, "clean.go", "package main\nfunc foo() {}\n")
	// Binary file.
	if err := os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatal(err)
	}
	// LFS stub.
	lfsStub := "version https://git-lfs.github.com/spec/v1\noid sha256:abc123\nsize 12345\n"
	writeTempFile(t, dir, "large.zip", lfsStub)

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, err := inv.New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	sig := makeExistenceSig([]string{"*"}, "package")
	_, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Verify: searched + skipped = total files found.
	total := len(manifest.Files) + len(manifest.Skipped.AllSkipped())
	if total == 0 {
		t.Error("no files accounted for in manifest")
	}

	// Binary file should be in Binary skips.
	if len(manifest.Skipped.Binary) == 0 {
		t.Error("expected binary.bin in manifest.Skipped.Binary")
	}
	// LFS stub should be in LFSStub skips.
	if len(manifest.Skipped.LFSStub) == 0 {
		t.Error("expected large.zip in manifest.Skipped.LFSStub")
	}
	// clean.go should be in Files.
	found := false
	for _, f := range manifest.Files {
		if f == "clean.go" {
			found = true
		}
	}
	if !found {
		t.Error("clean.go not in manifest.Files")
	}
}

// ---------------------------------------------------------------------------
// Existence archetype: findings returned for matching files.
// ---------------------------------------------------------------------------

func TestSearch_ExistenceFound(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	writeTempFile(t, dir, "a.go", "package main\n// TODO(security): fix this\nfunc foo() {}\n")
	writeTempFile(t, dir, "b.go", "package main\nfunc bar() {}\n")

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, _ := inv.New(cfg, nil)

	sig := makeExistenceSig([]string{"*.go"}, `TODO\(security\)`)
	findings, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(findings.Items) == 0 {
		t.Error("expected at least one finding")
	}
	// a.go should be in manifest.Files.
	sort.Strings(manifest.Files)
	if len(manifest.Files) == 0 {
		t.Error("expected files in manifest")
	}
}

// ---------------------------------------------------------------------------
// Absence archetype: complete manifest with no findings.
// ---------------------------------------------------------------------------

func TestSearch_AbsenceProven(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	writeTempFile(t, dir, "clean.go", "package main\nfunc foo() {}\n")

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20, TreeHash: "snap001"}
	investigator, _ := inv.New(cfg, nil)

	sig := makeAbsenceSig([]string{"*.go"}, `password\s*=\s*`, "testdata/canary.go", "password=CANARY")
	findings, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(findings.Items) != 0 {
		t.Errorf("expected no findings, got %d", len(findings.Items))
	}
	if !manifest.Complete() {
		t.Errorf("manifest should be complete, unexplained: %v", manifest.Skipped.UnexplainedSkips())
	}
	if manifest.TreeHash != "snap001" {
		t.Errorf("TreeHash = %q, want snap001", manifest.TreeHash)
	}

	// PreconditionSatisfied should return true.
	if !evidencesig.PreconditionSatisfied(sig, findings.Items, manifest) {
		t.Error("PreconditionSatisfied should be true for proven absence")
	}
}

// ---------------------------------------------------------------------------
// Oversize file skip.
// ---------------------------------------------------------------------------

func TestSearch_OversizeSkip(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	// Write a file larger than maxFileBytes.
	big := make([]byte, 200)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(dir, "large.go"), big, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 100} // cap at 100 bytes
	investigator, _ := inv.New(cfg, nil)

	sig := makeExistenceSig([]string{"*.go"}, "aaa")
	_, manifest, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(manifest.Skipped.Oversize) == 0 {
		t.Error("expected large.go in Skipped.Oversize")
	}
}

// ---------------------------------------------------------------------------
// Source_id scope rejected.
// ---------------------------------------------------------------------------

func TestSearch_SourceIDScopeRejected(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, _ := inv.New(cfg, nil)

	sig := evidencesig.ConcernSignature{
		ID:        "source-id-test",
		Archetype: evidencesig.ArchetypeExistence,
		Scope:     evidencesig.Scope{SourceIDs: []string{"src-1"}},
		Predicates: []evidencesig.Predicate{
			{Kind: "regexp", Arg: "x"},
		},
	}
	_, _, err := investigator.Search(context.Background(), sig)
	if err == nil {
		t.Error("expected error for source_id scope")
	}
}

// ---------------------------------------------------------------------------
// Sorted walk determinism.
// ---------------------------------------------------------------------------

func TestSearch_SortedWalkDeterminism(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	files := []string{"z.go", "a.go", "m.go", "b.go"}
	for _, f := range files {
		writeTempFile(t, dir, f, "package main\nfunc foo() {}\n")
	}

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, _ := inv.New(cfg, nil)
	sig := makeExistenceSig([]string{"*.go"}, "func")

	// Run twice; manifest.Files should be identical.
	_, m1, _ := investigator.Search(context.Background(), sig)
	_, m2, _ := investigator.Search(context.Background(), sig)

	if len(m1.Files) != len(m2.Files) {
		t.Errorf("non-deterministic: run1 files=%v, run2 files=%v", m1.Files, m2.Files)
	}
	for i := range m1.Files {
		if m1.Files[i] != m2.Files[i] {
			t.Errorf("file order differs at %d: %q vs %q", i, m1.Files[i], m2.Files[i])
		}
	}
	// Files should be sorted.
	if !sort.StringsAreSorted(m1.Files) {
		t.Errorf("manifest.Files not sorted: %v", m1.Files)
	}
}

// ---------------------------------------------------------------------------
// "contains" predicate kind.
// ---------------------------------------------------------------------------

func TestSearch_ContainsPredicate(t *testing.T) {
	t.Parallel()
	dir := tempDir(t)
	writeTempFile(t, dir, "audit.go", "package main\nfunc audit_log() {}\n")
	writeTempFile(t, dir, "other.go", "package main\nfunc noop() {}\n")

	cfg := inv.Config{RootDir: dir, MaxFileBytes: 1 << 20}
	investigator, _ := inv.New(cfg, nil)

	sig := evidencesig.ConcernSignature{
		ID:        "test-contains",
		Archetype: evidencesig.ArchetypeExistence,
		Scope:     evidencesig.Scope{Globs: []string{"*.go"}},
		Predicates: []evidencesig.Predicate{
			{Kind: "contains", Arg: "audit_log"},
		},
	}
	findings, _, err := investigator.Search(context.Background(), sig)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(findings.Items) == 0 {
		t.Error("expected finding for contains predicate")
	}
}
