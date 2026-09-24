package main

// route_test.go — DP-9 (docs/plans/decision-port.md): POST /route.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// errBoom is a fixed sentinel error for the decider-error test below.
var errBoom = errors.New("boom: mock decider failure")

func postRoute(t *testing.T, handler http.Handler, prompt string) (*http.Response, routeResponse) {
	t.Helper()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	body, err := json.Marshal(routeRequest{Prompt: prompt})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := http.Post(srv.URL+"/route", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /route: %v", err)
	}
	defer resp.Body.Close()
	var out routeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp, out
}

// TestRouteHandler_NoneAndLLM pins DP-9's reachable_from (routeHandler,
// package cmd/fathom): nil ports and a None-decider ports both answer the
// fixed none-shaped body with no decider call and no recorder row; a
// proceed-band mock answer is recorded applied:true; a confirm-band answer
// is recorded applied:false; a decider error degrades to a 200 none-shaped
// body carrying "error", never a 5xx; GET is refused with 405.
func TestRouteHandler_NoneAndLLM(t *testing.T) {
	t.Run("nil ports", func(t *testing.T) {
		_, out := postRoute(t, routeHandler(nil), "why does the linter crash on foo.go")
		want := routeResponse{Shape: "", Verb: "", Confidence: 0, Band: "unsure", Provider: "none"}
		if out != want {
			t.Errorf("out = %+v, want %+v", out, want)
		}
	})

	t.Run("provider none", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		ports := &decide.Ports{Decider: decision.None{}, Cfg: config.Decide{Provider: "none"}, Recorder: rec}
		_, out := postRoute(t, routeHandler(ports), "why does the linter crash on foo.go")
		want := routeResponse{Shape: "", Verb: "", Confidence: 0, Band: "unsure", Provider: "none"}
		if out != want {
			t.Errorf("out = %+v, want %+v", out, want)
		}
		if buf.Len() != 0 {
			t.Errorf("decisions.jsonl got %d bytes written, want 0 (no decider call for None)", buf.Len())
		}
	})

	t.Run("proceed band is applied", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		mock := decision.NewMockDecider("mock", decision.MockEntry{
			Response: decision.Response{
				Provider: "mock", Model: "mock-model",
				Answers: map[string]decision.Answer{
					"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "localize", Confidence: 0.95},
				},
			},
		})
		ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Provider: "llm", RouteConfidence: 0.9, Sites: []string{decide.SiteRoute}}, Recorder: rec}
		resp, out := postRoute(t, routeHandler(ports), "TypeError: cannot read property 'x' of undefined")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if out.Verb != "localize" || out.Shape != "issue" || out.Band != "proceed" || out.Provider != "mock" {
			t.Errorf("out = %+v, want verb=localize shape=issue band=proceed provider=mock", out)
		}
		if out.Confidence != 0.95 {
			t.Errorf("Confidence = %v, want 0.95", out.Confidence)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 1 {
			t.Fatalf("decisions.jsonl has %d row(s), want 1:\n%s", len(lines), buf.String())
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatalf("row not JSON: %v", err)
		}
		if row["site"] != "route" {
			t.Errorf("site = %v, want %q", row["site"], "route")
		}
		if applied, ok := row["applied"].(bool); !ok || !applied {
			t.Errorf("applied = %v, want true (proceed band, confidence >= route_confidence)", row["applied"])
		}
	})

	t.Run("confirm band is never applied", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		mock := decision.NewMockDecider("mock", decision.MockEntry{
			Response: decision.Response{
				Provider: "mock",
				Answers: map[string]decision.Answer{
					"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "obligations", Confidence: 0.7},
				},
			},
		})
		ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Provider: "llm", RouteConfidence: 0.9, Sites: []string{decide.SiteRoute}}, Recorder: rec}
		_, out := postRoute(t, routeHandler(ports), "what rules govern this file")
		if out.Band != "confirm" || out.Verb != "obligations" || out.Shape != "what-governs" {
			t.Errorf("out = %+v, want verb=obligations shape=what-governs band=confirm", out)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatalf("row not JSON: %v", err)
		}
		if applied, ok := row["applied"].(bool); !ok || applied {
			t.Errorf("applied = %v, want false (confirm band is never applied)", row["applied"])
		}
	})

	t.Run("decider error degrades to 200 none-shaped with error", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		mock := decision.NewMockDecider("mock", decision.MockEntry{Err: errBoom})
		ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Provider: "llm", Sites: []string{decide.SiteRoute}}, Recorder: rec}
		resp, out := postRoute(t, routeHandler(ports), "some prompt")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (a decider error must never 5xx)", resp.StatusCode)
		}
		if out.Provider != "none" || out.Verb != "" || out.Band != "unsure" {
			t.Errorf("out = %+v, want the none-shaped fallback", out)
		}
		if out.Error == "" {
			t.Errorf("Error is empty, want it to name the decider failure")
		}

		// DP-22 (O109): the decider error is also logged as a decisions.jsonl
		// error row — silence there must never read as "route was never
		// asked" when it was asked and failed.
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 1 {
			t.Fatalf("decisions.jsonl has %d row(s), want 1:\n%s", len(lines), buf.String())
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatalf("row not JSON: %v", err)
		}
		if row["site"] != "route" {
			t.Errorf("site = %v, want %q", row["site"], "route")
		}
		if row["provider"] != "mock" {
			t.Errorf("provider = %v, want %q", row["provider"], "mock")
		}
		if row["error"] != errBoom.Error() {
			t.Errorf("error = %v, want %q", row["error"], errBoom.Error())
		}
		if applied, ok := row["applied"].(bool); !ok || applied {
			t.Errorf("applied = %v, want false", row["applied"])
		}
	})

	t.Run("route not in [decide] sites makes no call and discloses why", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		mock := decision.NewMockDecider("mock", decision.MockEntry{
			Response: decision.Response{
				Provider: "mock",
				Answers: map[string]decision.Answer{
					"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "localize", Confidence: 0.95},
				},
			},
		})
		// Sites lists a real site, just not "route" — proves the gate is
		// per-site, not merely "Sites is unset".
		ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Provider: "llm", Sites: []string{decide.SiteCertifySuggest}}, Recorder: rec}
		resp, out := postRoute(t, routeHandler(ports), "some prompt")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if len(mock.Calls()) != 0 {
			t.Errorf("Decide was called %d time(s), want 0 (site not allowed)", len(mock.Calls()))
		}
		if out.Provider != "none" || out.Verb != "" || out.Band != "unsure" {
			t.Errorf("out = %+v, want the none-shaped fallback", out)
		}
		if !strings.Contains(out.Error, "route") || !strings.Contains(out.Error, "sites") {
			t.Errorf("Error = %q, want it to name the blocked site and [decide] sites", out.Error)
		}
		if buf.Len() != 0 {
			t.Errorf("decisions.jsonl got %d bytes written, want 0 (no call, nothing to log)", buf.Len())
		}
	})

	t.Run("GET is refused", func(t *testing.T) {
		srv := httptest.NewServer(routeHandler(nil))
		defer srv.Close()
		resp, err := http.Get(srv.URL + "/route")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})

	t.Run("bad JSON body is 400", func(t *testing.T) {
		srv := httptest.NewServer(routeHandler(nil))
		defer srv.Close()
		resp, err := http.Post(srv.URL+"/route", "application/json", strings.NewReader("{not json"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

// slowDecider is a decision.Decider whose Decide call blocks for delay
// (or until ctx is done, whichever comes first) before answering resp —
// used to prove routeHandler actually bounds the call by
// ports.Cfg.RouteTimeoutMS rather than the old fixed 3s constant.
type slowDecider struct {
	delay time.Duration
	resp  decision.Response
}

func (s slowDecider) Name() string { return "slow" }

func (s slowDecider) Decide(ctx context.Context, _ decision.Request) (decision.Response, error) {
	select {
	case <-time.After(s.delay):
		return s.resp, nil
	case <-ctx.Done():
		return decision.Response{}, ctx.Err()
	}
}

// TestRouteHandler_ConfigurableTimeout is O109's own DP-21 reachable_from:
// routeDecideTimeout(ports) (cmd/fathom/route.go) must resolve
// ports.Cfg.RouteTimeoutMS, not the old fixed 3s routeDecideTimeout
// constant — a 100ms budget against a decider slower than that must time
// out well before the decider's own delay elapses, and a budget larger
// than the decider's delay must not time out at all.
func TestRouteHandler_ConfigurableTimeout(t *testing.T) {
	t.Run("short route_timeout_ms times out a slow decider", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		slow := slowDecider{delay: 2 * time.Second, resp: decision.Response{
			Provider: "slow",
			Answers: map[string]decision.Answer{
				"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "localize", Confidence: 0.95},
			},
		}}
		ports := &decide.Ports{Decider: slow, Cfg: config.Decide{Provider: "llm", RouteTimeoutMS: 100, Sites: []string{decide.SiteRoute}}, Recorder: rec}

		start := time.Now()
		resp, out := postRoute(t, routeHandler(ports), "some prompt")
		elapsed := time.Since(start)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (a timeout must never 5xx)", resp.StatusCode)
		}
		if out.Provider != "none" || out.Verb != "" || out.Band != "unsure" {
			t.Errorf("out = %+v, want the none-shaped fallback (timed out)", out)
		}
		if out.Error == "" {
			t.Errorf("Error is empty, want it to name the timeout")
		}
		// The 2s decider delay must never be waited out — a generous upper
		// bound (well under the decider's own 2s delay, comfortably above
		// the 100ms budget) proves the SHORT timeout fired, not the old
		// fixed 3s one and not the decider's own delay.
		if elapsed >= time.Second {
			t.Errorf("elapsed = %v, want well under 1s (route_timeout_ms=100 should have fired, not the decider's 2s delay or the old fixed 3s)", elapsed)
		}
	})

	t.Run("larger route_timeout_ms lets a slower-than-default-but-fast-enough decider answer", func(t *testing.T) {
		var buf bytes.Buffer
		rec := decide.NewRecorder(&buf, time.Now, "v-test")
		slow := slowDecider{delay: 50 * time.Millisecond, resp: decision.Response{
			Provider: "slow",
			Answers: map[string]decision.Answer{
				"verb": {ID: "verb", Kind: decision.KindChoice, Choice: "localize", Confidence: 0.95},
			},
		}}
		ports := &decide.Ports{Decider: slow, Cfg: config.Decide{Provider: "llm", RouteTimeoutMS: 5000, Sites: []string{decide.SiteRoute}}, Recorder: rec}

		resp, out := postRoute(t, routeHandler(ports), "some prompt")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if out.Provider != "slow" || out.Verb != "localize" || out.Error != "" {
			t.Errorf("out = %+v, want a real answer (route_timeout_ms=5000 should comfortably cover the decider's 50ms delay)", out)
		}
	})

	t.Run("routeDecideTimeout falls back to the default when Cfg.RouteTimeoutMS is unset", func(t *testing.T) {
		ports := &decide.Ports{Cfg: config.Decide{}}
		if got := routeDecideTimeout(ports); got != routeDecideTimeoutDefault {
			t.Errorf("routeDecideTimeout(unset) = %v, want the default %v", got, routeDecideTimeoutDefault)
		}
		if got := routeDecideTimeout(nil); got != routeDecideTimeoutDefault {
			t.Errorf("routeDecideTimeout(nil) = %v, want the default %v", got, routeDecideTimeoutDefault)
		}
		want := 1500 * time.Millisecond
		ports2 := &decide.Ports{Cfg: config.Decide{RouteTimeoutMS: 1500}}
		if got := routeDecideTimeout(ports2); got != want {
			t.Errorf("routeDecideTimeout(1500ms configured) = %v, want %v", got, want)
		}
	})
}
