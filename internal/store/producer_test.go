package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// RT-153 (design §2.1): store row decoding asks the producer governor every
// 1,000 rows — a tree- or fleet-class scan halts at refuse, any scan at the
// exit mark, within 1,000 rows of the rung.
func TestRowDecodingHaltsEvery1000Rows(t *testing.T) {
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	edges := make([]core.Edge, 2500)
	for i := range edges {
		edges[i] = core.Edge{FromService: "svc", FromFile: "a.go", FromSymbol: fmt.Sprintf("F%04d", i), ToSymbol: "G", Kind: core.EdgeKind("calls")}
	}
	if err := s.PutTypedEdges(ctx, "svc", "p", edges); err != nil {
		t.Fatal(err)
	}
	tree := tenancy.WithProducerClass(ctx, tenancy.ClassTree)
	if got, err := s.TypedEdges(tree, "svc"); err != nil || len(got) != 2500 {
		t.Fatalf("normal: %d edges, %v", len(got), err)
	}
	tenancy.PublishLevel(tenancy.LevelRefuse)
	if _, err := s.TypedEdges(tree, "svc"); !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("a tree-class scan at refuse must halt: %v", err)
	}
	// An index-class scan (the default for a store read) still serves.
	if got, err := s.TypedEdges(ctx, "svc"); err != nil || len(got) != 2500 {
		t.Fatalf("an index read at refuse: %d, %v", len(got), err)
	}
	tenancy.PublishLevel(tenancy.LevelExit)
	if _, err := s.TypedEdges(ctx, "svc"); !errors.Is(err, tenancy.ErrOverBudget) {
		t.Fatalf("every scan halts at the exit mark: %v", err)
	}
	if _, err := s.QueryObservations(ctx, "svc", ""); err != nil {
		t.Fatalf("an empty scan never reaches a check: %v", err)
	}
}
