package mcp_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

func reviewRound3Fixture(t *testing.T) *reviewFakeBackend {
	t.Helper()
	root := t.TempDir()
	writeCorpusFile(t, root, ".fathom/rules/archconformance.cue", "rules: [{from_pkg: \"api\", to_pkg: \"store\", allow: false}]\n")
	return &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{svcPath: root, services: []string{"svc"}, nodes: []core.SymbolNode{
			{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
			{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
		}},
		sources: map[string][]byte{"spec.md": []byte("// implements [US-001]\n"), "api/handler.go": reviewHandlerSource},
	}
}

func reviewRound3Call(t *testing.T, srv *mcp.Server, changed []string) map[string]any {
	t.Helper()
	return extToolPayload(t, roundtrip(t, srv, request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": changed}}}))
}

func assertReviewRound3Persisted(t *testing.T, b *reviewFakeBackend, p map[string]any) {
	t.Helper()
	if p["findings_persisted"] != true || b.putCalls == 0 || len(b.claims) == 0 {
		t.Errorf("findings not persisted: puts=%d payload=%v", b.putCalls, p)
	}
	for _, raw := range p["dimensions"].([]any) {
		d := raw.(map[string]any)
		if d["name"] == review.DimSpecCoverage {
			if d["checked"] != true || d["count"] != float64(1) {
				t.Errorf("spec coverage = %v, want checked with one finding", d)
			}
			return
		}
	}
	t.Fatal("no spec coverage dimension")
}

type analysisElapsedReviewBackend struct {
	*reviewFakeBackend
	now           time.Time
	advanced      bool
	citationReads int
	advanceReads  bool
}

func (b *analysisElapsedReviewBackend) ClaimsForService(ctx context.Context, svc, layer string) ([]core.SemanticClaim, error) {
	// This boundary is reached after review.Run, before citation reads and
	// revalidation. Model six seconds of analysis without an elapsed-time test.
	if layer == review.ClaimsLayer && !b.advanced {
		b.now = b.now.Add(6 * time.Second)
		b.advanced = true
	}
	return b.reviewFakeBackend.ClaimsForService(ctx, svc, layer)
}

func (b *analysisElapsedReviewBackend) ReadSourceFiles(ctx context.Context, svc string, paths []string) ([]core.SourceFile, error) {
	if b.advanceReads && paths[0] == "api/handler.go" {
		b.citationReads++
		if b.citationReads == 1 {
			b.now = b.now.Add(4 * time.Second)
		} else {
			b.now = b.now.Add(2 * time.Second)
		}
	}
	return b.reviewFakeBackend.ReadSourceFiles(ctx, svc, paths)
}

func TestReviewDiff_AnalysisTimeDoesNotConsumeSourceWindows(t *testing.T) {
	b := &analysisElapsedReviewBackend{reviewFakeBackend: reviewRound3Fixture(t), now: time.Unix(1800000000, 0), advanceReads: true}
	p := reviewRound3Call(t, mcp.NewServer(b, func() time.Time { return b.now }), []string{"spec.md"})
	assertReviewRound3Persisted(t, b.reviewFakeBackend, p)
	if !b.advanced {
		t.Fatal("analysis interval not exercised")
	}
	if b.citationReads != 2 {
		t.Fatalf("citation reads=%d, want citation read and revalidation in separate windows", b.citationReads)
	}
	if p["corpus_partial"] != nil {
		t.Errorf("analysis time consumed read budget: %v", p["corpus_partial"])
	}
	// The architecture finding cites a source outside the diff, requiring a
	// new citation read after the simulated analysis interval.
	cites := false
	for _, links := range b.links {
		for _, l := range links {
			if strings.HasPrefix(l.TargetRef, "api/handler.go#L") {
				cites = true
			}
		}
	}
	if !cites {
		t.Errorf("outside-diff citation not anchored: %v", b.links)
	}
}

func TestReviewDiff_NamedExclusionsKeepPersistenceAndSpecCoverage(t *testing.T) {
	for _, kind := range []string{"oversize", "binary", "both"} {
		t.Run(kind, func(t *testing.T) {
			b := reviewRound3Fixture(t)
			changed := []string{"spec.md"}
			want := []string{}
			if kind != "binary" {
				b.sources["generated.lock"] = append([]byte("// Code generated. DO NOT EDIT.\n"), bytes.Repeat([]byte("x"), 3<<20)...)
				changed = append(changed, "generated.lock")
				want = append(want, "generated.lock:over 2 MiB")
			}
			if kind != "oversize" {
				b.sources["asset.bin"] = []byte{'x', 0, 'y'}
				changed = append(changed, "asset.bin")
				want = append([]string{"asset.bin:binary"}, want...)
			}
			p := reviewRound3Call(t, mcp.NewServer(b), changed)
			assertReviewRound3Persisted(t, b, p)
			if p["corpus_partial"] != nil {
				t.Errorf("excluded files became gaps: %v", p["corpus_partial"])
			}
			var got []string
			for _, raw := range asArray(p["excluded"]) {
				e := raw.(map[string]any)
				got = append(got, fmt.Sprint(e["path"])+":"+fmt.Sprint(e["reason"]))
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("excluded = %v, want %v", got, want)
			}
		})
	}
}

func asArray(v any) []any { a, _ := v.([]any); return a }

func TestReviewDiff_OptionalGraphFailureKeepsVerdict(t *testing.T) {
	b := &reviewReadErrorBackend{reviewFakeBackend: *reviewRound3Fixture(t), typedErr: memo.ErrWouldNotFinish}
	p := reviewRound3Call(t, mcp.NewServer(b), []string{"spec.md"})
	assertReviewRound3Persisted(t, &b.reviewFakeBackend, p)
	if !strings.Contains(fmt.Sprint(p["partial"]), "no build was started") {
		t.Errorf("missing graph partial: %v", p)
	}
	for _, field := range []string{"resolution", "layers", "receipt", "receipt_sarif", "receipt_check"} {
		if p[field] != nil {
			t.Errorf("%s present without optional graphs", field)
		}
	}
}

func TestReviewDiff_SnapshotFailureIsStrictAndNamed(t *testing.T) {
	b := &graphLoadFailBackend{extendedFakeBackend{services: []string{"svc"}}}
	resp := roundtrip(t, mcp.NewServer(b), request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"spec.md"}}}})
	e, _ := resp["error"].(map[string]any)
	if !strings.Contains(fmt.Sprint(e["message"]), "call graph for service svc could not be loaded") || !strings.Contains(fmt.Sprint(e["message"]), errGraphLoad.Error()) {
		t.Fatalf("snapshot failure not named: %v", resp)
	}
}

type oversizeReviewBackend struct{ *reviewFakeBackend }

func (b *oversizeReviewBackend) ReadSourceFiles(ctx context.Context, svc string, paths []string) ([]core.SourceFile, error) {
	if paths[0] == "generated.lock" {
		return nil, &connector.OversizeError{Path: "generated.lock", Size: 3 << 20, Cap: 2 << 20}
	}
	return b.reviewFakeBackend.ReadSourceFiles(ctx, svc, paths)
}
func TestReviewDiff_ConnectorOversizeSurvivesRefreshWrapper(t *testing.T) {
	b := &oversizeReviewBackend{reviewRound3Fixture(t)}
	b.sources["generated.lock"] = nil // membership, without retaining oversized content
	p := reviewRound3Call(t, mcp.NewServer(mcp.NewAutoRefreshForTest(t, b)), []string{"generated.lock", "spec.md"})
	assertReviewRound3Persisted(t, b.reviewFakeBackend, p)
	excluded := asArray(p["excluded"])
	if len(excluded) != 1 || excluded[0].(map[string]any)["path"] != "generated.lock" || excluded[0].(map[string]any)["reason"] != "over 2 MiB" {
		t.Errorf("missing connector exclusion: %v", p)
	}
}
