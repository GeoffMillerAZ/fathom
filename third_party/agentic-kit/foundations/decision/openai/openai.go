package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/internal/apikey"
)

// DefaultOpenAIBaseURL is the BaseURL OpenAIConfig uses when unset, in the
// OpenAI SDKs' convention (the API version is part of the base URL).
const DefaultOpenAIBaseURL = "https://api.openai.com/v1"

// DefaultOpenAIModel is the Model OpenAIConfig uses when unset: the only
// model the Decisions API supports (OpenAI's Decisions guide, 2026-10-09).
const DefaultOpenAIModel = "gpt-6-luna"

// openaiEndpoint is the path appended to OpenAIConfig.BaseURL for every
// request.
const openaiEndpoint = "/decisions"

// openaiErrorBodyCap bounds OpenAIError.Message to 512 bytes of the
// redacted response body.
const openaiErrorBodyCap = 512

// OpenAIConfig configures an OpenAI Decider.
//
// The key, sent as "Authorization: Bearer <key>" on every request, comes
// from exactly one source, tried in this order: APIKey (set in code),
// APIKeyFile, APIKeyEnv. NewOpenAI errors when none is set. Configuration
// names a file or a variable, never the key itself.
type OpenAIConfig struct {
	// APIKey is the key itself, for a caller that resolved it in code (a
	// keychain, a secret manager). It wins over APIKeyFile and APIKeyEnv.
	APIKey string
	// APIKeyFile is the path of a file holding the key ("~" expands to the
	// home directory); it must grant no access to group or other users
	// (chmod 600). Read at the first Decide. It wins over APIKeyEnv.
	APIKeyFile string
	// APIKeyEnv is the name of an environment variable holding the key,
	// for example "OPENAI_API_KEY". Read at the first Decide.
	APIKeyEnv string
	// BaseURL overrides DefaultOpenAIBaseURL. A trailing slash is trimmed;
	// requests go to BaseURL+"/decisions".
	BaseURL string
	// Model overrides DefaultOpenAIModel.
	Model string
	// HTTPClient overrides the default &http.Client{Timeout: 30 *
	// time.Second}.
	HTTPClient *http.Client
	// Now overrides the clock used to compute Response.Latency. The default
	// is time.Now; tests use this to pin Latency deterministically.
	Now func() time.Time
}

// OpenAI is a decision.Decider backed by OpenAI's Decisions API: every
// Question in a Decide call's Request is rendered into one request and
// answered in one round trip. See the package doc for the wire, the answer
// mapping, refusals and credentials.
//
// Concurrency: OpenAI is safe for concurrent use. Its base URL, model, HTTP
// client and clock are fixed at NewOpenAI time; the key read from a file or
// variable at first use is guarded by a sync.Mutex.
type OpenAI struct {
	key        *apikey.Lazy
	baseURL    string
	model      string
	httpClient *http.Client
	now        func() time.Time
}

// NewOpenAI constructs an OpenAI Decider over cfg. It returns an error when
// cfg sets no key source (APIKey, APIKeyFile or APIKeyEnv); every other
// field defaults per OpenAIConfig's doc comment. It reads no key file or
// variable: that happens at the first Decide. A caller with no key
// configured builds another Decider (decision.None, say) rather than call
// NewOpenAI at all; OpenAI never falls back on its own.
func NewOpenAI(cfg OpenAIConfig) (*OpenAI, error) {
	src := apikey.Source{Value: cfg.APIKey, File: cfg.APIKeyFile, Env: cfg.APIKeyEnv}
	if !src.Configured() {
		return nil, fmt.Errorf("decision: OpenAIConfig sets no API key: set APIKeyFile or APIKeyEnv (or APIKey in code)")
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultOpenAIBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = DefaultOpenAIModel
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &OpenAI{
		key:        apikey.NewLazy(src, apikey.Resolver{}),
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		model:      model,
		httpClient: httpClient,
		now:        now,
	}, nil
}

// Name returns "openai".
func (o *OpenAI) Name() string { return "openai" }

// Decide sends req to the Decisions API as ONE request (see the package doc
// for the wire) and maps the reply, per question: a valid answer lands in
// Response.Answers, a refusal in Response.Declined, and a malformed or
// out-of-range answer, or one the reply omits, in Response.Invalid with the
// reason. A KindScore question with neither Levels nor an integral
// [Min, Max] range cannot be rendered and lands in Invalid without reaching
// the wire.
//
// Decide returns an error only when req fails Validate, the key cannot be
// read (the error names its source), building or issuing the HTTP request
// fails (including ctx cancellation), the reply is a non-2xx status (an
// *OpenAIError), or the reply is not JSON at all. It issues no automatic
// retries: a caller wanting backoff checks OpenAIError.Retryable.
func (o *OpenAI) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if err := decision.Validate(req); err != nil {
		return decision.Response{}, err
	}
	start := o.now()

	wireReq, plans, invalid := openaiRenderRequest(o.model, req)
	if len(wireReq.Questions) == 0 {
		return decision.Response{
			Provider: o.Name(),
			Model:    o.model,
			Answers:  map[string]decision.Answer{},
			Invalid:  invalid,
			Latency:  o.now().Sub(start),
		}, nil
	}

	key, err := o.key.Key()
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: openai: API key from %s: %w", o.key.Source(), err)
	}

	body, err := json.MarshalIndent(wireReq, "", "  ")
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: marshaling openai request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+openaiEndpoint, bytes.NewReader(body))
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: building openai request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: openai request: %w", err)
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: reading openai response: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		oe := &OpenAIError{Status: httpResp.StatusCode, Message: openaiTruncate(apikey.Redact(string(respBody), key), openaiErrorBodyCap)}
		var body struct {
			Error struct {
				Type string `json:"type"`
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &body) == nil {
			oe.Type, oe.Code = body.Error.Type, body.Error.Code
		}
		return decision.Response{}, oe
	}

	var env openaiWireResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		return decision.Response{}, fmt.Errorf("decision: openai response is not JSON: %w", err)
	}
	answers, declined := openaiMapAnswers(req, env, plans, invalid)

	model := env.Model
	if model == "" {
		model = o.model
	}
	resp := decision.Response{
		Provider: o.Name(),
		Model:    model,
		Answers:  answers,
		Invalid:  invalid,
		Latency:  o.now().Sub(start),
	}
	if len(declined) > 0 {
		resp.Declined = declined
	}
	if env.Usage != nil {
		resp.Usage = decision.Usage{InputTokens: env.Usage.InputTokens, OutputTokens: env.Usage.OutputTokens}
	}
	return resp, nil
}

// Ping sends one minimal predicate question and reports success when the
// API answers it: a 2xx reply carrying a valid answer. A refusal is a
// failure. Ping satisfies llmclient.Pinger's method set (same signature,
// same nil-means-healthy contract) without importing llmclient.
func (o *OpenAI) Ping(ctx context.Context) error {
	req := decision.Request{
		State: "ping",
		Questions: []decision.Question{
			{ID: "ping", Kind: decision.KindPredicate, Instructions: "The statement is a ping."},
		},
	}
	resp, err := o.Decide(ctx, req)
	if err != nil {
		return err
	}
	if _, ok := resp.Answers["ping"]; ok {
		return nil
	}
	reason := resp.Invalid["ping"]
	if why, declined := resp.Declined["ping"]; declined {
		reason = "declined: " + why
	}
	if reason == "" {
		reason = "no answer"
	}
	return fmt.Errorf("decision: openai ping: %s", reason)
}

// openaiTruncate returns s cut to at most n bytes.
func openaiTruncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// OpenAIError signals a non-2xx HTTP response from the Decisions API.
// Message is the response body with the key, any Authorization header or
// bearer token it echoes and any "sk-" key shape redacted, then trimmed to
// 512 bytes; it and Error() never contain the API key.
type OpenAIError struct {
	// Status is the HTTP status code of the failed response.
	Status int
	// Message is the response body, redacted and trimmed to 512 bytes.
	Message string
	// Type and Code are the body's error.type and error.code when it is
	// OpenAI's JSON error envelope, else empty. A 429 whose Type is
	// "insufficient_quota" is an account with no credit left (Code
	// "credit_balance_exhausted" in the live reply of 2026-10-09), not a
	// rate limit.
	Type, Code string
}

// openaiQuotaType is the error.type OpenAI sends with a 429 when the
// account has no quota or credit left.
const openaiQuotaType = "insufficient_quota"

// QuotaExhausted reports whether the failure is an account with no quota or
// credit left: a 429 of type "insufficient_quota". Retrying cannot succeed
// until someone adds credit.
func (e *OpenAIError) QuotaExhausted() bool {
	return e.Status == http.StatusTooManyRequests && e.Type == openaiQuotaType
}

// Error implements the error interface, naming the HTTP status and the
// redacted, truncated response body.
func (e *OpenAIError) Error() string {
	return fmt.Sprintf("openai: status %d: %s", e.Status, e.Message)
}

// Retryable reports whether the failure is transient and worth retrying
// with backoff: true for 429 (rate limited) and 500, 502, 503 and 504
// (server or gateway trouble), false for every other status (401 bad key,
// 400 rejected body: an identical request cannot succeed) and for a 429
// that is QuotaExhausted (no credit left). Decide issues no automatic
// retries.
func (e *OpenAIError) Retryable() bool {
	if e.QuotaExhausted() {
		return false
	}
	switch e.Status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}
