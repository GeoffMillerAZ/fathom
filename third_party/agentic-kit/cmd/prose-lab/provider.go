package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// lane is study data. Prices are micro-USD per million tokens, not hard-coded
// vendor pricing. Reservation rates must cover the selected model's highest tier.
type lane struct {
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	ResolvedModel      string `json:"resolved_model,omitempty"`
	PricingSource      string `json:"pricing_source"`
	MaxInputTokens     int64  `json:"max_input_tokens"`
	MaxOutputTokens    int64  `json:"max_output_tokens"`
	InputRate          int64  `json:"input_rate"`
	OutputRate         int64  `json:"output_rate"`
	CacheWriteRate     int64  `json:"cache_write_rate"`
	CacheReadRate      int64  `json:"cache_read_rate"`
	ReserveInputRate   int64  `json:"reserve_input_rate"`
	ReserveOutputRate  int64  `json:"reserve_output_rate"`
	TierThreshold      int64  `json:"tier_threshold"`
	HighInputRate      int64  `json:"high_input_rate"`
	HighOutputRate     int64  `json:"high_output_rate"`
	HighCacheWriteRate int64  `json:"high_cache_write_rate"`
	HighCacheReadRate  int64  `json:"high_cache_read_rate"`
}

func (l lane) validate() error {
	if (l.Provider != "anthropic" && l.Provider != "openrouter-jev") || l.Model == "" || l.PricingSource == "" || l.MaxInputTokens < 1 || l.MaxInputTokens > 2_000_000 || l.MaxOutputTokens < 0 || l.MaxOutputTokens > 131072 {
		return errors.New("invalid study lane")
	}
	for _, v := range []int64{l.InputRate, l.OutputRate, l.CacheWriteRate, l.CacheReadRate, l.ReserveInputRate, l.ReserveOutputRate, l.HighInputRate, l.HighOutputRate, l.HighCacheWriteRate, l.HighCacheReadRate} {
		if v < 0 || v > 100_000_000 {
			return errors.New("invalid pricing bound")
		}
	}
	if l.TierThreshold < 0 || l.TierThreshold > l.MaxInputTokens {
		return errors.New("invalid pricing threshold")
	}
	if l.TierThreshold > 0 && (l.HighInputRate < l.InputRate || l.HighOutputRate < l.OutputRate || l.HighCacheWriteRate < l.CacheWriteRate || l.HighCacheReadRate < l.CacheReadRate || l.ReserveInputRate < l.HighInputRate || l.ReserveInputRate < l.HighCacheWriteRate || l.ReserveInputRate < l.HighCacheReadRate || l.ReserveOutputRate < l.HighOutputRate) {
		return errors.New("reservation does not cover high pricing tier")
	}
	if l.TierThreshold == 0 && (l.HighInputRate != 0 || l.HighOutputRate != 0 || l.HighCacheWriteRate != 0 || l.HighCacheReadRate != 0) {
		return errors.New("high pricing requires threshold")
	}
	if l.InputRate == 0 || l.ReserveInputRate < l.InputRate || l.ReserveOutputRate < l.OutputRate || l.ReserveInputRate < l.CacheWriteRate || l.ReserveInputRate < l.CacheReadRate || l.ReserveInputRate == 0 || (l.Provider == "anthropic" && (l.MaxOutputTokens == 0 || l.OutputRate == 0)) {
		return errors.New("reservation does not cover declared pricing")
	}
	if l.Provider == "openrouter-jev" && (l.OutputRate != 0 || l.ReserveOutputRate != 0 || l.HighOutputRate != 0) {
		return errors.New("Jev study requires documented free output pricing")
	}
	if !modelIDPattern.MatchString(l.Model) || (l.ResolvedModel != "" && !modelIDPattern.MatchString(l.ResolvedModel)) {
		return errors.New("model identities must be exact IDs")
	}
	return nil
}

func (l lane) acceptsModel(model string) bool {
	return model == l.Model || (l.ResolvedModel != "" && model == l.ResolvedModel)
}

func tokenCost(n, rate int64) int64 { return (n*rate + 999_999) / 1_000_000 }
func (l lane) reserveCost() int64 {
	// Usage settlement rounds input, cache write and cache read independently.
	// At most one extra micro-USD per additional priced input bucket is needed
	// above rounding their combined maximum. Reserve it before dispatch too.
	var roundingSlack int64
	if l.CacheWriteRate > 0 || l.HighCacheWriteRate > 0 {
		roundingSlack++
	}
	if l.CacheReadRate > 0 || l.HighCacheReadRate > 0 {
		roundingSlack++
	}
	return tokenCost(l.MaxInputTokens, l.ReserveInputRate) + roundingSlack + tokenCost(l.MaxOutputTokens, l.ReserveOutputRate)
}

type usageReading struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheWrite int64 `json:"cache_creation_input_tokens"`
	CacheRead  int64 `json:"cache_read_input_tokens"`
}

func readUsage(raw []byte, l lane) (model string, u usageReading, cost int64, err error) {
	obj, err := jsonwire.Object(raw)
	if err != nil {
		return "", u, 0, err
	}
	if json.Unmarshal(obj["model"], &model) != nil || model == "" {
		return "", u, 0, errors.New("resolved model missing")
	}
	if !l.acceptsModel(model) {
		return model, u, 0, errors.New("unselected resolved model")
	}
	m, err := jsonwire.Object(obj["usage"])
	if err != nil {
		return model, u, 0, errors.New("usage object missing")
	}
	read := func(name string, required bool) (int64, error) {
		b, ok := m[name]
		if !ok && !required {
			return 0, nil
		}
		var v int64
		if !ok || json.Unmarshal(b, &v) != nil || bytes.Equal(bytes.TrimSpace(b), []byte("null")) || v < 0 || v > 2_000_000 {
			return 0, errors.New("usage field missing or invalid")
		}
		return v, nil
	}
	u.Input, err = read("input_tokens", true)
	if err != nil {
		return model, u, 0, err
	}
	u.Output, err = read("output_tokens", true)
	if err != nil {
		return model, u, 0, err
	}
	u.CacheWrite, err = read("cache_creation_input_tokens", false)
	if err != nil {
		return model, u, 0, err
	}
	u.CacheRead, err = read("cache_read_input_tokens", false)
	if err != nil {
		return model, u, 0, err
	}
	if u.Input+u.CacheWrite+u.CacheRead > l.MaxInputTokens || u.Output > l.MaxOutputTokens {
		return model, u, 0, errors.New("usage exceeds selected token bounds")
	}
	inRate, outRate, writeRate, readRate := l.InputRate, l.OutputRate, l.CacheWriteRate, l.CacheReadRate
	if l.TierThreshold > 0 && u.Input+u.CacheWrite+u.CacheRead > l.TierThreshold {
		inRate, outRate, writeRate, readRate = l.HighInputRate, l.HighOutputRate, l.HighCacheWriteRate, l.HighCacheReadRate
	}
	if (u.CacheWrite > 0 && writeRate == 0) || (u.CacheRead > 0 && readRate == 0) {
		return model, u, 0, errors.New("unpriced cache usage")
	}
	cost = tokenCost(u.Input, inRate) + tokenCost(u.Output, outRate) + tokenCost(u.CacheWrite, writeRate) + tokenCost(u.CacheRead, readRate)
	return model, u, cost, nil
}

// captureTransport observes exact usage-field presence and resolved model, which
// the shared completion response intentionally does not promise. It enforces one
// exact synthetic-study endpoint, bounded response bytes and no automatic retries.
type captureTransport struct {
	mu             sync.Mutex
	inner          http.RoundTripper
	origin, path   string
	active, called bool
	status         int
	body           []byte
	bindWire       func([]byte) error
}

func (t *captureTransport) begin() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.active {
		return errors.New("concurrent study call refused")
	}
	t.active = true
	t.called = false
	t.status = 0
	t.body = nil
	return nil
}

func (t *captureTransport) end() (int, []byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active = false
	t.bindWire = nil
	return t.status, append([]byte(nil), t.body...), t.called
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if !t.active || t.called || req.Method != http.MethodPost || req.URL.Scheme+"://"+req.URL.Host != t.origin || req.URL.Path != t.path || req.URL.RawPath != "" || req.URL.RawQuery != "" || req.URL.Fragment != "" || req.URL.User != nil {
		t.mu.Unlock()
		return nil, errors.New("study transport target or attempt refused")
	}
	t.called = true
	bind := t.bindWire
	t.mu.Unlock()
	if bind == nil {
		return nil, errors.New("wire reservation boundary missing")
	}
	if req.Body == nil {
		return nil, errors.New("wire request body missing")
	}
	wire, readReqErr := io.ReadAll(io.LimitReader(req.Body, (64<<10)+1))
	closeReqErr := req.Body.Close()
	if readReqErr != nil || closeReqErr != nil || len(wire) > 64<<10 {
		return nil, errors.New("wire request byte bound")
	}
	req.Body = io.NopCloser(bytes.NewReader(wire))
	if err := bind(wire); err != nil {
		return nil, err
	}
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	b, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil || len(b) > 1<<20 {
		return nil, errors.New("bounded study response read failed")
	}
	for _, header := range []string{"X-Api-Key", "Authorization"} {
		secret := strings.TrimPrefix(req.Header.Get(header), "Bearer ")
		if secret != "" && bytes.Contains(b, []byte(secret)) {
			return nil, errors.New("provider response echoed credential; body withheld")
		}
	}
	t.mu.Lock()
	t.status = resp.StatusCode
	t.body = append([]byte(nil), b...)
	t.mu.Unlock()
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp, nil
}

func studyHTTP(provider string) (*http.Client, *captureTransport) {
	t := &captureTransport{inner: http.DefaultTransport, origin: "https://api.anthropic.com", path: "/v1/messages"}
	if provider == "openrouter-jev" {
		t.origin = "https://openrouter.ai"
		t.path = "/api/alpha/decisions"
	}
	return &http.Client{Transport: t, Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, t
}

type attemptRunner struct {
	journal               *journal
	lane                  lane
	capture               *captureTransport
	now                   func() time.Time
	caseID                string
	expectedRequestDigest string
}

func (a *attemptRunner) call(ctx context.Context, request any, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.now == nil || a.journal == nil || a.capture == nil || a.lane.validate() != nil {
		return errors.New("invalid attempt runner")
	}
	b, err := json.Marshal(request)
	if err != nil || len(b) > 64<<10 {
		return errors.New("bounded attempt request required")
	}
	if err = a.capture.begin(); err != nil {
		return err
	}
	var r reservation
	reserved := false
	a.capture.mu.Lock()
	a.capture.bindWire = func(wire []byte) error {
		if a.expectedRequestDigest != "" && hashBytes(wire) != a.expectedRequestDigest {
			return errors.New("native request differs from prepared request")
		}
		obj, e := jsonwire.Object(wire)
		if e != nil {
			return e
		}
		var model string
		if json.Unmarshal(obj["model"], &model) != nil || model != a.lane.Model {
			return errors.New("wire model differs from selected lane")
		}
		// Byte fallback plus a conservative framing allowance. The study must
		// still reserve the documented model context/price tier; this sanity
		// bound does not certify a vendor tokenizer or invoice.
		if int64(len(wire))+4096 > a.lane.MaxInputTokens {
			return errors.New("wire request exceeds declared input reservation")
		}
		if a.lane.Provider == "anthropic" {
			var cap int64
			if json.Unmarshal(obj["max_tokens"], &cap) != nil || cap < 1 || cap > a.lane.MaxOutputTokens {
				return errors.New("wire output cap exceeds selected lane")
			}
		}
		r, e = a.journal.reserve(a.caseID, a.lane, wire)
		reserved = e == nil
		return e
	}
	a.capture.mu.Unlock()
	start := a.now()
	callErr := fn()
	status, raw, called := a.capture.end()
	if !reserved {
		return errors.Join(errors.New("study request refused before dispatch"), callErr)
	}
	latency := a.now().Sub(start)
	if latency < 0 {
		latency = 0
	}
	o := outcome{Status: "unknown", LatencyNanos: int64(latency), HTTPStatus: status}
	// Error bodies are not persisted: they may echo credentials. Successful
	// synthetic responses are still kept privately, never printed by this path.
	if called && callErr == nil && status >= 200 && status < 300 {
		model, _, cost, usageErr := readUsage(raw, a.lane)
		o.Model = model
		if usageErr == nil {
			o.Status = "completed"
			o.UsagePresent = true
			o.ActualMicroUSD = &cost
			o.Response = string(raw)
		} else {
			callErr = errors.New("provider usage/model missing or outside pricing bounds")
		}
	} else if !called {
		o.Status = "failed"
	}
	finishErr := a.journal.finish(r, o)
	if o.Status != "completed" {
		a.journal.mu.Lock()
		a.journal.closed = true
		a.journal.mu.Unlock()
		return errors.Join(errors.New("study stopped with uncertain provider spend; reservation retained"), finishErr)
	}
	return errors.Join(callErr, finishErr)
}

type labClient struct {
	inner   llmclient.LLMClient
	attempt *attemptRunner
}

func (c *labClient) Complete(ctx context.Context, r llmclient.Request) (out llmclient.Response, err error) {
	if r.Model != c.attempt.lane.Model || r.MaxTokens < 1 || int64(r.MaxTokens) > c.attempt.lane.MaxOutputTokens {
		return out, errors.New("unselected generation model or token limit")
	}
	err = c.attempt.call(ctx, r, func() error { var e error; out, e = c.inner.Complete(ctx, r); return e })
	return out, err
}

func (c *labClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("streaming is outside this study profile")
}

type labDecider struct {
	inner   decision.Decider
	attempt *attemptRunner
}

func (d *labDecider) Name() string { return d.inner.Name() }
func (d *labDecider) Decide(ctx context.Context, r decision.Request) (out decision.Response, err error) {
	err = d.attempt.call(ctx, r, func() error { var e error; out, e = d.inner.Decide(ctx, r); return e })
	return out, err
}
