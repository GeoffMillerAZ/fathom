package decide_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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

// TestSiteAllowed_OptInSitesRefusedUnderDefault pins JF-4
// (docs/design/judgment-foundations.md §4, R2): an opt-in site is refused
// under the default allowlist even with a provider set, refused when only a
// user-global config.toml names it, and allowed only when the repository's
// own .fathom/config.toml (the project layer) names it in [decide] sites.
func TestSiteAllowed_OptInSitesRefusedUnderDefault(t *testing.T) {
	optIn := []string{"triage_failure", "ask", "explore", "compact_advice", "judge_failure"}
	for _, want := range []string{"triage_failure", "ask", "explore", "compact_advice", "judge_*"} {
		found := false
		for _, s := range decide.OptInSites {
			found = found || s == want
		}
		if !found {
			t.Errorf("OptInSites = %v, want it to list %q", decide.OptInSites, want)
		}
	}
	for _, site := range decide.AllSites {
		if decide.IsOptInSite(site) {
			t.Errorf("AllSites lists %q, an opt-in name: opt-in sites have no call site yet (R2) and never join the default list", site)
		}
	}

	t.Run("the default refuses every opt-in site, even with a provider set", func(t *testing.T) {
		cfg := config.DefaultDecide
		cfg.Provider = "typesafe"
		for _, site := range optIn {
			if !decide.IsOptInSite(site) {
				t.Errorf("IsOptInSite(%q) = false, want true", site)
			}
			if decide.SiteAllowed(cfg, site) {
				t.Errorf("SiteAllowed(DefaultDecide with provider typesafe, %q) = true, want false (opt-in)", site)
			}
		}
	})

	sitesTOML := "[decide]\nprovider = \"typesafe\"\nsites = [\"compile_prefilter\", \"triage_failure\", \"ask\", \"explore\", \"compact_advice\", \"judge_failure\"]\n"

	t.Run("a user-global config.toml cannot opt a repository in", func(t *testing.T) {
		dir := t.TempDir()
		user := filepath.Join(dir, "user.toml")
		if err := os.WriteFile(user, []byte(sitesTOML), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := config.Load(user, filepath.Join(dir, "absent", "config.toml"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, site := range optIn {
			if decide.SiteAllowed(loaded.Config.Decide, site) {
				t.Errorf("SiteAllowed(%q) = true with only the user layer naming it, want false (the repository's own [decide] sites must name it)", site)
			}
		}
		if !decide.SiteAllowed(loaded.Config.Decide, decide.SiteCompilePrefilter) {
			t.Error("a user layer still narrows an ordinary site: compile_prefilter refused, want allowed")
		}
	})

	t.Run("the repository's own [decide] sites opts it in", func(t *testing.T) {
		dir := t.TempDir()
		project := filepath.Join(dir, ".fathom", "config.toml")
		if err := os.MkdirAll(filepath.Dir(project), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(project, []byte(sitesTOML), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := config.Load(filepath.Join(dir, "no-user.toml"), project)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, site := range optIn {
			if !decide.SiteAllowed(loaded.Config.Decide, site) {
				t.Errorf("SiteAllowed(%q) = false with the project layer naming it, want true", site)
			}
		}
		if decide.SiteAllowed(loaded.Config.Decide, "judge_other") {
			t.Error("SiteAllowed(judge_other) = true, want false: naming judge_failure opts in that one site, not the family")
		}
	})
}

// TestSiteAllowed_Allowlist pins CV-42 (docs/design/refactor-conservation.md
// §7, ADR-044 decision 8, I14) and JF-5 (judgment-foundations §4, §6): a
// site absent from cfg.Sites makes no call, whatever Provider is set to;
// under the documented default every site in AllSites minus OptInSites is
// allowed, except the one stated exclusion, route (section 6: off by
// default, its code kept), so the unconfigured behaviour of every other
// existing site is unchanged (I42).
func TestSiteAllowed_Allowlist(t *testing.T) {
	t.Run("AllSites minus OptInSites are allowed by default, route the one stated exclusion", func(t *testing.T) {
		cfg := config.DefaultDecide
		for _, site := range decide.AllSites {
			want := !decide.IsOptInSite(site) && !slices.Contains(decide.DefaultOffSites, site)
			if got := decide.SiteAllowed(cfg, site); got != want {
				t.Errorf("SiteAllowed(DefaultDecide, %q) = %v, want %v", site, got, want)
			}
		}
		if !slices.Equal(decide.DefaultOffSites, []string{decide.SiteRoute}) {
			t.Errorf("DefaultOffSites = %v, want exactly [route] (section 6)", decide.DefaultOffSites)
		}
		if decide.SiteAllowed(cfg, decide.SiteRoute) {
			t.Error("SiteAllowed(DefaultDecide, route) = true, want false (route is off by default)")
		}
		if !decide.SiteAllowed(config.Decide{Sites: []string{decide.SiteRoute}}, decide.SiteRoute) {
			t.Error("route named in [decide] sites is refused, want allowed: route is a stated exclusion, not an opt-in site")
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
