package client_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// scopeRecorder is a decider that records the decide.Scope each ask
// carries, then answers as inner does.
type scopeRecorder struct {
	inner decision.Decider

	mu     sync.Mutex
	scopes []decide.Scope
}

func (s *scopeRecorder) Name() string { return s.inner.Name() }

func (s *scopeRecorder) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	s.mu.Lock()
	s.scopes = append(s.scopes, decide.ScopeFrom(ctx))
	s.mu.Unlock()
	return s.inner.Decide(ctx, req)
}

// TestClient_DecisionSitesReachThePort is KS-14 (O1131): the client's
// compile pre-filter and obligations re-rank ask the decision port with
// their site on the context and the caller's session kept, so a live kill
// is disclosed once per session and site.
func TestClient_DecisionSitesReachThePort(t *testing.T) {
	ctx := decide.WithSession(context.Background(), "s1")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Rules\n\n- Run tests before committing.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package main\nfunc Foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}
	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`
	c.WithLLM(llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: mockJSON, Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5}}}), "mock-model", 0, 0)
	if _, err := c.CompileObligations(ctx, "svc", false); err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}

	rec := &scopeRecorder{inner: decision.NewMockDecider("mock-decider", decision.MockEntry{Response: decision.Response{Provider: "mock-decider"}})}
	c.WithDecision(rec, config.Decide{RankWithinTier: true, CompileThreshold: 0.9, Sites: []string{decide.SiteObligationsRerank, decide.SiteCompilePrefilter}}, nil)
	if _, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10); err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if _, err := c.CompileObligations(ctx, "svc", true); err != nil {
		t.Fatalf("CompileObligations (dry run): %v", err)
	}
	for _, want := range []decide.Scope{{Session: "s1", Site: decide.SiteObligationsRerank}, {Session: "s1", Site: decide.SiteCompilePrefilter}} {
		if !slices.Contains(rec.scopes, want) {
			t.Errorf("no ask carried %+v; the port saw %+v", want, rec.scopes)
		}
	}
}
