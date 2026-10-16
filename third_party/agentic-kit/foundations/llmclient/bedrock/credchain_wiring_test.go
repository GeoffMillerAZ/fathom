package bedrock

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/bedrock/awscred"
)

// TestNew_FallsBackToSharedProfile proves New is genuinely wired to the full
// awscred chain, not just able to call it in isolation: with no explicit
// Config credentials and no AWS_* env vars set, a fixture ~/.aws/credentials
// file (under a per-test HOME) resolves the client's credentials and the
// fixture ~/.aws/config file's "region" key resolves its region — AC-6's
// profile-region fallback, exercised end-to-end through New.
func TestNew_FallsBackToSharedProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(awsDir, "credentials"), []byte(
		"[default]\naws_access_key_id = AKIDFROMFILE\naws_secret_access_key = secretfromfile\n",
	), 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(
		"[default]\nregion = us-gov-west-1\n",
	), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, err := New(Config{Now: fixedClock()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.credCache.creds.accessKeyID != "AKIDFROMFILE" || c.credCache.creds.secretAccessKey != "secretfromfile" {
		t.Errorf("creds = %+v, want resolved from the shared credentials file", c.credCache.creds)
	}
	if c.region != "us-gov-west-1" {
		t.Errorf("region = %q, want resolved from the shared config file's region key", c.region)
	}
}

// TestNew_NoCredentialsAnywhere_ReturnsWrappedAwscredError proves New wraps
// (not swallows) an exhausted-chain error from awscred, so a caller sees the
// SAME actionable detail awscred.Resolve produced, prefixed for bedrock.
func TestNew_NoCredentialsAnywhere_ReturnsWrappedAwscredError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // empty — no .aws directory at all

	_, err := New(Config{Region: "us-gov-west-1"})
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "bedrock: resolving credentials:") {
		t.Errorf("error %q must be wrapped with the bedrock prefix", err)
	}
	if !strings.Contains(err.Error(), "access key") {
		t.Errorf("error %q must retain awscred's detail", err)
	}
}

// TestComplete_ReResolvesNearExpiry_SignsWithFreshCredentials is the core
// proof of expiry-aware re-resolution end to end: two Complete calls against
// a Client whose credCache.resolve is injected (white-box construction) to
// return a DIFFERENT access-key id on its second invocation, with a clock
// that only crosses the expiry margin between the two calls. The SigV4
// Authorization header's Credential=<access-key-id> must differ between the
// two requests, proving the second call actually re-signed with the freshly
// resolved credential rather than reusing the first.
func TestComplete_ReResolvesNearExpiry_SignsWithFreshCredentials(t *testing.T) {
	var gotAuthHeaders []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeaders = append(gotAuthHeaders, r.Header.Get("Authorization"))
		io.ReadAll(r.Body)
		w.Header().Set("content-type", "application/json")
		io.WriteString(w, `{"id":"msg","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`)
	}))
	defer srv.Close()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clockTime := base
	clock := func() time.Time { return clockTime }

	// expiry is far enough out (10min) that the first Complete call, at
	// clockTime==base, is comfortably outside the 5-minute margin and must
	// use the pre-seeded cache WITHOUT calling resolve at all — resolve is
	// exercised for the first time only once the test advances clockTime
	// into the margin below, proving re-resolution is expiry-triggered, not
	// unconditional.
	expiry := base.Add(10 * time.Minute)
	resolveCalls := 0
	resolve := func(ctx context.Context) (awscred.Credentials, error) {
		resolveCalls++
		return awscred.Credentials{AccessKeyID: "AKIDSECOND", SecretAccessKey: "secret2", Expiry: expiry.Add(10 * time.Minute)}, nil
	}

	c := &Client{
		credCache: &credentialCache{
			now:     clock,
			resolve: resolve,
			creds:   credentials{accessKeyID: "AKIDFIRST", secretAccessKey: "secret1"},
			expiry:  expiry,
		},
		region:         "us-gov-west-1",
		baseURL:        srv.URL,
		host:           strings.TrimPrefix(srv.URL, "http://"),
		model:          "anthropic.claude-3-5-sonnet-20241022-v2:0",
		maxTokens:      defaultMaxTokens,
		completeClient: srv.Client(),
		streamClient:   srv.Client(),
		now:            clock,
	}

	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "hi"}); err != nil {
		t.Fatalf("Complete #1: %v", err)
	}

	// Advance past the expiry margin: the second call must re-resolve.
	clockTime = expiry.Add(-4 * time.Minute)
	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "hi again"}); err != nil {
		t.Fatalf("Complete #2: %v", err)
	}

	if resolveCalls != 1 {
		t.Fatalf("resolve called %d times, want exactly 1 (only the second Complete call is within the expiry margin)", resolveCalls)
	}
	if len(gotAuthHeaders) != 2 {
		t.Fatalf("got %d requests, want 2", len(gotAuthHeaders))
	}
	if !strings.Contains(gotAuthHeaders[0], "Credential=AKIDFIRST/") {
		t.Errorf("request 1 Authorization = %q, want Credential=AKIDFIRST", gotAuthHeaders[0])
	}
	if !strings.Contains(gotAuthHeaders[1], "Credential=AKIDSECOND/") {
		t.Errorf("request 2 Authorization = %q, want Credential=AKIDSECOND (re-resolved)", gotAuthHeaders[1])
	}
}

// TestComplete_CredentialResolutionFailure_MapsToAuthError proves a
// credCache.get failure (e.g. an expired SSO token discovered on
// re-resolution) surfaces as a typed, non-retryable *llmclient.AuthError
// rather than a bare error or a retryable ServerError.
func TestComplete_CredentialResolutionFailure_MapsToAuthError(t *testing.T) {
	wantErr := &awscred.ExpiredSSOTokenError{Profile: "myprofile"}
	c := &Client{
		credCache: &credentialCache{
			now:     time.Now,
			resolve: func(ctx context.Context) (awscred.Credentials, error) { return awscred.Credentials{}, wantErr },
			// zero expiry with a non-nil resolve but no cached creds yet would
			// take the fast path (expiry.IsZero() short-circuits); force a
			// non-zero PAST expiry so get() re-resolves immediately.
			expiry: time.Now().Add(-time.Hour),
		},
		region:         "us-gov-west-1",
		baseURL:        "http://unused.invalid",
		host:           "unused.invalid",
		model:          "m",
		maxTokens:      defaultMaxTokens,
		completeClient: http.DefaultClient,
		streamClient:   http.DefaultClient,
		now:            time.Now,
	}

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "hi"})
	var authErr *llmclient.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("err = %T %v, want *llmclient.AuthError", err, err)
	}
	if !strings.Contains(authErr.Message, "aws sso login") {
		t.Errorf("AuthError.Message = %q, want to carry the SSO error's actionable text", authErr.Message)
	}
}
