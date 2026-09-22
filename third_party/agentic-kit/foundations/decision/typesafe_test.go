package decision_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// typesafeGoldenRequest is the one fixed Request pinned by
// testdata/typesafe_request.golden — one choice, one score (with Levels),
// one noul with criteria. Changing it (or the rendering it drives) is a
// wire-drift event and must update the golden file deliberately, in
// review — see TestTypeSafe_Decide_GoldenRequest.
func typesafeGoldenRequest() decision.Request {
	return decision.Request{
		State: "widget on line 3 stopped moving after the motor made a grinding noise",
		Questions: []decision.Question{
			{
				ID:           "root-cause",
				Kind:         decision.KindChoice,
				Instructions: "what caused the stoppage?",
				Options: []decision.Option{
					{Name: "mechanical", Criteria: "gears, belts, bearings"},
					{Name: "electrical"},
					{Name: "operator-error"},
				},
			},
			{
				ID:           "severity",
				Kind:         decision.KindScore,
				Instructions: "how severe is this on a 0-4 scale?",
				Levels:       []string{"cosmetic", "minor", "moderate", "major", "line-down"},
			},
			{
				ID:            "restart-safe",
				Kind:          decision.KindNoul,
				Instructions:  "is it safe to restart the line without inspection?",
				TrueCriteria:  "no visible damage and grinding has stopped",
				FalseCriteria: "any visible damage or persistent noise",
			},
		},
	}
}

// typesafeCapture is a scriptable httptest handler that records every
// request it receives (method, URL path, headers, body) and replies with a
// fixed status and body.
type typesafeCapture struct {
	mu       sync.Mutex
	requests []capturedRequest
	status   int
	body     string
}

type capturedRequest struct {
	method string
	path   string
	auth   string
	ctype  string
	body   string
}

func (c *typesafeCapture) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.requests = append(c.requests, capturedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			ctype:  r.Header.Get("Content-Type"),
			body:   string(body),
		})
		status := c.status
		replyBody := c.body
		c.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(replyBody))
	}
}

func (c *typesafeCapture) calls() []capturedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]capturedRequest, len(c.requests))
	copy(cp, c.requests)
	return cp
}

func TestNewTypeSafe_EmptyKeyErrors(t *testing.T) {
	if _, err := decision.NewTypeSafe(decision.TypeSafeConfig{}); err == nil {
		t.Fatal("NewTypeSafe() error = nil, want non-nil for empty APIKey")
	}
}

func TestTypeSafe_Name(t *testing.T) {
	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ts.Name(); got != "typesafe" {
		t.Errorf("Name() = %q, want %q", got, "typesafe")
	}
}

func TestTypeSafe_Decide_GoldenRequest(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Decide(context.Background(), typesafeGoldenRequest()); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	calls := cap.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	got := calls[0]

	if got.method != http.MethodPost {
		t.Errorf("method = %q, want POST", got.method)
	}
	if got.path != "/v1/systemone" {
		t.Errorf("path = %q, want /v1/systemone", got.path)
	}
	if got.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got.auth, "Bearer test-key")
	}
	if got.ctype != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got.ctype)
	}

	want := readGolden(t, "testdata/typesafe_request.golden")
	if got.body != want {
		t.Errorf("request body does not match testdata/typesafe_request.golden\ngot:\n%s\nwant:\n%s", got.body, want)
	}
	if !strings.Contains(got.body, `"jev-latest"`) {
		t.Error("request body should default model to jev-latest")
	}
}

func TestTypeSafe_Decide_MixedBatch(t *testing.T) {
	respBody := `{
		"model": "jev-1.13.0",
		"answers": {
			"root-cause": {"type":"choice","choice":"mechanical","confidence":1.0,"probabilities":{"mechanical":0.7,"electrical":0.2,"operator-error":0.1}},
			"severity": {"type":"score","score":2.5,"confidence":0.35,"legend":{"0":"cosmetic","1":"minor","2":"moderate","3":"major","4":"line-down"},"probabilities":{"0":0.0,"1":0.1,"2":0.4,"3":0.4,"4":0.1}},
			"restart-safe": {"type":"noul","noul":0.2}
		},
		"usage": {"input_tokens": 392, "output_tokens": 65}
	}`
	cap := &typesafeCapture{body: respBody}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	now := func() time.Time {
		calls++
		if calls == 1 {
			return start
		}
		return start.Add(300 * time.Millisecond)
	}

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL, Now: now})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := ts.Decide(context.Background(), typesafeGoldenRequest())
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if len(resp.Invalid) != 0 {
		t.Fatalf("Invalid = %v, want empty", resp.Invalid)
	}
	if len(resp.Answers) != 3 {
		t.Fatalf("len(Answers) = %d, want 3", len(resp.Answers))
	}

	if resp.Provider != "typesafe" {
		t.Errorf("Provider = %q, want typesafe", resp.Provider)
	}
	if resp.Model != "jev-1.13.0" {
		t.Errorf("Model = %q, want jev-1.13.0 (the envelope's model, not the request default)", resp.Model)
	}
	if resp.Usage.InputTokens != 392 || resp.Usage.OutputTokens != 65 {
		t.Errorf("Usage = %+v, want {392 65}", resp.Usage)
	}
	if resp.Latency != 300*time.Millisecond {
		t.Errorf("Latency = %v, want 300ms", resp.Latency)
	}

	rc := resp.Answers["root-cause"]
	if rc.Choice != "mechanical" {
		t.Errorf("root-cause.Choice = %q, want mechanical", rc.Choice)
	}
	wantConf := decision.Confidence(map[string]float64{"mechanical": 0.7, "electrical": 0.2, "operator-error": 0.1})
	if !floatsClose(rc.Confidence, wantConf) {
		t.Errorf("root-cause.Confidence = %v, want %v (this package's formula, never the wire's)", rc.Confidence, wantConf)
	}

	sev := resp.Answers["severity"]
	if sev.Score != 2.5 {
		t.Errorf("severity.Score = %v, want 2.5 (offset 0, Levels explicit)", sev.Score)
	}
	wantSevConf := decision.Confidence(map[string]float64{"0": 0.0, "1": 0.1, "2": 0.4, "3": 0.4, "4": 0.1})
	if !floatsClose(sev.Confidence, wantSevConf) {
		t.Errorf("severity.Confidence = %v, want %v", sev.Confidence, wantSevConf)
	}

	rs := resp.Answers["restart-safe"]
	if rs.Noul != 0.2 {
		t.Errorf("restart-safe.Noul = %v, want 0.2", rs.Noul)
	}
	if rs.Confidence != 0.8 {
		t.Errorf("restart-safe.Confidence = %v, want 0.8 (max(noul,1-noul))", rs.Confidence)
	}
}

func TestTypeSafe_Decide_InvalidBranches(t *testing.T) {
	req := decision.Request{
		State: "s",
		Questions: []decision.Question{
			{ID: "missing", Kind: decision.KindNoul},
			{ID: "type-mismatch", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "b"}}},
			{ID: "malformed", Kind: decision.KindNoul},
			{ID: "bad-choice", Kind: decision.KindChoice, Options: []decision.Option{{Name: "x"}, {Name: "y"}}},
			{ID: "bad-probs", Kind: decision.KindChoice, Options: []decision.Option{{Name: "p"}, {Name: "q"}}},
			{ID: "bad-noul", Kind: decision.KindNoul},
			{ID: "bad-score", Kind: decision.KindScore, Levels: []string{"lo", "mid", "hi"}},
			{ID: "unrenderable-score", Kind: decision.KindScore, Min: 0, Max: 1.5},
			{ID: "ok-noul", Kind: decision.KindNoul},
		},
	}

	respBody := `{
		"model": "jev-1.13.0",
		"answers": {
			"type-mismatch": {"type":"noul","noul":0.5},
			"malformed": 123,
			"bad-choice": {"type":"choice","choice":"z","probabilities":{"x":0.5,"y":0.5}},
			"bad-probs": {"type":"choice","choice":"p","probabilities":{"p":0.2,"q":0.2}},
			"bad-noul": {"type":"noul","noul":1.5},
			"bad-score": {"type":"score","score":5,"probabilities":{"0":0.1,"1":0.1,"2":0.8}},
			"ok-noul": {"type":"noul","noul":0.9}
		},
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`
	cap := &typesafeCapture{body: respBody}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	wantInvalid := []string{"missing", "type-mismatch", "malformed", "bad-choice", "bad-probs", "bad-noul", "bad-score", "unrenderable-score"}
	for _, id := range wantInvalid {
		t.Run(id, func(t *testing.T) {
			if _, ok := resp.Answers[id]; ok {
				t.Errorf("%s should be absent from Answers", id)
			}
			if reason, ok := resp.Invalid[id]; !ok || reason == "" {
				t.Errorf("Invalid[%s] = %q, ok=%v; want a non-empty reason", id, reason, ok)
			}
		})
	}
	if _, ok := resp.Answers["ok-noul"]; !ok {
		t.Error("ok-noul should still be answered despite the rest of the batch being invalid")
	}

	// unrenderable-score must never reach the wire at all.
	calls := cap.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if strings.Contains(calls[0].body, "unrenderable-score") {
		t.Error("unrenderable-score should never be sent to the wire")
	}
}

func TestTypeSafe_Decide_AllQuestionsUnrenderableSkipsNetwork(t *testing.T) {
	cap := &typesafeCapture{body: `{}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindScore, Min: 0, Max: 1.5}},
	}
	resp, err := ts.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if len(resp.Answers) != 0 {
		t.Errorf("Answers = %v, want empty", resp.Answers)
	}
	if reason := resp.Invalid["q1"]; reason == "" {
		t.Error("q1 should be Invalid")
	}
	if len(cap.calls()) != 0 {
		t.Errorf("len(calls) = %d, want 0 (no network call when nothing renders)", len(cap.calls()))
	}
}

func TestTypeSafe_Decide_HTTPErrors(t *testing.T) {
	tests := []struct {
		status        int
		wantRetryable bool
	}{
		{http.StatusUnauthorized, false},
		{http.StatusUnprocessableEntity, false},
		{http.StatusTooManyRequests, true},
		{529, true},
	}

	const apiKey = "sekret-test-key-do-not-leak"
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			cap := &typesafeCapture{status: tt.status, body: `{"error":"request failed"}`}
			srv := httptest.NewServer(cap.handler())
			defer srv.Close()

			ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: apiKey, BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}

			_, err = ts.Decide(context.Background(), req)
			if err == nil {
				t.Fatal("Decide() error = nil, want non-nil")
			}

			var tsErr *decision.TypeSafeError
			if !errors.As(err, &tsErr) {
				t.Fatalf("error = %v (%T), want *decision.TypeSafeError", err, err)
			}
			if tsErr.Status != tt.status {
				t.Errorf("Status = %d, want %d", tsErr.Status, tt.status)
			}
			if tsErr.Retryable() != tt.wantRetryable {
				t.Errorf("Retryable() = %v, want %v", tsErr.Retryable(), tt.wantRetryable)
			}
			if strings.Contains(err.Error(), apiKey) {
				t.Errorf("error text leaks the API key: %v", err)
			}

			// Confirm the key really was sent (so the leak-check above is
			// meaningful, not vacuous).
			calls := cap.calls()
			if len(calls) != 1 || calls[0].auth != "Bearer "+apiKey {
				t.Fatalf("calls = %+v, want one call with Authorization: Bearer %s", calls, apiKey)
			}
		})
	}
}

func TestTypeSafe_Decide_NonJSONReplyErrors(t *testing.T) {
	cap := &typesafeCapture{body: "not json at all"}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}
	if _, err := ts.Decide(context.Background(), req); err == nil {
		t.Fatal("Decide() error = nil, want non-nil for a non-JSON reply")
	}
}

func TestTypeSafe_Decide_InvalidRequestRejected(t *testing.T) {
	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Decide(context.Background(), decision.Request{}); err == nil {
		t.Fatal("Decide() error = nil, want a Validate error for empty State")
	}
}

func TestTypeSafe_Decide_ContextCancelled(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{},"usage":{}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}
	_, err = ts.Decide(ctx, req)
	if err == nil {
		t.Fatal("Decide() error = nil, want non-nil for a cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Decide() error = %v, want wrapping context.Canceled", err)
	}
}

func TestTypeSafe_Ping_Success(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{"ping":{"type":"noul","noul":0.99}},"usage":{}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestTypeSafe_Ping_HTTPFailure(t *testing.T) {
	cap := &typesafeCapture{status: http.StatusUnauthorized, body: `{"error":"bad key"}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	err = ts.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() error = nil, want non-nil")
	}
	var tsErr *decision.TypeSafeError
	if !errors.As(err, &tsErr) {
		t.Errorf("Ping() error = %v (%T), want *decision.TypeSafeError", err, err)
	}
}

func TestTypeSafe_Ping_UnparseableAnswer(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{},"usage":{}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.Ping(context.Background()); err == nil {
		t.Fatal("Ping() error = nil, want non-nil when the ping question is unanswered")
	}
}

func TestTypeSafe_Decide_SynthesizedLevelsRoundTrip(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{"q1":{"type":"score","score":2.0,"probabilities":{"0":0,"1":0,"2":1,"3":0,"4":0}}},"usage":{}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	req := decision.Request{
		State:     "s",
		Questions: []decision.Question{{ID: "q1", Kind: decision.KindScore, Min: 1, Max: 5}},
	}
	resp, err := ts.Decide(context.Background(), req)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if len(resp.Invalid) != 0 {
		t.Fatalf("Invalid = %v, want empty", resp.Invalid)
	}
	if resp.Answers["q1"].Score != 3.0 {
		t.Errorf("Score = %v, want 3.0 (Min 1 + wire score 2.0)", resp.Answers["q1"].Score)
	}

	calls := cap.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	// Min=1, Max=5 -> 5 synthesized levels ["1","2","3","4","5"] (Rubric
	// empty, so bare numbers).
	for _, want := range []string{`"1"`, `"2"`, `"3"`, `"4"`, `"5"`} {
		if !strings.Contains(calls[0].body, want) {
			t.Errorf("request body missing synthesized level %s:\n%s", want, calls[0].body)
		}
	}
}

func TestTypeSafe_Decide_Concurrent(t *testing.T) {
	cap := &typesafeCapture{body: `{"model":"jev-1.13.0","answers":{"q1":{"type":"noul","noul":0.5}},"usage":{}}`}
	srv := httptest.NewServer(cap.handler())
	defer srv.Close()

	ts, err := decision.NewTypeSafe(decision.TypeSafeConfig{APIKey: "test-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q1", Kind: decision.KindNoul}}}

	const n = 20
	var wg sync.WaitGroup
	var failures int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := ts.Decide(context.Background(), req)
			if err != nil || resp.Answers["q1"].Noul != 0.5 {
				atomic.AddInt64(&failures, 1)
			}
		}()
	}
	wg.Wait()
	if failures != 0 {
		t.Errorf("%d/%d concurrent Decide calls failed", failures, n)
	}
}
