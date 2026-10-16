package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// TestMain establishes a hermetic AWS environment baseline for every test in
// this package, BEFORE the awscred credential chain existed this was moot —
// New only ever consulted four specific env vars, all of which individual
// tests already t.Setenv their way around. Now that New falls back through
// the full awscred chain (shared config/credentials files, SSO, IMDS) when
// explicit Config/env credentials are incomplete, a test host's REAL
// ~/.aws/config, ~/.aws/credentials, AWS_PROFILE, or reachable IMDS endpoint
// could otherwise leak into test outcomes non-deterministically (e.g. a
// developer machine with `aws configure` already run). HOME is pointed at a
// fresh, empty temp directory (no .aws subdirectory ever created there, so
// the shared-file leg always finds nothing) and every AWS_* environment
// variable this package or awscred reads is cleared; individual tests still
// t.Setenv specific vars back on top of this clean baseline exactly as
// before — t.Setenv restores its PRIOR value (here, the TestMain baseline)
// when that test ends, so this is fully compatible with the existing
// per-test env overrides throughout this file.
func TestMain(m *testing.M) {
	// os.Exit skips deferred calls, so the temp-HOME cleanup lives in a
	// helper whose defers run before the exit code reaches os.Exit.
	os.Exit(testMainRun(m))
}

// testMainRun sets the hermetic baseline, runs the suite, and returns the
// exit code — its defer (unlike one in TestMain itself) actually runs.
func testMainRun(m *testing.M) int {
	dir, err := os.MkdirTemp("", "bedrock-test-home-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bedrock test: creating hermetic HOME:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	os.Setenv("HOME", dir)
	os.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, k := range []string{
		"AWS_PROFILE", "AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE",
		"AWS_DEFAULT_REGION", envRegion, envAccessKeyID, envSecretAccessKey, envSessionToken,
	} {
		os.Unsetenv(k)
	}

	return m.Run()
}

// fixedClock pins the signing clock for deterministic X-Amz-Date / signatures.
func fixedClock() func() time.Time {
	t := time.Date(2024, 6, 10, 9, 30, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// newTestClient spins up an httptest server around h and returns a Client
// pointed at it. Zero-valued cfg fields get test defaults (region, creds,
// model, fixed clock).
func newTestClient(t *testing.T, h http.Handler, cfg Config) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	if cfg.Region == "" {
		cfg.Region = "us-gov-west-1"
	}
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = "AKIDTEST"
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = "secret-test-key"
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "anthropic.claude-3-5-sonnet-20241022-v2:0"
	}
	if cfg.Now == nil {
		cfg.Now = fixedClock()
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// jstr JSON-encodes s (for embedding text in fixture JSON).
func jstr(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return string(b)
}

// textResponseBody is a minimal valid non-streaming InvokeModel success body.
func textResponseBody(t *testing.T, text string) string {
	t.Helper()
	return `{"id":"msg_01","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":` +
		jstr(t, text) + `}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":7,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("content-type", "application/json")
	io.WriteString(w, body)
}

func TestNewConfigResolution(t *testing.T) {
	t.Run("FieldsWin", func(t *testing.T) {
		t.Setenv(envRegion, "env-region")
		t.Setenv(envAccessKeyID, "env-akid")
		t.Setenv(envSecretAccessKey, "env-secret")
		c, err := New(Config{Region: "us-gov-west-1", AccessKeyID: "AKID", SecretAccessKey: "secret"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.region != "us-gov-west-1" || c.credCache.creds.accessKeyID != "AKID" || c.credCache.creds.secretAccessKey != "secret" {
			t.Errorf("fields did not win over env: %+v", c.credCache.creds)
		}
	})
	t.Run("EnvFallback", func(t *testing.T) {
		t.Setenv(envRegion, "us-gov-east-1")
		t.Setenv(envAccessKeyID, "ENVKEY")
		t.Setenv(envSecretAccessKey, "ENVSECRET")
		t.Setenv(envSessionToken, "ENVTOKEN")
		c, err := New(Config{})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.region != "us-gov-east-1" || c.credCache.creds.accessKeyID != "ENVKEY" ||
			c.credCache.creds.secretAccessKey != "ENVSECRET" || c.credCache.creds.sessionToken != "ENVTOKEN" {
			t.Errorf("env not read: %+v region=%s", c.credCache.creds, c.region)
		}
	})
	t.Run("DefaultGovCloudHost", func(t *testing.T) {
		c, err := New(Config{Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.baseURL != "https://bedrock-runtime.us-gov-west-1.amazonaws.com" {
			t.Errorf("baseURL = %q", c.baseURL)
		}
		if c.host != "bedrock-runtime.us-gov-west-1.amazonaws.com" {
			t.Errorf("host = %q", c.host)
		}
	})
	t.Run("MissingRegion", func(t *testing.T) {
		t.Setenv(envRegion, "")
		_, err := New(Config{AccessKeyID: "k", SecretAccessKey: "s"})
		if err == nil || !strings.Contains(err.Error(), "region") {
			t.Fatalf("want region error, got %v", err)
		}
	})
	t.Run("MissingAccessKey", func(t *testing.T) {
		t.Setenv(envAccessKeyID, "")
		_, err := New(Config{Region: "r", SecretAccessKey: "s"})
		if err == nil || !strings.Contains(err.Error(), "access key") {
			t.Fatalf("want access-key error, got %v", err)
		}
	})
	t.Run("MissingSecret", func(t *testing.T) {
		t.Setenv(envSecretAccessKey, "")
		_, err := New(Config{Region: "r", AccessKeyID: "k"})
		if err == nil || !strings.Contains(err.Error(), "secret") {
			t.Fatalf("want secret error, got %v", err)
		}
	})
	t.Run("Defaults", func(t *testing.T) {
		c, err := New(Config{Region: "r", AccessKeyID: "k", SecretAccessKey: "s"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.maxTokens != 4096 {
			t.Errorf("maxTokens = %d, want 4096", c.maxTokens)
		}
		if c.completeClient.Timeout == 0 {
			t.Error("default complete client must carry an overall timeout")
		}
		if c.streamClient.Timeout != 0 {
			t.Error("default stream client must NOT carry an overall timeout (ctx governs)")
		}
	})
	t.Run("NegativeMaxTokensRejected", func(t *testing.T) {
		_, err := New(Config{Region: "r", AccessKeyID: "k", SecretAccessKey: "s", DefaultMaxTokens: -1})
		if err == nil {
			t.Fatal("want error for negative DefaultMaxTokens")
		}
	})
	t.Run("BaseURLTrailingSlashTrimmed", func(t *testing.T) {
		// Region ("example") is a substring of the BaseURL host
		// ("vpce.example") so this stays clear of the Region/BaseURL
		// mismatch guard (TestNew_RegionBaseURLMismatch) — this subtest is
		// only about trailing-slash trimming.
		c, err := New(Config{Region: "example", AccessKeyID: "k", SecretAccessKey: "s", BaseURL: "https://vpce.example/"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.baseURL != "https://vpce.example" || c.host != "vpce.example" {
			t.Errorf("baseURL=%q host=%q", c.baseURL, c.host)
		}
	})
	t.Run("InjectedHTTPClientUsedForBoth", func(t *testing.T) {
		hc := &http.Client{}
		c, err := New(Config{Region: "r", AccessKeyID: "k", SecretAccessKey: "s", HTTPClient: hc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.completeClient != hc || c.streamClient != hc {
			t.Error("injected HTTPClient must be used verbatim for both Complete and Stream")
		}
	})
}

// TestNew_RegionBaseURLMismatch covers KP-2: New rejects an explicitly-set
// BaseURL whose host does not contain the explicitly-set Region, but leaves
// Region-only and BaseURL-only configs — and hermetic test infrastructure —
// untouched. New never dials anything during construction (credential
// resolution here always uses the explicit AccessKeyID/SecretAccessKey leg,
// which touches no network), so every case below is offline.
func TestNew_RegionBaseURLMismatch(t *testing.T) {
	t.Run("GovRegionCommercialURL", func(t *testing.T) {
		_, err := New(Config{
			Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s",
			BaseURL: "https://bedrock-runtime.us-east-1.amazonaws.com",
		})
		var mismatch *RegionBaseURLMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("New err = %v (%T), want *RegionBaseURLMismatchError", err, err)
		}
		if mismatch.Region != "us-gov-west-1" || mismatch.BaseURL != "https://bedrock-runtime.us-east-1.amazonaws.com" {
			t.Errorf("mismatch = %+v, want both offending values named", mismatch)
		}
		if !strings.Contains(err.Error(), "us-gov-west-1") || !strings.Contains(err.Error(), "us-east-1") {
			t.Errorf("error message %q must name both Region and BaseURL", err.Error())
		}
	})

	t.Run("CommercialRegionGovURL_Reverse", func(t *testing.T) {
		_, err := New(Config{
			Region: "us-east-1", AccessKeyID: "k", SecretAccessKey: "s",
			BaseURL: "https://bedrock-runtime.us-gov-west-1.amazonaws.com",
		})
		var mismatch *RegionBaseURLMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("New err = %v (%T), want *RegionBaseURLMismatchError", err, err)
		}
	})

	t.Run("MatchingRegionAndURL_NoError", func(t *testing.T) {
		_, err := New(Config{
			Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s",
			BaseURL: "https://bedrock-runtime.us-gov-west-1.amazonaws.com",
		})
		if err != nil {
			t.Fatalf("New: %v, want nil (Region matches BaseURL's host)", err)
		}
	})

	t.Run("RegionOnly_Untouched", func(t *testing.T) {
		// No BaseURL at all: the host is derived FROM Region, so it can
		// never mismatch — the guard must not even need to run.
		_, err := New(Config{Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s"})
		if err != nil {
			t.Fatalf("New: %v, want nil (Region-only config)", err)
		}
	})

	t.Run("BaseURLOnly_Untouched_EvenWithEnvResolvedRegion", func(t *testing.T) {
		// Config.Region itself is empty; the region only resolves via
		// AWS_REGION. The guard must key off the raw Config field, not the
		// resolved region, so this must succeed even though the env-resolved
		// region ("us-gov-west-1") does not appear in the BaseURL host.
		t.Setenv(envRegion, "us-gov-west-1")
		c, err := New(Config{
			AccessKeyID: "k", SecretAccessKey: "s",
			BaseURL: "https://bedrock-runtime.us-east-1.amazonaws.com",
		})
		if err != nil {
			t.Fatalf("New: %v, want nil (BaseURL-only config; Config.Region was never set)", err)
		}
		if c.region != "us-gov-west-1" {
			t.Errorf("region = %q, want the env-resolved us-gov-west-1", c.region)
		}
	})

	t.Run("LoopbackBaseURL_Exempt", func(t *testing.T) {
		// Hermetic test infrastructure (httptest.Server and this package's
		// own newTestClient helper) always pairs an explicit Region with a
		// loopback BaseURL that structurally cannot contain a region
		// substring. New must never dial the BaseURL during construction, so
		// no live listener is needed for this case.
		for _, baseURL := range []string{
			"http://127.0.0.1:54321",
			"http://localhost:54321",
			"http://[::1]:54321",
		} {
			_, err := New(Config{Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s", BaseURL: baseURL})
			if err != nil {
				t.Errorf("New with BaseURL %q: %v, want nil (loopback host is exempt from the mismatch guard)", baseURL, err)
			}
		}
	})
}

func TestCompleteHappyPath(t *testing.T) {
	var gotBody []byte
	var gotHeader http.Header
	var gotPath, gotMethod, gotHost string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotHeader = r.Header.Clone()
		gotPath, gotMethod, gotHost = r.URL.Path, r.Method, r.Host
		writeJSON(w, `{"id":"msg_01","type":"message","role":"assistant","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":25,"cache_creation_input_tokens":7,"cache_read_input_tokens":11}}`)
	}), Config{})

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "say hi"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Request side: method + path map InvokeModel, model id is in the path
	// (URL-escaped ':'), NOT the body.
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	wantPath := "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/invoke"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}

	// SigV4 Authorization header present and well-formed.
	auth := gotHeader.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization not SigV4: %q", auth)
	}
	if !strings.Contains(auth, "Credential=AKIDTEST/20240610/us-gov-west-1/bedrock/aws4_request") {
		t.Errorf("Authorization scope wrong: %q", auth)
	}
	if !strings.Contains(auth, "Signature=") || !strings.Contains(auth, "SignedHeaders=") {
		t.Errorf("Authorization missing parts: %q", auth)
	}
	if gotHeader.Get("X-Amz-Date") != "20240610T093000Z" {
		t.Errorf("X-Amz-Date = %q", gotHeader.Get("X-Amz-Date"))
	}
	if gotHeader.Get("X-Amz-Content-Sha256") == "" {
		t.Error("X-Amz-Content-Sha256 must be set (signed payload hash)")
	}
	// The signed host must be the request host (httptest server host).
	if !strings.Contains(auth, "host") {
		t.Errorf("host must be a signed header: %q", auth)
	}
	_ = gotHost

	// Body: Anthropic-on-Bedrock shape — anthropic_version in body, NO model,
	// NO stream flag.
	wantBody := `{"anthropic_version":"bedrock-2023-05-31","max_tokens":4096,"messages":[{"role":"user","content":[{"type":"text","text":"say hi"}]}]}`
	if string(gotBody) != wantBody {
		t.Errorf("body:\ngot  %s\nwant %s", gotBody, wantBody)
	}
	if strings.Contains(string(gotBody), `"model"`) {
		t.Error("model must NOT appear in the Bedrock body (it is the URL path)")
	}
	if strings.Contains(string(gotBody), `"stream"`) {
		t.Error("stream flag must NOT appear in the Bedrock body")
	}

	// Response side: full mapping including cache usage.
	// parseBlocks sets Raw on every inbound block (verbatim provider JSON).
	wantText1 := llmclient.TextBlock("Hello ")
	wantText1.Raw = json.RawMessage(`{"type":"text","text":"Hello "}`)
	wantText2 := llmclient.TextBlock("world")
	wantText2.Raw = json.RawMessage(`{"type":"text","text":"world"}`)
	want := llmclient.Response{
		Text:       "Hello world",
		Content:    []llmclient.ContentBlock{wantText1, wantText2},
		StopReason: llmclient.StopEndTurn,
		Usage: llmclient.Usage{
			InputTokens: 10, OutputTokens: 25,
			CacheCreationInputTokens: 7, CacheReadInputTokens: 11,
		},
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("response:\ngot  %+v\nwant %+v", resp, want)
	}
}

// TestBuildBodyMapping is a table-driven check of the request-body mapping for
// the cases that differ from a bare prompt: system, tools, tool_choice,
// temperature, stop sequences, multi-block messages, and unknown-block
// passthrough.
func TestBuildBodyMapping(t *testing.T) {
	cases := []struct {
		name string
		req  llmclient.Request
		want string
	}{
		{
			name: "SystemAndTemperatureAndStops",
			req: llmclient.Request{
				Prompt: "hi", System: "be terse", Temperature: 0.5,
				StopSequences: []string{"STOP"},
			},
			want: `{"anthropic_version":"bedrock-2023-05-31","max_tokens":4096,"system":"be terse","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"temperature":0.5,"stop_sequences":["STOP"]}`,
		},
		{
			name: "ToolsAndForcedToolChoice",
			req: llmclient.Request{
				Prompt: "look",
				Tools: []llmclient.ToolDef{{
					Name: "search", Description: "find", InputSchema: json.RawMessage(`{"type":"object"}`),
				}},
				ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "search"},
			},
			want: `{"anthropic_version":"bedrock-2023-05-31","max_tokens":4096,"messages":[{"role":"user","content":[{"type":"text","text":"look"}]}],"tools":[{"name":"search","description":"find","input_schema":{"type":"object"}}],"tool_choice":{"type":"tool","name":"search"}}`,
		},
		{
			name: "UnknownBlockVerbatim",
			req: llmclient.Request{
				Messages: []llmclient.Message{{
					Role: llmclient.RoleAssistant,
					Content: []llmclient.ContentBlock{
						{Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"hmm"}`)},
					},
				}},
			},
			want: `{"anthropic_version":"bedrock-2023-05-31","max_tokens":4096,"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"}]}]}`,
		},
		{
			name: "MaxTokensOverride",
			req:  llmclient.Request{Prompt: "x", MaxTokens: 99},
			want: `{"anthropic_version":"bedrock-2023-05-31","max_tokens":99,"messages":[{"role":"user","content":[{"type":"text","text":"x"}]}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
			body, model, err := c.buildBody(tc.req)
			if err != nil {
				t.Fatalf("buildBody: %v", err)
			}
			if model == "" {
				t.Error("model must be resolved")
			}
			if string(body) != tc.want {
				t.Errorf("body:\ngot  %s\nwant %s", body, tc.want)
			}
		})
	}
}

// TestBuildBodyDeterministic proves the same request always serializes to
// byte-identical body bytes (field order is fixed; no map-ordering leaks).
func TestBuildBodyDeterministic(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	req := llmclient.Request{
		Prompt: "deterministic", System: "sys", Temperature: 0.7,
		Tools: []llmclient.ToolDef{
			{Name: "a", InputSchema: json.RawMessage(`{"type":"object"}`)},
			{Name: "b", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		StopSequences: []string{"X", "Y"},
	}
	var first []byte
	for i := 0; i < 16; i++ {
		body, _, err := c.buildBody(req)
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		if i == 0 {
			first = body
			continue
		}
		if string(body) != string(first) {
			t.Fatalf("non-deterministic body at iter %d:\n%s\n%s", i, first, body)
		}
	}
}

// TestEscapingSafety proves JSON-special and control characters in user content
// are properly escaped (encoding/json), and that a model id with reserved URL
// characters is path-escaped so it cannot break out of the path segment.
func TestEscapingSafety(t *testing.T) {
	t.Run("ContentEscaped", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
		nasty := "quote\" brace} newline\n tab\t backslash\\ </script>"
		body, _, err := c.buildBody(llmclient.Request{Prompt: nasty})
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		// Must be valid JSON and round-trip the exact bytes.
		var parsed wireRequest
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("body is not valid JSON: %v\n%s", err, body)
		}
		var block struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(parsed.Messages[0].Content[0], &block); err != nil {
			t.Fatalf("block JSON: %v", err)
		}
		if block.Text != nasty {
			t.Errorf("escaping corrupted content:\ngot  %q\nwant %q", block.Text, nasty)
		}
		// Raw bytes must not contain an unescaped literal newline inside the
		// JSON string.
		if strings.Contains(string(body), "newline\n tab") {
			t.Error("control characters were not escaped in the JSON body")
		}
	})

	t.Run("ModelIDPathEscaped", func(t *testing.T) {
		var gotRawPath string
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRawPath = r.URL.EscapedPath()
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{DefaultModel: "arn:aws:bedrock:us-gov-west-1::inference-profile/foo"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		// The ':' and '/' in the ARN must be percent-encoded in the path
		// segment so they do not split the route.
		if strings.Contains(gotRawPath, "arn:aws") {
			t.Errorf("model id colons not escaped in path: %q", gotRawPath)
		}
		if !strings.Contains(gotRawPath, "arn%3Aaws") {
			t.Errorf("expected escaped ARN in path: %q", gotRawPath)
		}
	})
}

func TestCacheBreakpointsRejected(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("must reject before any HTTP")
	}), Config{})
	req := llmclient.Request{
		System: "s", SystemCacheHint: true,
		Tools: []llmclient.ToolDef{
			{Name: "a", CacheHint: true, InputSchema: json.RawMessage(`{}`)},
			{Name: "b", CacheHint: true, InputSchema: json.RawMessage(`{}`)},
		},
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockText, Text: "x", CacheHint: true},
			{Type: llmclient.BlockText, Text: "y", CacheHint: true},
		}}},
	}
	_, err := c.Complete(context.Background(), req)
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
	}
}

func TestNoModelRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("must reject before HTTP")
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{Region: "us-gov-west-1", AccessKeyID: "k", SecretAccessKey: "s", BaseURL: srv.URL, Now: fixedClock()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
	}
}

// TestErrorMapping is a table-driven check of HTTP status -> typed error.
func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		errType string
		body    string
		assert  func(t *testing.T, err error)
	}{
		{
			name: "ValidationException400", status: 400,
			errType: "ValidationException", body: `{"message":"bad input"}`,
			assert: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T, want InvalidRequestError", err)
				}
				if !strings.Contains(e.Message, "bad input") || !strings.Contains(e.Message, "ValidationException") {
					t.Errorf("message = %q", e.Message)
				}
			},
		},
		{
			name: "Throttling429", status: 429,
			errType: "ThrottlingException", body: `{"message":"slow down"}`,
			assert: func(t *testing.T, err error) {
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T, want RateLimitError", err)
				}
			},
		},
		{
			name: "ThrottlingVia200Header", status: 400,
			errType: "ThrottlingException", body: `{"message":"throttled"}`,
			assert: func(t *testing.T, err error) {
				// errType ThrottlingException promotes even a 400 to rate-limit.
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T, want RateLimitError", err)
				}
			},
		},
		{
			name: "AccessDenied403", status: 403,
			errType: "AccessDeniedException", body: `{"message":"no"}`,
			assert: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T, want AuthError", err)
				}
			},
		},
		{
			name: "Internal500", status: 500,
			errType: "InternalServerException", body: `{"message":"boom"}`,
			assert: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T, want ServerError", err)
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
			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
			tc.assert(t, err)
		})
	}
}

// TestSecretNeverInErrors proves the secret key cannot appear in an error,
// even if a pathological server echoes it.
func TestSecretNeverInErrors(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		io.WriteString(w, `{"message":"your key secret-test-key is invalid"}`)
	}), Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "secret-test-key") {
		t.Errorf("error leaked the secret key: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Errorf("echoed secret should be redacted: %v", err)
	}
}

// TestTransportErrorTyped proves a severed connection surfaces as a typed
// retryable ServerError on Complete.
func TestTransportErrorTyped(t *testing.T) {
	hangup := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("not a hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	})
	c := newTestClient(t, hangup, Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var se *llmclient.ServerError
	if !errors.As(err, &se) {
		t.Fatalf("err = %T %v, want retryable ServerError", err, err)
	}
}

// TestCtxCancellationPassesThrough proves a cancelled context is not turned
// into a retryable error.
func TestCtxCancellationPassesThrough(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("no request should complete under a dead context")
	}), Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Complete(ctx, llmclient.Request{Prompt: "p"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var se *llmclient.ServerError
	if errors.As(err, &se) {
		t.Fatalf("ctx error must NOT be a retryable ServerError: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — Thinking, Effort, ProviderParams, CapabilityCheck
// ---------------------------------------------------------------------------

// TestThinkingWireShape proves that Request.Thinking maps to the "thinking"
// object in the Bedrock InvokeModel body, which is the Anthropic Messages body
// format (identical path to the public Anthropic adapter).
func TestThinkingWireShape(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	req := llmclient.Request{
		Prompt:   "think hard",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 1024},
	}
	body, _, err := c.buildBody(req)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	raw, ok := m["thinking"]
	if !ok {
		t.Fatalf("thinking missing from wire body: %s", body)
	}
	th, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("thinking is %T, want object", raw)
	}
	if th["type"] != "enabled" {
		t.Errorf("thinking.type = %v, want enabled", th["type"])
	}
	if th["budget_tokens"] != float64(1024) {
		t.Errorf("thinking.budget_tokens = %v, want 1024", th["budget_tokens"])
	}
}

// TestThinkingNilOmitted proves nil Thinking produces no "thinking" key.
func TestThinkingNilOmitted(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	body, _, err := c.buildBody(llmclient.Request{Prompt: "p"})
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := m["thinking"]; ok {
		t.Errorf("nil Thinking must not appear in wire body: %s", body)
	}
}

// TestEffortRejected proves bedrock returns a typed InvalidRequestError for a
// non-empty Effort (no Bedrock knob exists), pre-HTTP.
func TestEffortRejected(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	_, _, err := c.buildBody(llmclient.Request{Prompt: "p", Effort: "medium"})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want *InvalidRequestError for Effort", err, err)
	}
	if !strings.Contains(ire.Error(), "Effort") {
		t.Errorf("error should name Effort: %q", ire.Error())
	}
}

// TestProviderParamsMerge proves ProviderParams is merged into the Bedrock
// body, with ProviderParams keys overriding adapter-set keys.
func TestProviderParamsMerge(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	req := llmclient.Request{
		Prompt:         "p",
		ProviderParams: json.RawMessage(`{"anthropic_version":"custom-override","extra":"val"}`),
	}
	body, _, err := c.buildBody(req)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	// ProviderParams overrides the adapter-set anthropic_version.
	if m["anthropic_version"] != "custom-override" {
		t.Errorf("anthropic_version = %v, want custom-override (ProviderParams must override adapter-set keys)", m["anthropic_version"])
	}
	// New key from ProviderParams appears verbatim.
	if m["extra"] != "val" {
		t.Errorf("extra = %v, want val", m["extra"])
	}
	// Adapter-set keys not overridden are preserved.
	if _, ok := m["max_tokens"]; !ok {
		t.Errorf("max_tokens absent — non-overridden adapter keys must survive ProviderParams merge")
	}
}

// TestProviderParamsEmptyIsNoOp proves nil/empty ProviderParams produce the
// same body as a baseline request with no ProviderParams field.
func TestProviderParamsEmptyIsNoOp(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	base, _, err := c.buildBody(llmclient.Request{Prompt: "p"})
	if err != nil {
		t.Fatalf("buildBody base: %v", err)
	}
	withNil, _, err := c.buildBody(llmclient.Request{Prompt: "p", ProviderParams: nil})
	if err != nil {
		t.Fatalf("buildBody nil pp: %v", err)
	}
	withEmpty, _, err := c.buildBody(llmclient.Request{Prompt: "p", ProviderParams: json.RawMessage("")})
	if err != nil {
		t.Fatalf("buildBody empty pp: %v", err)
	}
	if string(base) != string(withNil) {
		t.Errorf("nil ProviderParams changed body:\nbase %s\nnil  %s", base, withNil)
	}
	if string(base) != string(withEmpty) {
		t.Errorf("empty ProviderParams changed body:\nbase  %s\nempty %s", base, withEmpty)
	}
}

// TestProviderParamsNonObjectRejected proves non-object ProviderParams returns
// a typed InvalidRequestError before any HTTP.
func TestProviderParamsNonObjectRejected(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{})
	for _, pp := range []json.RawMessage{
		json.RawMessage(`[1,2]`),
		json.RawMessage(`"str"`),
		json.RawMessage(`42`),
		json.RawMessage(`null`),
	} {
		_, _, err := c.buildBody(llmclient.Request{Prompt: "p", ProviderParams: pp})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Errorf("pp=%s: err = %T %v, want *InvalidRequestError", pp, err, err)
		}
	}
}

// TestCapabilityCheckCalled proves Config.CapabilityCheck fires before HTTP
// and its error is returned as-is; nil check is a no-op (permissive).
func TestCapabilityCheckCalled(t *testing.T) {
	checkErr := &llmclient.InvalidRequestError{Message: "capability-check: unsupported feature"}

	// With a check that rejects thinking: fires pre-HTTP.
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{
		CapabilityCheck: func(req llmclient.Request) error {
			if req.Thinking != nil {
				return checkErr
			}
			return nil
		},
	})

	// No thinking: check passes.
	_, _, err := c.buildBody(llmclient.Request{Prompt: "p"})
	if err != nil {
		t.Fatalf("check should pass for no-thinking: %v", err)
	}

	// With thinking: check rejects.
	_, _, err = c.buildBody(llmclient.Request{
		Prompt:   "p",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled"},
	})
	if err != checkErr {
		t.Fatalf("CapabilityCheck error not returned: got %v, want %v", err, checkErr)
	}

	// Nil check: all requests pass.
	c2 := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), Config{
		CapabilityCheck: nil,
	})
	_, _, err = c2.buildBody(llmclient.Request{
		Prompt:   "p",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled"},
	})
	if err != nil {
		t.Fatalf("nil CapabilityCheck: unexpected error %v", err)
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — OutputSchema and ToolDef.Strict (Bedrock adapter)
// ---------------------------------------------------------------------------

// TestOutputSchemaWireShape_Bedrock proves that a non-empty Request.OutputSchema
// is mapped to output_config.format.{type:"json_schema", schema:...} in the
// Bedrock wire body, and that the field is absent when OutputSchema is nil.
func TestOutputSchemaWireShape_Bedrock(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)

	t.Run("Present", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		body, _, err := c.buildBody(llmclient.Request{Prompt: "p", OutputSchema: schema})
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		oc, ok := m["output_config"].(map[string]any)
		if !ok {
			t.Fatalf("output_config missing or not object; body: %s", body)
		}
		format, ok := oc["format"].(map[string]any)
		if !ok {
			t.Fatalf("output_config.format missing; body: %s", body)
		}
		if format["type"] != "json_schema" {
			t.Errorf("output_config.format.type = %v, want json_schema", format["type"])
		}
		if format["schema"] == nil {
			t.Errorf("output_config.format.schema missing")
		}
	})

	t.Run("AbsentWhenNil", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		body, _, err := c.buildBody(llmclient.Request{Prompt: "p"})
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if _, ok := m["output_config"]; ok {
			t.Errorf("output_config must be absent when OutputSchema is nil; body: %s", body)
		}
	})

	t.Run("NonObjectRejected", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		_, _, err := c.buildBody(llmclient.Request{
			Prompt:       "p",
			OutputSchema: json.RawMessage(`[1,2,3]`),
		})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
		}
		if !strings.Contains(ire.Error(), "OutputSchema") {
			t.Errorf("error should name OutputSchema: %q", ire.Error())
		}
	})
}

// TestToolDefStrictWireShape_Bedrock proves ToolDef.Strict=true adds
// "strict":true to the typed tool wire object on the Bedrock adapter.
func TestToolDefStrictWireShape_Bedrock(t *testing.T) {
	t.Run("StrictTrue", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		body, _, err := c.buildBody(llmclient.Request{
			Prompt: "p",
			Tools: []llmclient.ToolDef{{
				Name:        "calc",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				Strict:      true,
			}},
		})
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(m["tools"], &tools); err != nil {
			t.Fatalf("unmarshal tools: %v", err)
		}
		strictRaw, ok := tools[0]["strict"]
		if !ok {
			t.Fatalf("strict missing from tool wire object; tool: %v", tools[0])
		}
		if string(strictRaw) != "true" {
			t.Errorf("strict = %s, want true", strictRaw)
		}
	})

	t.Run("StrictFalseOmitted", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		body, _, err := c.buildBody(llmclient.Request{
			Prompt: "p",
			Tools: []llmclient.ToolDef{{
				Name:        "calc",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				Strict:      false,
			}},
		})
		if err != nil {
			t.Fatalf("buildBody: %v", err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(m["tools"], &tools); err != nil {
			t.Fatalf("unmarshal tools: %v", err)
		}
		if _, ok := tools[0]["strict"]; ok {
			t.Errorf("strict must be absent when Strict=false; tool: %v", tools[0])
		}
	})
}
