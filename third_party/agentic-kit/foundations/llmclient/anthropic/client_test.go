package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// newTestClient spins up an httptest server around h and returns a Client
// pointed at it. Zero-valued cfg fields get test defaults.
func newTestClient(t *testing.T, h http.Handler, cfg Config) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	if cfg.APIKey == "" {
		cfg.APIKey = "test-key"
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

// jstr JSON-encodes s (for embedding text in fixture JSON).
func jstr(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal %q: %v", s, err)
	}
	return string(b)
}

// textResponseBody is a minimal valid non-streaming success body.
func textResponseBody(t *testing.T, text string) string {
	t.Helper()
	return `{"id":"msg_01","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":` +
		jstr(t, text) + `}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":7,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("content-type", "application/json")
	io.WriteString(w, body)
}

func writeAPIError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"type":"error","error":{"type":%q,"message":%q}}`, errType, message)
}

func TestNewConfigResolution(t *testing.T) {
	t.Run("APIKeyWins", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		c, err := New(Config{APIKey: "literal-key"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.key != "literal-key" {
			t.Errorf("key = %q, want literal-key", c.key)
		}
	})
	t.Run("DefaultEnvVar", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		c, err := New(Config{})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.key != "env-key" {
			t.Errorf("key = %q, want env-key", c.key)
		}
	})
	t.Run("CustomEnvVar", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "wrong")
		t.Setenv("CUSTOM_KEY_VAR", "custom-key")
		c, err := New(Config{APIKeyEnv: "CUSTOM_KEY_VAR"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.key != "custom-key" {
			t.Errorf("key = %q, want custom-key", c.key)
		}
	})
	t.Run("NoKeyErrors", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		_, err := New(Config{})
		if err == nil {
			t.Fatal("New with no key: want error, got nil")
		}
		if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
			t.Errorf("error %q should name the env var", err)
		}
	})
	t.Run("Defaults", func(t *testing.T) {
		c, err := New(Config{APIKey: "k"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.baseURL != "https://api.anthropic.com" {
			t.Errorf("baseURL = %q", c.baseURL)
		}
		if c.maxTokens != 4096 {
			t.Errorf("maxTokens = %d, want 4096", c.maxTokens)
		}
		if c.version != "2023-06-01" {
			t.Errorf("version = %q", c.version)
		}
		if c.authScheme != AuthSchemeAPIKey {
			t.Errorf("authScheme = %q, want %q (the default)", c.authScheme, AuthSchemeAPIKey)
		}
		if c.betaHeaders != nil {
			t.Errorf("betaHeaders = %v, want none by default", c.betaHeaders)
		}
		if c.completeClient.Timeout == 0 {
			t.Error("default complete client must carry an overall timeout")
		}
		if c.streamClient.Timeout != 0 {
			t.Error("default stream client must NOT carry an overall timeout (ctx governs)")
		}
	})
	t.Run("BearerSchemeAccepted", func(t *testing.T) {
		c, err := New(Config{APIKey: "tok", AuthScheme: AuthSchemeBearer})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.authScheme != AuthSchemeBearer {
			t.Errorf("authScheme = %q, want %q", c.authScheme, AuthSchemeBearer)
		}
	})
	t.Run("UnknownAuthSchemeRejected", func(t *testing.T) {
		_, err := New(Config{APIKey: "k", AuthScheme: "basic"})
		if err == nil {
			t.Fatal("want error for unknown AuthScheme")
		}
		if !strings.Contains(err.Error(), "AuthScheme") {
			t.Errorf("error %q should name the field", err)
		}
	})
	t.Run("EmptyBetaHeaderRejected", func(t *testing.T) {
		_, err := New(Config{APIKey: "k", BetaHeaders: []string{"oauth-2025-04-20", ""}})
		if err == nil {
			t.Fatal("want error for an empty BetaHeaders entry")
		}
		if !strings.Contains(err.Error(), "BetaHeaders[1]") {
			t.Errorf("error %q should name the offending index", err)
		}
	})
	t.Run("BetaHeadersCopied", func(t *testing.T) {
		betas := []string{"oauth-2025-04-20"}
		c, err := New(Config{APIKey: "k", BetaHeaders: betas})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		betas[0] = "mutated-after-construction"
		if c.betaHeaders[0] != "oauth-2025-04-20" {
			t.Error("Client must hold a defensive copy of BetaHeaders (immutability is the concurrency contract)")
		}
	})
	t.Run("TrailingSlashTrimmed", func(t *testing.T) {
		c, err := New(Config{APIKey: "k", BaseURL: "https://example.test/"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.baseURL != "https://example.test" {
			t.Errorf("baseURL = %q", c.baseURL)
		}
	})
	t.Run("NegativeMaxTokensRejected", func(t *testing.T) {
		if _, err := New(Config{APIKey: "k", DefaultMaxTokens: -1}); err == nil {
			t.Fatal("want error for negative DefaultMaxTokens")
		}
	})
	t.Run("InjectedHTTPClientUsedForBoth", func(t *testing.T) {
		hc := &http.Client{}
		c, err := New(Config{APIKey: "k", HTTPClient: hc})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if c.completeClient != hc || c.streamClient != hc {
			t.Error("injected HTTPClient must be used verbatim for both Complete and Stream")
		}
	})
}

func TestCompleteHappyPath(t *testing.T) {
	var gotBody []byte
	var gotHeader http.Header
	var gotPath, gotMethod string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotHeader = r.Header.Clone()
		gotPath, gotMethod = r.URL.Path, r.Method
		writeJSON(w, `{"id":"msg_01","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":25,"cache_creation_input_tokens":7,"cache_read_input_tokens":11}}`)
	}), Config{})

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "say hi"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Request side.
	if gotMethod != http.MethodPost || gotPath != "/v1/messages" {
		t.Errorf("got %s %s, want POST /v1/messages", gotMethod, gotPath)
	}
	if k := gotHeader.Get("x-api-key"); k != "test-key" {
		t.Errorf("x-api-key = %q", k)
	}
	if v := gotHeader.Get("anthropic-version"); v != "2023-06-01" {
		t.Errorf("anthropic-version = %q", v)
	}
	if ct := gotHeader.Get("content-type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	wantBody := `{"model":"model-x","max_tokens":4096,"messages":[{"role":"user","content":[{"type":"text","text":"say hi"}]}]}`
	if string(gotBody) != wantBody {
		t.Errorf("body:\ngot  %s\nwant %s", gotBody, wantBody)
	}

	// Response side: full mapping, including the cache usage fields and Raw.
	// Typed inbound blocks now carry their verbatim wire JSON in Raw; the
	// expected blocks are built with their Raw bytes set to match the provider
	// fixture (order matches the JSON object from the server).
	wantText1 := llmclient.TextBlock("Hello ")
	wantText1.Raw = json.RawMessage(`{"type":"text","text":"Hello "}`)
	wantText2 := llmclient.TextBlock("world")
	wantText2.Raw = json.RawMessage(`{"type":"text","text":"world"}`)
	want := llmclient.Response{
		Text:       "Hello world",
		Content:    []llmclient.ContentBlock{wantText1, wantText2},
		StopReason: llmclient.StopEndTurn,
		Usage: llmclient.Usage{
			InputTokens:              10,
			OutputTokens:             25,
			CacheCreationInputTokens: 7,
			CacheReadInputTokens:     11,
		},
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("Response:\ngot  %+v\nwant %+v", resp, want)
	}
}

// TestAuthSchemeHeaders pins exactly which header carries the credential per
// scheme — and that the other scheme's header is absent — plus the
// anthropic-beta lines, on both the Complete and Stream paths (they share
// request construction; both are exercised to keep that fact pinned).
func TestAuthSchemeHeaders(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		// wantAuthz "" means the Authorization header must be absent.
		wantAuthz   string
		wantXAPIKey string
		wantBetas   []string
	}{
		{
			name:        "DefaultXAPIKey",
			cfg:         Config{APIKey: "test-key"},
			wantXAPIKey: "test-key",
		},
		{
			name:      "BearerSendsAuthorization",
			cfg:       Config{APIKey: "oauth-token", AuthScheme: AuthSchemeBearer},
			wantAuthz: "Bearer oauth-token",
		},
		{
			name: "BearerWithOAuthBeta",
			cfg: Config{
				APIKey:      "oauth-token",
				AuthScheme:  AuthSchemeBearer,
				BetaHeaders: []string{"oauth-2025-04-20"},
			},
			wantAuthz: "Bearer oauth-token",
			wantBetas: []string{"oauth-2025-04-20"},
		},
		{
			name: "MultipleBetaLines",
			cfg: Config{
				APIKey:      "test-key",
				BetaHeaders: []string{"oauth-2025-04-20", "context-1m-2025-08-07"},
			},
			wantXAPIKey: "test-key",
			wantBetas:   []string{"oauth-2025-04-20", "context-1m-2025-08-07"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotHeader http.Header
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Clone()
				body, _ := io.ReadAll(r.Body)
				if isStreamRequest(body) {
					f := w.(http.Flusher)
					streamPreamble(w, f)
					streamTextDeltas(t, w, f, "ok")
					streamFinish(w, f, "end_turn")
					return
				}
				writeJSON(w, textResponseBody(t, "ok"))
			}), tc.cfg)

			assertHeaders := func(t *testing.T) {
				t.Helper()
				if got := gotHeader.Get("Authorization"); got != tc.wantAuthz {
					t.Errorf("Authorization = %q, want %q", got, tc.wantAuthz)
				}
				if got := gotHeader.Get("x-api-key"); got != tc.wantXAPIKey {
					t.Errorf("x-api-key = %q, want %q", got, tc.wantXAPIKey)
				}
				if got := gotHeader.Values("anthropic-beta"); !reflect.DeepEqual(got, tc.wantBetas) {
					t.Errorf("anthropic-beta = %v, want %v", got, tc.wantBetas)
				}
				if got := gotHeader.Get("anthropic-version"); got != "2023-06-01" {
					t.Errorf("anthropic-version = %q", got)
				}
			}

			t.Run("Complete", func(t *testing.T) {
				if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
					t.Fatalf("Complete: %v", err)
				}
				assertHeaders(t)
			})
			t.Run("Stream", func(t *testing.T) {
				ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
				if err != nil {
					t.Fatalf("Stream: %v", err)
				}
				drainStream(t, ch)
				assertHeaders(t)
			})
		})
	}
}

func TestCompleteToolUseResponse(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"toolu_9","name":"search","input":{"q":"weather"}}],"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":6}}`)
	}), Config{})

	resp, err := c.Complete(context.Background(), llmclient.Request{
		Prompt: "look it up",
		Tools:  []llmclient.ToolDef{{Name: "search", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.StopReason != llmclient.StopToolUse {
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
	if resp.Text != "Checking." {
		t.Errorf("Text = %q", resp.Text)
	}
	calls := resp.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(calls))
	}
	if calls[0].ID != "toolu_9" || calls[0].Name != "search" {
		t.Errorf("call = %q/%q", calls[0].ID, calls[0].Name)
	}
	if string(calls[0].Input) != `{"q":"weather"}` {
		t.Errorf("Input = %s", calls[0].Input)
	}
}

func TestUnknownBlockRoundTrip(t *testing.T) {
	const thinking = `{"type":"thinking","thinking":"hmm, let me reason","signature":"sig01"}`
	var second []byte
	calls := 0
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, `{"content":[`+thinking+`,{"type":"text","text":"the answer"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":2}}`)
			return
		}
		second, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{Prompt: "go"}
	resp, err := c.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete 1: %v", err)
	}
	if len(resp.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(resp.Content))
	}
	if resp.Content[0].Type != llmclient.BlockUnknown {
		t.Fatalf("Content[0].Type = %q, want unknown", resp.Content[0].Type)
	}
	if string(resp.Content[0].Raw) != thinking {
		t.Errorf("Raw not verbatim inbound:\ngot  %s\nwant %s", resp.Content[0].Raw, thinking)
	}
	if resp.Text != "the answer" {
		t.Errorf("Text = %q (unknown blocks must not contribute)", resp.Text)
	}

	// Round-trip: append the assistant turn and call again; the thinking
	// block must reach the wire byte-verbatim.
	req.Messages = []llmclient.Message{
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("go")}},
		resp.AsMessage(),
		{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("continue")}},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete 2: %v", err)
	}
	if !strings.Contains(string(second), thinking) {
		t.Errorf("outbound body must contain the verbatim unknown block:\nbody %s\nwant substring %s", second, thinking)
	}
}

func TestWireShapeGoldenSystemAndToolCache(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	req := llmclient.Request{
		System:          "You are terse.",
		SystemCacheHint: true,
		Messages: []llmclient.Message{
			{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
		},
		StopSequences: []string{"END"},
		Tools: []llmclient.ToolDef{{
			Name:        "search",
			Description: "find things",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			CacheHint:   true,
		}},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	want := `{"model":"model-x","max_tokens":4096,` +
		`"system":[{"type":"text","text":"You are terse.","cache_control":{"type":"ephemeral"}}],` +
		`"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],` +
		`"stop_sequences":["END"],` +
		`"tools":[{"name":"search","description":"find things","input_schema":{"type":"object","properties":{"q":{"type":"string"}}},"cache_control":{"type":"ephemeral"}}]}`
	if string(gotBody) != want {
		t.Errorf("wire shape:\ngot  %s\nwant %s", gotBody, want)
	}
}

func TestMaxTokensAlwaysPresent(t *testing.T) {
	cases := []struct {
		name      string
		cfgMax    int
		reqMax    int
		wantValue float64
	}{
		{"DefaultApplied", 0, 0, 4096},
		{"ConfigDefault", 1234, 0, 1234},
		{"RequestWins", 1234, 99, 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{DefaultMaxTokens: tc.cfgMax})

			if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p", MaxTokens: tc.reqMax}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(gotBody, &m); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			got, ok := m["max_tokens"]
			if !ok {
				t.Fatal("max_tokens missing from body — the API requires it on every request")
			}
			if got != tc.wantValue {
				t.Errorf("max_tokens = %v, want %v", got, tc.wantValue)
			}
		})
	}
}

func TestTemperatureOmittedWhenZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		temp float64
		want bool // present?
	}{
		{"ZeroOmitted", 0, false},
		{"NonZeroSent", 0.7, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{})

			if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p", Temperature: tc.temp}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(gotBody, &m); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			v, present := m["temperature"]
			if present != tc.want {
				t.Fatalf("temperature present = %v, want %v (body %s)", present, tc.want, gotBody)
			}
			if present && v != tc.temp {
				t.Errorf("temperature = %v, want %v", v, tc.temp)
			}
		})
	}
}

func TestModelFallbackAndMissingModel(t *testing.T) {
	t.Run("RequestModelWins", func(t *testing.T) {
		var gotBody []byte
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{DefaultModel: "default-model"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p", Model: "override-model"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if !strings.Contains(string(gotBody), `"model":"override-model"`) {
			t.Errorf("body = %s, want model override-model", gotBody)
		}
	})
	t.Run("DefaultModelFallback", func(t *testing.T) {
		var gotBody []byte
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{DefaultModel: "default-model"})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if !strings.Contains(string(gotBody), `"model":"default-model"`) {
			t.Errorf("body = %s, want model default-model", gotBody)
		}
	})
	t.Run("MissingModelTypedErrorBeforeHTTP", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no HTTP request may be made when the model is missing")
		}))
		t.Cleanup(srv.Close)
		c, err := New(Config{APIKey: "k", BaseURL: srv.URL}) // no DefaultModel
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *llmclient.InvalidRequestError", err)
		}
		// Stream must fail the same way, pre-stream.
		if _, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"}); !errors.As(err, &ire) {
			t.Fatalf("Stream err = %v, want *llmclient.InvalidRequestError", err)
		}
	})
}

func TestCompleteImageBlock(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		writeJSON(w, `{"id":"m","type":"message","role":"assistant","model":"model-x","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
	}), Config{})

	_, err := c.Complete(context.Background(), llmclient.Request{
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			llmclient.ImageBlock("image/png", "aGk="),
			llmclient.TextBlock("what is this?"),
		}}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	for _, want := range []string{
		`"type":"image"`,
		`"source":{"type":"base64","media_type":"image/png","data":"aGk="}`,
	} {
		if !strings.Contains(string(gotBody), want) {
			t.Errorf("image block wire body missing %s:\n%s", want, gotBody)
		}
	}
}

func TestImageBlockRequiresData(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request must not be sent when an image block is invalid")
	}), Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{
		Messages: []llmclient.Message{{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{
			{Type: llmclient.BlockImage, MediaType: "image/png"}, // no Data
		}}},
	})
	if err == nil {
		t.Fatal("expected an error for an image block missing Data")
	}
}

func TestTooManyCacheHints(t *testing.T) {
	hintBlock := llmclient.ContentBlock{Type: llmclient.BlockText, Text: "b", CacheHint: true}
	mk := func(systemHint bool, toolHints, blockHints int) llmclient.Request {
		req := llmclient.Request{
			System:          "sys",
			SystemCacheHint: systemHint,
			Messages: []llmclient.Message{
				{Role: llmclient.RoleUser, Content: []llmclient.ContentBlock{llmclient.TextBlock("hi")}},
			},
		}
		for i := 0; i < toolHints; i++ {
			req.Tools = append(req.Tools, llmclient.ToolDef{
				Name: fmt.Sprintf("t%d", i), InputSchema: json.RawMessage(`{"type":"object"}`), CacheHint: true,
			})
		}
		for i := 0; i < blockHints; i++ {
			req.Messages[0].Content = append(req.Messages[0].Content, hintBlock)
		}
		return req
	}

	t.Run("FiveHintsRejectedBeforeHTTP", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no HTTP request may be made when cache hints exceed the limit")
		}))
		t.Cleanup(srv.Close)
		c, err := New(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = c.Complete(context.Background(), mk(true, 2, 2)) // 1+2+2 = 5
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *llmclient.InvalidRequestError", err)
		}
		if !strings.Contains(ire.Message, "cache breakpoints") {
			t.Errorf("message %q should explain the breakpoint limit", ire.Message)
		}
	})
	t.Run("FourHintsAccepted", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		if _, err := c.Complete(context.Background(), mk(true, 2, 1)); err != nil { // 1+2+1 = 4
			t.Fatalf("4 breakpoints must be accepted, got %v", err)
		}
	})
	t.Run("UnknownBlockHintNotCounted", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, textResponseBody(t, "ok"))
		}), Config{})
		req := mk(true, 2, 1) // 4 counted hints...
		req.Messages[0].Content = append(req.Messages[0].Content, llmclient.ContentBlock{
			Type: llmclient.BlockUnknown, Raw: json.RawMessage(`{"type":"thinking","thinking":"x"}`), CacheHint: true,
		}) // ...plus an ignored one on an unknown block
		if _, err := c.Complete(context.Background(), req); err != nil {
			t.Fatalf("CacheHint on BlockUnknown must be ignored, got %v", err)
		}
	})
}

func TestToolChoiceMapping(t *testing.T) {
	cases := []struct {
		name   string
		choice llmclient.ToolChoice
		want   string // "" means tool_choice must be absent
	}{
		{"AutoOmitted", llmclient.ToolChoice{}, ""},
		{"Any", llmclient.ToolChoice{Mode: llmclient.ToolChoiceAny}, `{"type":"any"}`},
		{"None", llmclient.ToolChoice{Mode: llmclient.ToolChoiceNone}, `{"type":"none"}`},
		{"SpecificTool", llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool, Name: "search"}, `{"type":"tool","name":"search"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{})
			req := llmclient.Request{
				Prompt:     "p",
				Tools:      []llmclient.ToolDef{{Name: "search", InputSchema: json.RawMessage(`{"type":"object"}`)}},
				ToolChoice: tc.choice,
			}
			if _, err := c.Complete(context.Background(), req); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(gotBody, &m); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			raw, present := m["tool_choice"]
			if tc.want == "" {
				if present {
					t.Fatalf("tool_choice must be omitted for auto, got %s", raw)
				}
				return
			}
			if !present {
				t.Fatalf("tool_choice missing (body %s)", gotBody)
			}
			if string(raw) != tc.want {
				t.Errorf("tool_choice = %s, want %s", raw, tc.want)
			}
		})
	}

	t.Run("ToolModeWithoutNameRejected", func(t *testing.T) {
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("no HTTP request may be made for an invalid tool choice")
		}), Config{})
		_, err := c.Complete(context.Background(), llmclient.Request{
			Prompt:     "p",
			ToolChoice: llmclient.ToolChoice{Mode: llmclient.ToolChoiceTool},
		})
		var ire *llmclient.InvalidRequestError
		if !errors.As(err, &ire) {
			t.Fatalf("err = %v, want *llmclient.InvalidRequestError", err)
		}
	})
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		errType string
		headers map[string]string
		check   func(t *testing.T, err error)
	}{
		{
			name: "400Invalid", status: 400, errType: "invalid_request_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
				}
				if !strings.Contains(e.Message, "boom") {
					t.Errorf("message %q should carry the API message", e.Message)
				}
			},
		},
		{
			name: "401Auth", status: 401, errType: "authentication_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.AuthError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want AuthError", err, err)
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
			name: "404Invalid", status: 404, errType: "not_found_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
				}
			},
		},
		{
			name: "413Invalid", status: 413, errType: "request_too_large",
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
				}
			},
		},
		{
			name: "422Invalid", status: 422, errType: "invalid_request_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.InvalidRequestError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want InvalidRequestError", err, err)
				}
			},
		},
		{
			name: "429RateLimitWithSeconds", status: 429, errType: "rate_limit_error",
			headers: map[string]string{"Retry-After": "7"},
			check: func(t *testing.T, err error) {
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want RateLimitError", err, err)
				}
				if e.RetryAfter != 7*time.Second {
					t.Errorf("RetryAfter = %v, want 7s", e.RetryAfter)
				}
			},
		},
		{
			name: "429RateLimitNoHeader", status: 429, errType: "rate_limit_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want RateLimitError", err, err)
				}
				if e.RetryAfter != 0 {
					t.Errorf("RetryAfter = %v, want 0 (unknown)", e.RetryAfter)
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
					t.Fatalf("err = %T %v, want ServerError (529 is retryable)", err, err)
				}
				if e.StatusCode != 529 {
					t.Errorf("StatusCode = %d, want 529", e.StatusCode)
				}
			},
		},
		{
			name: "503Server", status: 503, errType: "api_error",
			check: func(t *testing.T, err error) {
				var e *llmclient.ServerError
				if !errors.As(err, &e) {
					t.Fatalf("err = %T %v, want ServerError", err, err)
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
			_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
			if err == nil {
				t.Fatal("want error, got nil")
			}
			tc.check(t, err)
		})
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	t0 := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", t0.Add(90*time.Second).Format(http.TimeFormat))
		writeAPIError(w, 429, "rate_limit_error", "slow down")
	}), Config{})
	c.now = func() time.Time { return t0 } // injected clock: deterministic HTTP-date parsing

	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var e *llmclient.RateLimitError
	if !errors.As(err, &e) {
		t.Fatalf("err = %T %v, want RateLimitError", err, err)
	}
	if e.RetryAfter != 90*time.Second {
		t.Errorf("RetryAfter = %v, want 90s", e.RetryAfter)
	}
}

func TestMalformedErrorBody(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "<html>gateway exploded</html>")
	}), Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var e *llmclient.ServerError
	if !errors.As(err, &e) {
		t.Fatalf("err = %T %v, want ServerError", err, err)
	}
	if !strings.Contains(e.Message, "gateway exploded") {
		t.Errorf("message %q should carry a body snippet for debuggability", e.Message)
	}
}

func TestMalformedSuccessBody(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"content":[{"type":"text","text":"trunca`) // cut JSON
	}), Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var e *llmclient.ServerError
	if !errors.As(err, &e) {
		t.Fatalf("err = %T %v, want ServerError (garbled body is transient)", err, err)
	}
}

// TestKeyNeverInErrors uses a sentinel credential and a server that echoes it
// in every error body; no error path output may contain the credential. The
// whole suite runs once per auth scheme — the no-leak guarantee applies to
// bearer tokens exactly as it does to API keys (redaction works on the
// resolved credential, not on the header it travels in).
func TestKeyNeverInErrors(t *testing.T) {
	const sentinel = "sk-ant-SENTINEL-never-leak-0123"

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

	for _, scheme := range []string{AuthSchemeAPIKey, AuthSchemeBearer} {
		t.Run(scheme, func(t *testing.T) {
			cfg := func() Config { return Config{APIKey: sentinel, AuthScheme: scheme} }

			statuses := []int{400, 401, 403, 404, 413, 422, 429, 500, 529}
			for _, status := range statuses {
				t.Run(fmt.Sprintf("Status%d", status), func(t *testing.T) {
					c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						writeAPIError(w, status, "invalid_request_error", "your key "+sentinel+" was rejected")
					}), cfg())
					_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
					assertClean(t, err)
					if !strings.Contains(err.Error(), "[redacted]") {
						t.Errorf("server-echoed credential should be redacted, not dropped: %v", err)
					}
				})
			}

			t.Run("MalformedBodyEchoesKey", func(t *testing.T) {
				c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(500)
					io.WriteString(w, "panic: key="+sentinel+" invalid")
				}), cfg())
				_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
				assertClean(t, err)
			})

			t.Run("PreStreamErrorEchoesKey", func(t *testing.T) {
				c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					writeAPIError(w, 401, "authentication_error", "bad key: "+sentinel)
				}), cfg())
				_, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
				assertClean(t, err)
			})

			t.Run("MidStreamErrorEventEchoesKey", func(t *testing.T) {
				c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("content-type", "text/event-stream")
					io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n")
					io.WriteString(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"key "+sentinel+" rejected mid-stream\"}}\n\n")
				}), cfg())
				ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
				if err != nil {
					t.Fatalf("Stream: %v", err)
				}
				var last llmclient.StreamChunk
				for chunk := range ch {
					last = chunk
				}
				assertClean(t, last.Err)
			})

			t.Run("PingErrorEchoesKey", func(t *testing.T) {
				c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					writeAPIError(w, 401, "authentication_error", "token "+sentinel+" expired")
				}), cfg())
				err := c.Ping(context.Background())
				assertClean(t, err)
				if !strings.Contains(err.Error(), "[redacted]") {
					t.Errorf("server-echoed credential should be redacted, not dropped: %v", err)
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — Thinking, Effort, ProviderParams, CapabilityCheck
// ---------------------------------------------------------------------------

// captureBody returns an httptest-backed Client that captures the request body
// and always replies with a minimal text success response.
func captureBody(t *testing.T, cfg Config) (*Client, *[]byte) {
	t.Helper()
	var got []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		writeJSON(w, textResponseBody(t, "ok"))
	}), cfg)
	return c, &got
}

// TestThinkingWireShape proves that a Request.Thinking with Type "enabled" and
// a BudgetTokens value is serialised as the "thinking" wire object, and that
// the stream path (Stream=true in the body) carries the same object.
func TestThinkingWireShape(t *testing.T) {
	c, body := captureBody(t, Config{})

	req := llmclient.Request{
		Prompt:   "think hard",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled", BudgetTokens: 2048},
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(*body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	raw, ok := m["thinking"]
	if !ok {
		t.Fatalf("thinking missing from wire body: %s", *body)
	}
	th, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("thinking is %T, want object", raw)
	}
	if th["type"] != "enabled" {
		t.Errorf("thinking.type = %v, want enabled", th["type"])
	}
	if th["budget_tokens"] != float64(2048) {
		t.Errorf("thinking.budget_tokens = %v, want 2048", th["budget_tokens"])
	}
}

// TestThinkingNilOmitted proves that a nil Thinking field produces no
// "thinking" key in the wire body — zero-value is a clean no-op.
func TestThinkingNilOmitted(t *testing.T) {
	c, body := captureBody(t, Config{})
	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(*body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := m["thinking"]; ok {
		t.Errorf("nil Thinking must not appear in wire body: %s", *body)
	}
}

// TestEffortRejected proves unsupported values fail before any HTTP call.
func TestEffortRejected(t *testing.T) {
	httpCalled := false
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{})

	_, err := c.Complete(context.Background(), llmclient.Request{
		Prompt: "p",
		Effort: "unsupported",
	})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T %v, want *InvalidRequestError for Effort", err, err)
	}
	if !strings.Contains(ire.Error(), "Effort") {
		t.Errorf("error should name Effort: %q", ire.Error())
	}
	if httpCalled {
		t.Error("Effort rejection must fail before any HTTP call")
	}
}

// TestProviderParamsMerge is the key ProviderParams test: a JSON object set on
// Request.ProviderParams is merged verbatim into the wire body, and keys the
// adapter already set are OVERRIDDEN by ProviderParams (documented precedence).
func TestProviderParamsMerge(t *testing.T) {
	c, body := captureBody(t, Config{})

	// Override the adapter-set "model" key and add a new "extra_field".
	req := llmclient.Request{
		Prompt:         "p",
		ProviderParams: json.RawMessage(`{"model":"overridden-model","extra_field":"hello"}`),
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(*body, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	// ProviderParams overrides the adapter-set model.
	if m["model"] != "overridden-model" {
		t.Errorf("model = %v, want overridden-model (ProviderParams must override adapter-set keys)", m["model"])
	}
	// New keys from ProviderParams appear verbatim.
	if m["extra_field"] != "hello" {
		t.Errorf("extra_field = %v, want hello", m["extra_field"])
	}
	// The adapter's own fields that ProviderParams did NOT mention are preserved.
	if _, ok := m["max_tokens"]; !ok {
		t.Errorf("max_tokens absent from merged body — adapter-set non-overridden keys must survive")
	}
}

// TestProviderParamsEmptyIsNoOp proves nil and empty ProviderParams do not
// affect the wire body (no extra keys, no errors, same body as a baseline).
func TestProviderParamsEmptyIsNoOp(t *testing.T) {
	c, body := captureBody(t, Config{})
	_, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p", ProviderParams: nil})
	if err != nil {
		t.Fatalf("Complete with nil ProviderParams: %v", err)
	}
	nilBody := string(*body)

	c2, body2 := captureBody(t, Config{})
	_, err = c2.Complete(context.Background(), llmclient.Request{Prompt: "p", ProviderParams: json.RawMessage("")})
	if err != nil {
		t.Fatalf("Complete with empty ProviderParams: %v", err)
	}
	if nilBody != string(*body2) {
		t.Errorf("empty ProviderParams changed wire body:\nnil   %s\nempty %s", nilBody, *body2)
	}
}

// TestProviderParamsNonObjectRejected proves the adapter returns a typed
// InvalidRequestError for ProviderParams that is not a JSON object, before
// any HTTP call.
func TestProviderParamsNonObjectRejected(t *testing.T) {
	cases := []struct {
		name string
		pp   json.RawMessage
	}{
		{"array", json.RawMessage(`[1,2,3]`)},
		{"string", json.RawMessage(`"hello"`)},
		{"number", json.RawMessage(`42`)},
		{"null", json.RawMessage(`null`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			httpCalled := false
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				httpCalled = true
				writeJSON(w, textResponseBody(t, "ok"))
			}), Config{})

			_, err := c.Complete(context.Background(), llmclient.Request{
				Prompt:         "p",
				ProviderParams: tc.pp,
			})
			var ire *llmclient.InvalidRequestError
			if !errors.As(err, &ire) {
				t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
			}
			if !strings.Contains(ire.Error(), "ProviderParams") {
				t.Errorf("error should name ProviderParams: %q", ire.Error())
			}
			if httpCalled {
				t.Error("ProviderParams rejection must fail before any HTTP call")
			}
		})
	}
}

// TestCapabilityCheckCalled proves that Config.CapabilityCheck is invoked
// before any HTTP when set, and that its error is returned as-is.
func TestCapabilityCheckCalled(t *testing.T) {
	httpCalled := false
	checkErr := &llmclient.InvalidRequestError{Message: "capability-check: model does not support thinking"}

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{
		CapabilityCheck: func(req llmclient.Request) error {
			// Simulate: reject if thinking is requested.
			if req.Thinking != nil {
				return checkErr
			}
			return nil
		},
	})

	// Request without thinking: check passes, HTTP proceeds.
	if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
		t.Fatalf("check should pass for no-thinking request: %v", err)
	}
	if !httpCalled {
		t.Error("HTTP must be called when CapabilityCheck passes")
	}

	// Request with thinking: check fires and returns the error pre-HTTP.
	httpCalled = false
	_, err := c.Complete(context.Background(), llmclient.Request{
		Prompt:   "think",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled"},
	})
	if err != checkErr {
		t.Fatalf("CapabilityCheck error not returned: got %v, want %v", err, checkErr)
	}
	if httpCalled {
		t.Error("HTTP must not be called when CapabilityCheck returns an error")
	}
}

// TestCapabilityCheckNilPermissive proves that a nil CapabilityCheck is a
// no-op: all requests proceed to HTTP without error.
func TestCapabilityCheckNilPermissive(t *testing.T) {
	httpCalled := false
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalled = true
		writeJSON(w, textResponseBody(t, "ok"))
	}), Config{CapabilityCheck: nil})

	if _, err := c.Complete(context.Background(), llmclient.Request{
		Prompt:   "p",
		Thinking: &llmclient.ThinkingConfig{Type: "enabled"},
	}); err != nil {
		t.Fatalf("nil CapabilityCheck: unexpected error %v", err)
	}
	if !httpCalled {
		t.Error("HTTP must be called when CapabilityCheck is nil")
	}
}

// TestProviderParamsMergeOnStream proves that ProviderParams is also merged
// on the streaming path (the Stream method calls buildBody with stream=true).
func TestProviderParamsMergeOnStream(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		// Minimal SSE response that satisfies the stream parser.
		w.Header().Set("content-type", "text/event-stream")
		io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
		io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n")
		io.WriteString(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		io.WriteString(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n")
		io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}), Config{})

	ch, err := c.Stream(context.Background(), llmclient.Request{
		Prompt:         "p",
		ProviderParams: json.RawMessage(`{"stream_extra":"yes"}`),
	})
	if err != nil {
		t.Fatalf("Stream pre-stream error: %v", err)
	}
	for range ch {
	} // drain

	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if m["stream_extra"] != "yes" {
		t.Errorf("stream_extra = %v, want yes; ProviderParams must apply on Stream path too", m["stream_extra"])
	}
	// The adapter's own stream:true must survive (not overridden by our params).
	if m["stream"] != true {
		t.Errorf("stream = %v, want true (adapter stream flag must survive ProviderParams merge)", m["stream"])
	}
}

// ---------------------------------------------------------------------------
// Wave 1 — OutputSchema and ToolDef.Strict
// ---------------------------------------------------------------------------

// TestOutputSchemaWireShape proves that a non-empty Request.OutputSchema is
// mapped to output_config.format.{type:"json_schema", schema:...} in the wire
// body, and that the field is absent when OutputSchema is nil.
func TestOutputSchemaWireShape(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)

	t.Run("Present", func(t *testing.T) {
		c, body := captureBody(t, Config{})
		req := llmclient.Request{
			Prompt:       "answer me",
			OutputSchema: schema,
		}
		if _, err := c.Complete(context.Background(), req); err != nil {
			t.Fatalf("Complete: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal(*body, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		oc, ok := m["output_config"].(map[string]any)
		if !ok {
			t.Fatalf("output_config missing or not an object; body: %s", *body)
		}
		format, ok := oc["format"].(map[string]any)
		if !ok {
			t.Fatalf("output_config.format missing or not an object; output_config: %v", oc)
		}
		if format["type"] != "json_schema" {
			t.Errorf("output_config.format.type = %v, want json_schema", format["type"])
		}
		if format["schema"] == nil {
			t.Errorf("output_config.format.schema missing; format: %v", format)
		}
	})

	t.Run("AbsentWhenNil", func(t *testing.T) {
		c, body := captureBody(t, Config{})
		if _, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"}); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(*body, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if _, ok := m["output_config"]; ok {
			t.Errorf("output_config must be absent when OutputSchema is nil; body: %s", *body)
		}
	})

	t.Run("NonObjectRejectedPreHTTP", func(t *testing.T) {
		cases := []struct {
			name string
			s    json.RawMessage
		}{
			{"array", json.RawMessage(`[1,2]`)},
			{"string", json.RawMessage(`"hi"`)},
			{"number", json.RawMessage(`42`)},
			{"null", json.RawMessage(`null`)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				httpCalled := false
				c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httpCalled = true
					writeJSON(w, textResponseBody(t, "ok"))
				}), Config{})
				_, err := c.Complete(context.Background(), llmclient.Request{
					Prompt:       "p",
					OutputSchema: tc.s,
				})
				var ire *llmclient.InvalidRequestError
				if !errors.As(err, &ire) {
					t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
				}
				if !strings.Contains(ire.Error(), "OutputSchema") {
					t.Errorf("error should name OutputSchema: %q", ire.Error())
				}
				if httpCalled {
					t.Error("OutputSchema validation must fail before any HTTP call")
				}
			})
		}
	})

	// OutputSchema passes through on the Stream path identically.
	t.Run("StreamPath", func(t *testing.T) {
		var gotBody []byte
		c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("content-type", "text/event-stream")
			f := w.(http.Flusher)
			streamPreamble(w, f)
			streamTextDeltas(t, w, f, "ok")
			streamFinish(w, f, "end_turn")
		}), Config{})

		ch, err := c.Stream(context.Background(), llmclient.Request{
			Prompt:       "p",
			OutputSchema: schema,
		})
		if err != nil {
			t.Fatalf("Stream: %v", err)
		}
		drainStream(t, ch)

		var m map[string]any
		if err := json.Unmarshal(gotBody, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if _, ok := m["output_config"]; !ok {
			t.Errorf("output_config missing on Stream path; body: %s", gotBody)
		}
	})
}

// TestOutputSchemaGoldenWireBody is a golden string test pinning the exact
// JSON shape of output_config.format in the wire body.
func TestOutputSchemaGoldenWireBody(t *testing.T) {
	c, body := captureBody(t, Config{})
	req := llmclient.Request{
		Prompt:       "go",
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"score":{"type":"integer"}}}`),
	}
	if _, err := c.Complete(context.Background(), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// The golden output_config fragment that must appear in the wire body.
	// Field order is determined by Go's json.Marshal (struct field order).
	const wantFragment = `"output_config":{"format":{"type":"json_schema","schema":{"type":"object","properties":{"score":{"type":"integer"}}}}}`
	if !strings.Contains(string(*body), wantFragment) {
		t.Errorf("wire body missing expected output_config fragment:\ngot  %s\nwant substring %s", *body, wantFragment)
	}
}

// A raw output_config must not silently remove a typed schema.
func TestOutputSchemaProviderParamsConflict(t *testing.T) {
	c, body := captureBody(t, Config{})
	req := llmclient.Request{
		Prompt:         "p",
		OutputSchema:   json.RawMessage(`{"type":"object"}`),
		ProviderParams: json.RawMessage(`{"output_config":{"format":{"type":"text"}}}`),
	}
	_, err := c.Complete(context.Background(), req)
	var invalid *llmclient.InvalidRequestError
	if !errors.As(err, &invalid) || len(*body) != 0 {
		t.Fatalf("conflict must fail before HTTP: body=%s error=%v", *body, err)
	}
}

// TestToolDefStrictWireShape proves ToolDef.Strict=true adds "strict":true to
// the typed tool wire object, and that Strict=false omits the field.
func TestToolDefStrictWireShape(t *testing.T) {
	t.Run("StrictTrue", func(t *testing.T) {
		c, body := captureBody(t, Config{})
		req := llmclient.Request{
			Prompt: "p",
			Tools: []llmclient.ToolDef{{
				Name:        "calc",
				Description: "a calculator",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"number"}}}`),
				Strict:      true,
			}},
		}
		if _, err := c.Complete(context.Background(), req); err != nil {
			t.Fatalf("Complete: %v", err)
		}

		var m map[string]json.RawMessage
		if err := json.Unmarshal(*body, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(m["tools"], &tools); err != nil {
			t.Fatalf("unmarshal tools: %v", err)
		}
		if len(tools) != 1 {
			t.Fatalf("tools len = %d, want 1", len(tools))
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
		c, body := captureBody(t, Config{})
		req := llmclient.Request{
			Prompt: "p",
			Tools: []llmclient.ToolDef{{
				Name:        "calc",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				Strict:      false,
			}},
		}
		if _, err := c.Complete(context.Background(), req); err != nil {
			t.Fatalf("Complete: %v", err)
		}

		var m map[string]json.RawMessage
		if err := json.Unmarshal(*body, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		var tools []map[string]json.RawMessage
		if err := json.Unmarshal(m["tools"], &tools); err != nil {
			t.Fatalf("unmarshal tools: %v", err)
		}
		if _, ok := tools[0]["strict"]; ok {
			t.Errorf("strict must be omitted when Strict=false; got %v", tools[0])
		}
	})

	t.Run("RawToolStrictIgnored", func(t *testing.T) {
		// When ToolDef.Raw is set, Strict is ignored — the caller owns the JSON.
		// The raw tool is emitted verbatim; no "strict" injection happens.
		c, body := captureBody(t, Config{})
		rawTool := json.RawMessage(`{"name":"raw_tool","description":"raw","input_schema":{"type":"object"}}`)
		req := llmclient.Request{
			Prompt: "p",
			Tools: []llmclient.ToolDef{{
				Name:   "raw_tool",
				Raw:    rawTool,
				Strict: true, // ignored when Raw is set
			}},
		}
		if _, err := c.Complete(context.Background(), req); err != nil {
			t.Fatalf("Complete: %v", err)
		}

		var m map[string]json.RawMessage
		if err := json.Unmarshal(*body, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		var tools []json.RawMessage
		if err := json.Unmarshal(m["tools"], &tools); err != nil {
			t.Fatalf("unmarshal tools: %v", err)
		}
		if string(tools[0]) != string(rawTool) {
			t.Errorf("raw tool was not emitted verbatim:\ngot  %s\nwant %s", tools[0], rawTool)
		}
	})
}
