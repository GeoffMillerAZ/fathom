// decidedby_wire_test.go — JG-2 (ADR-073 decision 6, DMP-3): the
// obligations envelope carries decided_by beside ranked_by only when a
// configured port was asked for it, and with provider none it is the I42
// golden byte for byte.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// jg2Status answers every provider request with one status, no socket.
type jg2Status struct {
	mu     sync.Mutex
	status int
}

func (s *jg2Status) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &http.Response{StatusCode: s.status, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"error":"try later"}`)), Request: r}, nil
}

// jg2Envelope serves the obligations tool for the two I42 claims through c
// and returns the envelope's text.
func jg2Envelope(t *testing.T, c *client.Client) string {
	t.Helper()
	resp := roundtrip(t, mcp.NewServer(c), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "obligations", "arguments": map[string]any{"service": "svc", "file": rerankWireTestFile}},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations tool error: %v", resp["error"])
	}
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content: %v", resp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func jg2Client(t *testing.T) *client.Client {
	t.Helper()
	st := store.NewMemoryStore()
	seedWireClaim(t, st, "i42-1", "i42-1: requires review before any change.")
	seedWireClaim(t, st, "i42-2", "i42-2: requires review before any change too.")
	c := client.New(connector.NewLocalConnector(), st)
	t.Cleanup(func() { c.Close() })
	return c
}

func jg2DecidedBy(t *testing.T, text string) (string, bool) {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v\n%s", err, text)
	}
	v, ok := env["decided_by"]
	s, _ := v.(string)
	return s, ok
}

// TestObligationsWire_DecidedByOnlyWhenConfigured is JG-2's reachable_from:
// with provider none the envelope is the I42 golden; with a configured port
// whose obligations re-rank site is closed it is the same bytes (the port
// was not asked); a judged re-rank names judged(obligations · <provider>/
// <model> · conf <x> · calibration uncalibrated 0/100) from the weakest
// answer it acted on; a provider that misses names `computed · not
// consulted (<class>)`.
// calibrateRerank writes, under the per-user fathom directory, a reading
// that lets the obligations re-rank ask provider typesafe (SR-58's gate):
// the miss below must reach the provider to be one.
func calibrateRerank(t *testing.T) {
	t.Helper()
	samples := make([]decision.LabeledSample, 100)
	for i := range samples {
		samples[i] = decision.LabeledSample{Confidence: 0.95, Correct: true, Provenance: decision.ProvenanceComputed}
	}
	site := decide.SiteObligationsRerank
	r, err := decision.NewReading(decision.ReadingKey{Site: site, TemplateVersion: decide.TemplateVersion(site), Provider: "typesafe"}, samples)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decide.WriteReadingJSON(decide.ReadingsDir(), "2026-10-20", decide.CalibrationReading{Taken: "2026-10-20T00:00:00Z", Reading: r, Baseline: &decide.ABReading{N: 100, Correct: 50}}); err != nil {
		t.Fatal(err)
	}
}

func TestObligationsWire_DecidedByOnlyWhenConfigured(t *testing.T) {
	golden, err := os.ReadFile(i42ObligationsGolden)
	if err != nil {
		t.Fatalf("reading %s: %v", i42ObligationsGolden, err)
	}
	rerankCfg := config.Decide{Provider: "llm", RankWithinTier: true, CompileThreshold: 0.5, Sites: []string{decide.SiteObligationsRerank}}

	t.Run("provider none is the I42 golden", func(t *testing.T) {
		loaded, err := config.Load("", "")
		if err != nil {
			t.Fatal(err)
		}
		c := jg2Client(t)
		c.WithDecision(decision.None{}, loaded.Config.Decide, nil)
		text := jg2Envelope(t, c)
		if text+"\n" != string(golden) {
			t.Errorf("provider none envelope moved off the I42 golden:\n--- got\n%s\n--- want\n%s", text, golden)
		}
		if _, ok := jg2DecidedBy(t, text); ok {
			t.Errorf("provider none carries decided_by: %s", text)
		}
	})

	t.Run("a configured port whose site is closed is the I42 golden", func(t *testing.T) {
		c := jg2Client(t)
		mock := decision.NewMockDecider("mock")
		closed := rerankCfg
		closed.Sites = []string{decide.SiteRoute}
		c.WithDecision(mock, closed, nil)
		text := jg2Envelope(t, c)
		if text+"\n" != string(golden) {
			t.Errorf("a port not asked moved the envelope off the I42 golden:\n--- got\n%s\n--- want\n%s", text, golden)
		}
		if len(mock.Calls()) != 0 {
			t.Errorf("the closed site asked the port %d times", len(mock.Calls()))
		}
	})

	t.Run("a judged re-rank names the weakest answer", func(t *testing.T) {
		c := jg2Client(t)
		mock := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{Provider: "mock", Model: "m-1", Answers: map[string]decision.Answer{
			"i42-1": {ID: "i42-1", Kind: decision.KindPredicate, Noul: 0.55, Confidence: 0.55},
			"i42-2": {ID: "i42-2", Kind: decision.KindPredicate, Noul: 0.95, Confidence: 0.95},
		}}})
		c.WithDecision(mock, rerankCfg, nil)
		text := jg2Envelope(t, c)
		got, ok := jg2DecidedBy(t, text)
		want := "judged(obligations · mock/m-1 · conf 0.55 · calibration uncalibrated 0/100)"
		if !ok || got != want {
			t.Errorf("decided_by = %q (present %v), want %q:\n%s", got, ok, want, text)
		}
		if i, j := strings.Index(text, `"ranked_by"`), strings.Index(text, `"decided_by"`); i < 0 || j < 0 {
			t.Errorf("ranked_by and decided_by must both ride the envelope:\n%s", text)
		}
	})

	t.Run("a miss names not consulted with its class", func(t *testing.T) {
		t.Setenv("FATHOM_HOME", t.TempDir())
		t.Setenv(decide.KillEnv, "")
		t.Setenv("FATHOM_TEST_JG2_KEY", "test-key-jg2-not-a-real-key")
		calibrateRerank(t)
		var disclose bytes.Buffer
		cfg := config.Decide{Provider: "typesafe", APIKeyEnv: "FATHOM_TEST_JG2_KEY", RankWithinTier: true, CompileThreshold: 0.5,
			RerankTimeoutMS: int((2 * time.Second).Milliseconds()), Sites: []string{decide.SiteObligationsRerank}}
		ports, err := decide.Open(cfg, nil, "", nil, decide.WithHTTPClient(&http.Client{Transport: &jg2Status{status: http.StatusTooManyRequests}}),
			decide.WithDisclose(&disclose), decide.WithUserConfig(""), decide.WithKeyIO(config.KeyIO{
				LookupEnv: func(name string) (string, bool) { return "test-key-jg2-not-a-real-key", name == "FATHOM_TEST_JG2_KEY" },
				Stat:      os.Stat,
				RunCmd:    func(context.Context, []string) (string, error) { return "", nil },
			}))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		c := jg2Client(t)
		c.WithDecision(ports.Decider, cfg, nil)
		text := jg2Envelope(t, c)
		got, ok := jg2DecidedBy(t, text)
		if want := "computed · not consulted (rate limited)"; !ok || got != want {
			t.Errorf("decided_by = %q (present %v), want %q:\n%s", got, ok, want, text)
		}
		if strings.Contains(text, "judged") {
			t.Errorf("a missed ask carries a judged token:\n%s", text)
		}
	})
}
