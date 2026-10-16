package openai_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/openai"
)

// capture is an httptest handler that records every request and replies
// with status and body.
type capture struct {
	mu       sync.Mutex
	requests []captured
	status   int
	body     string
}

type captured struct {
	method, path, auth, ctype, body string
}

func (c *capture) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.requests = append(c.requests, captured{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(body)})
		status, reply := c.status, c.body
		c.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}
}

func (c *capture) calls() []captured {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]captured(nil), c.requests...)
}

// serve starts a capture server replying body and returns it with an
// OpenAI built over it with cfg (BaseURL set to the server).
func serve(t *testing.T, status int, body string, cfg openai.OpenAIConfig) (*capture, *openai.OpenAI) {
	t.Helper()
	c := &capture{status: status, body: body}
	srv := httptest.NewServer(c.handler())
	t.Cleanup(srv.Close)
	cfg.BaseURL = srv.URL
	if cfg.APIKey == "" && cfg.APIKeyFile == "" && cfg.APIKeyEnv == "" {
		cfg.APIKey = "test-key"
	}
	o, err := openai.NewOpenAI(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c, o
}

func readGolden(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.TrimSuffix(string(b), "\n")
}

// goldenRequest is the one fixed Request testdata/openai_request.golden
// pins: a predicate with criteria, a choice with option descriptions, a
// score over explicit Levels with a rubric and a score over an integral
// range.
func goldenRequest() decision.Request {
	return decision.Request{
		State: "widget on line 3 stopped after a grinding noise",
		Questions: []decision.Question{
			{ID: "restart-safe", Kind: decision.KindPredicate, Instructions: "Is it safe to restart the line?", TrueCriteria: "no visible damage", FalseCriteria: "any damage or noise"},
			{ID: "root-cause", Kind: decision.KindChoice, Instructions: "What caused the stoppage?", Options: []decision.Option{{Name: "mechanical", Criteria: "gears, belts, bearings"}, {Name: "electrical"}, {Name: "operator-error"}}},
			{ID: "severity", Kind: decision.KindScore, Instructions: "How severe is it?", Rubric: "line-down is the worst", Levels: []string{"cosmetic", "degraded", "line-down"}},
			{ID: "urgency", Kind: decision.KindScore, Instructions: "How urgent, 1 to 3?", Min: 1, Max: 3},
		},
	}
}

func TestNewOpenAIRequiresAKeySource(t *testing.T) {
	_, err := openai.NewOpenAI(openai.OpenAIConfig{})
	if err == nil || !strings.Contains(err.Error(), "APIKeyFile or APIKeyEnv") {
		t.Fatalf("NewOpenAI() error = %v, want one naming the key fields", err)
	}
	o, err := openai.NewOpenAI(openai.OpenAIConfig{APIKeyEnv: "DECISION_OPENAI_TEST_UNSET"})
	if err != nil {
		t.Fatalf("NewOpenAI() error = %v; a variable is read at first use, not at construction", err)
	}
	if got := o.Name(); got != "openai" {
		t.Errorf("Name() = %q, want openai", got)
	}
}

func TestOpenAIDecideGoldenRequest(t *testing.T) {
	c, o := serve(t, 0, `{"answers":[]}`, openai.OpenAIConfig{})
	if _, err := o.Decide(context.Background(), goldenRequest()); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	calls := c.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1: every question in one request", len(calls))
	}
	got := calls[0]
	if got.method != http.MethodPost || got.path != "/decisions" || got.auth != "Bearer test-key" || got.ctype != "application/json" {
		t.Errorf("request = %s %s (auth set: %v, content type %q), want POST /decisions with the bearer key and JSON", got.method, got.path, got.auth == "Bearer test-key", got.ctype)
	}
	if want := readGolden(t, "testdata/openai_request.golden"); got.body != want {
		t.Errorf("request body does not match testdata/openai_request.golden\ngot:\n%s\nwant:\n%s", got.body, want)
	}
}

func TestOpenAIDecideGoldenRequestWithImages(t *testing.T) {
	c, o := serve(t, 0, `{"answers":[]}`, openai.OpenAIConfig{})
	req := decision.Request{
		State:     "a photo of the belt on line 3",
		Questions: []decision.Question{{ID: "visible_damage", Kind: decision.KindPredicate, Instructions: "Is there visible damage?"}},
		Images:    []decision.Image{{MediaType: "image/png", Data: []byte("first image")}, {MediaType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff}}},
	}
	if _, err := o.Decide(context.Background(), req); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	calls := c.calls()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if want := readGolden(t, "testdata/openai_request_images.golden"); calls[0].body != want {
		t.Errorf("request body does not match testdata/openai_request_images.golden\ngot:\n%s\nwant:\n%s", calls[0].body, want)
	}
}

func TestOpenAIBaseURLDefaultAndTrailingSlash(t *testing.T) {
	if openai.DefaultOpenAIBaseURL != "https://api.openai.com/v1" || openai.DefaultOpenAIModel != "gpt-6-luna" {
		t.Errorf("defaults = %q, %q", openai.DefaultOpenAIBaseURL, openai.DefaultOpenAIModel)
	}
	c := &capture{body: `{"answers":[]}`}
	srv := httptest.NewServer(c.handler())
	defer srv.Close()
	o, err := openai.NewOpenAI(openai.OpenAIConfig{APIKey: "test-key", BaseURL: srv.URL + "/v1/", Model: "model-example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}}); err != nil {
		t.Fatal(err)
	}
	calls := c.calls()
	if len(calls) != 1 || calls[0].path != "/v1/decisions" || !strings.Contains(calls[0].body, `"model": "model-example"`) {
		t.Errorf("calls = %+v, want POST /v1/decisions with the configured model", calls)
	}
}

func TestOpenAIDecideMapsEveryAnswerType(t *testing.T) {
	const reply = `{
  "model": "gpt-6-luna-2026-10-01",
  "answers": [
    {"type": "score", "name": "urgency", "score": 1.5, "probabilities": [
      {"value": 0, "label": "1", "probability": 0.1}, {"value": 1, "label": "2", "probability": 0.3}, {"value": 2, "label": "3", "probability": 0.6}], "confidence": 0.42},
    {"type": "predicate", "name": "restart-safe", "probability": 0.2},
    {"type": "choice", "name": "root-cause", "choice": "mechanical", "probabilities": [
      {"value": "mechanical", "probability": 0.7}, {"value": "electrical", "probability": 0.2}, {"value": "operator-error", "probability": 0.1}], "confidence": 0.123},
    {"type": "refusal", "name": "severity"},
    {"type": "predicate", "name": "not-asked", "probability": 1}
  ],
  "usage": {"input_tokens": 120, "output_tokens": 0}
}`
	now := time.Unix(100, 0)
	clock := func() time.Time { now = now.Add(5 * time.Millisecond); return now }
	_, o := serve(t, 0, reply, openai.OpenAIConfig{Now: clock})
	resp, err := o.Decide(context.Background(), goldenRequest())
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if resp.Provider != "openai" || resp.Model != "gpt-6-luna-2026-10-01" {
		t.Errorf("Response labeled %q/%q, want openai and the model the reply names", resp.Provider, resp.Model)
	}
	if resp.Usage != (decision.Usage{InputTokens: 120}) || resp.Latency != 5*time.Millisecond {
		t.Errorf("Usage, Latency = %+v, %v", resp.Usage, resp.Latency)
	}
	if len(resp.Invalid) != 0 {
		t.Errorf("Invalid = %v, want none", resp.Invalid)
	}

	pred := resp.Answers["restart-safe"]
	if pred.Kind != decision.KindPredicate || pred.Noul != 0.2 || math.Abs(pred.Confidence-0.8) > 1e-9 {
		t.Errorf("predicate = %+v, want Noul 0.2 and Confidence 0.8", pred)
	}

	choice := resp.Answers["root-cause"]
	wantProbs := map[string]float64{"mechanical": 0.7, "electrical": 0.2, "operator-error": 0.1}
	if choice.Choice != "mechanical" || len(choice.Probabilities) != 3 || choice.Probabilities["electrical"] != 0.2 {
		t.Errorf("choice = %+v", choice)
	}
	if want := decision.Confidence(wantProbs); math.Abs(choice.Confidence-want) > 1e-9 || choice.Confidence == 0.123 {
		t.Errorf("choice Confidence = %v, want the package's %v, never the wire's 0.123", choice.Confidence, want)
	}

	score := resp.Answers["urgency"]
	if math.Abs(score.Score-2.5) > 1e-9 {
		t.Errorf("score = %v, want 2.5 (the wire's 1.5 shifted by Min 1)", score.Score)
	}
	levelProbs := map[string]float64{"0": 0.1, "1": 0.3, "2": 0.6}
	if len(score.Probabilities) != 3 || score.Probabilities["2"] != 0.6 {
		t.Errorf("score distribution = %v, want %v", score.Probabilities, levelProbs)
	}
	if want := decision.Confidence(levelProbs); math.Abs(score.Confidence-want) > 1e-9 || score.Confidence == 0.42 {
		t.Errorf("score Confidence = %v, want the package's %v, never the wire's 0.42", score.Confidence, want)
	}

	if got := resp.Declined["severity"]; got == "" {
		t.Errorf("Declined = %v, want the refused severity", resp.Declined)
	}
	if _, ok := resp.Answers["severity"]; ok {
		t.Error("a refusal was read as an answer")
	}
	if _, ok := resp.Answers["not-asked"]; ok {
		t.Error("an answer to a question not asked was kept")
	}
}

func TestOpenAIDecideClassifiesBadAnswersPerQuestion(t *testing.T) {
	pred := decision.Question{ID: "p", Kind: decision.KindPredicate}
	choice := decision.Question{ID: "c", Kind: decision.KindChoice, Options: []decision.Option{{Name: "a"}, {Name: "b"}}}
	score := decision.Question{ID: "s", Kind: decision.KindScore, Levels: []string{"low", "mid", "high"}}
	cases := []struct {
		name   string
		q      decision.Question
		answer string
		want   string
	}{
		{"no answer", pred, ``, "no answer"},
		{"two answers", pred, `{"type":"predicate","name":"p","probability":0.5},{"type":"predicate","name":"p","probability":0.6}`, "more than one answer"},
		{"type mismatch", pred, `{"type":"choice","name":"p","choice":"a"}`, `answer type "choice" does not match question kind "predicate"`},
		{"missing probability", pred, `{"type":"predicate","name":"p"}`, "predicate probability missing"},
		{"probability above 1", pred, `{"type":"predicate","name":"p","probability":1.5}`, "outside [0, 1]"},
		{"malformed probability", pred, `{"type":"predicate","name":"p","probability":"high"}`, "malformed predicate answer"},
		{"choice not an option", choice, `{"type":"choice","name":"c","choice":"z","probabilities":[{"value":"a","probability":1}]}`, `choice "z" is not one of`},
		{"probability for an unknown option", choice, `{"type":"choice","name":"c","choice":"a","probabilities":[{"value":"a","probability":0.5},{"value":"z","probability":0.5}]}`, `probability for "z"`},
		{"repeated option probability", choice, `{"type":"choice","name":"c","choice":"a","probabilities":[{"value":"a","probability":0.5},{"value":"a","probability":0.5}]}`, `more than one probability for option "a"`},
		{"choice distribution off", choice, `{"type":"choice","name":"c","choice":"a","probabilities":[{"value":"a","probability":0.5},{"value":"b","probability":0.2}]}`, "sum to 0.7"},
		{"missing score", score, `{"type":"score","name":"s","probabilities":[{"value":0,"probability":1}]}`, `missing "score"`},
		{"score above the top level", score, `{"type":"score","name":"s","score":2.5,"probabilities":[{"value":2,"probability":1}]}`, "outside [0, 2]"},
		{"score without a distribution", score, `{"type":"score","name":"s","score":1}`, "no level distribution"},
		{"level outside the levels", score, `{"type":"score","name":"s","score":1,"probabilities":[{"value":3,"probability":1}]}`, "names no level"},
		{"fractional level", score, `{"type":"score","name":"s","score":1,"probabilities":[{"value":0.5,"probability":1}]}`, "names no level"},
		{"repeated level", score, `{"type":"score","name":"s","score":1,"probabilities":[{"value":1,"probability":0.5},{"value":1,"probability":0.5}]}`, "more than one probability for level 1"},
		{"score distribution off", score, `{"type":"score","name":"s","score":1,"probabilities":[{"value":1,"probability":0.5}]}`, "sum to 0.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			good := decision.Question{ID: "good", Kind: decision.KindPredicate}
			body := `{"answers":[{"type":"predicate","name":"good","probability":0.9}`
			if c.answer != "" {
				body += "," + c.answer
			}
			body += `]}`
			_, o := serve(t, 0, body, openai.OpenAIConfig{})
			resp, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{good, c.q}})
			if err != nil {
				t.Fatalf("Decide() error = %v; one bad answer never fails the batch", err)
			}
			if why := resp.Invalid[c.q.ID]; !strings.Contains(why, c.want) {
				t.Errorf("Invalid[%q] = %q, want it to contain %q", c.q.ID, why, c.want)
			}
			if _, ok := resp.Answers[c.q.ID]; ok {
				t.Errorf("bad answer %q landed in Answers", c.q.ID)
			}
			if _, ok := resp.Answers["good"]; !ok {
				t.Error("the good answer beside it is missing")
			}
		})
	}
}

func TestOpenAIUnrenderableScoreNeverReachesTheWire(t *testing.T) {
	c, o := serve(t, 0, `{"answers":[]}`, openai.OpenAIConfig{})
	resp, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "s", Kind: decision.KindScore, Min: 0, Max: 0.5}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.calls()) != 0 || !strings.Contains(resp.Invalid["s"], "integral [Min, Max] range") {
		t.Errorf("Response = %+v after %d call(s), want s invalid and no call", resp, len(c.calls()))
	}
}

func TestOpenAIHTTPErrorsAreRedactedAndClassified(t *testing.T) {
	const key = "fake-key-0001"
	cases := []struct {
		status    int
		body      string
		retryable bool
	}{
		{http.StatusUnauthorized, `{"error":{"message":"Incorrect API key provided: ` + key + `. Header was Authorization: Bearer ` + key + `","type":"invalid_request_error","code":"invalid_api_key"}}`, false},
		{http.StatusBadRequest, `{"error":{"message":"bad question"}}`, false},
		{http.StatusTooManyRequests, `{"error":{"message":"slow down"}}`, true},
		// The live reply of 2026-10-09 from an account with no credit: a 429
		// that no backoff can fix.
		{http.StatusTooManyRequests, `{"error":{"message":"You have no credits remaining. Add credits to continue using the API.","type":"insufficient_quota","param":null,"code":"credit_balance_exhausted"}}`, false},
		{http.StatusInternalServerError, `oops`, true},
		{http.StatusServiceUnavailable, `busy`, true},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, http.StatusText(c.status)), func(t *testing.T) {
			calls, o := serve(t, c.status, c.body, openai.OpenAIConfig{APIKey: key})
			_, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}})
			var oe *openai.OpenAIError
			if !errors.As(err, &oe) || oe.Status != c.status {
				t.Fatalf("error = %v, want an *OpenAIError with status %d", err, c.status)
			}
			if oe.Retryable() != c.retryable {
				t.Errorf("Retryable() = %v, want %v", oe.Retryable(), c.retryable)
			}
			if quota := strings.Contains(c.body, "insufficient_quota"); oe.QuotaExhausted() != quota || (quota && oe.Code != "credit_balance_exhausted") {
				t.Errorf("QuotaExhausted() = %v Type %q Code %q, want %v", oe.QuotaExhausted(), oe.Type, oe.Code, quota)
			}
			if strings.Contains(err.Error(), key) {
				t.Errorf("error echoes the key: %q", err)
			}
			if c.status == http.StatusUnauthorized && (!strings.Contains(oe.Message, "[REDACTED]") || !strings.Contains(oe.Message, "invalid_api_key")) {
				t.Errorf("Message = %q, want the provider's message with the key redacted", oe.Message)
			}
			// The key really was sent, so the leak check is not vacuous.
			if got := calls.calls(); len(got) != 1 || got[0].auth != "Bearer "+key {
				t.Error("the request did not carry the key")
			}
		})
	}

	t.Run("a long body is truncated after redaction", func(t *testing.T) {
		_, o := serve(t, http.StatusBadRequest, strings.Repeat("x", 600)+key, openai.OpenAIConfig{APIKey: key})
		_, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}})
		var oe *openai.OpenAIError
		if !errors.As(err, &oe) || len(oe.Message) != 512 {
			t.Fatalf("error = %v, want a 512-byte Message", err)
		}
	})
}

func TestOpenAIDecideHardErrors(t *testing.T) {
	t.Run("a reply that is not JSON", func(t *testing.T) {
		_, o := serve(t, 0, `not json`, openai.OpenAIConfig{})
		_, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}})
		if err == nil || !strings.Contains(err.Error(), "not JSON") {
			t.Errorf("Decide() error = %v, want not JSON", err)
		}
	})
	t.Run("an invalid Request never reaches the wire", func(t *testing.T) {
		c, o := serve(t, 0, `{"answers":[]}`, openai.OpenAIConfig{})
		if _, err := o.Decide(context.Background(), decision.Request{}); err == nil {
			t.Error("Decide() error = nil for an empty State")
		}
		if len(c.calls()) != 0 {
			t.Error("an invalid Request reached the wire")
		}
	})
	t.Run("a cancelled context", func(t *testing.T) {
		_, o := serve(t, 0, `{"answers":[]}`, openai.OpenAIConfig{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := o.Decide(ctx, decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}}); !errors.Is(err, context.Canceled) {
			t.Errorf("Decide() error = %v, want context.Canceled", err)
		}
	})
}

func TestOpenAIKeySources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not map onto Windows ACLs")
	}
	const fromFile, fromEnv = "fake-file-key", "fake-env-key"
	t.Setenv("DECISION_OPENAI_TEST_KEY", fromEnv)
	write := func(t *testing.T, mode os.FileMode) string {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte(fromFile+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	req := decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}}
	const reply = `{"answers":[{"type":"predicate","name":"q","probability":0.9}]}`

	t.Run("a 0600 file wins over the variable", func(t *testing.T) {
		c, o := serve(t, 0, reply, openai.OpenAIConfig{APIKeyFile: write(t, 0o600), APIKeyEnv: "DECISION_OPENAI_TEST_KEY"})
		if _, err := o.Decide(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if got := c.calls(); len(got) != 1 || got[0].auth != "Bearer "+fromFile {
			t.Error("the request did not carry the file's key")
		}
	})
	t.Run("the variable alone", func(t *testing.T) {
		c, o := serve(t, 0, reply, openai.OpenAIConfig{APIKeyEnv: "DECISION_OPENAI_TEST_KEY"})
		if _, err := o.Decide(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if got := c.calls(); len(got) != 1 || got[0].auth != "Bearer "+fromEnv {
			t.Error("the request did not carry the variable's key")
		}
	})
	t.Run("a 0644 file is refused before any call", func(t *testing.T) {
		path := write(t, 0o644)
		c, o := serve(t, 0, reply, openai.OpenAIConfig{APIKeyFile: path})
		_, err := o.Decide(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "chmod 600") {
			t.Errorf("Decide() error = %v, want the path and the fix", err)
		}
		if err != nil && strings.Contains(err.Error(), fromFile) {
			t.Error("the error leaks the key")
		}
		if len(c.calls()) != 0 {
			t.Error("a call was made with a refused key file")
		}
	})
	t.Run("an unset variable is a configuration error naming it", func(t *testing.T) {
		c, o := serve(t, 0, reply, openai.OpenAIConfig{APIKeyEnv: "DECISION_OPENAI_TEST_UNSET"})
		_, err := o.Decide(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), `"DECISION_OPENAI_TEST_UNSET"`) {
			t.Errorf("Decide() error = %v, want the variable named", err)
		}
		if len(c.calls()) != 0 {
			t.Error("a call was made without a key")
		}
	})
}

func TestOpenAIPing(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
		wantErr    string
	}{
		{"answered", `{"answers":[{"type":"predicate","name":"ping","probability":0.99}]}`, 0, ""},
		{"refused", `{"answers":[{"type":"refusal","name":"ping"}]}`, 0, "declined"},
		{"unparseable", `{"answers":[{"type":"predicate","name":"ping"}]}`, 0, "missing"},
		{"http failure", `{"error":{"message":"down"}}`, http.StatusServiceUnavailable, "status 503"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, o := serve(t, c.status, c.body, openai.OpenAIConfig{})
			err := o.Ping(context.Background())
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("Ping() error = %v", err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Errorf("Ping() error = %v, want %q", err, c.wantErr)
			}
		})
	}
}

func TestOpenAICapabilities(t *testing.T) {
	got := decision.CapabilitiesOf(&openai.OpenAI{})
	want := decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		SingleRoundTrip:     true,
		Latency:             decision.LatencyUnknown,
		Cost:                decision.CostLow,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
		Images:              true,
	}
	if got.Declared != want.Declared || got.Images != want.Images || got.ModelPinning || got.Cost != want.Cost || got.ConfidenceSemantics != want.ConfidenceSemantics || len(got.Kinds) != 3 || got.MaxOptions != 0 || got.MaxScoreLevels != 0 {
		t.Errorf("CapabilitiesOf(OpenAI) = %+v, want %+v", got, want)
	}
	if got.ConfidenceSemantics == decision.ConfidenceCalibratedProbability {
		t.Error("OpenAI claims calibrated probabilities; its guide does not")
	}
}

func TestOpenAIConcurrentUse(t *testing.T) {
	_, o := serve(t, 0, `{"answers":[{"type":"predicate","name":"q","probability":0.9}]}`, openai.OpenAIConfig{APIKeyEnv: "DECISION_OPENAI_TEST_CONCURRENT"})
	t.Setenv("DECISION_OPENAI_TEST_CONCURRENT", "fake-env-key")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := o.Decide(context.Background(), decision.Request{State: "s", Questions: []decision.Question{{ID: "q", Kind: decision.KindPredicate}}})
			if err != nil || resp.Answers["q"].Noul != 0.9 {
				t.Errorf("Decide() = %+v, %v", resp, err)
			}
		}()
	}
	wg.Wait()
}
