package decide_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// TestDecideOpenUsesSubpackageAdaptersAndStillReadsNoulRows pins JF-24
// (docs/plans/judgment-foundations.md): after the kit hardening (agentic-kit
// DP-12, DP-14) Open builds the adapters from their own packages,
// decision/llm and decision/typesafe; a yes/no answer is logged under the
// generic kind "predicate"; and a decisions.jsonl row written before the
// rename, kind "noul", still reads, as predicate, beside the new rows.
func TestDecideOpenUsesSubpackageAdaptersAndStillReadsNoulRows(t *testing.T) {
	t.Run("llm opens decision/llm", func(t *testing.T) {
		lane := func(string) (llmclient.LLMClient, error) { return llmclient.NewMockClient(), nil }
		ports, err := decide.Open(config.Decide{Provider: "llm"}, lane, "test-model", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, ok := ports.Decider.(*llm.LLM); !ok {
			t.Errorf("Decider = %T, want *llm.LLM (foundations/decision/llm)", ports.Decider)
		}
	})

	t.Run("typesafe opens decision/typesafe", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "test-key")
		ports, err := decide.Open(config.Decide{Provider: "typesafe"}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, ok := ports.Decider.(*typesafe.TypeSafe); !ok {
			t.Errorf("Decider = %T, want *typesafe.TypeSafe (foundations/decision/typesafe)", ports.Decider)
		}
	})

	t.Run("a predicate is logged as predicate and a legacy noul row still reads", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		rec.Record("obligations", decision.Response{
			Provider: "typesafe", Model: "jev-1.13.0",
			Answers: map[string]decision.Answer{"q": {ID: "q", Kind: decision.KindPredicate, Noul: 0.95, Confidence: 0.95}},
		}, func(decision.Answer) bool { return true })
		var row struct {
			SchemaVersion int    `json:"schema_version"`
			Kind          string `json:"kind"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &row); err != nil {
			t.Fatalf("decoding the written row %q: %v", buf.String(), err)
		}
		if row.Kind != "predicate" || row.SchemaVersion != 2 {
			t.Errorf("written kind %q at schema_version %d, want %q at 2 (the kind's value changed)", row.Kind, row.SchemaVersion, "predicate")
		}

		legacy := `{"ts":"2026-09-21T10:00:00Z","site":"obligations","provider":"llm","kind":"noul","confidence":0.7,"latency_ms":600,"applied":true}` + "\n"
		m := console.DecideView(console.DecideInputs{LogPath: "decisions.jsonl", Log: append([]byte(legacy), buf.Bytes()...)}, console.DecideFilter{}, 1, 50)
		if len(m.Rows) != 2 {
			t.Fatalf("read %d rows, want 2 (the legacy noul row and the new predicate row)", len(m.Rows))
		}
		for _, r := range m.Rows {
			if r.Kind != string(decision.KindPredicate) {
				t.Errorf("row %s kind = %q, want %q (noul is the decode alias of predicate)", r.TS, r.Kind, decision.KindPredicate)
			}
		}
	})
}

// TestOpen_NoneAndLLM pins DP-2 (docs/plans/decision-port.md): "none" builds
// the kit's zero-configuration decision.None without ever calling lane;
// "llm" calls lane with cfg.Profile and wraps the returned client in
// llm.NewLLM; any other provider is refused by name; a lane failure
// propagates.
func TestOpen_NoneAndLLM(t *testing.T) {
	t.Run("none never calls lane", func(t *testing.T) {
		called := false
		lane := func(profile string) (llmclient.LLMClient, error) {
			called = true
			return nil, nil
		}
		ports, err := decide.Open(config.Decide{Provider: "none"}, lane, "test-model", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if called {
			t.Error("lane must not be called for provider \"none\"")
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Errorf("Decider = %T, want decision.None", ports.Decider)
		}
	})

	t.Run("empty provider defaults like none", func(t *testing.T) {
		ports, err := decide.Open(config.Decide{}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, ok := ports.Decider.(decision.None); !ok {
			t.Errorf("Decider = %T, want decision.None", ports.Decider)
		}
	})

	t.Run("llm calls lane with the configured profile", func(t *testing.T) {
		var gotProfile string
		client := llmclient.NewMockClient()
		lane := func(profile string) (llmclient.LLMClient, error) {
			gotProfile = profile
			return client, nil
		}
		ports, err := decide.Open(config.Decide{Provider: "llm", Profile: "work"}, lane, "test-model", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if gotProfile != "work" {
			t.Errorf("lane called with profile %q, want %q", gotProfile, "work")
		}
		l, ok := ports.Decider.(*llm.LLM)
		if !ok {
			t.Fatalf("Decider = %T, want *llm.LLM", ports.Decider)
		}
		if l.Name() != "llm" {
			t.Errorf("Name() = %q, want %q", l.Name(), "llm")
		}
		if ports.Cfg.Provider != "llm" {
			t.Errorf("Cfg = %+v, want it echoed back", ports.Cfg)
		}
	})

	t.Run("unknown provider refused by name", func(t *testing.T) {
		_, err := decide.Open(config.Decide{Provider: "jev"}, nil, "", nil)
		if err == nil {
			t.Fatal("expected an error for an unknown provider")
		}
		if !strings.Contains(err.Error(), "jev") {
			t.Errorf("error = %v, want it to name %q", err, "jev")
		}
	})

	t.Run("lane error propagates", func(t *testing.T) {
		wantErr := errors.New("boom: no credential")
		lane := func(profile string) (llmclient.LLMClient, error) { return nil, wantErr }
		_, err := decide.Open(config.Decide{Provider: "llm"}, lane, "test-model", nil)
		if err == nil || !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want a wrap of %v", err, wantErr)
		}
	})
}

// TestOpen_TypeSafe pins DP-16 (docs/plans/decision-port.md): provider
// "typesafe" builds the kit's TypeSafe Decider directly (no lane) over a
// key resolved by config.Decide.ResolveAPIKey; an empty key is an error
// (never a silent None — decide.Open never falls back on its own, only the
// cmd/fathom caller does, per I14); cfg.Endpoint is honored as the base URL
// a real Decide call hits.
func TestOpen_TypeSafe(t *testing.T) {
	t.Run("empty key is an error", func(t *testing.T) {
		_, err := decide.Open(config.Decide{Provider: "typesafe"}, nil, "", nil)
		if err == nil {
			t.Fatal("expected an error for a typesafe provider with no API key")
		}
		if !strings.Contains(err.Error(), "typesafe") {
			t.Errorf("error = %v, want it to name the typesafe provider", err)
		}
	})

	t.Run("key set via env builds a typesafe Decider", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "test-key")
		ports, err := decide.Open(config.Decide{Provider: "typesafe"}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Decider == nil || ports.Decider.Name() != "typesafe" {
			t.Errorf("Decider = %v, want Name() == %q", ports.Decider, "typesafe")
		}
		if ports.Cfg.Provider != "typesafe" {
			t.Errorf("Cfg = %+v, want it echoed back", ports.Cfg)
		}
	})

	t.Run("endpoint honored by a fake server on Decide", func(t *testing.T) {
		var gotPath, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		t.Setenv("TYPESAFE_TEST_KEY", "sekret")
		ports, err := decide.Open(config.Decide{Provider: "typesafe", Endpoint: srv.URL, APIKeyEnv: "TYPESAFE_TEST_KEY"}, nil, "", nil)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		resp, err := ports.Decider.Decide(context.Background(), decision.Request{
			State:     "ping",
			Questions: []decision.Question{{ID: "ping", Kind: decision.KindPredicate, Instructions: "The statement is a ping."}},
		})
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if _, ok := resp.Answers["ping"]; !ok {
			t.Errorf("Answers = %+v, want a \"ping\" answer", resp.Answers)
		}
		if gotPath != "/v1/systemone" {
			t.Errorf("request path = %q, want /v1/systemone (cfg.Endpoint %q honored)", gotPath, srv.URL)
		}
		if gotAuth != "Bearer sekret" {
			t.Errorf("Authorization = %q, want Bearer sekret", gotAuth)
		}
	})
}

// TestSiteAllowed_Allowlist pins CV-42 (docs/design/refactor-conservation.md
// §7, ADR-044 decision 8, I14): a site absent from cfg.Sites makes no call,
// whatever Provider is set to; the four site name constants this package
// exports are exactly what config.DefaultDecide.Sites lists by default, so
// an unconfigured [decide] block calls every site it always called before
// Sites existed.
func TestSiteAllowed_Allowlist(t *testing.T) {
	t.Run("every default site is allowed under the documented default", func(t *testing.T) {
		cfg := config.DefaultDecide
		for _, site := range decide.AllSites {
			if !decide.SiteAllowed(cfg, site) {
				t.Errorf("SiteAllowed(DefaultDecide, %q) = false, want true (unconfigured behaviour must stay unchanged)", site)
			}
		}
	})

	t.Run("a narrowed allowlist excludes every site not listed", func(t *testing.T) {
		cfg := config.Decide{Sites: []string{decide.SiteRoute, decide.SiteCertifySuggest}}
		if !decide.SiteAllowed(cfg, decide.SiteRoute) {
			t.Error("SiteAllowed(route) = false, want true (listed)")
		}
		if !decide.SiteAllowed(cfg, decide.SiteCertifySuggest) {
			t.Error("SiteAllowed(certify_suggest) = false, want true (listed)")
		}
		if decide.SiteAllowed(cfg, decide.SiteCompilePrefilter) {
			t.Error("SiteAllowed(compile_prefilter) = true, want false (not listed)")
		}
		if decide.SiteAllowed(cfg, decide.SiteObligationsRerank) {
			t.Error("SiteAllowed(obligations_rerank) = true, want false (not listed)")
		}
	})

	t.Run("assess is a known site (ADR-046): the priced lane speaks only through the port", func(t *testing.T) {
		if decide.SiteAssess != "assess" {
			t.Errorf("SiteAssess = %q, want \"assess\" (the [decide] sites vocabulary)", decide.SiteAssess)
		}
		found := false
		for _, s := range decide.AllSites {
			found = found || s == decide.SiteAssess
		}
		if !found {
			t.Error("AllSites does not list assess — doctor could never name it as blocked")
		}
		if decide.SiteAllowed(config.Decide{Sites: []string{decide.SiteRoute}}, decide.SiteAssess) {
			t.Error("a narrowed allowlist without assess must refuse it")
		}
	})

	t.Run("preflight is a known site (WI-18): task preflight's Score speaks only through the port", func(t *testing.T) {
		if decide.SitePreflight != "preflight" {
			t.Errorf("SitePreflight = %q, want \"preflight\" (the [decide] sites vocabulary)", decide.SitePreflight)
		}
		found := false
		for _, s := range decide.AllSites {
			found = found || s == decide.SitePreflight
		}
		if !found {
			t.Error("AllSites does not list preflight — doctor could never name it as blocked")
		}
		if decide.SiteAllowed(config.Decide{Sites: []string{decide.SiteRoute}}, decide.SitePreflight) {
			t.Error("a narrowed allowlist without preflight must refuse it")
		}
	})

	t.Run("a nil or empty Sites allows nothing (fail-closed egress, never fail-open)", func(t *testing.T) {
		if decide.SiteAllowed(config.Decide{}, decide.SiteRoute) {
			t.Error("SiteAllowed with a zero-value Decide{} (nil Sites) = true, want false")
		}
		if decide.SiteAllowed(config.Decide{Sites: []string{}}, decide.SiteRoute) {
			t.Error("SiteAllowed with an explicit empty Sites = true, want false")
		}
	})

	t.Run("an unknown site name is never allowed, even when Sites is unrestricted-looking", func(t *testing.T) {
		cfg := config.Decide{Sites: []string{"route", "compile_prefilter", "obligations_rerank", "certify_suggest"}}
		if decide.SiteAllowed(cfg, "not_a_real_site") {
			t.Error("SiteAllowed(\"not_a_real_site\") = true, want false")
		}
	})
}
