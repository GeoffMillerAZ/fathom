package typesafe

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
)

// DefaultTypeSafeBaseURL is the BaseURL TypeSafeConfig uses when unset.
const DefaultTypeSafeBaseURL = "https://api.typesafe.ai"

// DefaultTypeSafeModel is the Model TypeSafeConfig uses when unset. It is
// TypeSafe's rolling alias for the latest System One model (currently
// jev-1.13.0) — the response's Model always names the version that
// actually ran.
const DefaultTypeSafeModel = "jev-latest"

// typesafeEndpoint is the fixed path appended to TypeSafeConfig.BaseURL for
// every request.
const typesafeEndpoint = "/v1/systemone"

// typesafeErrorBodyCap bounds TypeSafeError.Message to 512 bytes of the
// response body.
const typesafeErrorBodyCap = 512

// TypeSafeConfig configures a TypeSafe Decider.
type TypeSafeConfig struct {
	// APIKey is the TypeSafe System One API key, sent as "Authorization:
	// Bearer <APIKey>" on every request. Required — NewTypeSafe errors when
	// empty. Never appears in an error message or log line this package
	// produces.
	APIKey string
	// BaseURL overrides DefaultTypeSafeBaseURL. A trailing slash is
	// trimmed; requests go to BaseURL+"/v1/systemone".
	BaseURL string
	// Model overrides DefaultTypeSafeModel.
	Model string
	// HTTPClient overrides the default &http.Client{Timeout: 30 *
	// time.Second}.
	HTTPClient *http.Client
	// Now overrides the clock used to compute Response.Latency. The default
	// is time.Now; tests use this to pin Latency deterministically.
	Now func() time.Time
}

// TypeSafe is a decision.Decider backed directly by TypeSafe AI's System
// One API (POST {BaseURL}/v1/systemone): every Question in a Decide call's
// Request is rendered into one request and answered in one round trip,
// mirroring llm.LLM's one-completion-per-batch contract but speaking
// TypeSafe's native wire shape instead of a generic llmclient completion
// asked to emulate it.
//
// # Kind mapping
//
// The package's three generic kinds map onto System One's primitives. The
// vendor's names live here and in the adapter's wire code only; the
// predicate kind's vendor term "noul" is also accepted as a decode alias
// (see ParseKind).
//
//	decision.Kind    System One primitive   Shape
//	KindChoice       Choice                 N named options -> a
//	                                         probability distribution
//	KindScore        Score                  an ordered set of 2..10
//	                                         Levels (explicit, or
//	                                         synthesized from an
//	                                         integral [Min, Max])
//	KindPredicate    Noul                   a probability that a
//	                                         stated criterion is true
//
// The wire request batches every Question in a Decide call's Request into
// ONE POST body keyed by Question.ID (a Go map — deterministic on the wire
// because encoding/json always emits map[string]T keys sorted); the reply
// is the same shape keyed by Question.ID back. A KindScore question's
// Levels are sent verbatim as the "score" criteria; with Levels empty, an
// integral [Min, Max] range of 2..10 values is synthesized into integer
// levels. A KindScore question with neither cannot be sent to the "score"
// primitive at all, and one whose level count is outside the declared 2..10
// cannot either: both are recorded in Response.Invalid instead, never
// reaching the wire. The wire response's own "confidence" field is never
// trusted: Confidence is recomputed by decision.Confidence (see the decision
// package doc).
//
// Concurrency: TypeSafe holds no mutable state after construction — API
// key, base URL, model, HTTP client and clock are fixed at NewTypeSafe time
// — so it is safe for concurrent use.
type TypeSafe struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	now        func() time.Time
}

// NewTypeSafe constructs a TypeSafe Decider over cfg. It returns an error
// when cfg.APIKey is empty; every other field defaults per TypeSafeConfig's
// doc comment. A caller with no TypeSafe API key configured should fall
// back to another Decider (e.g. llm.LLM, or decision.None) rather than call
// NewTypeSafe at all — see the package doc.
func NewTypeSafe(cfg TypeSafeConfig) (*TypeSafe, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("decision: TypeSafeConfig.APIKey must not be empty")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultTypeSafeBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	model := cfg.Model
	if model == "" {
		model = DefaultTypeSafeModel
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &TypeSafe{
		apiKey:     cfg.APIKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: httpClient,
		now:        now,
	}, nil
}

// Name returns "typesafe".
func (t *TypeSafe) Name() string { return "typesafe" }

// Decide sends req.State and req.Questions to TypeSafe's System One API as
// ONE request (see the package doc for the wire shape) and parses the
// reply.
//
// Per question, the reply is validated against the Question and classified
// exactly like llm.LLM.Decide: a valid answer lands in Response.Answers; a
// missing id, a type mismatch, a choice not among the Question's Options,
// an out-of-range score/noul, or probabilities that do not sum to within
// 0.01 of 1 lands in Response.Invalid instead, naming the reason — Decide
// never fails the whole batch for one bad question. A question outside the
// limits TypeSafe declares in its Capabilities (a score with more than 10
// or fewer than 2 levels, whether explicit Levels or an integral [Min, Max]
// range: the score-level limit is the vendor's, which Validate no longer
// enforces, DP-13), or a KindScore question that cannot be rendered at all
// (no Levels and no integral [Min, Max] range), is likewise recorded in
// Invalid, with a reason naming the limit, and never sent. Confidence is
// always computed by decision.Confidence (never trusted from the wire's own
// "confidence" field — see the decision package doc's confidence
// invariant).
//
// Decide returns an error only when req fails Validate, building or
// issuing the HTTP request fails (including ctx cancellation), or the
// reply is not JSON at all. A non-2xx HTTP response is reported as a
// *TypeSafeError. Decide issues no automatic retries — a caller wanting
// backoff on a transient failure checks TypeSafeError.Retryable().
func (t *TypeSafe) Decide(ctx context.Context, req decision.Request) (decision.Response, error) {
	if err := decision.Validate(req); err != nil {
		return decision.Response{}, err
	}

	start := t.now()

	wireReq := typesafeWireRequest{
		Model:     t.model,
		State:     req.State,
		Questions: make(map[string]typesafeWireQuestion, len(req.Questions)),
	}
	plans := make(map[string]typesafeScorePlan, len(req.Questions))
	invalid := make(map[string]string)

	for _, q := range req.Questions {
		wire, plan, reason, ok := typesafeRenderQuestion(q)
		if !ok {
			invalid[q.ID] = reason
			continue
		}
		wireReq.Questions[q.ID] = wire
		if q.Kind == decision.KindScore {
			plans[q.ID] = plan
		}
	}

	if len(wireReq.Questions) == 0 {
		return decision.Response{
			Provider: t.Name(),
			Model:    t.model,
			Answers:  map[string]decision.Answer{},
			Invalid:  invalid,
			Latency:  t.now().Sub(start),
		}, nil
	}

	body, err := json.MarshalIndent(wireReq, "", "  ")
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: marshaling typesafe request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+typesafeEndpoint, bytes.NewReader(body))
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: building typesafe request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+t.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: typesafe request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return decision.Response{}, fmt.Errorf("decision: reading typesafe response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return decision.Response{}, &TypeSafeError{Status: httpResp.StatusCode, Message: typesafeTruncate(respBody, typesafeErrorBodyCap)}
	}

	var env typesafeWireResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		return decision.Response{}, fmt.Errorf("decision: typesafe response is not JSON: %w", err)
	}

	answers := make(map[string]decision.Answer, len(wireReq.Questions))
	for _, q := range req.Questions {
		if _, skipped := invalid[q.ID]; skipped {
			continue
		}
		raw, ok := env.Answers[q.ID]
		if !ok {
			invalid[q.ID] = "no answer"
			continue
		}
		a, reason, ok := typesafeParseAnswer(q, raw, plans[q.ID])
		if !ok {
			invalid[q.ID] = reason
			continue
		}
		answers[q.ID] = a
	}

	return decision.Response{
		Provider: t.Name(),
		Model:    env.Model,
		Answers:  answers,
		Invalid:  invalid,
		Usage: decision.Usage{
			InputTokens:  env.Usage.InputTokens,
			OutputTokens: env.Usage.OutputTokens,
		},
		Latency: t.now().Sub(start),
	}, nil
}

// Ping sends one minimal predicate question ("The statement is a ping.",
// the vendor's "noul" primitive) and reports success when TypeSafe answers
// it: a 2xx HTTP response carrying a parseable noul answer. Ping satisfies
// llmclient.Pinger's method set (same signature, same nil-means-healthy
// contract) without importing llmclient, so TypeSafe can be probed
// alongside llmclient adapters via llmclient.ProbeAll without this package
// depending on it.
func (t *TypeSafe) Ping(ctx context.Context) error {
	req := decision.Request{
		State: "ping",
		Questions: []decision.Question{
			{ID: "ping", Kind: decision.KindPredicate, Instructions: "The statement is a ping."},
		},
	}
	resp, err := t.Decide(ctx, req)
	if err != nil {
		return err
	}
	if _, ok := resp.Answers["ping"]; ok {
		return nil
	}
	reason := resp.Invalid["ping"]
	if reason == "" {
		reason = "no answer"
	}
	return fmt.Errorf("decision: typesafe ping: %s", reason)
}

// typesafeTruncate returns body as a string, cut to at most n bytes.
func typesafeTruncate(body []byte, n int) string {
	if len(body) > n {
		body = body[:n]
	}
	return string(body)
}

// TypeSafeError signals a non-2xx HTTP response from TypeSafe's System One
// API. Message is the response body trimmed to 512 bytes; it and Error()
// never contain the API key, which is never echoed back by the API and
// never assembled into this error by this package.
type TypeSafeError struct {
	// Status is the HTTP status code of the failed response.
	Status int
	// Message is the response body, trimmed to 512 bytes.
	Message string
}

// Error implements the error interface, naming the HTTP status and the
// (truncated) response body — never the API key.
func (e *TypeSafeError) Error() string {
	return fmt.Sprintf("typesafe: status %d: %s", e.Status, e.Message)
}

// Retryable reports whether the failure is transient and worth retrying
// with backoff: true for 429 (rate limited) and 529 (overloaded), false
// for every other status (e.g. 401 missing/invalid key, 422 body failed
// validation — retrying an identical request cannot succeed). Decide
// itself issues no automatic retries — see its doc comment — so a caller
// wanting backoff calls Retryable() itself.
func (e *TypeSafeError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == 529
}
