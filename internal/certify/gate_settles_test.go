package certify

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type gateCountingStore struct {
	store.Store
	symbolLists, observationLists, typedReads, pointReads int
}

func (s *gateCountingStore) ListSymbolNodes(ctx context.Context, service string) ([]core.SymbolNode, error) {
	s.symbolLists++
	return s.Store.ListSymbolNodes(ctx, service)
}

func (s *gateCountingStore) QueryObservations(ctx context.Context, service, concern string) ([]core.Observation, error) {
	s.observationLists++
	return s.Store.QueryObservations(ctx, service, concern)
}

func (s *gateCountingStore) TypedEdges(ctx context.Context, service string) ([]core.Edge, error) {
	s.typedReads++
	return s.Store.TypedEdges(ctx, service)
}

func (s *gateCountingStore) ObservationsForSymbol(ctx context.Context, service, name string) ([]core.Observation, error) {
	s.pointReads++
	return s.Store.ObservationsForSymbol(ctx, service, name)
}

func TestCertifyGateSettlesWithUnresolvedDeclaration(t *testing.T) {
	for _, allMissing := range []bool{false, true} {
		t.Run(map[bool]string{false: "mixed", true: "all-missing"}[allMissing], func(t *testing.T) {
			ctx, base := seedCompileFixture(t)
			decls := []store.CertifyDeclaration{{TestSymbolKey: symKey("TestMissing"), Kind: "invariant", TargetID: "I1", Strength: "asserts"}}
			if !allMissing {
				decls = append(decls, store.CertifyDeclaration{TestSymbolKey: symKey("TestLive"), Kind: "invariant", TargetID: "I1", Strength: "asserts"})
			}
			if err := base.PutCertifyDeclarations(ctx, compileSvc, decls); err != nil {
				t.Fatal(err)
			}
			st := &gateCountingStore{Store: base}
			now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
			if _, err := AfterIngest(ctx, st, compileSvc, fixtureCommit, "", now, false); err != nil {
				t.Fatal(err)
			}
			st.symbolLists, st.observationLists, st.typedReads, st.pointReads = 0, 0, 0, 0
			out, err := AfterIngest(ctx, st, compileSvc, fixtureCommit, "", now, false)
			if err != nil {
				t.Fatal(err)
			}
			if out.Step != StepSweepSkipped {
				t.Errorf("second ingest step=%v, want sweep-skipped", out.Step)
			}
			if st.symbolLists != 0 || st.observationLists != 0 || st.typedReads != 0 || st.pointReads == 0 {
				t.Errorf("second ingest lists=%d observations=%d typed=%d points=%d; want 0,0,0,positive", st.symbolLists, st.observationLists, st.typedReads, st.pointReads)
			}
		})
	}
}
