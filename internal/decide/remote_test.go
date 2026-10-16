package decide_test

// remote_test.go — DP-31..DP-33 (docs/plans/decision-port.md section
// H): provider "openai" and Jev over OpenRouter open through the kit's
// adapters, the key is named by a file or a variable and reaches only the
// provider's own host, and a refusal degrades like any provider failure.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/typesafe"
)

// testKey stands in for a provider key: it must reach the fake server's
// Authorization header and nothing else.
const testKey = "test-key-6f1d2a-not-a-real-key"

// unsetProviderKeys makes the well-known key variables absent for one test,
// whatever the machine running it exports.
func unsetProviderKeys(t *testing.T) {
	t.Helper()
	for _, v := range []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "TYPESAFE_API_KEY"} {
		t.Setenv(v, "")
		if err := os.Unsetenv(v); err != nil {
			t.Fatal(err)
		}
	}
}

// writeKeyFile writes testKey to a file with mode perm and returns its path.
func writeKeyFile(t *testing.T, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(testKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

// capture is a fake provider: it records one request and answers body.
type capture struct {
	path, auth string
	body       map[string]any
	reply      string
}

func (c *capture) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.path, c.auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &c.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(c.reply))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func predicateRequest(ids ...string) decision.Request {
	req := decision.Request{State: "diff: internal/config/config.go"}
	for _, id := range ids {
		req.Questions = append(req.Questions, decision.Question{ID: id, Kind: decision.KindPredicate, Instructions: "The rule governs the edited file."})
	}
	return req
}

func TestOpen_OpenAI(t *testing.T) {
	t.Run("a key file reaches only the Authorization header of the provider's request", func(t *testing.T) {
		unsetProviderKeys(t)
		c := &capture{reply: `{"model":"gpt-6-luna","answers":[{"type":"predicate","name":"q1","probability":0.8}],"usage":{"input_tokens":12,"output_tokens":0}}`}
		srv := c.server(t)
		var disclose bytes.Buffer
		ports, err := decide.Open(config.Decide{Provider: "openai", BaseURL: srv.URL, APIKeyFile: writeKeyFile(t, 0o600)}, nil, "", nil, decide.WithDisclose(&disclose))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if ports.Decider.Name() != "openai" {
			t.Errorf("Decider.Name() = %q, want openai", ports.Decider.Name())
		}
		if _, ok := ports.Adapter.(*openai.OpenAI); !ok {
			t.Errorf("Adapter = %T, want *openai.OpenAI", ports.Adapter)
		}
		resp, err := ports.Decider.Decide(context.Background(), predicateRequest("q1"))
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if a, ok := resp.Answers["q1"]; !ok || a.Noul != 0.8 {
			t.Errorf("Answers = %+v, want q1 at 0.8", resp.Answers)
		}
		if c.path != "/decisions" || c.auth != "Bearer "+testKey {
			t.Errorf("request path %q auth %q, want /decisions with the file's key", c.path, c.auth)
		}
		if c.body["model"] != openai.DefaultOpenAIModel {
			t.Errorf("model on the wire = %v, want %s", c.body["model"], openai.DefaultOpenAIModel)
		}
		if disclose.Len() != 0 {
			t.Errorf("an answered call disclosed %q", disclose.String())
		}
	})

	t.Run("no key source names the variable it checked", func(t *testing.T) {
		unsetProviderKeys(t)
		_, err := decide.Open(config.Decide{Provider: "openai"}, nil, "", nil)
		if err == nil || !strings.Contains(err.Error(), `"OPENAI_API_KEY"`) || !strings.Contains(err.Error(), "openai") {
			t.Fatalf("Open err = %v, want it to name provider openai and OPENAI_API_KEY", err)
		}
	})

	t.Run("a key file others can read is refused naming the fix", func(t *testing.T) {
		unsetProviderKeys(t)
		_, err := decide.Open(config.Decide{Provider: "openai", APIKeyFile: writeKeyFile(t, 0o644)}, nil, "", nil)
		if err == nil || !strings.Contains(err.Error(), "chmod 600") {
			t.Fatalf("Open err = %v, want the chmod 600 fix", err)
		}
		if strings.Contains(err.Error(), testKey) {
			t.Errorf("Open err carries the key: %v", err)
		}
	})
}

func TestOpen_TypeSafeOverOpenRouter(t *testing.T) {
	unsetProviderKeys(t)
	c := &capture{reply: `{"model":"typesafe/jev-1.13-20260917","answers":{"q1":{"type":"noul","noul":0.7}},"usage":{"input_tokens":9,"output_tokens":1}}`}
	srv := c.server(t)
	t.Setenv("OPENROUTER_API_KEY", testKey)
	ports, err := decide.Open(config.Decide{Provider: "typesafe", Transport: "openrouter", BaseURL: srv.URL}, nil, "", nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if ports.Decider.Name() != "typesafe" {
		t.Errorf("Decider.Name() = %q, want typesafe", ports.Decider.Name())
	}
	if _, err := ports.Decider.Decide(context.Background(), predicateRequest("q1")); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if c.path != typesafe.DefaultOpenRouterPath || c.auth != "Bearer "+testKey {
		t.Errorf("request path %q auth %q, want %s with the OPENROUTER_API_KEY key", c.path, c.auth, typesafe.DefaultOpenRouterPath)
	}
	if c.body["model"] != typesafe.DefaultOpenRouterModel {
		t.Errorf("model on the wire = %v, want %s", c.body["model"], typesafe.DefaultOpenRouterModel)
	}
	if got := decide.ResolvedModel(ports.Cfg); got != typesafe.DefaultOpenRouterModel {
		t.Errorf("ResolvedModel = %q, want %s", got, typesafe.DefaultOpenRouterModel)
	}
}

// TestOpen_KeyStaysWithTheProvidersHost: a key goes to the provider's own
// host, a loopback host, or a host the user's own config names; a
// repository's config never redirects it, and plain http never carries it.
func TestOpen_KeyStaysWithTheProvidersHost(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfg     config.Decide
		wantErr string
	}{
		{"the provider's own host from the project layer", config.Decide{Provider: "openai", BaseURL: "https://api.openai.com/v1", BaseURLSource: config.SourceProject}, ""},
		{"another host from the project layer", config.Decide{Provider: "openai", BaseURL: "https://collector.example/v1", BaseURLSource: config.SourceProject}, "~/.fathom/config.toml"},
		{"another host through the older endpoint key", config.Decide{Provider: "typesafe", Endpoint: "https://collector.example", BaseURLSource: config.SourceProject}, "~/.fathom/config.toml"},
		{"another host from the user layer", config.Decide{Provider: "openai", BaseURL: "https://gateway.example/v1", BaseURLSource: config.SourceUser}, ""},
		{"another host from no known layer", config.Decide{Provider: "openai", BaseURL: "https://gateway.example/v1"}, "~/.fathom/config.toml"},
		{"openrouter's host for typesafe over openrouter", config.Decide{Provider: "typesafe", Transport: "openrouter", BaseURL: "https://openrouter.ai", BaseURLSource: config.SourceProject}, ""},
		{"openrouter's host for typesafe's own API", config.Decide{Provider: "typesafe", BaseURL: "https://openrouter.ai/api", BaseURLSource: config.SourceProject}, "~/.fathom/config.toml"},
		{"plain http off the machine", config.Decide{Provider: "openai", BaseURL: "http://gateway.example/v1", BaseURLSource: config.SourceUser}, "plain http"},
		{"a loopback host from the project layer", config.Decide{Provider: "openai", BaseURL: "http://127.0.0.1:9/v1", BaseURLSource: config.SourceProject}, ""},
		{"a transport the provider does not speak", config.Decide{Provider: "openai", Transport: "openrouter"}, "transport"},
		{"credentials inside the base URL", config.Decide{Provider: "openai", BaseURL: "https://user:" + testKey + "@api.openai.com/v1", BaseURLSource: config.SourceUser}, "carries credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsetProviderKeys(t)
			tc.cfg.APIKeyFile = writeKeyFile(t, 0o600)
			_, err := decide.Open(tc.cfg, nil, "", nil)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Open: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Open err = %v, want it to contain %q", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("Open err echoes a credential: %v", err)
			}
			if got := decide.ResolvedEndpoint(tc.cfg); strings.Contains(got, testKey) {
				t.Errorf("ResolvedEndpoint = %q, echoes a credential", got)
			}
		})
	}
}

// TestOpen_DeclinedDegrades: a refusal is a disclosed miss (JF-6): one line,
// and the site gets the no-provider signal, never a partial answer set and
// never a refusal read as false or zero.
func TestOpen_DeclinedDegrades(t *testing.T) {
	unsetProviderKeys(t)
	c := &capture{reply: `{"answers":[{"type":"predicate","name":"q1","probability":0.9},{"type":"refusal","name":"q2"}]}`}
	srv := c.server(t)
	var disclose bytes.Buffer
	ports, err := decide.Open(config.Decide{Provider: "openai", BaseURL: srv.URL, APIKeyFile: writeKeyFile(t, 0o600)}, nil, "", nil, decide.WithDisclose(&disclose))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	resp, err := ports.Decider.Decide(context.Background(), predicateRequest("q1", "q2"))
	if err == nil || !errors.Is(err, decision.ErrNoProvider) || !errors.Is(err, decision.ErrDeclined) {
		t.Fatalf("Decide err = %v, want one that is decision.ErrNoProvider and decision.ErrDeclined", err)
	}
	if len(resp.Answers) != 0 {
		t.Errorf("Answers = %+v, want none (the site takes its deterministic path)", resp.Answers)
	}
	lines := strings.Split(strings.TrimSpace(disclose.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "decision port degraded (declined)") || !strings.Contains(lines[0], "q2") {
		t.Errorf("disclosed %q, want exactly one declined line naming q2", disclose.String())
	}
}

// TestOpen_OpenAIFailuresNameTheirClass: the degrade line names why OpenAI
// answered nothing. The quota body is the live reply of 2026-10-09 (an
// account with no credit): a 429 that is not a rate limit, so the line
// must not say "rate limited".
func TestOpen_OpenAIFailuresNameTheirClass(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"quota exhausted", http.StatusTooManyRequests, `{"error":{"message":"You have no credits remaining.","type":"insufficient_quota","param":null,"code":"credit_balance_exhausted"}}`, "decision port degraded (quota exhausted)"},
		{"rate limited", http.StatusTooManyRequests, `{"error":{"message":"slow down","type":"requests","code":"rate_limit_exceeded"}}`, "decision port degraded (rate limited)"},
		{"unavailable", http.StatusServiceUnavailable, `busy`, "decision port degraded (unavailable)"},
		{"bad key", http.StatusUnauthorized, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`, "decision port degraded (error)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsetProviderKeys(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			var disclose bytes.Buffer
			ports, err := decide.Open(config.Decide{Provider: "openai", BaseURL: srv.URL, APIKeyFile: writeKeyFile(t, 0o600)}, nil, "", nil, decide.WithDisclose(&disclose))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			_, err = ports.Decider.Decide(context.Background(), predicateRequest("q1"))
			if !errors.Is(err, decision.ErrNoProvider) {
				t.Fatalf("Decide err = %v, want the no-provider signal", err)
			}
			lines := strings.Split(strings.TrimSpace(disclose.String()), "\n")
			if len(lines) != 1 || !strings.Contains(lines[0], tc.want) {
				t.Errorf("disclosed %q, want exactly one line saying %q", disclose.String(), tc.want)
			}
			if strings.Contains(disclose.String(), testKey) {
				t.Errorf("the line carries the key: %q", disclose.String())
			}
		})
	}
}
