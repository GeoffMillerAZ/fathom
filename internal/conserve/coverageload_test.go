package conserve

// coverageload_test.go — LoadConfiguredCoverage (design §3.5, CV-70,
// I44): the shared coverage-loading defects. TestLoadConfiguredCoverage_
// ExplicitMissingMetaIsReported pins that a user-named --coverage-meta
// path that does not exist is a reportable failure, never silently
// treated the same as "nothing configured". TestLoadConfiguredCoverage_
// ProjectConfigIsResolvedFromRootNotCWD pins that the project layer is
// read from ROOT's own .fathom/config.toml, not the process's current
// working directory. TestLoadConfiguredCoverage_EveryBlockSkippedIs
// Reported pins that a profile that loads but maps NO block to any
// repository file (a stale results_dir from another module) is named,
// never silently accepted as an empty-but-usable profile.
// TestLoadConfiguredCoverage_UnusableMetaSkipsTheProfileParse pins the
// perf fix: a meta that already fails the bound/dirty/passed bar never
// pays for LoadCoverage's own profile parse and module walk, and
// Residue's own reason-naming still works from the meta alone.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestLoadConfiguredCoverage_ExplicitMissingMetaIsReported pins that an
// explicit --coverage-meta naming a path that does not exist is NOT the
// same as "nothing was ever configured" (the default results_dir guess's
// own silent exemption): the user asked for it by name, so I44's "the
// report names why" applies.
func TestLoadConfiguredCoverage_ExplicitMissingMetaIsReported(t *testing.T) {
	root := t.TempDir()
	in := &Input{}
	LoadConfiguredCoverage(in, root, filepath.Join(root, "definitely", "not", "here.json"))
	if in.CoverageLoadError == "" {
		t.Fatal("CoverageLoadError = \"\", want a reason naming the missing explicit --coverage-meta path")
	}
	if in.Coverage != nil {
		t.Errorf("Coverage = %+v, want nil", in.Coverage)
	}
}

// TestLoadConfiguredCoverage_DefaultMissingMetaIsSilent pins the OTHER
// half: the default results_dir guess simply finding nothing there is
// NOT a reportable failure — most runs have no profile at all.
func TestLoadConfiguredCoverage_DefaultMissingMetaIsSilent(t *testing.T) {
	root := t.TempDir()
	in := &Input{}
	LoadConfiguredCoverage(in, root, "") // no --coverage-meta, no .fathom/config.toml at all
	if in.CoverageLoadError != "" {
		t.Errorf("CoverageLoadError = %q, want empty (nothing was ever configured)", in.CoverageLoadError)
	}
	if in.Coverage != nil {
		t.Errorf("Coverage = %+v, want nil", in.Coverage)
	}
}

// TestLoadConfiguredCoverage_ProjectConfigIsResolvedFromRootNotCWD pins
// that [tests] results_dir is read from ROOT's own .fathom/config.toml —
// never the process's current working directory, which may be a
// subdirectory of root (or somewhere else entirely) when a caller like
// `fathom conserve` is invoked from deeper in the tree.
func TestLoadConfiguredCoverage_ProjectConfigIsResolvedFromRootNotCWD(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"),
		[]byte("[tests]\nresults_dir = \"results\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "results"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "results", "coverage.meta.json"),
		[]byte(`{"schema":"fathom.coverage/v1","commit":"c1","dirty":false,"race":false,"profile":"coverage.out","passed":false}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// The current working directory when this runs is wherever `go
	// test` happens to invoke it from — deliberately NEVER root itself
	// and never touched by this test, proving the resolution used root,
	// not os.Getwd().
	in := &Input{}
	LoadConfiguredCoverage(in, root, "")
	if in.CoverageMeta.Commit != "c1" {
		t.Fatalf("CoverageMeta = %+v, want the results_dir meta read via ROOT (commit c1) — got the cwd-based default instead", in.CoverageMeta)
	}
}

// TestLoadConfiguredCoverage_EveryBlockSkippedIsReported pins that a
// profile that loads without error but whose EVERY block's file key
// matches no known module (a stale results_dir from a different module,
// or a renamed module) is named as unusable, rather than silently
// accepted as an empty Coverage that "usable" checks then pass with
// nothing to say.
func TestLoadConfiguredCoverage_EveryBlockSkippedIsReported(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/outer\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "coverage.out"),
		[]byte("mode: set\nexample.com/renamed/pkg/a.go:3.10,5.2 2 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "coverage.meta.json"),
		[]byte(`{"schema":"fathom.coverage/v1","commit":"c1","dirty":false,"race":false,"profile":"coverage.out","passed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	in := &Input{}
	LoadConfiguredCoverage(in, root, filepath.Join(root, "coverage.meta.json"))
	if in.CoverageLoadError == "" {
		t.Fatal("CoverageLoadError = \"\", want a reason — every block's file key matched no module under root")
	}
}

// TestLoadConfiguredCoverage_UnusableMetaSkipsTheProfileParse pins the
// O31 perf fix: a meta that already fails CV-70's bound/dirty/passed bar
// never pays for LoadCoverage's own full profile parse and repo-wide
// module walk — proven here by pointing `profile` at a file that does
// not exist at all: if LoadCoverage were still called, it would fail
// with an "opening ...: no such file" CoverageLoadError instead of the
// meta's own "did not pass" reason. Residue's own coverageUsableGlobally
// must still name the correct reason from CoverageMeta alone.
func TestLoadConfiguredCoverage_UnusableMetaSkipsTheProfileParse(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "coverage.meta.json"),
		[]byte(`{"schema":"fathom.coverage/v1","commit":"c1","dirty":false,"race":false,"profile":"this-file-does-not-exist.out","passed":false}`), 0o644); err != nil {
		t.Fatal(err)
	}

	in := &Input{Root: root}
	LoadConfiguredCoverage(in, root, filepath.Join(root, "coverage.meta.json"))
	if in.CoverageLoadError != "" {
		t.Fatalf("CoverageLoadError = %q, want empty — the meta's own reason must come from Residue's coverageUsableGlobally, not a profile-parse failure this optimization must never trigger", in.CoverageLoadError)
	}
	if in.Coverage == nil {
		t.Fatal("Coverage = nil, want a non-nil (empty) profile so coverageUsableGlobally still names the meta's own reason")
	}

	ok, reason := coverageUsableGlobally(*in)
	if ok {
		t.Fatalf("coverageUsableGlobally = true, want false (passed=false)")
	}
	if reason != "coverage meta's run did not pass" {
		t.Errorf("reason = %q, want %q", reason, "coverage meta's run did not pass")
	}

	// End to end: Residue itself must report the SAME reason, never an
	// empty CoverageUnusable, for a changed function under this Input.
	st := store.NewMemoryStore()
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "svc", Package: "pkg", Symbol: "F", SourceKind: core.KindGoFunc, SourceRef: "pkg/f.go", Line: 1},
	}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	in.Service = "svc"
	in.ChangedFiles = []string{"pkg/f.go"}
	in.HeadFile = mapFile(map[string]string{"pkg/f.go": "package pkg\n\nfunc F() {}\n"})
	in.BaseFile = mapFile(nil)
	in.Store = st
	in.IndexAtHead = true

	rep := Residue(ctx, *in, 0)
	if rep.CoverageUnusable != "coverage meta's run did not pass" {
		t.Errorf("Residue's CoverageUnusable = %q, want %q", rep.CoverageUnusable, "coverage meta's run did not pass")
	}
}
