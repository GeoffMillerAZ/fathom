package typesafe_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
)

// keyFile writes key to a file with mode in a fresh directory and returns
// its path.
func keyFile(t *testing.T, key string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// predicateRequest is a one-question Request.
func predicateRequest() decision.Request {
	return decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindPredicate, Instructions: "is it?"}}}
}

const okReply = `{"model":"jev-1.13.0","answers":{"q1":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}`

func TestTypeSafe_KeySources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not map onto Windows ACLs")
	}
	const fromFile, fromEnv = "fake-file-key", "fake-env-key"
	t.Setenv("DECISION_TYPESAFE_TEST_KEY", fromEnv)

	cases := []struct {
		name     string
		cfg      func(t *testing.T) typesafe.TypeSafeConfig
		wantAuth string
		wantErr  []string
	}{
		{"a 0600 key file is sent", func(t *testing.T) typesafe.TypeSafeConfig {
			return typesafe.TypeSafeConfig{APIKeyFile: keyFile(t, fromFile, 0o600)}
		}, "Bearer " + fromFile, nil},
		{"a key file wins over a variable", func(t *testing.T) typesafe.TypeSafeConfig {
			return typesafe.TypeSafeConfig{APIKeyFile: keyFile(t, fromFile, 0o600), APIKeyEnv: "DECISION_TYPESAFE_TEST_KEY"}
		}, "Bearer " + fromFile, nil},
		{"a variable is sent", func(*testing.T) typesafe.TypeSafeConfig {
			return typesafe.TypeSafeConfig{APIKeyEnv: "DECISION_TYPESAFE_TEST_KEY"}
		}, "Bearer " + fromEnv, nil},
		{"a 0644 key file is refused before any call", func(t *testing.T) typesafe.TypeSafeConfig {
			return typesafe.TypeSafeConfig{APIKeyFile: keyFile(t, fromFile, 0o644)}
		}, "", []string{"key file", "chmod 600"}},
		{"an unset variable is a configuration error naming it", func(*testing.T) typesafe.TypeSafeConfig {
			return typesafe.TypeSafeConfig{APIKeyEnv: "DECISION_TYPESAFE_TEST_UNSET"}
		}, "", []string{`"DECISION_TYPESAFE_TEST_UNSET" is not set`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cap := &typesafeCapture{body: okReply}
			srv := httptest.NewServer(cap.handler())
			defer srv.Close()
			cfg := c.cfg(t)
			cfg.BaseURL = srv.URL
			ts, err := typesafe.NewTypeSafe(cfg)
			if err != nil {
				t.Fatalf("NewTypeSafe() error = %v; a key source is read at first use, not at construction", err)
			}
			_, err = ts.Decide(context.Background(), predicateRequest())
			calls := cap.calls()
			if c.wantErr != nil {
				if err == nil {
					t.Fatal("Decide() error = nil")
				}
				for _, w := range c.wantErr {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error %q does not contain %q", err, w)
					}
				}
				if strings.Contains(err.Error(), fromFile) || strings.Contains(err.Error(), fromEnv) {
					t.Errorf("error leaks a key: %q", err)
				}
				if len(calls) != 0 {
					t.Errorf("%d call(s) reached the provider without a key", len(calls))
				}
				return
			}
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}
			if len(calls) != 1 || calls[0].auth != c.wantAuth {
				t.Errorf("calls = %d, Authorization matches = %v; want one call with the expected source's key", len(calls), len(calls) == 1 && calls[0].auth == c.wantAuth)
			}
		})
	}

	t.Run("no key source is a construction error", func(t *testing.T) {
		_, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{BaseURL: "http://127.0.0.1:1"})
		if err == nil || !strings.Contains(err.Error(), "APIKeyFile or APIKeyEnv") {
			t.Errorf("NewTypeSafe() error = %v, want one naming the key fields", err)
		}
	})
}

func TestTypeSafe_ErrorRedactsEchoedAuthorization(t *testing.T) {
	const key = "fake-key-0001"
	cap := &typesafeCapture{status: http.StatusUnauthorized, body: `{"error":{"code":401,"message":"bad header Authorization: Bearer ` + key + ` for key ` + key + `"}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()
	ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: key, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Decide(context.Background(), predicateRequest())
	var tsErr *typesafe.TypeSafeError
	if !errors.As(err, &tsErr) || tsErr.Status != http.StatusUnauthorized {
		t.Fatalf("error = %v, want a 401 *TypeSafeError", err)
	}
	if strings.Contains(err.Error(), key) || strings.Contains(tsErr.Message, key) {
		t.Errorf("error text echoes the key: %q", err)
	}
	if !strings.Contains(tsErr.Message, "[REDACTED]") || !strings.Contains(tsErr.Message, "bad header") {
		t.Errorf("Message = %q, want the provider's message with the key redacted", tsErr.Message)
	}
}

func TestTypeSafe_OpenRouterTransport(t *testing.T) {
	// The reply is OpenRouter's documented Decisions response shape: extra
	// id, provider and usage.cost fields, a score with a legend.
	const reply = `{
  "id": "gen-dec-1",
  "model": "typesafe/jev-1.13-20260917",
  "provider": "TypeSafe",
  "answers": {
    "is_bug": {"type": "noul", "noul": 0.96},
    "team": {"type": "choice", "choice": "payments", "confidence": 0.75, "probabilities": {"account": 0, "frontend": 0.16, "payments": 0.84}},
    "urgency": {"type": "score", "score": 1.99, "confidence": 0.99, "probabilities": {"0": 0, "1": 0.01, "2": 0.99}, "legend": {"0": "later", "1": "this week", "2": "now"}}
  },
  "usage": {"input_tokens": 476, "output_tokens": 70, "cost": 0.000019992}
}`
	cap := &typesafeCapture{body: reply}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: "test-key", Transport: typesafe.TransportOpenRouter, BaseURL: srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ts.Name(); got != "typesafe-openrouter" {
		t.Errorf("Name() = %q, want typesafe-openrouter", got)
	}
	req := decision.Request{State: "My checkout page is blank after I click Pay.", Questions: []decision.Question{
		{ID: "is_bug", Kind: decision.KindPredicate, TrueCriteria: "broken behavior", FalseCriteria: "a question"},
		{ID: "team", Kind: decision.KindChoice, Instructions: "Which team owns it?", Options: []decision.Option{{Name: "account"}, {Name: "frontend"}, {Name: "payments", Criteria: "checkout and billing"}}},
		{ID: "urgency", Kind: decision.KindScore, Instructions: "How urgent?", Levels: []string{"later", "this week", "now"}},
	}}
	resp, err := ts.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	calls := cap.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if calls[0].path != typesafe.DefaultOpenRouterPath {
		t.Errorf("path = %q, want %q", calls[0].path, typesafe.DefaultOpenRouterPath)
	}
	if calls[0].auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want the bearer key", calls[0].auth)
	}
	if want := readGolden(t, "testdata/openrouter_request.golden"); calls[0].body != want {
		t.Errorf("request body does not match testdata/openrouter_request.golden\ngot:\n%s\nwant:\n%s", calls[0].body, want)
	}

	if resp.Provider != "typesafe-openrouter" || resp.Model != "typesafe/jev-1.13-20260917" {
		t.Errorf("Response labeled %q/%q", resp.Provider, resp.Model)
	}
	if len(resp.Invalid) != 0 || len(resp.Answers) != 3 {
		t.Fatalf("Response = %+v, want three answers", resp)
	}
	if a := resp.Answers["is_bug"]; a.Noul != 0.96 || math.Abs(a.Confidence-0.96) > 1e-9 {
		t.Errorf("is_bug = %+v", a)
	}
	if a := resp.Answers["team"]; a.Choice != "payments" || math.Abs(a.Confidence-decision.Confidence(a.Probabilities)) > 1e-9 || a.Confidence == 0.75 {
		t.Errorf("team = %+v, want payments with the package's own confidence, not the wire's 0.75", a)
	}
	if a := resp.Answers["urgency"]; math.Abs(a.Score-1.99) > 1e-9 || a.Probabilities["2"] != 0.99 || a.Confidence == 0.99 {
		t.Errorf("urgency = %+v, want score 1.99 with its level distribution and the package's own confidence", a)
	}
	if resp.Usage.InputTokens != 476 || resp.Usage.OutputTokens != 70 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestTypeSafe_TransportDefaultsAndRefusals(t *testing.T) {
	for _, c := range []struct {
		name    string
		cfg     typesafe.TypeSafeConfig
		wantErr string
	}{
		{"an unknown transport", typesafe.TypeSafeConfig{APIKey: "k", Transport: "carrier-pigeon"}, `Transport "carrier-pigeon" is unknown`},
		{"a relative path", typesafe.TypeSafeConfig{APIKey: "k", Path: "api/alpha/decisions"}, `must begin with "/"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := typesafe.NewTypeSafe(c.cfg); err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("NewTypeSafe() error = %v, want %q", err, c.wantErr)
			}
		})
	}

	t.Run("an explicit path and model override the transport's", func(t *testing.T) {
		cap := &typesafeCapture{body: okReply}
		srv := httptest.NewServer(cap.handler())
		defer srv.Close()
		ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: "k", Transport: typesafe.TransportOpenRouter, BaseURL: srv.URL, Path: "/api/v1/systemone", Model: "jev-1.13"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ts.Decide(context.Background(), predicateRequest()); err != nil {
			t.Fatal(err)
		}
		calls := cap.calls()
		if len(calls) != 1 || calls[0].path != "/api/v1/systemone" || !strings.Contains(calls[0].body, `"model": "jev-1.13"`) {
			t.Errorf("calls = %+v, want the overridden path and model", calls)
		}
	})

	t.Run("the default transport keeps its name, path and model", func(t *testing.T) {
		ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: "k", Transport: typesafe.TransportTypeSafe})
		if err != nil || ts.Name() != "typesafe" {
			t.Errorf("NewTypeSafe() = %v, %v; want the typesafe transport", ts, err)
		}
	})
}

func TestTypeSafe_ImagesAreRefusedWithoutACall(t *testing.T) {
	cap := &typesafeCapture{body: okReply}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()
	ts, err := typesafe.NewTypeSafe(typesafe.TypeSafeConfig{APIKey: "k", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	req := predicateRequest()
	req.Images = []decision.Image{{MediaType: "image/png", Data: []byte("png")}}
	resp, err := ts.Decide(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(cap.calls()) != 0 || len(resp.Answers) != 0 || !strings.Contains(resp.Invalid["q1"], "takes no image input") {
		t.Errorf("Response = %+v after %d call(s), want q1 invalid and no call", resp, len(cap.calls()))
	}
}
