package bedrock

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// -------------------------------------------------------------------------
// controlPlaneURL derivation
// -------------------------------------------------------------------------

func TestControlPlaneURLDerivation(t *testing.T) {
	cases := []struct {
		runtime string
		want    string
	}{
		{
			"https://bedrock-runtime.us-gov-west-1.amazonaws.com",
			"https://bedrock.us-gov-west-1.amazonaws.com",
		},
		{
			"https://bedrock-runtime.us-east-1.amazonaws.com",
			"https://bedrock.us-east-1.amazonaws.com",
		},
		{
			// VPC endpoint: "bedrock-runtime." appears in the hostname; the
			// substitution fires and derives the control-plane VPC host.
			"https://vpce-abc123.bedrock-runtime.us-gov-west-1.vpce.amazonaws.com",
			"https://vpce-abc123.bedrock.us-gov-west-1.vpce.amazonaws.com",
		},
		{
			// FIPS endpoint: the subdomain is "bedrock-runtime-fips" (no dot
			// after "runtime"); "bedrock-runtime." does not match, so the URL
			// is returned unchanged.
			"https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com",
			"https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com",
		},
	}
	for _, tc := range cases {
		got := controlPlaneURL(tc.runtime)
		if got != tc.want {
			t.Errorf("controlPlaneURL(%q) = %q, want %q", tc.runtime, got, tc.want)
		}
	}
}

// -------------------------------------------------------------------------
// Ping happy path
// -------------------------------------------------------------------------

// TestPingHappyPath proves a 200 response returns nil and that the probe
// issues a signed GET to /foundation-models on the control-plane host (with
// "bedrock-runtime" substituted for "bedrock" in the default hostname).
func TestPingHappyPath(t *testing.T) {
	var gotMethod, gotPath string
	var gotHeader http.Header

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotHeader = r.Header.Clone()
		writeJSON(w, `{"models":[]}`)
	}), Config{})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != foundationModelsPath {
		t.Errorf("path = %q, want %q", gotPath, foundationModelsPath)
	}

	// Probe must carry a SigV4 Authorization header.
	auth := gotHeader.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization not SigV4: %q", auth)
	}
	if !strings.Contains(auth, "Credential=AKIDTEST/") {
		t.Errorf("Authorization missing Credential: %q", auth)
	}
	if !strings.Contains(auth, "/bedrock/aws4_request") {
		t.Errorf("Authorization credential scope should name service 'bedrock': %q", auth)
	}
	if !strings.Contains(auth, "Signature=") {
		t.Errorf("Authorization missing Signature: %q", auth)
	}

	// Probe must carry the date and content-sha256 headers.
	if gotHeader.Get("X-Amz-Date") == "" {
		t.Error("X-Amz-Date must be set")
	}
	if gotHeader.Get("X-Amz-Content-Sha256") != emptyPayloadHash {
		t.Errorf("X-Amz-Content-Sha256 = %q, want emptyPayloadHash for a body-less GET",
			gotHeader.Get("X-Amz-Content-Sha256"))
	}
}

// TestPingBodyIsEmpty proves no request body is sent with the probe — it is a
// pure GET (no content-type, no body bytes).
func TestPingBodyIsEmpty(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, `{}`)
	}), Config{})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if len(gotBody) != 0 {
		t.Errorf("Ping sent a non-empty body: %q", gotBody)
	}
}

// TestPingWithSessionToken proves the session token is included in the signed
// headers when the Client was constructed with temporary credentials.
func TestPingWithSessionToken(t *testing.T) {
	var gotHeader http.Header
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		writeJSON(w, `{}`)
	}), Config{SessionToken: "SomeSTSToken"})

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got := gotHeader.Get("X-Amz-Security-Token"); got != "SomeSTSToken" {
		t.Errorf("X-Amz-Security-Token = %q, want SomeSTSToken", got)
	}
	auth := gotHeader.Get("Authorization")
	if !strings.Contains(auth, "x-amz-security-token") {
		t.Errorf("session token must appear in SignedHeaders: %q", auth)
	}
}

// -------------------------------------------------------------------------
// Ping error mapping
// -------------------------------------------------------------------------

// TestPingErrorMapping pins the typed-error mapping for every relevant HTTP
// status category, mirroring the same mapping as Complete and Stream so that
// callers can use one set of typed checks everywhere.
func TestPingErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		errType string
		body    string
		check   func(t *testing.T, err error)
	}{
		{
			name:    "401Unauthorized",
			status:  401,
			errType: "UnrecognizedClientException",
			body:    `{"message":"token invalid"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *AuthError", err, err)
				}
				if !strings.Contains(e.Message, "token invalid") {
					t.Errorf("AuthError.Message = %q should carry API message", e.Message)
				}
			},
		},
		{
			name:    "403AccessDenied",
			status:  403,
			errType: "AccessDeniedException",
			body:    `{"message":"no permission"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *AuthError", err, err)
				}
			},
		},
		{
			name:    "429ThrottlingException",
			status:  429,
			errType: "ThrottlingException",
			body:    `{"message":"slow down"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *RateLimitError", err, err)
				}
			},
		},
		{
			name:    "400ValidationException",
			status:  400,
			errType: "ValidationException",
			body:    `{"message":"bad request"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
				}
			},
		},
		{
			name:    "500InternalServer",
			status:  500,
			errType: "InternalServerException",
			body:    `{"message":"oops"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *ServerError", err, err)
				}
				if e.StatusCode != 500 {
					t.Errorf("StatusCode = %d, want 500", e.StatusCode)
				}
			},
		},
		{
			name:    "503ServiceUnavailable",
			status:  503,
			errType: "ServiceUnavailableException",
			body:    `{"message":"down for maintenance"}`,
			check: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want *ServerError (retryable)", err, err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.errType != "" {
					w.Header().Set("X-Amzn-ErrorType", tc.errType)
				}
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}), Config{})

			err := c.Ping(context.Background())
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			tc.check(t, err)
		})
	}
}

// TestPingRetryAfterParsed proves the Retry-After header on a 429 response is
// parsed into RateLimitError.RetryAfter.
func TestPingRetryAfterParsed(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Amzn-ErrorType", "ThrottlingException")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"message":"slow down"}`)
	}), Config{})

	err := c.Ping(context.Background())
	var rle *llmclient.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("err = %T %v, want *RateLimitError", err, err)
	}
	if rle.RetryAfter != 30e9 { // 30 * time.Second in nanoseconds
		t.Errorf("RetryAfter = %v, want 30s", rle.RetryAfter)
	}
}

// -------------------------------------------------------------------------
// Ping transport and context error handling
// -------------------------------------------------------------------------

// TestPingTransportErrorTyped proves a severed connection surfaces as a typed
// retryable ServerError, matching the Complete and Stream transport-error contract.
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

// TestPingCtxCancellationPassesThrough proves a cancelled context is not
// converted into a retryable error: the caller chose to stop.
func TestPingCtxCancellationPassesThrough(t *testing.T) {
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
		t.Fatalf("ctx cancellation must NOT surface as a retryable ServerError, got %v", err)
	}
}

// TestPingCtxDeadlinePassesThrough proves an already-expired deadline is not
// converted into a retryable error.
func TestPingCtxDeadlinePassesThrough(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no HTTP request should complete under an expired deadline")
	}), Config{})

	ctx, cancel := context.WithTimeout(context.Background(), -1)
	defer cancel()

	err := c.Ping(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want errors.Is(_, context.DeadlineExceeded)", err)
	}
	var se *llmclient.ServerError
	if errors.As(err, &se) {
		t.Fatalf("ctx deadline must NOT surface as a retryable ServerError, got %v", err)
	}
}

// -------------------------------------------------------------------------
// Ping secret non-leak
// -------------------------------------------------------------------------

// TestPingSecretNeverLeaked proves the secret access key never appears in the
// Ping error, even when the pathological test server echoes it back in the
// error body.
func TestPingSecretNeverLeaked(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		// Echo the test secret (which the newTestClient helper sets to
		// "secret-test-key") so we can verify the redaction path fires.
		io.WriteString(w, `{"message":"your key secret-test-key is invalid"}`)
	}), Config{})

	err := c.Ping(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if strings.Contains(err.Error(), "secret-test-key") {
		t.Errorf("secret leaked into Ping error: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Errorf("expected [redacted] marker in error: %v", err)
	}
}

// -------------------------------------------------------------------------
// Ping satisfies Pinger interface (compile-time assertion)
// -------------------------------------------------------------------------

var _ interface {
	Ping(ctx context.Context) error
} = (*Client)(nil)
