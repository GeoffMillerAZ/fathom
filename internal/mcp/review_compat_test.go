package mcp_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type legacyReviewBackend struct {
	reviewFakeBackend
	SourcePaths     struct{}
	ReadSourceFiles struct{}
}

func (b *legacyReviewBackend) ScanSources(ctx context.Context, service string, bud corpus.Budget, visit func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := b.Sources(ctx, service)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, visit)
}

func reviewCompatCall(t *testing.T, b mcp.QueryInterface, changed string) map[string]any {
	t.Helper()
	resp := roundtrip(t, mcp.NewServer(b), request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{changed}}}})
	return extToolPayload(t, resp)
}

func TestReviewDiffLegacyBackendKeepsFindingPersistence(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom", "rules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "rules", "archconformance.cue"), []byte("rules: [{from_pkg: \"api\", to_pkg: \"store\", allow: false}]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b := &legacyReviewBackend{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{svcPath: root, services: []string{"svc"}, nodes: []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}}}}
	p := reviewCompatCall(t, b, "api/handler.go")
	if p["findings_persisted"] != true || b.putCalls == 0 || len(b.claims) == 0 {
		t.Fatalf("legacy persistence lost: put=%d payload=%v", b.putCalls, p)
	}
}

type reviewReadErrorBackend struct {
	reviewFakeBackend
	typedErr, metaErr, obligationErr error
}

func (b *reviewReadErrorBackend) TypedEdges(context.Context, string) ([]core.Edge, error) {
	return nil, b.typedErr
}
func (b *reviewReadErrorBackend) ServiceMeta(ctx context.Context, service string) (string, string, bool, error) {
	if b.metaErr != nil {
		return "", "", false, b.metaErr
	}
	return b.reviewFakeBackend.ServiceMeta(ctx, service)
}
func (b *reviewReadErrorBackend) Obligations(ctx context.Context, service, path string, max int) (semantic.Obligations, error) {
	if b.obligationErr != nil {
		return semantic.Obligations{}, b.obligationErr
	}
	return b.reviewFakeBackend.Obligations(ctx, service, path, max)
}

func TestReviewDiffTypedReadFailureUsesSyntaxGraph(t *testing.T) {
	for _, readErr := range []error{errors.New("typed unavailable"), context.DeadlineExceeded, context.Canceled} {
		t.Run(readErr.Error(), func(t *testing.T) {
			b := &reviewReadErrorBackend{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}}, typedErr: readErr}
			p := reviewCompatCall(t, b, "pkg/c.go")
			if !strings.Contains(fmt.Sprint(p["typed_edges_note"]), readErr.Error()) {
				t.Fatalf("fallback not named: %v", p)
			}
			for _, raw := range p["dimensions"].([]any) {
				d := raw.(map[string]any)
				if d["name"] == review.DimImpact && (d["checked"] != true || d["count"].(float64) == 0) {
					t.Fatalf("syntax impact unavailable: %v", d)
				}
			}
		})
	}
}

func TestReviewDiffMetadataAndObligationErrorsKeepBestEffort(t *testing.T) {
	for _, kind := range []string{"metadata", "obligation"} {
		t.Run(kind, func(t *testing.T) {
			b := &reviewReadErrorBackend{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}}}
			if kind == "metadata" {
				b.metaErr = errors.New("metadata unavailable")
			} else {
				b.obligationErr = errors.New("obligation unavailable")
			}
			p := reviewCompatCall(t, b, "pkg/a.go")
			if p["findings_persisted"] != true {
				t.Fatalf("best-effort read error refused findings: %v", p)
			}
			for _, raw := range p["dimensions"].([]any) {
				d := raw.(map[string]any)
				if d["name"] == review.DimObligations && d["checked"] != true {
					t.Fatalf("obligation error changed base status: %v", d)
				}
			}
		})
	}
}

type reviewIdentityBackend struct {
	reviewFakeBackend
	mode, commit, generation, id string
}

func (b *reviewIdentityBackend) StoreID() string { return b.id }
func (b *reviewIdentityBackend) StoreWatermark(context.Context, string) (string, error) {
	return b.generation, nil
}
func (b *reviewIdentityBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return "", b.commit, true, nil
}
func (b *reviewIdentityBackend) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	if layer == review.ClaimsLayer {
		if b.mode == "commit" {
			b.commit = "next"
		} else {
			b.generation = "next"
		}
	}
	return b.reviewFakeBackend.ClaimsForService(ctx, service, layer)
}
func TestReviewDiffIndexIdentityDriftRefusesFindingWrites(t *testing.T) {
	for _, mode := range []string{"commit", "generation"} {
		t.Run(mode, func(t *testing.T) {
			b := &reviewIdentityBackend{mode: mode, commit: "base", generation: "base", id: t.TempDir(), reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}}}
			p := reviewCompatCall(t, b, "pkg/a.go")
			if b.putCalls != 0 || b.setCalls != 0 || p["findings_persisted"] != false || !strings.Contains(fmt.Sprint(p["corpus_partial"]), "index identity changed") {
				t.Fatalf("identity drift wrote findings: put=%d set=%d payload=%v", b.putCalls, b.setCalls, p)
			}
		})
	}
}
