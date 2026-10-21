package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type generationVerifyBackend struct {
	*verifyFakeBackend
	p     store.SourceRefProjection
	found bool
	scans int
}

func (b *generationVerifyBackend) SourceRefProjection(context.Context, string) (store.SourceRefProjection, bool, error) {
	return b.p, b.found, nil
}

func (b *generationVerifyBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	b.scans++
	return b.verifyFakeBackend.ScanSources(ctx, svc, bud, fn)
}

func generationFixture() store.SourceRefProjection {
	h := sha256.Sum256([]byte("[FR-1]"))
	p := store.SourceRefProjection{
		ServiceID: "svc", Scope: store.SourceRefScopeIngestGeneration,
		CurrentWorktreeFreshness: "unknown", Root: "/fixture", Policy: "policy",
		Watermark: "head@1", ExtractorVersion: store.SourceRefExtractorVersion,
		Rows: []store.SourceRefFact{
			{Path: "a.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: store.SourceRefExtractorVersion, Refs: []string{"[FR-1]"}},
			{Path: "empty.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: store.SourceRefExtractorVersion, Refs: []string{}},
			{Path: "large.dat", ExtractorVersion: store.SourceRefExtractorVersion, Excluded: "oversize"},
		},
	}
	p.MembershipHash = store.SourceRefMembershipHash(p.Rows)
	return p
}

func generationPlanCall(t *testing.T, b *generationVerifyBackend, scope string) map[string]any {
	t.Helper()
	args := map[string]any{
		"service": "svc", "source_scope": scope,
		"tasks": []any{map[string]any{"id": "T-1", "title": "reference", "symbol": "DoWork", "spec_ids": []string{"[FR-1]"}, "claimed": true}},
	}
	return roundtrip(t, mcp.NewServer(b), request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "verify_plan", "arguments": args}})
}

func TestVerifyPlanGenerationProjectionExplicitAndUnknown(t *testing.T) {
	b := &generationVerifyBackend{verifyFakeBackend: newVerifyBackend(), p: generationFixture(), found: true}
	resp := generationPlanCall(t, b, store.SourceRefScopeIngestGeneration)
	if resp["error"] != nil || b.scans != 0 {
		t.Fatalf("generation request failed or scanned live source: %v scans=%d", resp, b.scans)
	}
	p := xtraPayload(t, resp)
	if p["source_scope"] != store.SourceRefScopeIngestGeneration || p["current_worktree_freshness"] != "unknown" {
		t.Fatalf("scope disclosure: %v", p)
	}
	proj := p["source_reference_projection"].(map[string]any)
	if proj["examined"] != float64(2) || proj["total"] != float64(3) || proj["complete"] != false {
		t.Fatalf("coverage: %v", proj)
	}
	ex := proj["excluded"].([]any)
	if len(ex) != 1 || ex[0].(map[string]any)["path"] != "large.dat" || ex[0].(map[string]any)["reason"] != "oversize" {
		t.Fatalf("exclusions: %v", ex)
	}
	task := p["tasks"].([]any)[0].(map[string]any)
	if task["status"] != "unknown" || task["label"] != "spec-generation-only" {
		t.Fatalf("generation facts falsely gated a current row: %v", task)
	}
	_ = generationPlanCall(t, b, "current_worktree")
	if b.scans != 1 {
		t.Fatalf("default live scan bypassed: scans=%d", b.scans)
	}
}

func TestVerifyPlanGenerationRefusesMissingOrInvalidSeal(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(*generationVerifyBackend)
	}{
		{"old store", func(b *generationVerifyBackend) { b.found = false }},
		{"stale watermark", func(b *generationVerifyBackend) { b.p.Watermark = "" }},
		{"stale root", func(b *generationVerifyBackend) { b.p.Root = "" }},
		{"policy", func(b *generationVerifyBackend) { b.p.Policy = "" }},
		{"version", func(b *generationVerifyBackend) { b.p.ExtractorVersion = "old" }},
		{"membership", func(b *generationVerifyBackend) { b.p.Rows = b.p.Rows[:2] }},
		{"ref hash", func(b *generationVerifyBackend) {
			b.p.Rows[0].SourceHash = "bad"
			b.p.MembershipHash = store.SourceRefMembershipHash(b.p.Rows)
		}},
		{"nil refs", func(b *generationVerifyBackend) {
			b.p.Rows[1].Refs = nil
			b.p.MembershipHash = store.SourceRefMembershipHash(b.p.Rows)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			b := &generationVerifyBackend{verifyFakeBackend: newVerifyBackend(), p: generationFixture(), found: true}
			mutate.fn(b)
			resp := generationPlanCall(t, b, store.SourceRefScopeIngestGeneration)
			if !strings.Contains(strings.ToLower(respErrorText(resp)), "reference facts") || b.scans != 0 {
				t.Fatalf("invalid projection accepted or live fallback: %v scans=%d", resp, b.scans)
			}
		})
	}
}

func TestVerifyPlanGenerationRejectsUnknownScope(t *testing.T) {
	b := &generationVerifyBackend{verifyFakeBackend: newVerifyBackend(), p: generationFixture(), found: true}
	resp := generationPlanCall(t, b, "unknown")
	if !strings.Contains(respErrorText(resp), "unsupported source_scope") || b.scans != 0 {
		t.Fatalf("unknown scope accepted: %v scans=%d", resp, b.scans)
	}
}

func respErrorText(resp map[string]any) string {
	err, ok := resp["error"].(map[string]any)
	if !ok {
		return ""
	}
	msg, _ := err["message"].(string)
	return msg
}
