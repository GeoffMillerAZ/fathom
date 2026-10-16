package main

// compile_prefilter_once_test.go — EW-77 (G12, ruling X3): a spending
// `fathom compile` through the daemon sizes its client timeout from one
// estimate, and that sizing call does not ask the decision port's compile
// prefilter, so `[decide] compile_prefilter` costs one prefilter question
// per section, not two. A fake Decider counts the questions through a real
// daemon chain (mcp server over a client on a real ingested store, served
// by httptest); the LLM is a mock. No model is called.

import (
	"bytes"
	"context"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// countingPrefilterDecider keeps every section (Noul 1) and counts how many
// times each section's question was asked.
type countingPrefilterDecider struct {
	mu    sync.Mutex
	asked map[string]int
}

func (d *countingPrefilterDecider) Name() string { return "counting-fake" }

func (d *countingPrefilterDecider) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	resp := decision.Response{Provider: d.Name(), Answers: map[string]decision.Answer{}}
	for _, q := range req.Questions {
		d.asked[q.ID]++
		resp.Answers[q.ID] = decision.Answer{ID: q.ID, Kind: q.Kind, Noul: 1, Confidence: 1}
	}
	return resp, nil
}

func TestCompileSizingCallsPrefilterOnce(t *testing.T) {
	isolateConfigDirs(t)
	storePath, _ := newGuardTestStore(t)
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text:  `{"claims":[]}`,
		Usage: llmclient.Usage{InputTokens: 100, OutputTokens: 10},
	}})
	c.WithLLM(mock, "mock-model", knownModelInRatePerKTok, knownModelOutRatePerKTok)
	dec := &countingPrefilterDecider{asked: map[string]int{}}
	cfg := config.DefaultDecide
	cfg.Provider = "counting-fake"
	cfg.Sites = []string{"compile_prefilter"}
	c.WithDecision(dec, cfg, nil)

	srv := mcp.NewServer(c)
	h := newHTTPHandlerLogged(srv, c, "127.0.0.1:0", nil, testDiscardLogger())
	ts := httptest.NewServer(newDaemonServer(h, daemonServerOpts{}).Handler)
	t.Cleanup(ts.Close)

	var stdout, stderr bytes.Buffer
	if code := compileViaHTTP(ts.URL, "svc", "obligations", false, &stdout, &stderr); code != 0 {
		t.Fatalf("spending compile exit %d: %s", code, stderr.String())
	}
	dec.mu.Lock()
	defer dec.mu.Unlock()
	if len(dec.asked) == 0 {
		t.Fatalf("the prefilter was never asked; the fixture has no section for it:\n%s", stdout.String())
	}
	ids := make([]string, 0, len(dec.asked))
	for id := range dec.asked {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if n := dec.asked[id]; n != 1 {
			t.Errorf("section %s: the prefilter was asked %d times, want once (the sizing dry run must not ask it)", id, n)
		}
	}
	if mock.CallCount() == 0 {
		t.Errorf("the spending compile never reached the (mock) model: the prefilter's one pass must be the spend's own")
	}
}
