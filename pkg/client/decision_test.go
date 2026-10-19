package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestClient_WithDecision_RankedBy pins DP-9's client-level wiring
// (docs/plans/decision-port.md): a Client that never calls WithDecision, or
// is explicitly wired with decision.None, serves obligations with
// ranked_by:"none" (obligationsCore's own untouched path) — a Client wired
// with a real decision.Decider AND rank_within_tier on serves ranked_by
// naming that Decider, proving Obligations is actually routed through
// semantic.ObligationsForOpts and not the pre-DP-9 ObligationsFor.
func TestClient_WithDecision_RankedBy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	if _, err := c.CompileObligations(ctx, "svc", false); err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}

	// Baseline: WithDecision never called — ranked_by "none".
	obl, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (no decision port): %v", err)
	}
	if len(obl.Rules) == 0 {
		t.Fatal("expected at least one rule for pkg/foo.go — test fixture is broken")
	}
	if obl.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (no WithDecision call)", obl.RankedBy, "none")
	}

	// decision.None wired explicitly — still "none": None must never rank.
	c.WithDecision(decision.None{}, config.Decide{RankWithinTier: true}, nil)
	oblNone, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (None decider): %v", err)
	}
	if oblNone.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (decision.None wired)", oblNone.RankedBy, "none")
	}

	// A real (mock) Decider with rank_within_tier on — ranked_by names it.
	mock := decision.NewMockDecider("mock-decider", decision.MockEntry{
		Response: decision.Response{Provider: "mock-decider"},
	})
	c.WithDecision(mock, config.Decide{RankWithinTier: true, Sites: []string{decide.SiteObligationsRerank}}, nil)
	oblMock, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (mock decider): %v", err)
	}
	if oblMock.RankedBy != "mock-decider" {
		t.Errorf("RankedBy = %q, want %q (mock decider wired, rank_within_tier on)", oblMock.RankedBy, "mock-decider")
	}
	if len(mock.Calls()) != 1 {
		t.Errorf("mock decider called %d times, want 1", len(mock.Calls()))
	}

	// rank_within_tier OFF gates the call even with a real Decider wired —
	// both gates (Decider != nil AND RankWithinTier) must hold.
	c.WithDecision(mock, config.Decide{RankWithinTier: false}, nil)
	oblGated, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations (rank_within_tier off): %v", err)
	}
	if oblGated.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (rank_within_tier off)", oblGated.RankedBy, "none")
	}
}

// TestClient_WithDecision_SiteNotAllowedMakesNoCall pins CV-42 (ADR-044
// decision 8, I14): a real, active decider (RankWithinTier on, a live
// MockDecider) whose [decide] sites excludes "obligations_rerank" must
// serve obligations exactly as if WithDecision had never been called —
// ranked_by "none", zero Decide calls — the SAME degrade
// TestClient_WithDecision_RankedBy's "rank_within_tier off" case already
// pins for the OTHER gate (RankWithinTier), proving Sites is a second,
// independent gate a real decider must also clear.
func TestClient_WithDecision_SiteNotAllowedMakesNoCall(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	if _, err := c.CompileObligations(ctx, "svc", false); err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}

	mock := decision.NewMockDecider("mock-decider", decision.MockEntry{
		Response: decision.Response{Provider: "mock-decider"},
	})
	// RankWithinTier is on and the decider is real — Sites lists a real
	// site, just not obligations_rerank, so this proves the gate is
	// per-site, not merely "Sites is unset".
	c.WithDecision(mock, config.Decide{RankWithinTier: true, Sites: []string{decide.SiteRoute}}, nil)
	obl, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if obl.RankedBy != "none" {
		t.Errorf("RankedBy = %q, want %q (obligations_rerank not in [decide] sites)", obl.RankedBy, "none")
	}
	if len(mock.Calls()) != 0 {
		t.Errorf("mock decider called %d time(s), want 0 (site not allowed)", len(mock.Calls()))
	}
}

// TestClient_WithDecision_NoneLeavesCompileUntouched pins the CompileLayer
// half of DP-9: a Client wired with decision.None (or never wired at all)
// compiles with SectionsPrefiltered == 0 and no LLM prefilter call — the
// prefilter's own identity fast path, reached only because
// Client.decisionActive gates Compiler.Decider/PrefilterThreshold off for a
// None decider instead of wiring it through unconditionally.
func TestClient_WithDecision_NoneLeavesCompileUntouched(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	c.WithDecision(decision.None{}, config.Decide{CompileThreshold: 0.9}, nil)

	run, err := c.CompileObligations(ctx, "svc", false)
	if err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}
	if run.SectionsPrefiltered != 0 {
		t.Errorf("SectionsPrefiltered = %d, want 0 (decision.None must never skip a section)", run.SectionsPrefiltered)
	}
	if len(run.Decisions) != 0 {
		t.Errorf("Decisions = %d rows, want 0 (decision.None makes no call, records no decision)", len(run.Decisions))
	}
}

// TestClient_WithDecision_CompilePrefilterSiteNotAllowedMakesNoCall pins
// CV-42's other half: a REAL (non-None) decider wired through WithDecision
// but with [decide] sites excluding "compile_prefilter" must leave
// CompileLayer untouched exactly like TestClient_WithDecision_
// NoneLeavesCompileUntouched — SectionsPrefiltered 0, no Decisions rows —
// and, unlike that test, the mock decider itself proves NO call was even
// attempted (a None decider can never be called at all; a real one can, so
// only its own Calls() count actually distinguishes "gated by Sites" from
// "gated by the decider's own identity").
func TestClient_WithDecision_CompilePrefilterSiteNotAllowedMakesNoCall(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	mock := decision.NewMockDecider("mock-decider", decision.MockEntry{
		Response: decision.Response{Provider: "mock-decider"},
	})
	// Sites lists a real site, just not compile_prefilter — proves the
	// gate is per-site, not merely "Sites is unset".
	c.WithDecision(mock, config.Decide{CompileThreshold: 0.9, Sites: []string{decide.SiteRoute}}, nil)

	run, err := c.CompileObligations(ctx, "svc", false)
	if err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}
	if run.SectionsPrefiltered != 0 {
		t.Errorf("SectionsPrefiltered = %d, want 0 (compile_prefilter not in [decide] sites)", run.SectionsPrefiltered)
	}
	if len(run.Decisions) != 0 {
		t.Errorf("Decisions = %d rows, want 0 (site not allowed, no call made)", len(run.Decisions))
	}
	if len(mock.Calls()) != 0 {
		t.Errorf("mock decider called %d time(s), want 0 (site not allowed)", len(mock.Calls()))
	}
}

// typesafeSystemOneFake is an httptest handler for TypeSafe's System One
// wire (POST {BaseURL}/v1/systemone): it answers every question in the
// request — whatever IDs it asked, echoed back — as a noul at 0.95, the
// same envelope shape docs.typesafe.ai and this wave's brief document.
func typesafeSystemOneFake(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		answers := make(map[string]any, len(body.Questions))
		for id := range body.Questions {
			answers[id] = map[string]any{"type": "noul", "noul": 0.95}
		}
		resp := map[string]any{
			"model":   "jev-1.13.0",
			"answers": answers,
			"usage":   map[string]any{"input_tokens": 42, "output_tokens": 7},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// TestWithDecision_TypeSafeFake pins DP-18: with a real kit TypeSafe
// Decider (opened by decide.Open over an httptest fake System One server
// answering every noul at 0.95) wired through WithDecision, the obligations
// envelope's RankedBy is "typesafe" and the shared decisions.jsonl recorder
// gets a row naming provider "typesafe" — proving decide.Open's typesafe branch (DP-16) is
// not just constructible but actually reachable end to end through the
// same Client.Obligations path TestClient_WithDecision_RankedBy already
// pins for "llm"/mock.
func TestWithDecision_TypeSafeFake(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	claudeMD := []byte("# Rules\n\n- Run tests before committing.\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["pkg/**"],"cite_start":3,"cite_end":3}]}`
	mockLLM := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
		},
	})
	c.WithLLM(mockLLM, "mock-model", 0, 0)
	if _, err := c.CompileObligations(ctx, "svc", false); err != nil {
		t.Fatalf("CompileObligations: %v", err)
	}

	srv := typesafeSystemOneFake(t)
	defer srv.Close()

	// Through decide.Open, as the daemon builds it: only internal/decide
	// imports a provider adapter package (KS-9, ADR-073 decision 2).
	t.Setenv("FATHOM_HOME", t.TempDir()) // no kill file from the machine running the test
	t.Setenv(decide.KillEnv, "")
	keyFile := filepath.Join(t.TempDir(), "typesafe.key")
	if err := os.WriteFile(keyFile, []byte("test-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	rec := decide.NewRecorder(&log, time.Now, "test-version")
	cfg := config.Decide{Provider: "typesafe", BaseURL: srv.URL, APIKeyFile: keyFile, RankWithinTier: true, Sites: []string{decide.SiteObligationsRerank}}
	ports, err := decide.Open(cfg, nil, "", rec, decide.WithUserConfig(""))
	if err != nil {
		t.Fatalf("decide.Open: %v", err)
	}
	c.WithDecision(ports.Decider, ports.Cfg, rec)

	obl, err := c.Obligations(ctx, "svc", "pkg/foo.go", 10)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(obl.Rules) == 0 {
		t.Fatal("expected at least one rule for pkg/foo.go — test fixture is broken")
	}
	if obl.RankedBy != "typesafe" {
		t.Errorf("RankedBy = %q, want %q", obl.RankedBy, "typesafe")
	}

	logged := strings.TrimSpace(log.String())
	if logged == "" {
		t.Fatal("decisions.jsonl recorder got no rows from a typesafe rerank call")
	}
	found := false
	for _, line := range strings.Split(logged, "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("recorder row not JSON: %v (%s)", err, line)
		}
		if row["provider"] == "typesafe" {
			found = true
		}
	}
	if !found {
		t.Errorf("no recorder row named provider \"typesafe\":\n%s", logged)
	}
}
