package conserve

// plane_integrity_test.go — the "integrity" plane's own wiring tests. The
// underlying analysis (guardedness, per-kind classification, generated-
// file hand edits) is already covered exhaustively at
// internal/analysis/integrity's own test suite; these tests exercise
// PLANE-LEVEL responsibilities only: registration, Go-only scoping with a
// non-Go language reported as missing (never silently skipped), the
// conserve.Finding mapping, and the Store-backed reach oracle actually
// reaching through to a real guarded-test decision.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func fr(files map[string][]byte) func(string) ([]byte, bool, error) {
	return func(path string) ([]byte, bool, error) {
		b, ok := files[path]
		return b, ok, nil
	}
}

// TestIntegrityPlane_Registered: the plane registers itself from its own
// file's init() (ADR-044 §0's "each plane is a self-contained addition").
func TestIntegrityPlane_Registered(t *testing.T) {
	found := false
	for _, p := range Planes() {
		if p.Name() == integrityPlaneName {
			found = true
		}
	}
	if !found {
		t.Fatalf("%q not found among Planes(): %v", integrityPlaneName, Planes())
	}
}

// TestIntegrityPlane_DeletedGuardedTestIsAFinding: a guarded test deleted
// at head produces one "deleted" conserve.Finding, never excused
// (Declared is always "none" for this plane — see toConserveFinding's doc
// comment).
func TestIntegrityPlane_DeletedGuardedTestIsAFinding(t *testing.T) {
	const path = "pkg/x_test.go"
	base := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n\n// certifies: invariant I1 (asserts)\nfunc TestGuarded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	head := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n"),
	}
	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service:      "svc",
		ChangedFiles: []string{path},
		BaseFile:     fr(base),
		HeadFile:     fr(head),
	})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Plane != integrityPlaneName || f.Kind != "deleted" || f.Key != "TestGuarded" || f.Declared != DeclaredNone {
		t.Errorf("got %+v, want plane=integrity kind=deleted key=TestGuarded declared=none", f)
	}
	if f.Base == "" {
		t.Errorf("Base citation is empty, want a file:line")
	}
	// RK-36: File/Line is the guarded test's own location — KindDeleted
	// has no head site, so it must fall back to base.
	if got := fmt.Sprintf("%s:%d", f.File, f.Line); got != f.Base {
		t.Errorf("File/Line = %s, want it to match Base citation %s (deleted test has no head location)", got, f.Base)
	}
}

// TestIntegrityPlane_NonGoTestFileIsMissing: design §1.3's "other
// languages report unproven" — a changed Python TEST file is named in
// Missing, never silently skipped.
func TestIntegrityPlane_NonGoTestFileIsMissing(t *testing.T) {
	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/test_widget.py"},
		BaseFile:     fr(map[string][]byte{"pkg/test_widget.py": []byte("def test_x(): pass\n")}),
		HeadFile:     fr(map[string][]byte{"pkg/test_widget.py": []byte("def test_x(): pass\n")}),
	})
	if len(findings) != 0 {
		t.Errorf("got findings %+v, want none", findings)
	}
	if len(missing) != 1 {
		t.Fatalf("got missing=%v, want exactly one entry", missing)
	}
}

// TestIntegrityPlane_NonGoNonTestFileIsNotMissing: an ordinary non-Go,
// non-test-shaped changed file (a doc) carries no guarded-test claim —
// never named in Missing, or the plane would go permanently unproven on
// any repo that ever touches a README (see the package doc comment).
func TestIntegrityPlane_NonGoNonTestFileIsNotMissing(t *testing.T) {
	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service:      "svc",
		ChangedFiles: []string{"README.md"},
		BaseFile:     fr(map[string][]byte{"README.md": []byte("# hi\n")}),
		HeadFile:     fr(map[string][]byte{"README.md": []byte("# hi there\n")}),
	})
	if len(findings) != 0 || len(missing) != 0 {
		t.Errorf("got findings=%+v missing=%v, want both empty", findings, missing)
	}
}

// TestIntegrityPlane_ReachabilityFromStore: a Store-backed reach oracle
// guards a test that has NO marker/manifest/invariant/surface reason at
// all, purely because it statically reaches a symbol defined in a changed
// file — proving reachabilityFrom's callgraph wiring genuinely guards
// (and therefore genuinely catches a loss on) a test the other four
// reasons would have missed entirely.
func TestIntegrityPlane_ReachabilityFromStore(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	const svc = "svc"
	const changedFile = "pkg/widget.go"
	const testFile = "pkg/widget_test.go"

	obs := []core.Observation{
		{ServiceID: svc, SourceRef: changedFile, SourceKind: core.KindGoFunc, Symbol: "Widget", Package: "pkg", EvidenceTier: core.TierVerified, IngestedAt: time.Now()},
		{ServiceID: svc, SourceRef: testFile, SourceKind: core.KindGoFunc, Symbol: "TestReachesWidget", Package: "pkg", EvidenceTier: core.TierVerified, IngestedAt: time.Now()},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	testKey := core.SymbolKey(svc, core.QualifiedPackage(testFile, "pkg"), "TestReachesWidget")
	widgetKey := core.SymbolKey(svc, core.QualifiedPackage(changedFile, "pkg"), "Widget")
	edges := []core.Edge{{FromService: svc, FromFile: testFile, FromSymbol: testKey, ToSymbol: widgetKey, Kind: core.EdgeKind("typed")}}
	if err := st.PutTypedEdges(ctx, svc, "pkg", edges); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}

	base := map[string][]byte{
		testFile: []byte("package pkg\n\nimport \"testing\"\n\nfunc TestReachesWidget(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	head := map[string][]byte{
		testFile: []byte("package pkg\n\nimport \"testing\"\n"), // deleted at head
	}

	p := integrityPlane{}
	findings, missing := p.Check(ctx, Input{
		Service:      svc,
		ChangedFiles: []string{changedFile, testFile},
		BaseFile:     fr(base),
		HeadFile:     fr(head),
		Store:        st,
	})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (TestReachesWidget guarded purely by reach): %+v", len(findings), findings)
	}
	if findings[0].Key != "TestReachesWidget" || findings[0].Kind != "deleted" {
		t.Errorf("got %+v, want key=TestReachesWidget kind=deleted", findings[0])
	}
}

// TestIntegrityPlane_EditedFindingDoesNotForceLosses: design §1.3 says
// `edited` is "informational; never a loss on its own." A guarded test
// whose body changed (a tightened assertion message, say) but tripped no
// other kind must still surface the `edited` Finding — a caller inspecting
// findings directly should see it — but must NOT, by itself, tip the
// plane's Run-computed verdict to Losses, or an honest edit to a guarded
// test's body fails `fathom conserve --strict`/the pre-push hook with
// nothing actually lost.
func TestIntegrityPlane_EditedFindingDoesNotForceLosses(t *testing.T) {
	const path = "pkg/x_test.go"
	base := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n\n// certifies: invariant I1 (asserts)\nfunc TestGuarded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	head := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n\n// certifies: invariant I1 (asserts)\nfunc TestGuarded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom now\")\n\t}\n}\n"),
	}
	in := Input{Service: "svc", ChangedFiles: []string{path}, BaseFile: fr(base), HeadFile: fr(head)}

	rep := Run(context.Background(), in, []Plane{integrityPlane{}})
	if len(rep.Planes) != 1 {
		t.Fatalf("got %d plane reports, want 1", len(rep.Planes))
	}
	pr := rep.Planes[0]
	if len(pr.Findings) != 1 || pr.Findings[0].Kind != "edited" {
		t.Fatalf("got findings %+v, want exactly one edited finding", pr.Findings)
	}
	if pr.Verdict != Conserved {
		t.Errorf("verdict = %q, want conserved (an edited finding is informational, never a loss on its own)", pr.Verdict)
	}
}

// TestIntegrityPlane_CleanRenameDoesNotForceLosses: a test guarded ONLY by
// reach (no marker/manifest/invariant/surface citation of its name) is
// renamed, body untouched. Design §1.3: `renamed` is "not a loss unless a
// -run pattern, marker or enforcement cell still names the old name" —
// with no such citation at base (StillNamedElsewhere is false), this is
// the "clean rename" case and must not tip the plane to Losses.
func TestIntegrityPlane_CleanRenameDoesNotForceLosses(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	const svc = "svc"
	const changedFile = "pkg/widget.go"
	const testFile = "pkg/widget_test.go"

	obs := []core.Observation{
		{ServiceID: svc, SourceRef: changedFile, SourceKind: core.KindGoFunc, Symbol: "Widget", Package: "pkg", EvidenceTier: core.TierVerified, IngestedAt: time.Now()},
		{ServiceID: svc, SourceRef: testFile, SourceKind: core.KindGoFunc, Symbol: "TestReachesWidget", Package: "pkg", EvidenceTier: core.TierVerified, IngestedAt: time.Now()},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	testKey := core.SymbolKey(svc, core.QualifiedPackage(testFile, "pkg"), "TestReachesWidget")
	widgetKey := core.SymbolKey(svc, core.QualifiedPackage(changedFile, "pkg"), "Widget")
	edges := []core.Edge{{FromService: svc, FromFile: testFile, FromSymbol: testKey, ToSymbol: widgetKey, Kind: core.EdgeKind("typed")}}
	if err := st.PutTypedEdges(ctx, svc, "pkg", edges); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}

	base := map[string][]byte{
		testFile: []byte("package pkg\n\nimport \"testing\"\n\nfunc TestReachesWidget(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	head := map[string][]byte{
		// Same body, new name, nothing else in the file touched: a clean
		// rename with no explicit-naming reason at base to leave stale.
		testFile: []byte("package pkg\n\nimport \"testing\"\n\nfunc TestReachesWidgetRenamed(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}

	in := Input{
		Service:      svc,
		ChangedFiles: []string{changedFile, testFile},
		BaseFile:     fr(base),
		HeadFile:     fr(head),
		Store:        st,
	}
	rep := Run(ctx, in, []Plane{integrityPlane{}})
	pr := rep.Planes[0]
	if len(pr.Findings) != 1 || pr.Findings[0].Kind != "renamed" {
		t.Fatalf("got findings %+v, want exactly one renamed finding", pr.Findings)
	}
	if pr.Verdict != Conserved {
		t.Errorf("verdict = %q, want conserved (a clean rename with no still-named citation is not a loss)", pr.Verdict)
	}
}

// TestIntegrityPlane_RenameStillNamedIsALoss is the sanity check the two
// tests above need: the fix must not blanket-exempt every rename. A test
// guarded by a "// certifies:" marker at base — an explicit-naming reason
// — is renamed; StillNamedElsewhere is true (the marker's citation of the
// old behavior may still sit somewhere this package cannot itself verify
// was updated), so the plane must still report Losses.
func TestIntegrityPlane_RenameStillNamedIsALoss(t *testing.T) {
	const path = "pkg/x_test.go"
	base := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n\n// certifies: invariant I1 (asserts)\nfunc TestGuarded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	head := map[string][]byte{
		path: []byte("package pkg\n\nimport \"testing\"\n\nfunc TestGuardedRenamed(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
	}
	in := Input{Service: "svc", ChangedFiles: []string{path}, BaseFile: fr(base), HeadFile: fr(head)}

	rep := Run(context.Background(), in, []Plane{integrityPlane{}})
	pr := rep.Planes[0]
	if len(pr.Findings) != 1 || pr.Findings[0].Kind != "renamed" {
		t.Fatalf("got findings %+v, want exactly one renamed finding", pr.Findings)
	}
	if pr.Verdict != Losses {
		t.Errorf("verdict = %q, want losses (a marker-guarded rename is still-named, never excused)", pr.Verdict)
	}
}

// TestIntegrityPlane_InvariantGuardReadFromBase is the i28-laundering
// repro (design §1.2's "at base"): a test guarded ONLY by
// docs/intent/invariants.md's enforcement column — no marker, no manifest
// entry, no surface evidence — is deleted, and the SAME change ALSO drops
// its row from invariants.md's enforcement column (the exact
// guards-pilot-8 pattern: rename/delete plus laundering the record that
// would out it). Guardedness must be judged from invariants.md's content
// AT BASE (where the row still names the test), never from head's current
// tree — reading head would see the row already gone, guard the test on
// neither side, and report nothing: the loss becomes invisible outright,
// not merely excused.
func TestIntegrityPlane_InvariantGuardReadFromBase(t *testing.T) {
	const testPath = "pkg/x_test.go"
	const invPath = "docs/intent/invariants.md"

	base := map[string][]byte{
		testPath: []byte("package pkg\n\nimport \"testing\"\n\nfunc TestGuardedByInvariant(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n"),
		invPath: []byte("# Invariants\n\n" +
			"| # | Invariant | Enforced by | Source |\n" +
			"|---|---|---|---|\n" +
			"| I1 | Something always true | test `TestGuardedByInvariant` | ADR-1 |\n"),
	}
	head := map[string][]byte{
		testPath: []byte("package pkg\n\nimport \"testing\"\n"), // deleted
		invPath: []byte("# Invariants\n\n" +
			"| # | Invariant | Enforced by | Source |\n" +
			"|---|---|---|---|\n" +
			"| I1 | Something always true | code (no test) | ADR-1 |\n"), // row's test citation laundered away too
	}

	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service:      "svc",
		ChangedFiles: []string{testPath, invPath},
		BaseFile:     fr(base),
		HeadFile:     fr(head),
	})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (TestGuardedByInvariant deleted, guarded at base by invariants.md alone): %+v", len(findings), findings)
	}
	if findings[0].Key != "TestGuardedByInvariant" || findings[0].Kind != "deleted" {
		t.Errorf("got %+v, want key=TestGuardedByInvariant kind=deleted", findings[0])
	}
}

// TestIntegrityPlane_ManifestGuardReadFromBase is the same laundering
// repro for the OTHER current-tree read the same bug covered: a test
// guarded only by a tests/*.cue suite manifest's #Certifies entry is
// deleted, and the manifest's certifies entry is dropped in the same
// change. Manifest content must be read at BASE (via git show, same as
// invariants.md above) — a real git repo is needed here because
// manifestCertifiesAt lists tests/*.cue at a ref via `git ls-tree`.
func TestIntegrityPlane_ManifestGuardReadFromBase(t *testing.T) {
	repo := newInputTestRepo(t)
	repo.write("pkg/x_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestGuardedByManifest(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n")
	repo.write("tests/cart.cue", `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
	certifies: [
		{kind: "invariant", id: "I5", tests: ["TestGuardedByManifest"]},
	]
}
`)
	repo.commitAll("base commit")
	base := repo.headSHA()

	repo.write("pkg/x_test.go", "package pkg\n\nimport \"testing\"\n") // deleted
	repo.write("tests/cart.cue", `
component: "cart"
suites: unit: {
	class:  "unit"
	runner: "go-test"
	inputs: paths: ["."]
	needs: []
	runtime: {}
	flake: {}
	tier: "blocking"
}
`) // certifies entry dropped in the same change
	repo.commitAll("delete the test and its certifies entry together")

	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service:      "svc",
		Root:         repo.root,
		Base:         base,
		ChangedFiles: []string{"pkg/x_test.go", "tests/cart.cue"},
		BaseFile:     func(path string) ([]byte, bool, error) { return gitShowFile(repo.root, base, path) },
		HeadFile:     func(path string) ([]byte, bool, error) { return worktreeFile(repo.root, path) },
	})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (TestGuardedByManifest deleted, guarded at base by the manifest alone): %+v", len(findings), findings)
	}
	if findings[0].Key != "TestGuardedByManifest" || findings[0].Kind != "deleted" {
		t.Errorf("got %+v, want key=TestGuardedByManifest kind=deleted", findings[0])
	}
}

// TestIntegrityPlane_NoGuardedNoFindings: an ordinary, unguarded Go test
// change produces no findings and no missing entries — Check must never
// invent a claim for code the conservation checks have no opinion about.
func TestIntegrityPlane_NoGuardedNoFindings(t *testing.T) {
	const path = "pkg/plain_test.go"
	base := map[string][]byte{path: []byte("package pkg\n\nfunc TestPlain(t *testing.T) {}\n")}
	head := map[string][]byte{path: []byte("package pkg\n\nfunc TestPlain(t *testing.T) {\n\tpanic(\"rewritten\")\n}\n")}
	p := integrityPlane{}
	findings, missing := p.Check(context.Background(), Input{
		Service: "svc", ChangedFiles: []string{path}, BaseFile: fr(base), HeadFile: fr(head),
	})
	if len(findings) != 0 || len(missing) != 0 {
		t.Errorf("got findings=%+v missing=%v, want both empty", findings, missing)
	}
}
