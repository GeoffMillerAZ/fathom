package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// newAzureTestClient spins up an httptest server around h and returns a
// Client configured for Azure mode, pointed at it. cfg.BaseURL and
// cfg.AzureDeployment (when cfg leaves it empty) are always set by this
// helper — Azure mode requires both, and exercising it is the point.
func newAzureTestClient(t *testing.T, h http.Handler, cfg Config) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	if cfg.AzureDeployment == "" {
		cfg.AzureDeployment = "test-deployment"
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "model-x"
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// TestAzureConfigValidation proves the Config validation rules from the
// design: deployment without version defaults; version without deployment,
// deployment without BaseURL, and an unrecognized AzureAuthScheme all fail
// construction; the zero-value Azure fields (the shape every vanilla test
// in this package uses) are untouched.
func TestAzureConfigValidation(t *testing.T) {
	t.Run("VersionWithoutDeploymentRejected", func(t *testing.T) {
		_, err := New(Config{AzureAPIVersion: "2024-10-21", DefaultModel: "gpt-4o"})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *InvalidRequestError", err)
		}
	})

	t.Run("DeploymentWithoutVersionDefaults", func(t *testing.T) {
		c, err := New(Config{
			AzureDeployment: "dep",
			BaseURL:         "https://example-resource.openai.azure.com",
			DefaultModel:    "gpt-4o",
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.azureAPIVersion != DefaultAzureAPIVersion {
			t.Errorf("azureAPIVersion = %q, want default %q", c.azureAPIVersion, DefaultAzureAPIVersion)
		}
	})

	t.Run("DeploymentWithoutBaseURLRejected", func(t *testing.T) {
		_, err := New(Config{AzureDeployment: "dep", DefaultModel: "gpt-4o"})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *InvalidRequestError", err)
		}
	})

	t.Run("InvalidAuthSchemeRejected", func(t *testing.T) {
		_, err := New(Config{
			AzureDeployment: "dep",
			BaseURL:         "https://example-resource.openai.azure.com",
			AzureAuthScheme: "bogus",
			DefaultModel:    "gpt-4o",
		})
		if err == nil {
			t.Fatal("want error for an unrecognized AzureAuthScheme, got nil")
		}
	})

	t.Run("ValidAuthSchemesAccepted", func(t *testing.T) {
		for _, scheme := range []string{"", AzureAuthSchemeAPIKey, AzureAuthSchemeBearer} {
			if _, err := New(Config{
				AzureDeployment: "dep",
				BaseURL:         "https://example-resource.openai.azure.com",
				AzureAuthScheme: scheme,
				DefaultModel:    "gpt-4o",
			}); err != nil {
				t.Errorf("AzureAuthScheme=%q: New: %v", scheme, err)
			}
		}
	})

	t.Run("VanillaModeUnaffected", func(t *testing.T) {
		// The exact Config{} shape every existing test in this package
		// constructs — zero-value Azure fields must never perturb it.
		if _, err := New(Config{DefaultModel: "gpt-4o"}); err != nil {
			t.Fatalf("New: %v", err)
		}
	})
}

// TestBuildEndpoint is a table-driven, white-box test of buildEndpoint and
// azureCompletionsPath: vanilla mode ignores the azure path entirely and
// reproduces the pre-Azure URL exactly; Azure mode appends the mandatory
// api-version query parameter and escapes the deployment name as a single
// path segment.
func TestBuildEndpoint(t *testing.T) {
	t.Run("VanillaIgnoresAzurePath", func(t *testing.T) {
		c, err := New(Config{BaseURL: "https://api.openai.com/v1", DefaultModel: "m"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if got, want := c.buildEndpoint(completionsPath, "/should/be/ignored"), "https://api.openai.com/v1"+completionsPath; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := c.buildEndpoint(modelsPath, "/should/be/ignored"), "https://api.openai.com/v1"+modelsPath; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("AzureCompletionsDefaultVersion", func(t *testing.T) {
		c, err := New(Config{BaseURL: "https://res.openai.azure.com", AzureDeployment: "dep-1", DefaultModel: "m"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		want := "https://res.openai.azure.com/openai/deployments/dep-1/chat/completions?api-version=" + DefaultAzureAPIVersion
		if got := c.buildEndpoint(completionsPath, c.azureCompletionsPath()); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("AzureCompletionsExplicitVersion", func(t *testing.T) {
		c, err := New(Config{
			BaseURL:         "https://res.openai.azure.com",
			AzureDeployment: "dep-1",
			AzureAPIVersion: "2025-03-01-preview",
			DefaultModel:    "m",
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		want := "https://res.openai.azure.com/openai/deployments/dep-1/chat/completions?api-version=2025-03-01-preview"
		if got := c.buildEndpoint(completionsPath, c.azureCompletionsPath()); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("AzureModels", func(t *testing.T) {
		c, err := New(Config{BaseURL: "https://res.openai.azure.com", AzureDeployment: "dep-1", DefaultModel: "m"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		want := "https://res.openai.azure.com/openai/models?api-version=" + DefaultAzureAPIVersion
		if got := c.buildEndpoint(modelsPath, "/openai/models"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("DeploymentNameEscaped", func(t *testing.T) {
		c, err := New(Config{BaseURL: "https://res.openai.azure.com", AzureDeployment: "team/gpt 4o", DefaultModel: "m"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		want := "/openai/deployments/team%2Fgpt%204o/chat/completions"
		if got := c.azureCompletionsPath(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestAzureRequestRouting proves the URLs the client actually sends over
// HTTP for Complete, Stream, and Ping in Azure mode, and that vanilla mode
// is unaffected.
func TestAzureRequestRouting(t *testing.T) {
	t.Run("CompleteURL", func(t *testing.T) {
		var gotPath, gotQuery string
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{APIKey: "k", AzureDeployment: "gpt-4o-dep", AzureAPIVersion: "2025-01-01-preview"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if want := "/openai/deployments/gpt-4o-dep/chat/completions"; gotPath != want {
			t.Errorf("path = %q, want %q", gotPath, want)
		}
		if want := "api-version=2025-01-01-preview"; gotQuery != want {
			t.Errorf("query = %q, want %q", gotQuery, want)
		}
	})

	t.Run("StreamURL", func(t *testing.T) {
		var gotPath, gotQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			f := w.(http.Flusher)
			streamPreamble(w, f)
			streamTextDeltas(t, w, f, "hi")
			streamFinish(w, f, "stop")
		}))
		t.Cleanup(srv.Close)
		c, err := New(Config{BaseURL: srv.URL, AzureDeployment: "gpt-4o-dep", DefaultModel: "model-x", APIKey: "k"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		drainStream(t, ch)
		if want := "/openai/deployments/gpt-4o-dep/chat/completions"; gotPath != want {
			t.Errorf("path = %q, want %q", gotPath, want)
		}
		if want := "api-version=" + DefaultAzureAPIVersion; gotQuery != want {
			t.Errorf("query = %q, want %q", gotQuery, want)
		}
	})

	t.Run("PingURL", func(t *testing.T) {
		var gotPath, gotQuery string
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		}), Config{APIKey: "k"})
		if err := c.Ping(context.Background()); err != nil {
			t.Fatalf("Ping: %v", err)
		}
		if want := "/openai/models"; gotPath != want {
			t.Errorf("path = %q, want %q", gotPath, want)
		}
		if want := "api-version=" + DefaultAzureAPIVersion; gotQuery != want {
			t.Errorf("query = %q, want %q", gotQuery, want)
		}
	})

	t.Run("VanillaModeURLsUnaffected", func(t *testing.T) {
		var completePath, completeQuery, pingPath, pingQuery string
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				completePath, completeQuery = r.URL.Path, r.URL.RawQuery
				writeJSON(w, textResponseBody(t, "ok"))
				return
			}
			pingPath, pingQuery = r.URL.Path, r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
		}), Config{})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if err := c.Ping(context.Background()); err != nil {
			t.Fatalf("Ping: %v", err)
		}
		if completePath != "/v1/chat/completions" || completeQuery != "" {
			t.Errorf("complete path/query = %q %q, want %q with no query", completePath, completeQuery, "/v1/chat/completions")
		}
		if pingPath != "/v1/models" || pingQuery != "" {
			t.Errorf("ping path/query = %q %q, want %q with no query", pingPath, pingQuery, "/v1/models")
		}
	})
}

// TestAzureAuthHeaders proves the credential header choice: api-key by
// default in Azure mode, Authorization: Bearer for Entra-ID passthrough
// when configured, no header when the credential is empty, and — as a
// direct check on the "vanilla path is byte-preserved" claim — that
// vanilla mode still always sends Bearer regardless of these new fields.
func TestAzureAuthHeaders(t *testing.T) {
	t.Run("DefaultAPIKeyScheme", func(t *testing.T) {
		var gotAPIKey string
		var authPresent bool
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAPIKey = r.Header.Get("api-key")
			authPresent = r.Header.Get("Authorization") != ""
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{APIKey: "az-key-1"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if gotAPIKey != "az-key-1" {
			t.Errorf("api-key header = %q, want %q", gotAPIKey, "az-key-1")
		}
		if authPresent {
			t.Error("Authorization header must be absent in the api-key scheme")
		}
	})

	t.Run("BearerSchemeForEntraPassthrough", func(t *testing.T) {
		var gotAuth string
		var apiKeyPresent bool
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			apiKeyPresent = r.Header.Get("api-key") != ""
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{APIKey: "entra-token-1", AzureAuthScheme: AzureAuthSchemeBearer})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if gotAuth != "Bearer entra-token-1" {
			t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer entra-token-1")
		}
		if apiKeyPresent {
			t.Error("api-key header must be absent in the bearer scheme")
		}
	})

	t.Run("EmptyKeyNoHeader", func(t *testing.T) {
		var apiKeyPresent, authPresent bool
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKeyPresent = r.Header.Get("api-key") != ""
			authPresent = r.Header.Get("Authorization") != ""
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if apiKeyPresent || authPresent {
			t.Error("no credential header should be set when APIKey is empty")
		}
	})

	t.Run("VanillaModeStillBearer", func(t *testing.T) {
		var gotAuth string
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{APIKey: "vanilla-key-1"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if gotAuth != "Bearer vanilla-key-1" {
			t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer vanilla-key-1")
		}
	})
}

// TestAzureErrorEnvelopeConformance proves that Azure's error envelope maps
// through the exact same typed-error mapping as vanilla OpenAI, one fixture
// per mapped status class, while confirming each request actually rode the
// Azure endpoint shape (deployment path + api-version).
func TestAzureErrorEnvelopeConformance(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		retryAfter string
		check      func(t *testing.T, err error)
	}{
		{
			name:   "400InvalidRequest",
			status: 400,
			body:   `{"error":{"message":"deployment does not support this parameter","type":"invalid_request_error"}}`,
			check: func(t *testing.T, err error) {
				var ire *llmclient.InvalidRequestError
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
				}
			},
		},
		{
			name:   "401AuthError",
			status: 401,
			body:   `{"error":{"message":"Access denied due to invalid subscription key","type":"invalid_api_key"}}`,
			check: func(t *testing.T, err error) {
				var ae *llmclient.AuthError
				if !errors.As(err, &ae) {
					t.Fatalf("err = %T %v, want *AuthError", err, err)
				}
			},
		},
		{
			name:       "429RateLimit",
			status:     429,
			body:       `{"error":{"message":"Requests to the ChatCompletions_Create Operation have exceeded rate limit","type":"rate_limit_error"}}`,
			retryAfter: "20",
			check: func(t *testing.T, err error) {
				var rle *llmclient.RateLimitError
				if !errors.As(err, &rle) {
					t.Fatalf("err = %T %v, want *RateLimitError", err, err)
				}
				if rle.RetryAfter != 20*time.Second {
					t.Errorf("RetryAfter = %v, want 20s", rle.RetryAfter)
				}
			},
		},
		{
			name:   "500Server",
			status: 500,
			body:   `{"error":{"message":"internal server error","type":"server_error"}}`,
			check: func(t *testing.T, err error) {
				var se *llmclient.ServerError
				if !errors.As(err, &se) {
					t.Fatalf("err = %T %v, want *ServerError", err, err)
				}
			},
		},
		{
			name:   "503Server",
			status: 503,
			body:   `{"error":{"message":"service unavailable","type":"server_error"}}`,
			check: func(t *testing.T, err error) {
				var se *llmclient.ServerError
				if !errors.As(err, &se) {
					t.Fatalf("err = %T %v, want *ServerError", err, err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotQuery string
			c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}), Config{APIKey: "az-conform-key"})
			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
			tc.check(t, err)
			if !strings.Contains(gotPath, "/openai/deployments/") {
				t.Errorf("request did not route through the Azure deployment path: %q", gotPath)
			}
			if !strings.Contains(gotQuery, "api-version=") {
				t.Errorf("request missing the mandatory api-version query param: %q", gotQuery)
			}
		})
	}
}

// TestAzureAPIKeyNeverInErrors proves key redaction covers the credential
// regardless of which Azure header carried it — api-key or Bearer — since
// redact() matches on the resolved credential string, not the header name.
func TestAzureAPIKeyNeverInErrors(t *testing.T) {
	const sentinel = "az-key-SENTINEL-never-leak-4321"

	assertClean := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("want error, got nil")
		}
		for e := err; e != nil; e = errors.Unwrap(e) {
			if strings.Contains(e.Error(), sentinel) {
				t.Fatalf("credential leaked into error output: %v", e)
			}
		}
	}

	t.Run("APIKeyScheme", func(t *testing.T) {
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(401)
			fmt.Fprintf(w, `{"error":{"message":"api-key %s was rejected","type":"invalid_api_key"}}`, sentinel)
		}), Config{APIKey: sentinel})
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		assertClean(t, err)
	})

	t.Run("BearerScheme", func(t *testing.T) {
		c := newAzureTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", "application/json")
			w.WriteHeader(401)
			fmt.Fprintf(w, `{"error":{"message":"bearer token %s was rejected","type":"invalid_api_key"}}`, sentinel)
		}), Config{APIKey: sentinel, AzureAuthScheme: AzureAuthSchemeBearer})
		_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		assertClean(t, err)
	})
}

// TestAzureStreaming proves streaming end-to-end through the Azure path: a
// real SSE response is consumed via the existing stream test harness while
// asserting the request that produced it used the Azure URL shape and
// api-key header.
func TestAzureStreaming(t *testing.T) {
	var gotPath, gotQuery, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotAPIKey = r.Header.Get("api-key")
		f := w.(http.Flusher)
		streamPreamble(w, f)
		streamTextDeltas(t, w, f, "hello from azure")
		streamFinish(w, f, "stop")
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{
		BaseURL:         srv.URL,
		AzureDeployment: "gpt-4o-dep",
		AzureAPIVersion: "2024-10-21",
		APIKey:          "az-stream-key",
		DefaultModel:    "model-x",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	chunks := drainStream(t, ch)

	var text string
	var stop llmclient.StopReason
	var usage *llmclient.Usage
	for _, ck := range chunks {
		text += ck.TextDelta
		if ck.StopReason != "" {
			stop = ck.StopReason
		}
		if ck.Usage != nil {
			usage = ck.Usage
		}
	}
	if text != "hello from azure" {
		t.Errorf("assembled text = %q, want %q", text, "hello from azure")
	}
	if stop != llmclient.StopEndTurn {
		t.Errorf("StopReason = %q, want %q", stop, llmclient.StopEndTurn)
	}
	if usage == nil || usage.InputTokens != 5 || usage.OutputTokens != 7 {
		t.Errorf("Usage = %+v, want {InputTokens:5 OutputTokens:7}", usage)
	}

	if want := "/openai/deployments/gpt-4o-dep/chat/completions"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if want := "api-version=2024-10-21"; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}
	if gotAPIKey != "az-stream-key" {
		t.Errorf("api-key header = %q, want %q", gotAPIKey, "az-stream-key")
	}
}
