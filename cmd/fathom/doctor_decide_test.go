package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// fakePingerClient is a minimal llmclient.LLMClient that also implements
// llmclient.Pinger, so checkDecisionProvider's ping branch runs (and
// succeeds) without a real credential or network call.
type fakePingerClient struct {
	pingErr error
}

func (fakePingerClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	return llmclient.Response{Text: "ok"}, nil
}

func (fakePingerClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	ch := make(chan llmclient.StreamChunk)
	close(ch)
	return ch, nil
}

func (f fakePingerClient) Ping(ctx context.Context) error { return f.pingErr }

var (
	_ llmclient.LLMClient = fakePingerClient{}
	_ llmclient.Pinger    = fakePingerClient{}
)

// TestDoctorDecisionProvider pins DP-4 (docs/plans/decision-port.md):
// provider "none" (the default, absent config included) is skipped naming
// why; provider "llm" with a lane that pings successfully is a pass; an
// unknown provider value is a fail naming it. The "unknown provider" case
// is driven against decisionProviderVerdict directly — reaching it through
// a real config.toml is impossible by construction, since #Decide's
// provider enum already refuses that value at config.Load.
func TestDoctorDecisionProvider(t *testing.T) {
	t.Run("none is skipped", func(t *testing.T) {
		root := t.TempDir()
		build := func(config.Profile) (llmclient.LLMClient, error) {
			t.Fatal("build must not be called for provider none")
			return nil, nil
		}
		got := checkDecisionProvider(root, build)
		if got.Status != doctorSkipped {
			t.Fatalf("Status = %v, want skipped: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "none") {
			t.Errorf("Reason = %q, want it to name provider none", got.Reason)
		}
	})

	t.Run("llm with a fake lane pings and passes", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		toml := `[decide]
provider = "llm"
profile  = "work"

[profiles.work]
provider = "claude-cli"
`
		if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}

		var gotProfile config.Profile
		build := func(p config.Profile) (llmclient.LLMClient, error) {
			gotProfile = p
			return fakePingerClient{}, nil
		}
		got := checkDecisionProvider(root, build)
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		if gotProfile.Provider != "claude-cli" {
			t.Errorf("build called with profile %+v, want the [profiles.work] profile", gotProfile)
		}
		if !strings.Contains(got.Reason, "work") {
			t.Errorf("Reason = %q, want it to name the profile", got.Reason)
		}
	})

	t.Run("unknown provider fails naming it", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "jev"}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "jev") {
			t.Errorf("Reason = %q, want it to name %q", got.Reason, "jev")
		}
	})
}

// TestDoctorDecisionProvider_TypeSafe pins DP-17: provider "typesafe" with
// no resolvable key is a FAIL naming the missing key; a 401 from a real
// ping is a FAIL naming the endpoint; a 200 System One envelope is a PASS
// naming the model and endpoint. decide.Open is not exercised here — this
// drives decisionProviderVerdict directly, the same way the "unknown
// provider" case above does, since the ping path needs a real (fake)
// server, not a lane seam.
func TestDoctorDecisionProvider_TypeSafe(t *testing.T) {
	t.Run("no key fails", func(t *testing.T) {
		t.Setenv("TYPESAFE_API_KEY", "")
		os.Unsetenv("TYPESAFE_API_KEY")
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "typesafe"}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "no API key") {
			t.Errorf("Reason = %q, want it to name the missing key", got.Reason)
		}
	})

	t.Run("401 ping failure fails naming the endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "bad-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail {
			t.Fatalf("Status = %v, want fail: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, srv.URL) {
			t.Errorf("Reason = %q, want it to name the endpoint %q", got.Reason, srv.URL)
		}
		if strings.Contains(got.Reason, "bad-key") {
			t.Errorf("Reason = %q, must never carry the key", got.Reason)
		}
	})

	t.Run("200 envelope passes naming model and endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, Model: "jev-latest", APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "good-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		if !strings.Contains(got.Reason, "jev-latest") || !strings.Contains(got.Reason, srv.URL) {
			t.Errorf("Reason = %q, want it to name the model and endpoint", got.Reason)
		}
	})

	// ADR-044 decision 8 (I14): decision_provider's own Reason names any
	// [decide] sites a working provider is still excluded from — a
	// narrowed allowlist is disclosed, not merely enforced silently.
	t.Run("a narrowed sites allowlist is named in the passing Reason", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, Model: "jev-latest", APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
			Sites: []string{decide.SiteRoute},
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "good-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		for _, site := range []string{decide.SiteCompilePrefilter, decide.SiteObligationsRerank, decide.SiteCertifySuggest} {
			if !strings.Contains(got.Reason, site) {
				t.Errorf("Reason = %q, want it to name the excluded site %q", got.Reason, site)
			}
		}
		if strings.Contains(got.Reason, "sites off") && strings.Contains(got.Reason, decide.SiteRoute+",") {
			t.Errorf("Reason = %q, must not list the ALLOWED site %q among the blocked ones", got.Reason, decide.SiteRoute)
		}
	})

	// The default (unconfigured) allowlist excludes nothing, so the
	// Reason line is unchanged from before Sites existed.
	t.Run("an unnarrowed allowlist adds nothing to the Reason", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":2}}`))
		}))
		defer srv.Close()

		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "typesafe", Endpoint: srv.URL, Model: "jev-latest", APIKeyEnv: "FATHOM_TEST_TYPESAFE_KEY",
			Sites: decide.AllSites,
		}}}
		t.Setenv("FATHOM_TEST_TYPESAFE_KEY", "good-key")
		got := decisionProviderVerdict(loaded, nil, "")
		if strings.Contains(got.Reason, "sites off") {
			t.Errorf("Reason = %q, want no \"sites off\" note when every site is allowed", got.Reason)
		}
	})
}

// doctorKey stands in for a provider key in the doctor tests: it may reach
// the fake server's Authorization header, never a Reason or a Remedy.
const doctorKey = "doctor-key-3b9e-not-a-real-key"

// openAIPingServer answers OpenAI's Decisions wire for the doctor's ping.
func openAIPingServer(t *testing.T, gotPath *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer "+doctorKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"ping","probability":0.97}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func doctorKeyFile(t *testing.T, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(doctorKey), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertNoKey(t *testing.T, got doctorCheck) {
	t.Helper()
	if strings.Contains(got.Reason, doctorKey) || strings.Contains(got.Remedy, doctorKey) {
		t.Errorf("the row carries the key: %+v", got)
	}
}

// TestDoctorDecisionProvider_Remote pins DP-34 (docs/plans/decision-port.md
// section H): for provider openai and for typesafe over openrouter the row
// names the provider, model, transport and the key's SOURCE and whether it
// resolves (a file present and private, a variable set), never the key,
// and says the thresholds wait on a calibration reading.
func TestDoctorDecisionProvider_Remote(t *testing.T) {
	for _, v := range []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "TYPESAFE_API_KEY"} {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}

	t.Run("openai with a private key file passes naming every part", func(t *testing.T) {
		var path string
		srv := openAIPingServer(t, &path)
		keyFile := doctorKeyFile(t, 0o600)
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "openai", BaseURL: srv.URL, APIKeyFile: keyFile}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		for _, want := range []string{"provider=openai", "model=gpt-6-luna", "transport=openai", "key file \"" + keyFile + "\" (present, mode 0600)", "calibration reading"} {
			if !strings.Contains(got.Reason, want) {
				t.Errorf("Reason = %q, want it to contain %q", got.Reason, want)
			}
		}
		if path != "/decisions" {
			t.Errorf("ping path = %q, want /decisions", path)
		}
		assertNoKey(t, got)
	})

	t.Run("a key file others can read fails naming chmod 600", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "openai", APIKeyFile: doctorKeyFile(t, 0o644)}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail || !strings.Contains(got.Reason+got.Remedy, "chmod 600") {
			t.Fatalf("got %+v, want a FAIL naming chmod 600", got)
		}
		assertNoKey(t, got)
	})

	t.Run("an unset variable fails naming it and the daemon's environment", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "openai"}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail || !strings.Contains(got.Reason, `environment variable "OPENAI_API_KEY"`) {
			t.Fatalf("got %+v, want a FAIL naming OPENAI_API_KEY", got)
		}
		if !strings.Contains(got.Remedy, "api_key_file") || !strings.Contains(got.Remedy, "launchd") {
			t.Errorf("Remedy = %q, want it to point the launchd daemon at api_key_file", got.Remedy)
		}
	})

	t.Run("typesafe over openrouter names the transport and the router's path", func(t *testing.T) {
		var path, auth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path, auth = r.URL.Path, r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"typesafe/jev-1.13-20260917","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{"input_tokens":10,"output_tokens":1}}`))
		}))
		defer srv.Close()
		t.Setenv("OPENROUTER_API_KEY", doctorKey)
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{Provider: "typesafe", Transport: "openrouter", BaseURL: srv.URL}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorPass {
			t.Fatalf("Status = %v, want pass: %+v", got.Status, got)
		}
		for _, want := range []string{"provider=typesafe", "model=typesafe/jev-1.13", "transport=openrouter", srv.URL + "/api/alpha/decisions", `environment variable "OPENROUTER_API_KEY" (set)`} {
			if !strings.Contains(got.Reason, want) {
				t.Errorf("Reason = %q, want it to contain %q", got.Reason, want)
			}
		}
		if path != "/api/alpha/decisions" || auth != "Bearer "+doctorKey {
			t.Errorf("ping path %q auth ok=%v, want the router path with the key", path, auth == "Bearer "+doctorKey)
		}
		assertNoKey(t, got)
	})

	t.Run("a repository base_url off the provider's host fails before any call", func(t *testing.T) {
		loaded := config.Loaded{Config: config.Config{Decide: config.Decide{
			Provider: "openai", BaseURL: "https://collector.example/v1", BaseURLSource: config.SourceProject, APIKeyFile: doctorKeyFile(t, 0o600),
		}}}
		got := decisionProviderVerdict(loaded, nil, "")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "~/.fathom/config.toml") {
			t.Fatalf("got %+v, want a FAIL naming ~/.fathom/config.toml", got)
		}
	})
}

// TestDecideConfigOf_DropsCredentialsFromTheBaseURL: `fathom decide log` and
// the console's /decide page show the [decide] base URL without any user
// information in it, as the doctor's endpoint does (CheckRemote refuses such
// a base URL; no surface repeats the credential).
func TestDecideConfigOf_DropsCredentialsFromTheBaseURL(t *testing.T) {
	const secret = "sentinel-0d7f-credential"
	for _, d := range []config.Decide{
		{Provider: "openai", BaseURL: "https://user:" + secret + "@api.openai.com/v1"},
		{Provider: "typesafe", Endpoint: "https://" + secret + "@api.typesafe.ai"},
	} {
		got := decideConfigOf(config.Loaded{Config: config.Config{Decide: d}}, nil).Endpoint
		if strings.Contains(got, secret) {
			t.Errorf("Endpoint = %q, repeats the credential", got)
		}
		if !strings.Contains(got, "https://api.") {
			t.Errorf("Endpoint = %q, want the host kept", got)
		}
	}
}
