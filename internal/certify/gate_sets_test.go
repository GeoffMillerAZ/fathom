package certify

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Count the indexed lookup alongside the older point lookup without changing
// the existing gate settlement test's work bound.
func (s *gateCountingStore) SymbolSourcesForNames(ctx context.Context, service string, names []string) ([]store.SymbolSource, error) {
	s.pointReads++
	return s.Store.SymbolSourcesForNames(ctx, service, names)
}

type gateSetStore struct {
	store.Store
	names       []string
	legacyReads int
}

func (s *gateSetStore) SymbolSourcesForNames(ctx context.Context, svc string, names []string) ([]store.SymbolSource, error) {
	s.names = append(s.names, names...)
	return s.Store.SymbolSourcesForNames(ctx, svc, names)
}

func (s *gateSetStore) ObservationsForSymbol(ctx context.Context, svc, name string) ([]core.Observation, error) {
	s.legacyReads++
	return s.Store.ObservationsForSymbol(ctx, svc, name)
}

func gateDeclaration(name string) store.CertifyDeclaration {
	return store.CertifyDeclaration{TestSymbolKey: symKey(name), Kind: "invariant", TargetID: "I1", Strength: "asserts"}
}

func TestCertifyGateComparesDeclarationSets(t *testing.T) {
	ctx, base := seedCompileFixture(t)
	decls := []store.CertifyDeclaration{gateDeclaration("TestLive"), gateDeclaration("TestLater")}
	if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	if _, err := AfterIngest(ctx, base, compileSvc, fixtureCommit, "", now, false); err != nil {
		t.Fatal(err)
	}
	// Move one marker to an unindexed test while another declared test becomes
	// indexed. The claim and resolvable-declaration counts both remain one.
	decls[0] = gateDeclaration("TestMissing")
	if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
		t.Fatal(err)
	}
	if err := base.PutObservations(ctx, []core.Observation{{ServiceID: compileSvc, Package: "pkg", Symbol: "TestLater", SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc}}); err != nil {
		t.Fatal(err)
	}
	out, err := AfterIngest(ctx, base, compileSvc, fixtureCommit, "", now, false)
	if err != nil || out.Step != StepCompile {
		t.Fatalf("changed declaration set: step=%v err=%v, want compile", out.Step, err)
	}
	claims, err := base.ClaimsForService(ctx, compileSvc, Layer)
	wantID := core.ClaimID(Layer, Kind, "invariant:I1", symKey("TestLater"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range claims {
		if c.ClaimID == wantID {
			return
		}
	}
	t.Fatalf("new declaration never compiled: %+v", claims)
}

func TestCertifyGateDuplicateMarkersSettle(t *testing.T) {
	ctx, base := seedCompileFixture(t)
	d := gateDeclaration("TestLive")
	if err := base.PutCertifyDeclarations(ctx, compileSvc, []store.CertifyDeclaration{d, d}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	if _, err := AfterIngest(ctx, base, compileSvc, fixtureCommit, "", now, false); err != nil {
		t.Fatal(err)
	}
	st := &gateSetStore{Store: base}
	out, err := AfterIngest(ctx, st, compileSvc, fixtureCommit, "", now, false)
	if err != nil || out.Step != StepSweepSkipped {
		t.Fatalf("duplicate markers: step=%v err=%v, want sweep-skipped", out.Step, err)
	}
	if len(st.names) != 0 || st.legacyReads != 0 {
		t.Fatalf("already claimed declarations looked up: names=%v legacy=%d", st.names, st.legacyReads)
	}
}

func TestCertifyGateNewlyIndexedTestCompiles(t *testing.T) {
	ctx, base := seedCompileFixture(t)
	if err := base.PutCertifyDeclarations(ctx, compileSvc, []store.CertifyDeclaration{gateDeclaration("TestLater")}); err != nil {
		t.Fatal(err)
	}
	if got, err := nextStep(ctx, base, compileSvc, fixtureCommit); err != nil || got != StepSweep {
		t.Fatalf("unindexed: step=%v err=%v", got, err)
	}
	if err := base.PutObservations(ctx, []core.Observation{{ServiceID: compileSvc, Package: "pkg", Symbol: "TestLater", SourceRef: "pkg/x_test.go", SourceKind: core.KindGoFunc}}); err != nil {
		t.Fatal(err)
	}
	if got, err := nextStep(ctx, base, compileSvc, fixtureCommit); err != nil || got != StepCompile {
		t.Fatalf("newly indexed: step=%v err=%v, want compile", got, err)
	}
}

func TestCertifyGateLooksUpOnlyUnclaimedNames(t *testing.T) {
	ctx, base := seedCompileFixture(t)
	decls := []store.CertifyDeclaration{gateDeclaration("TestLive"), gateDeclaration("TestMissing"), gateDeclaration("TestMissing")}
	if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileCertifications(ctx, base, compileSvc, fixtureCommit, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), "", noopGitRunner); err != nil {
		t.Fatal(err)
	}
	st := &gateSetStore{Store: base}
	if got, err := nextStep(ctx, st, compileSvc, fixtureCommit); err != nil || got != StepSweep {
		t.Fatalf("step=%v err=%v", got, err)
	}
	if !reflect.DeepEqual(st.names, []string{"TestMissing"}) || st.legacyReads != 0 {
		t.Fatalf("lookups names=%v legacy=%d, want only unclaimed exact names", st.names, st.legacyReads)
	}
	// An identically named test in another package must not resolve this key.
	if err := base.PutObservations(ctx, []core.Observation{{ServiceID: compileSvc, Package: "other", Symbol: "TestMissing", SourceRef: "elsewhere/x_test.go"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := nextStep(ctx, st, compileSvc, fixtureCommit); err != nil || got != StepSweep {
		t.Fatalf("foreign package: step=%v err=%v", got, err)
	}
}
