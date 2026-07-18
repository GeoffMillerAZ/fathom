package evidencesignature_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	sig "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-signature"
)

// ---------------------------------------------------------------------------
// Scope validation
// ---------------------------------------------------------------------------

func TestScopeValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		scope   sig.Scope
		wantErr bool
	}{
		{"globs only", sig.Scope{Globs: []string{"**/*.go"}}, false},
		{"source_ids only", sig.Scope{SourceIDs: []string{"src-1"}}, false},
		{"both sets", sig.Scope{Globs: []string{"*.go"}, SourceIDs: []string{"src-1"}}, true},
		{"neither set", sig.Scope{}, true},
		{"empty glob string", sig.Scope{Globs: []string{""}}, true},
		{"empty source id", sig.Scope{SourceIDs: []string{""}}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.scope.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Scope.Validate() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ConcernSignature validation
// ---------------------------------------------------------------------------

func TestConcernSignatureValidate(t *testing.T) {
	t.Parallel()

	validExistence := sig.ConcernSignature{
		ID:        "test-existence",
		Archetype: sig.ArchetypeExistence,
		Scope:     sig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []sig.Predicate{
			{Kind: "regexp", Arg: "TODO"},
		},
	}

	cases := []struct {
		name    string
		sig     sig.ConcernSignature
		wantErr bool
	}{
		{
			name:    "valid existence",
			sig:     validExistence,
			wantErr: false,
		},
		{
			name: "valid absence with canary",
			sig: sig.ConcernSignature{
				ID:        "test-absence",
				Archetype: sig.ArchetypeAbsence,
				Scope:     sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{
					{Kind: "regexp", Arg: "password="},
				},
				Canary: &sig.Canary{Path: "testdata/canary.go", Pattern: "password=CANARY"},
			},
			wantErr: false,
		},
		{
			name: "valid threshold",
			sig: sig.ConcernSignature{
				ID:        "test-threshold",
				Archetype: sig.ArchetypeThreshold,
				Scope:     sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{
					{Kind: "contains", Arg: "audit"},
				},
				Threshold: &sig.Threshold{Op: sig.ThresholdGTE, Value: 3},
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			sig: func() sig.ConcernSignature {
				s := validExistence
				s.ID = ""
				return s
			}(),
			wantErr: true,
		},
		{
			name: "invalid archetype",
			sig: sig.ConcernSignature{
				ID:        "test-bad",
				Archetype: "unknown",
				Scope:     sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{
					{Kind: "regexp", Arg: "x"},
				},
			},
			wantErr: true,
		},
		{
			name: "absence without canary",
			sig: sig.ConcernSignature{
				ID:        "test-absence-no-canary",
				Archetype: sig.ArchetypeAbsence,
				Scope:     sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{
					{Kind: "regexp", Arg: "password="},
				},
			},
			wantErr: true,
		},
		{
			name: "threshold without threshold struct",
			sig: sig.ConcernSignature{
				ID:        "test-threshold-missing",
				Archetype: sig.ArchetypeThreshold,
				Scope:     sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{
					{Kind: "contains", Arg: "x"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty predicates",
			sig: sig.ConcernSignature{
				ID:         "test-no-preds",
				Archetype:  sig.ArchetypeExistence,
				Scope:      sig.Scope{Globs: []string{"**/*.go"}},
				Predicates: []sig.Predicate{},
			},
			wantErr: true,
		},
		{
			name: "both scope sets",
			sig: sig.ConcernSignature{
				ID:        "test-both-scopes",
				Archetype: sig.ArchetypeExistence,
				Scope:     sig.Scope{Globs: []string{"**/*.go"}, SourceIDs: []string{"src-1"}},
				Predicates: []sig.Predicate{
					{Kind: "regexp", Arg: "x"},
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.sig.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("ConcernSignature.Validate() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PreconditionSatisfied — existence archetype
// ---------------------------------------------------------------------------

func TestPreconditionSatisfied_Existence(t *testing.T) {
	t.Parallel()
	s := sig.ConcernSignature{
		ID:        "test-existence",
		Archetype: sig.ArchetypeExistence,
		Scope:     sig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []sig.Predicate{
			{Kind: "regexp", Arg: "TODO"},
		},
	}
	// Empty coverage manifest — not needed for existence.
	emptyCov := sig.CoverageManifest{Tool: "test"}

	if sig.PreconditionSatisfied(s, nil, emptyCov) {
		t.Error("existence with no findings should be false")
	}
	if sig.PreconditionSatisfied(s, []sig.Finding{{Path: "a.go", Decisive: false}}, emptyCov) {
		t.Error("existence with non-decisive finding should be false")
	}
	if !sig.PreconditionSatisfied(s, []sig.Finding{{Path: "a.go", Decisive: true}}, emptyCov) {
		t.Error("existence with decisive finding should be true")
	}
}

// ---------------------------------------------------------------------------
// PreconditionSatisfied — absence archetype (the ABSENT_PROVEN guarantee)
// ---------------------------------------------------------------------------

func TestPreconditionSatisfied_Absence(t *testing.T) {
	t.Parallel()
	s := sig.ConcernSignature{
		ID:        "test-absence",
		Archetype: sig.ArchetypeAbsence,
		Scope:     sig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []sig.Predicate{
			{Kind: "regexp", Arg: "password="},
		},
		Canary: &sig.Canary{Path: "testdata/canary.go", Pattern: "password=CANARY"},
	}

	// Complete manifest = no unexplained skips.
	completeCov := sig.CoverageManifest{
		Files:    []string{"a.go", "b.go"},
		Tool:     "direct-investigator/pure",
		TreeHash: "abc123",
	}

	// Incomplete manifest = has unexplained skips (binary files).
	incompleteCov := sig.CoverageManifest{
		Files: []string{"a.go"},
		Tool:  "direct-investigator/pure",
		Skipped: sig.SkippedFiles{
			Binary: []string{"data.bin"},
		},
	}

	if sig.PreconditionSatisfied(s, []sig.Finding{{Path: "a.go", Decisive: true}}, completeCov) {
		t.Error("absence with findings should be false")
	}
	if sig.PreconditionSatisfied(s, nil, incompleteCov) {
		t.Error("absence with incomplete manifest (binary skips) should be false — fail-closed")
	}
	if !sig.PreconditionSatisfied(s, nil, completeCov) {
		t.Error("absence with zero findings and complete manifest should be true (ABSENT_PROVEN)")
	}
	// Ignored files do NOT downgrade absence (they are in scope-defined exclusions).
	covWithIgnored := sig.CoverageManifest{
		Files: []string{"a.go"},
		Tool:  "direct-investigator/pure",
		Skipped: sig.SkippedFiles{
			Ignored: []string{"vendor/lib.go"},
		},
	}
	if !sig.PreconditionSatisfied(s, nil, covWithIgnored) {
		t.Error("absence with only ignored skips (explained) should be true")
	}
}

// ---------------------------------------------------------------------------
// PreconditionSatisfied — threshold archetype
// ---------------------------------------------------------------------------

func TestPreconditionSatisfied_Threshold(t *testing.T) {
	t.Parallel()
	makeThresholdSig := func(op sig.ThresholdOp, val float64) sig.ConcernSignature {
		return sig.ConcernSignature{
			ID:        "test-threshold",
			Archetype: sig.ArchetypeThreshold,
			Scope:     sig.Scope{Globs: []string{"**/*.go"}},
			Predicates: []sig.Predicate{
				{Kind: "contains", Arg: "audit"},
			},
			Threshold: &sig.Threshold{Op: op, Value: val},
		}
	}
	cov := sig.CoverageManifest{Tool: "test"}

	findings := []sig.Finding{
		{Path: "a.go", Decisive: true},
		{Path: "b.go", Decisive: true},
		{Path: "c.go", Decisive: false}, // non-decisive should not count
	}

	cases := []struct {
		name string
		sig  sig.ConcernSignature
		want bool
	}{
		{"gte satisfied", makeThresholdSig(sig.ThresholdGTE, 2), true},
		{"gte not satisfied", makeThresholdSig(sig.ThresholdGTE, 3), false},
		{"lte satisfied", makeThresholdSig(sig.ThresholdLTE, 5), true},
		{"lte not satisfied", makeThresholdSig(sig.ThresholdLTE, 1), false},
		{"eq satisfied", makeThresholdSig(sig.ThresholdEQ, 2), true},
		{"eq not satisfied", makeThresholdSig(sig.ThresholdEQ, 1), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sig.PreconditionSatisfied(tc.sig, findings, cov)
			if got != tc.want {
				t.Errorf("PreconditionSatisfied() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test class 1: Canary — planted positive is found by existence check
// ---------------------------------------------------------------------------

func TestCanaryTestClass(t *testing.T) {
	t.Parallel()
	// The canary is "planted" in our virtual fs as a file containing the pattern.
	// A Tier-2 engine would find it; here we verify PreconditionSatisfied logic:
	// an existence sig with the canary pattern found as a decisive finding = true.
	canary := sig.Canary{
		Path:    "testdata/canary_planted.go",
		Pattern: `password = "CANARY_VALUE_DO_NOT_SHIP"`,
	}

	absenceSig := sig.ConcernSignature{
		ID:        "canary-test",
		Archetype: sig.ArchetypeAbsence,
		Scope:     sig.Scope{Globs: []string{"**/*.go"}},
		Predicates: []sig.Predicate{
			{Kind: "regexp", Arg: `password\s*=\s*"[^"]+"`},
		},
		Canary: &canary,
	}
	if err := absenceSig.Validate(); err != nil {
		t.Fatalf("canary sig is invalid: %v", err)
	}

	// Simulate Tier-2 finding the canary (decisive hit = the engine is working).
	canaryFinding := sig.Finding{Path: canary.Path, Decisive: true}
	cov := sig.CoverageManifest{Files: []string{canary.Path}, Tool: "test"}

	// Canary found → absence condition NOT satisfied (correct: if the pattern
	// exists in the scope, it's not absent).
	if sig.PreconditionSatisfied(absenceSig, []sig.Finding{canaryFinding}, cov) {
		t.Error("absence sig: canary found means NOT absent — should be false")
	}

	// If Tier-2 searches the scope EXCLUDING the canary file (as a Tier-2 engine
	// would, since canary is a fixture not part of production code), and finds
	// nothing, and the manifest is complete: ABSENT_PROVEN.
	prodCov := sig.CoverageManifest{Files: []string{"main.go"}, Tool: "test"}
	if !sig.PreconditionSatisfied(absenceSig, nil, prodCov) {
		t.Error("absence with no production findings and complete manifest should be true")
	}
}

// ---------------------------------------------------------------------------
// Test class 2: Liveness — globs match live tree
// ---------------------------------------------------------------------------

func TestLivenessCheckTestClass(t *testing.T) {
	t.Parallel()

	// Build a small in-memory filesystem.
	memFS := fstest.MapFS{
		"foundations/evidence/evidence-signature/types.go": &fstest.MapFile{Data: []byte("package evidencesignature\n")},
		"foundations/evidence/evidence-grade/grade.go":     &fstest.MapFile{Data: []byte("package evidencegrade\n")},
		"docs/README.md": &fstest.MapFile{Data: []byte("# readme\n")},
	}

	sigs := []sig.ConcernSignature{
		{
			ID:        "live-1",
			Archetype: sig.ArchetypeExistence,
			Scope:     sig.Scope{Globs: []string{"foundations/evidence/evidence-signature/types.go"}},
			Predicates: []sig.Predicate{
				{Kind: "contains", Arg: "evidencesignature"},
			},
		},
		{
			ID:        "live-2",
			Archetype: sig.ArchetypeExistence,
			Scope:     sig.Scope{Globs: []string{"nonexistent/**/*.go"}},
			Predicates: []sig.Predicate{
				{Kind: "regexp", Arg: "something"},
			},
		},
		{
			ID:        "live-3",
			Archetype: sig.ArchetypeExistence,
			// This glob is annotated expected_empty via Constraints.
			Scope:       sig.Scope{Globs: []string{"legacy/**/*.go"}},
			Constraints: []string{"expected_empty:legacy/**/*.go"},
			Predicates: []sig.Predicate{
				{Kind: "regexp", Arg: "old"},
			},
		},
	}

	dead := sig.LivenessCheck(memFS, sigs)

	// "nonexistent/**/*.go" should be dead.
	// "legacy/**/*.go" should NOT be dead (annotated expected_empty).
	// "foundations/.../types.go" should be live.
	if len(dead) != 1 || dead[0] != "nonexistent/**/*.go" {
		t.Errorf("LivenessCheck() = %v, want [nonexistent/**/*.go]", dead)
	}
}

// ---------------------------------------------------------------------------
// Test class 3: Manifest — skip accounting sums to the walk
// ---------------------------------------------------------------------------

func TestManifestTestClass(t *testing.T) {
	t.Parallel()
	// Verify that Complete() fails when there are unexplained skips and
	// passes when all skips are explained (ignored/symlink).
	withBinary := sig.CoverageManifest{
		Files: []string{"a.go", "b.go"},
		Tool:  "test",
		Skipped: sig.SkippedFiles{
			Binary: []string{"image.png"},
		},
	}
	if withBinary.Complete() {
		t.Error("manifest with binary skips should not be Complete (unexplained)")
	}

	withIgnoredOnly := sig.CoverageManifest{
		Files: []string{"a.go"},
		Tool:  "test",
		Skipped: sig.SkippedFiles{
			Ignored: []string{"vendor/lib.go"},
			Symlink: []string{"link-to-lib.go"},
		},
	}
	if !withIgnoredOnly.Complete() {
		t.Error("manifest with only ignored+symlink skips should be Complete (explained)")
	}

	withOversize := sig.CoverageManifest{
		Files: []string{"a.go"},
		Tool:  "test",
		Skipped: sig.SkippedFiles{
			Oversize: []string{"huge.csv"},
		},
	}
	if withOversize.Complete() {
		t.Error("manifest with oversize skips should not be Complete")
	}

	// AllSkipped collects across all categories.
	mixed := sig.CoverageManifest{
		Files: []string{"a.go"},
		Tool:  "test",
		Skipped: sig.SkippedFiles{
			Ignored:  []string{"vendor/a.go"},
			Binary:   []string{"b.bin"},
			Oversize: []string{"c.csv"},
			Symlink:  []string{"d.link"},
			LFSStub:  []string{"e.lfs"},
			NonUTF8:  []string{"f.bin"},
		},
	}
	all := mixed.Skipped.AllSkipped()
	if len(all) != 6 {
		t.Errorf("AllSkipped() = %d entries, want 6", len(all))
	}
	unexplained := mixed.Skipped.UnexplainedSkips()
	if len(unexplained) != 4 { // binary, oversize, lfs_stub, non_utf8
		t.Errorf("UnexplainedSkips() = %d entries, want 4", len(unexplained))
	}
}

// ---------------------------------------------------------------------------
// CoverageManifest validate
// ---------------------------------------------------------------------------

func TestCoverageManifestValidate(t *testing.T) {
	t.Parallel()
	if err := (sig.CoverageManifest{Tool: ""}).Validate(); err == nil {
		t.Error("expected error for empty Tool")
	}
	if err := (sig.CoverageManifest{Tool: "direct-investigator/pure"}).Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PreconditionSatisfied with invalid sig returns false (not a panic)
// ---------------------------------------------------------------------------

func TestPreconditionSatisfied_InvalidSig(t *testing.T) {
	t.Parallel()
	badSig := sig.ConcernSignature{} // invalid: empty ID, no archetype, no scope, no predicates
	if sig.PreconditionSatisfied(badSig, nil, sig.CoverageManifest{Tool: "test"}) {
		t.Error("invalid sig should return false")
	}
}

// ---------------------------------------------------------------------------
// SkippedFiles.AllSkipped sorting stability (determinism)
// ---------------------------------------------------------------------------

func TestSkippedFilesOrder(t *testing.T) {
	t.Parallel()
	s := sig.SkippedFiles{
		Ignored:  []string{"a"},
		Binary:   []string{"b"},
		Oversize: []string{"c"},
		Symlink:  []string{"d"},
		LFSStub:  []string{"e"},
		NonUTF8:  []string{"f"},
	}
	all := s.AllSkipped()
	want := []string{"a", "b", "c", "d", "e", "f"}
	for i, v := range want {
		if all[i] != v {
			t.Errorf("AllSkipped()[%d] = %q, want %q", i, all[i], v)
		}
	}
}

// Ensure fs.FS interface used in LivenessCheck works with fstest.MapFS.
var _ fs.FS = fstest.MapFS{}
