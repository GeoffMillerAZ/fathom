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
