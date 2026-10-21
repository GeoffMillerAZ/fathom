package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func projectionStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "projection.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func projectedObservations(t *testing.T, s *SQLiteStore, ctx context.Context) []core.SymbolNode {
	t.Helper()
	obs, err := s.QueryObservations(ctx, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return nodes
}

func TestSymbolProjectionMatchesObservationPath(t *testing.T) {
	s := projectionStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	rows := []core.Observation{
		{ServiceID: "svc", ConcernID: "first", SourceRef: "a/file.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Duplicate", Package: "p", Claim: "func Duplicate()", Calls: []string{"p.A", "p.A"}, IngestedAt: base.Add(3 * time.Second), Complexity: 7, Line: 2, EndLine: 9, ToolKind: "go", Generated: true},
		{ServiceID: "svc", ConcernID: "second", SourceRef: "z/data.json", SourceKind: core.SourceKind("JSONKey"), EvidenceTier: core.TierPreScan, Symbol: "long.key.with.no.calls", Package: "data", Claim: "value", IngestedAt: base},
		{ServiceID: "svc", ConcernID: "third", SourceRef: "b/file.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Duplicate", Package: "p", Claim: "second identity", Calls: []string{}, IngestedAt: base},
		{ServiceID: "other", SourceRef: "other.go", Symbol: "NeverReturned", IngestedAt: base},
	}
	if err := s.PutObservations(ctx, rows); err != nil {
		t.Fatal(err)
	}
	want := projectedObservations(t, s, ctx)
	got, err := s.ListSymbolNodes(ctx, "svc")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("projection differs: got %#v, %v; want %#v", got, err, want)
	}
	if len(got) != 3 || got[0].FilePath != "z/data.json" || got[1].FilePath != "b/file.go" {
		t.Fatalf("stable timestamp/insertion order or complete row population changed: %#v", got)
	}
	empty, err := s.ListSymbolNodes(ctx, "absent")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty public projection = %#v, %v", empty, err)
	}
}

func TestSymbolProjectionSuppressionAndReversal(t *testing.T) {
	s := projectionStore(t)
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.gate = scopeGate{src: scope.NewSource(path, func() time.Time { return now })}
	rows := []core.Observation{
		{ServiceID: "svc", SourceRef: "secret/a.go", Symbol: "Hidden", IngestedAt: now},
		{ServiceID: "svc", SourceRef: "public/b.go", Symbol: "Visible", IngestedAt: now},
	}
	if err := s.PutObservations(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
	id, err := scope.AppendSuppression(path, scope.Suppression{Service: "svc", Pattern: "secret/**", Reason: "test boundary", CreatedAt: now, CreatedBy: scope.OriginCLI})
	if err != nil {
		t.Fatal(err)
	}
	oldCtx, oldCollector := scope.WithCollector(context.Background())
	newCtx, newCollector := scope.WithCollector(context.Background())
	want := projectedObservations(t, s, oldCtx)
	got, err := s.ListSymbolNodes(newCtx, "svc")
	if err != nil || !reflect.DeepEqual(got, want) || len(got) != 1 || !reflect.DeepEqual(oldCollector.For("svc"), newCollector.For("svc")) {
		t.Fatalf("scoped result/disclosure differs: %#v, %v; old=%#v new=%#v", got, err, oldCollector.For("svc"), newCollector.For("svc"))
	}
	if err := scope.RemoveSuppression(path, id, now); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListSymbolNodes(context.Background(), "svc")
	if err != nil || len(got) != 2 {
		t.Fatalf("same handle did not observe reversal: %#v, %v", got, err)
	}
}

func TestSymbolProjectionErrorAndCancellationDifferential(t *testing.T) {
	s := projectionStore(t)
	ctx := context.Background()
	if err := s.PutObservations(ctx, []core.Observation{{ServiceID: "svc", SourceRef: "a.go", Symbol: "A"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("UPDATE observations SET calls = '{' WHERE service_id = ?", "svc"); err != nil {
		t.Fatal(err)
	}
	_, oldErr := s.QueryObservations(ctx, "svc", "")
	nodes, newErr := s.ListSymbolNodes(ctx, "svc")
	if oldErr == nil || newErr == nil || oldErr.Error() != newErr.Error() || nodes != nil {
		t.Fatalf("malformed calls became success/partial: old=%v new=%v nodes=%#v", oldErr, newErr, nodes)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, oldErr = s.QueryObservations(canceled, "svc", "")
	nodes, newErr = s.ListSymbolNodes(canceled, "svc")
	if !errors.Is(oldErr, context.Canceled) || !errors.Is(newErr, context.Canceled) || nodes != nil {
		t.Fatalf("cancel differs: old=%v new=%v nodes=%#v", oldErr, newErr, nodes)
	}
}

type projectionErrorRows struct {
	remaining int
	scanErr   error
	rowErr    error
}

func (r *projectionErrorRows) Next() bool { r.remaining--; return r.remaining >= 0 }
func (r *projectionErrorRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*dest[9].(*string) = "null"
	return nil
}
func (r *projectionErrorRows) Err() error { return r.rowErr }

func TestSymbolProjectionTerminalAndGovernorFailures(t *testing.T) {
	sentinel := errors.New("row failure")
	for _, rows := range []*projectionErrorRows{{remaining: 1, scanErr: sentinel}, {remaining: 2, rowErr: sentinel}} {
		got, err := scanSymbolNodes(context.Background(), rows)
		if !errors.Is(err, sentinel) || got != nil {
			t.Fatalf("error returned partial nodes: %#v, %v", got, err)
		}
	}
	t.Cleanup(func() { tenancy.PublishLevel(tenancy.LevelNormal) })
	tenancy.PublishLevel(tenancy.LevelRefuse)
	ctx := tenancy.WithProducerClass(context.Background(), tenancy.ClassTree)
	got, err := scanSymbolNodes(ctx, &projectionErrorRows{remaining: 2500})
	if !errors.Is(err, tenancy.ErrOverBudget) || got != nil {
		t.Fatalf("tree row governor bypassed: %d, %v", len(got), err)
	}
	// The original scan's default index class still serves at refuse.
	got, err = scanSymbolNodes(context.Background(), &projectionErrorRows{remaining: 2})
	if err != nil || len(got) != 2 {
		t.Fatalf("default index class changed: %d, %v", len(got), err)
	}
}
