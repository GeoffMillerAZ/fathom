package mcp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

type demandNoSnapshotBackend struct {
	*coldGraphBackend
	listCalls int
}

func (b *demandNoSnapshotBackend) ListAllSymbols(ctx context.Context, service string) ([]core.SymbolNode, error) {
	b.listCalls++
	return b.coldGraphBackend.ListAllSymbols(ctx, service)
}

// A file-only plan has a verdict even while this service's graph input is
// unavailable. The blocked typed read makes accidental graph construction
// observable as an elided answer instead of relying on timing assertions.
func TestVerifyPlanFileOnlyDoesNotLoadGraph(t *testing.T) {
	gate := make(chan struct{})
	defer close(gate)
	b := &demandNoSnapshotBackend{coldGraphBackend: &coldGraphBackend{
		storeID: "file-only-" + t.Name(), mark: "w1", gate: gate, typedReads: map[string]int{},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv := NewServer(b)
	got := callToolWithin(t, ctx, srv, "verify_plan", map[string]any{
		"service": "svc",
		"tasks":   []any{map[string]any{"id": "N-1", "title": "no contract", "claimed": true}},
	})
	tasks, ok := got["tasks"].([]any)
	if !ok || len(tasks) != 1 || tasks[0].(map[string]any)["label"] != "no-contract" {
		t.Fatalf("no-contract verdict was elided by an unused graph: %+v", got)
	}
	oracle := callToolWithin(t, ctx, srv, "verify_plan", map[string]any{
		"service": "svc", "plan_markdown": "- [x] file {id: F-1, oracle: lexical, path: proof.txt}\n",
	})
	rows := oracle["tasks"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["label"] != "oracle-unevaluated" {
		t.Fatalf("unreadable file oracle lost its Unknown verdict: %+v", oracle)
	}
	empty := callToolWithin(t, ctx, srv, "verify_plan", map[string]any{"service": "svc"})
	if len(empty["tasks"].([]any)) != 0 || empty["summary"].(map[string]any)["total"] != float64(0) {
		t.Fatalf("empty plan did not report zero rows: %+v", empty)
	}
	b.mu.Lock()
	reads := b.typedReads["w1"]
	b.mu.Unlock()
	if reads != 0 {
		t.Fatalf("file-only plan read typed graph input %d times", reads)
	}
	if b.listCalls != 0 {
		t.Fatalf("file-only plan loaded the unused symbol snapshot %d times", b.listCalls)
	}
}

type demandSpecBackend struct {
	*coldGraphBackend
	scanErr error
	partial bool
}

func (b *demandSpecBackend) ScanSources(_ context.Context, _ string, _ corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	if b.scanErr != nil {
		return corpus.Coverage{}, b.scanErr
	}
	if b.partial {
		fn(core.SourceFile{Path: "one.go", Content: []byte("package one\n")})
		return corpus.Coverage{Scanned: 1, Total: 2, Stopped: corpus.StopBytes}, nil
	}
	return corpus.Coverage{}, nil
}

func TestVerifyPlanSpecOnlyKeepsSnapshotAndScanSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, spec, status string
		partial            bool
		scanErr            error
	}{
		{name: "symbol reference", spec: "[FR-1]", status: "pass"},
		{name: "partial source scan", spec: "[FR-2]", status: "fail", partial: true},
		{name: "busy source scan", spec: "[FR-1]", scanErr: core.ErrSourcesBusy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := make(chan struct{})
			defer close(gate)
			b := &demandSpecBackend{coldGraphBackend: &coldGraphBackend{
				storeID: "spec-only-" + t.Name(), mark: "w1", gate: gate,
				nodes:      []core.SymbolNode{{Service: "svc", Name: "Implement", Signature: "// [FR-1]", FilePath: "impl.go"}},
				typedReads: map[string]int{},
			}, partial: tc.partial, scanErr: tc.scanErr}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			args := map[string]any{"service": "svc", "tasks": []any{map[string]any{
				"id": "S-1", "title": "spec", "spec_ids": []string{tc.spec}, "claimed": true,
			}}}
			if tc.scanErr != nil {
				b.gate = nil
				msg := callToolFailure(t, NewServer(b), "verify_plan", args)
				if !strings.Contains(msg, tc.scanErr.Error()) {
					t.Fatalf("busy scan was not refused: %s", msg)
				}
			} else {
				got := callToolWithin(t, ctx, NewServer(b), "verify_plan", args)
				tasks, ok := got["tasks"].([]any)
				if !ok || len(tasks) != 1 || tasks[0].(map[string]any)["status"] != tc.status {
					t.Fatalf("spec-only verdict: %+v", got)
				}
				_, hasPartial := got["partial"]
				if hasPartial != tc.partial {
					t.Fatalf("partial disclosure = %v, want %v: %+v", hasPartial, tc.partial, got)
				}
			}
			b.mu.Lock()
			reads := b.typedReads["w1"]
			b.mu.Unlock()
			if reads != 0 {
				t.Fatalf("spec-only plan read typed edges %d times", reads)
			}
		})
	}
}

func TestVerifyPlanMixedAndGoneStillWaitForGraph(t *testing.T) {
	for _, tc := range []struct{ name, plan string }{
		{"mixed", "- [x] file {id: F-1, oracle: lexical, path: missing.txt}\n- [x] code {id: C-1, symbol: Run}\n"},
		{"gone", "- [x] remove callers {id: G-1, gone: callers Run}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := make(chan struct{})
			defer close(gate)
			b := &coldGraphBackend{storeID: "semantic-" + t.Name(), mark: "w1", gate: gate,
				nodes:      []core.SymbolNode{{Service: "svc", Name: "Run", Kind: core.KindGoFunc, FilePath: "run.go", Tier: core.TierVerified}},
				typedReads: map[string]int{},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			got := callToolWithin(t, ctx, NewServer(b), "verify_plan", map[string]any{
				"service": "svc", "plan_markdown": tc.plan,
			})
			if len(got["tasks"].([]any)) != 0 || got["elided"].(map[string]any)["count"] != float64(strings.Count(tc.plan, "- [x]")) {
				t.Fatalf("semantic plan bypassed unavailable graph: %+v", got)
			}
			b.mu.Lock()
			reads := b.typedReads["w1"]
			b.mu.Unlock()
			if reads != 1 {
				t.Fatalf("semantic plan loaded typed edges %d times, want 1", reads)
			}
		})
	}
}

// The same spec row must report the same criteria when a second, semantic
// row forces the full stage. Only the mixed plan's aggregate may differ.
func TestVerifyPlanSpecRowMatchesFullStage(t *testing.T) {
	b := &demandSpecBackend{coldGraphBackend: &coldGraphBackend{
		storeID: "spec-parity-" + t.Name(), mark: "w1", typedReads: map[string]int{},
		nodes: []core.SymbolNode{
			{Service: "svc", Name: "Implement", Signature: "// [FR-1]", FilePath: "impl.go"},
			{Service: "svc", Name: "Run", Kind: core.KindGoFunc, FilePath: "run.go", Tier: core.TierVerified},
		},
	}}
	srv := NewServer(b)
	spec := map[string]any{"id": "S-1", "title": "spec", "spec_ids": []string{"[FR-1]"}, "claimed": true}
	fileOnly := callToolWithin(t, context.Background(), srv, "verify_plan", map[string]any{
		"service": "svc", "tasks": []any{spec},
	})
	mixed := callToolWithin(t, context.Background(), srv, "verify_plan", map[string]any{
		"service": "svc", "tasks": []any{spec, map[string]any{"id": "C-1", "symbol": "Run"}},
	})
	want := fileOnly["tasks"].([]any)[0]
	var got any
	for _, row := range mixed["tasks"].([]any) {
		if row.(map[string]any)["id"] == "S-1" {
			got = row
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spec row changed with unused graph stage:\nonly: %+v\nmixed: %+v", want, got)
	}
	b.mu.Lock()
	reads := b.typedReads["w1"]
	b.mu.Unlock()
	if reads != 1 {
		t.Fatalf("mixed plan loaded typed edges %d times, want 1", reads)
	}
}

// An uncacheable backend makes each run cold. The bound covers response and
// matrix allocations, and rejects allocating graphs proportional to the corpus.
func TestVerifyPlanDemandAllocationBound(t *testing.T) {
	b := &demandSpecBackend{coldGraphBackend: &coldGraphBackend{typedReads: map[string]int{}}}
	for i := 0; i < 1024; i++ {
		b.nodes = append(b.nodes, core.SymbolNode{Service: "svc", Package: "p", Name: fmt.Sprintf("Name%d", i), FilePath: fmt.Sprintf("p/file%d.go", i), Kind: core.KindGoFunc, Signature: "// [FR-1]"})
	}
	srv := NewServer(b)
	for _, specs := range [][]string{nil, {"[FR-1]"}} {
		args := map[string]any{"service": "svc", "tasks": []any{map[string]any{"id": "S-1", "claimed": true, "spec_ids": specs}}}
		// Charge the required snapshot and matrix separately: this bound is on plan
		// evaluation overhead, not the corpus's unavoidable symbol input.
		required := float64(0)
		if len(specs) > 0 {
			required = testing.AllocsPerRun(3, func() {
				if _, err := serviceSnapshot(context.Background(), b, "svc"); err != nil {
					t.Fatal(err)
				}
			})
			required += testing.AllocsPerRun(3, func() {
				if _, _, err := buildSpecMatrix(context.Background(), b, "svc", b.nodes, specs); err != nil {
					t.Fatal(err)
				}
			})
		}
		allocs := testing.AllocsPerRun(3, func() { callToolWithin(t, context.Background(), srv, "verify_plan", args) })
		t.Logf("specs=%v allocations=%.0f required=%.0f overhead=%.0f", specs, allocs, required, allocs-required)
		if allocs-required > 4000 {
			t.Fatalf("file/spec-only evidence allocated a graph: %.0f allocations, bound 4000", allocs)
		}
	}
}
