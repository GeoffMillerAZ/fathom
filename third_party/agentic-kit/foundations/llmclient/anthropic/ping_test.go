package anthropic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// modelsListBody is a minimal /v1/models success body. Ping never parses it;
// it exists so the test server answers with realistic bytes.
const modelsListBody = `{"data":[{"type":"model","id":"model-x","display_name":"Model X"}],"has_more":false,"first_id":"model-x","last_id":"model-x"}`

func TestPingHappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotHeader http.Header
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotHeader = r.Header.Clone()
		writeJSON(w, modelsListBody)
	}), Config{})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/models" {
		t.Errorf("got %s %s, want GET /v1/models", gotMethod, gotPath)
	}
	if k := gotHeader.Get("x-api-key"); k != "test-key" {
		t.Errorf("x-api-key = %q", k)
	}
	if v := gotHeader.Get("anthropic-version"); v != "2023-06-01" {
		t.Errorf("anthropic-version = %q", v)
	}
	if b := gotHeader.Get("Content-Length"); b != "" && b != "0" {
		t.Errorf("Ping must send no body, got Content-Length %q", b)
	}
}

// TestPingBearerHeaders proves the probe authenticates exactly like
// Complete/Stream under the bearer scheme, including the anthropic-beta
// lines — a Ping that passed with different headers would validate nothing.
func TestPingBearerHeaders(t *testing.T) {
	var gotHeader http.Header
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		writeJSON(w, modelsListBody)
	}), Config{
		APIKey:      "oauth-token",
		AuthScheme:  AuthSchemeBearer,
		BetaHeaders: []string{"oauth-2025-04-20"},
	})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got := gotHeader.Get("Authorization"); got != "Bearer oauth-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer oauth-token")
	}
	if got := gotHeader.Get("x-api-key"); got != "" {
		t.Errorf("x-api-key = %q, want absent under the bearer scheme", got)
	}
	if got := gotHeader.Values("anthropic-beta"); !reflect.DeepEqual(got, []string{"oauth-2025-04-20"}) {
		t.Errorf("anthropic-beta = %v, want [oauth-2025-04-20]", got)
	}
}

// TestPingErrorMapping pins the typed-error mapping of the probe: the same
// status → error classes as Complete/Stream, so callers branch on one set of
// types everywhere.
func TestPingErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		errType string
		headers map[string]string
		check   func(t *testing.T, err error)
	}{
		{
			name: "401Auth", status: 401, errType: "authentication_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want AuthError", err, err)
				}
				if !strings.Contains(e.Message, "boom") {
					t.Errorf("message %q should carry the API message", e.Message)
				}
			},
		},
		{
			name: "403Auth", status: 403, errType: "permission_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want AuthError", err, err)
				}
			},
		},
		{
			name: "429RateLimit", status: 429, errType: "rate_limit_error",
			headers: map[string]string{"Retry-After": "11"},
			check: func(t *testing.T, err error) {
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want RateLimitError", err, err)
				}
				if e.RetryAfter != 11*time.Second {
					t.Errorf("RetryAfter = %v, want 11s", e.RetryAfter)
				}
			},
		},
		{
			name: "404Invalid", status: 404, errType: "not_found_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
				}
			},
		},
		{
			name: "500Server", status: 500, errType: "api_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want ServerError", err, err)
				}
				if e.StatusCode != 500 {
					t.Errorf("StatusCode = %d, want 500", e.StatusCode)
				}
			},
		},
		{
			name: "529Overloaded", status: 529, errType: "overloaded_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want ServerError (retryable)", err, err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				writeAPIError(w, tc.status, tc.errType, "boom")
			}), Config{})
			err := c.Ping(context.Background())
			if err == nil {
				t.Fatal("want error, got nil")
			}
			tc.check(t, err)
		})
	}
}

// TestPingTransportErrorTyped proves a severed connection surfaces as a typed
// retryable ServerError, same as the other paths.
func TestPingTransportErrorTyped(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer is not a hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	}), Config{})

	err := c.Ping(context.Background())
	var se *llmclient.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("err = %T %v, want retryable *llmclient.ServerError", err, err)
	}
}

// TestPingCtxErrorPassesThrough proves cancellation is not converted into a
// retryable error: the caller chose to stop.
func TestPingCtxErrorPassesThrough(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP request should complete under a dead context")
	}), Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Ping(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(_, context.Canceled)", err)
	}
	var se *llmclient.ServerError
	if errors.As(err, &se) {
		t.Fatalf("ctx error must NOT be typed retryable ServerError, got %v", err)
	}
}

// TestPingNeverLeaksCredential exercises the no-leak guarantee on the probe's
// own error paths for both schemes, beyond the shared coverage in
// TestKeyNeverInErrors: a malformed (non-envelope) error body that echoes the
// credential must come back redacted.
func TestPingNeverLeaksCredential(t *testing.T) {
	const sentinel = "sk-ant-PING-SENTINEL-4567"
	for _, scheme := range []string{AuthSchemeAPIKey, AuthSchemeBearer} {
		t.Run(scheme, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(503)
				io.WriteString(w, "upstream said credential "+sentinel+" is bad")
			}), Config{APIKey: sentinel, AuthScheme: scheme})

			err := c.Ping(context.Background())
			if err == nil {
				t.Fatal("want error, got nil")
			}
			for e := err; e != nil; e = errors.Unwrap(e) {
				if strings.Contains(e.Error(), sentinel) {
					t.Fatalf("credential leaked into Ping error output: %v", e)
				}
			}
		})
	}
}
