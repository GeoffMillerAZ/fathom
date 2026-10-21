package mcp_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

type incompleteReviewFake struct {
	reviewFakeBackend
	listErr, typedErr error
}

type commitDriftReviewFake struct {
	reviewFakeBackend
	commit string
}

func (f *commitDriftReviewFake) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return f.svcPath, f.commit, true, nil
}
func (f *commitDriftReviewFake) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	if layer == review.ClaimsLayer {
		f.commit = "new-commit"
	}
	return f.reviewFakeBackend.ClaimsForService(ctx, service, layer)
}

func TestReviewDiffIndexCommitDriftRefusesFindingWrites(t *testing.T) {
	f := &commitDriftReviewFake{reviewFakeBackend: reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: t.TempDir()},
		sources:             map[string][]byte{"pkg/a.go": reviewHandlerSource},
	}, commit: "old-commit"}
	resp := roundtrip(t, mcp.NewServer(f), request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}}}})
	if f.putCalls != 0 || f.setCalls != 0 {
		t.Fatalf("commit drift allowed finding writes: put=%d set=%d response=%v", f.putCalls, f.setCalls, resp)
	}
	if resp["error"] == nil {
		p := extToolPayload(t, resp)
		partial, _ := p["corpus_partial"].(string)
		if !strings.Contains(partial, "index identity changed") {
			t.Fatalf("commit drift not named: %v", p)
		}
	}
}

func (f *incompleteReviewFake) ListAllSymbols(ctx context.Context, service string) ([]core.SymbolNode, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.reviewFakeBackend.ListAllSymbols(ctx, service)
}
func (f *incompleteReviewFake) TypedEdges(ctx context.Context, service string) ([]core.Edge, error) {
	if f.typedErr != nil {
		return nil, f.typedErr
	}
	return f.reviewFakeBackend.TypedEdges(ctx, service)
}

func TestReviewDiffGraphReadFailurePrecedesFindingWrites(t *testing.T) {
	for _, tc := range []struct {
		name              string
		listErr, typedErr error
	}{
		{"symbols", errors.New("symbols unavailable"), nil},
		{"typed", nil, errors.New("typed unavailable")},
		{"deadline", nil, context.DeadlineExceeded},
		{"refuse", memo.ErrWouldNotFinish, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &incompleteReviewFake{reviewFakeBackend: reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}}, listErr: tc.listErr, typedErr: tc.typedErr}
			resp := roundtrip(t, mcp.NewServer(f), request{JSONRPC: "2.0", ID: 1, Method: "tools/call", Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}}}})
			if resp["error"] == nil {
				t.Fatalf("incomplete graph yielded answer: %v", resp)
			}
			message := resp["error"].(map[string]any)["message"].(string)
			if message == "" || (!strings.Contains(message, "unavailable") && !strings.Contains(message, "deadline") && !strings.Contains(message, "declined")) {
				t.Fatalf("read failure was not named: %s", message)
			}
			if f.putCalls != 0 || f.setCalls != 0 {
				t.Fatalf("finding writes after read failure: put=%d set=%d", f.putCalls, f.setCalls)
			}
		})
	}
}
