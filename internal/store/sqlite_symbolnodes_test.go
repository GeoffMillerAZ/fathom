package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
)

// Generated source tree and observations exercise every projected field,
// duplicate symbols, equal/out-of-order timestamps, and a second service.
func projectionStore(t *testing.T, count int) *SQLiteStore {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package sample\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewSQLiteStore(filepath.Join(root, "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	obs := make([]core.Observation, 0, count+1)
	for i := 0; i < count; i++ {
		calls := []string(nil)
		if i%2 == 0 {
			calls = []string{"p.Run", "p.Other"}
		}
		obs = append(obs, core.Observation{ServiceID: "svc", ConcernID: fmt.Sprintf("concern%d", i%7), SourceRef: "source.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: fmt.Sprintf("Name%d", i%1024), Package: "p", Claim: fmt.Sprintf("func Name%d()", i), ContentHash: "hash", Calls: calls, Complexity: i % 5, Line: i + 1, EndLine: i + 2, IngestedAt: time.Unix(100, int64((count-i)%13)), ToolKind: "go", Generated: i%3 == 0})
	}
	obs = append(obs, core.Observation{ServiceID: "other", SourceRef: "other.go", SourceKind: core.KindGoFunc, Symbol: "Foreign", IngestedAt: time.Unix(100, 0)})
	if err := s.PutObservations(context.Background(), obs); err != nil {
		t.Fatal(err)
	}
	return s
}

func observationProjection(ctx context.Context, s *SQLiteStore, service string) ([]core.SymbolNode, error) {
	obs, err := s.QueryObservations(ctx, service, "")
	if err != nil {
		return nil, err
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	return nodes, nil
}

func directProjection(ctx context.Context, s *SQLiteStore, service string) ([]core.SymbolNode, error) {
	if reader, ok := any(s).(interface {
		ListSymbolNodes(context.Context, string) ([]core.SymbolNode, error)
	}); ok {
		return reader.ListSymbolNodes(ctx, service)
	}
	return observationProjection(ctx, s, service)
}

func TestSymbolProjectionParity(t *testing.T) {
	sup := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, sup)
	s := projectionStore(t, 257)
	s.db.SetMaxOpenConns(1)
	for _, service := range []string{"svc", "other", "missing"} {
		want, err := observationProjection(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		got, err := directProjection(context.Background(), s, service)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("service %s projection differs: %v", service, err)
		}
	}
	if _, err := scope.AppendSuppression(sup, scope.Suppression{Service: "svc", Pattern: "source.go", Reason: "test", CreatedAt: time.Unix(100, 0), CreatedBy: scope.OriginCLI}); err != nil {
		t.Fatal(err)
	}
	want, err := observationProjection(context.Background(), s, "svc")
	if err != nil {
		t.Fatal(err)
	}
	got, err := directProjection(context.Background(), s, "svc")
	if err != nil || len(got) != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("suppression changed: %v", err)
	}
}

func TestSymbolProjectionErrorsAndCancellation(t *testing.T) {
	s := projectionStore(t, 1)
	if _, err := s.db.Exec(`UPDATE observations SET calls='{' WHERE service_id='svc'`); err != nil {
		t.Fatal(err)
	}
	_, want := observationProjection(context.Background(), s, "svc")
	got, err := directProjection(context.Background(), s, "svc")
	if want == nil || err == nil || err.Error() != want.Error() || got != nil {
		t.Fatalf("malformed calls parity: %v / %v", err, want)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = directProjection(ctx, s, "svc")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

// Count allocated bytes without measuring elapsed or CPU time. The frozen
// observation path provides the same corpus in the same binary.
func projectionAllocatedBytes(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

var projectionSink []core.SymbolNode

func TestSymbolProjectionAllocationBound(t *testing.T) {
	s := projectionStore(t, 4096)
	read := func(fn func(context.Context, *SQLiteStore, string) ([]core.SymbolNode, error)) func() {
		return func() {
			var err error
			projectionSink, err = fn(context.Background(), s, "svc")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	// Warm driver initialization before counting.
	read(observationProjection)()
	read(directProjection)()
	baseline := projectionAllocatedBytes(read(observationProjection))
	candidate := projectionAllocatedBytes(read(directProjection))
	t.Logf("observation=%d direct=%d allocated bytes", baseline, candidate)
	if candidate > baseline*85/100 {
		t.Fatalf("projection allocated %d bytes, bound 85%% of observation path %d", candidate, baseline)
	}
}
