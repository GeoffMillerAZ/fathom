package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	endpointguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/endpoint-guard"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
	answercache "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/answer-cache"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	circuitbreaker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/circuit-breaker"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/identity"
	proofofwork "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/proof-of-work"
	strikeledger "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/strike-ledger"
	topicgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/topic-gate"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/injection-detector"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/metrics"
)

// ---- helpers -----------------------------------------------------------------

// askResponse mirrors the JSON returned by /ask.
type askResponse struct {
	Action string `json:"action"`
	Answer string `json:"answer"`
	Reason string `json:"reason"`
	Cached bool   `json:"cached"`
}

func postAsk(t *testing.T, srv *httptest.Server, question string) (int, askResponse) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"question": question})
	resp, err := http.Post(srv.URL+"/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /ask: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ar askResponse
	_ = json.Unmarshal(raw, &ar)
	return resp.StatusCode, ar
}

// buildDemoGuard builds a Guard with all demo mocks and config tuned for tests.
// topicExemplars must be on-topic strings; onTopic/offTopic are used in tests.
func buildDemoGuard(t *testing.T, stateFile string, extraAnswers ...llmclient.MockEntry) *endpointguard.Guard {
	t.Helper()

	// Use very permissive thresholds: LowSim=0 so MockEmbedder always passes.
	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test", TrustedProxyHops: 0},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 1000, Burst: 1000},
		GlobalRatePerSec: 1000,
		GlobalBurst:      1000,
		Breaker: circuitbreaker.Config{
			FailureThreshold: 100,
			OpenCooldown:     time.Second,
			HalfOpenProbes:   1,
		},
		Budget: budgetguard.Config{
			MaxReqPerMin: 1000,
			MaxReqPerDay: 10000,
			MaxUSDPerDay: 100.0,
		},
		Strikes: strikeledger.Config{
			Threshold: 100,
			BaseBlock: time.Minute,
			MaxBlock:  time.Hour,
		},
		Topic: topicgate.Config{
			// Single exemplar matching "hello" — everything gets a decent score via MockEmbedder.
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    -1.0, // accept anything
			HighSim:   0.0,  // everything >= 0 is Allow
		},
		Cache: answercache.Config{
			TTL:          5 * time.Minute,
			SimThreshold: 0.99,
			MaxEntries:   64,
		},
		PoW:               endpointguard.PoWConfig{Enabled: false},
		SystemPrompt:      "test system",
		EstCostPerCallUSD: 0.001,
	}

	embedder := embeddingclient.NewMockEmbedder(0)
	detector, err := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	if err != nil {
		t.Fatalf("NewDetector: %v", err)
	}

	entries := []llmclient.MockEntry{
		llmclient.MockEntryOK("this is the answer"),
	}
	entries = append(entries, extraAnswers...)
	answerer := llmclient.NewMockClient(entries...)

	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  func() string { return "testrand" },
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}
	return guard
}

// buildDemoGuardWithAnswerer builds a demo Guard and also returns its
// MockClient answerer so tests can assert on LLM call counts.
func buildDemoGuardWithAnswerer(t *testing.T) (*endpointguard.Guard, *llmclient.MockClient) {
	t.Helper()

	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test", TrustedProxyHops: 0},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 1000, Burst: 1000},
		GlobalRatePerSec: 1000,
		GlobalBurst:      1000,
		Breaker:          circuitbreaker.Config{FailureThreshold: 100, OpenCooldown: time.Second, HalfOpenProbes: 1},
		Budget:           budgetguard.Config{MaxReqPerMin: 1000, MaxReqPerDay: 10000, MaxUSDPerDay: 100.0},
		Strikes:          strikeledger.Config{Threshold: 100, BaseBlock: time.Minute, MaxBlock: time.Hour},
		Topic: topicgate.Config{
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    -2.0,
			HighSim:   -2.0,
		},
		Cache:             answercache.Config{TTL: 5 * time.Minute, SimThreshold: 0.99, MaxEntries: 64},
		PoW:               endpointguard.PoWConfig{Enabled: false},
		SystemPrompt:      "test system",
		EstCostPerCallUSD: 0.001,
	}
	detector, err := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	if err != nil {
		t.Fatalf("NewDetector: %v", err)
	}
	answerer := llmclient.NewMockClient(llmclient.MockEntryOK("this is the answer"))
	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embeddingclient.NewMockEmbedder(0),
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  func() string { return "testrand" },
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}
	return guard, answerer
}

// ---- Test: on-topic question returns an answer (200) ------------------------

func TestOnTopicReturnsAnswer(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	guard := buildDemoGuard(t, stateFile)

	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: stateFile,
		stderr:    io.Discard,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	status, ar := postAsk(t, ts, "hello world")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d (action=%s reason=%s)", status, ar.Action, ar.Reason)
	}
	if ar.Action != "Answered" {
		t.Errorf("expected action=Answered, got %s", ar.Action)
	}
	if ar.Answer == "" {
		t.Error("expected non-empty Answer")
	}
}

// ---- Test: off-topic question returns 422 -----------------------------------

func TestOffTopicReturns422(t *testing.T) {
	dir := t.TempDir()
	// Build a guard with strict topic gate: HighSim=2.0 is unreachable
	// and LowSim=2.0 forces everything into SoftDeny.
	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test"},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 1000, Burst: 1000},
		GlobalRatePerSec: 1000,
		GlobalBurst:      1000,
		Breaker: circuitbreaker.Config{
			FailureThreshold: 100,
			OpenCooldown:     time.Second,
			HalfOpenProbes:   1,
		},
		Budget:  budgetguard.Config{MaxReqPerMin: 1000, MaxReqPerDay: 10000, MaxUSDPerDay: 100.0},
		Strikes: strikeledger.Config{Threshold: 100, BaseBlock: time.Minute, MaxBlock: time.Hour},
		Topic: topicgate.Config{
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    2.0, // impossible threshold — always SoftDeny
			HighSim:   3.0,
		},
		Cache: answercache.Config{TTL: time.Minute, SimThreshold: 0.99},
		PoW:   endpointguard.PoWConfig{Enabled: false},
	}

	embedder := embeddingclient.NewMockEmbedder(0)
	detector, _ := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	answerer := llmclient.NewMockClient()

	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  func() string { return "testrand" },
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}

	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: filepath.Join(dir, "state.json"),
		stderr:    io.Discard,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	status, ar := postAsk(t, ts, "what is the weather today?")
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d (action=%s reason=%s)", status, ar.Action, ar.Reason)
	}
	if ar.Action != "OffTopic" {
		t.Errorf("expected action=OffTopic, got %s", ar.Action)
	}
}

// ---- Test: flood trips to 429 -----------------------------------------------

func TestFloodTrips429(t *testing.T) {
	dir := t.TempDir()

	// Very tight rate limit: 1 req/s burst=1
	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test"},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 0.001, Burst: 1},
		GlobalRatePerSec: 0.001,
		GlobalBurst:      1,
		Breaker: circuitbreaker.Config{
			FailureThreshold: 100,
			OpenCooldown:     time.Second,
			HalfOpenProbes:   1,
		},
		Budget:  budgetguard.Config{MaxReqPerMin: 10000, MaxReqPerDay: 10000, MaxUSDPerDay: 100.0},
		Strikes: strikeledger.Config{Threshold: 100, BaseBlock: time.Minute, MaxBlock: time.Hour},
		Topic: topicgate.Config{
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    -1.0,
			HighSim:   0.0,
		},
		Cache:             answercache.Config{TTL: time.Minute, SimThreshold: 0.99},
		PoW:               endpointguard.PoWConfig{Enabled: false},
		EstCostPerCallUSD: 0.0,
	}

	embedder := embeddingclient.NewMockEmbedder(0)
	detector, _ := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	// Unlimited answers
	answerer := llmclient.NewMockClient()

	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  func() string { return "testrand" },
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}

	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: filepath.Join(dir, "state.json"),
		stderr:    io.Discard,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// Send many requests; at least one must be rate-limited.
	got429 := false
	for i := 0; i < 20; i++ {
		status, ar := postAsk(t, ts, fmt.Sprintf("hello world %d", i))
		if status == http.StatusTooManyRequests {
			got429 = true
			if ar.Action != "RateLimited" {
				t.Errorf("expected action=RateLimited on 429, got %s", ar.Action)
			}
			break
		}
	}
	if !got429 {
		t.Error("expected at least one 429 RateLimited response under flood")
	}
}

// ---- Test: snapshot file is written -----------------------------------------

func TestSnapshotFileIsWritten(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "subdir", "state.json")

	guard := buildDemoGuard(t, stateFile)
	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: stateFile,
		stderr:    io.Discard,
	}

	// Issue a request so there is something to snapshot.
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()
	postAsk(t, ts, "hello world")

	// Write snapshot.
	srv.saveSnapshot()

	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}
	data, _ := os.ReadFile(stateFile)
	var snap endpointguard.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("snapshot is not valid JSON: %v", err)
	}
}

// ---- Test: snapshot roundtrip ------------------------------------------------

func TestSnapshotRoundtrip(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	guard := buildDemoGuard(t, stateFile)
	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: stateFile,
		stderr:    io.Discard,
	}

	// Make a request to put state into the guard.
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()
	postAsk(t, ts, "hello world")

	srv.saveSnapshot()
	srv.loadSnapshot() // should succeed silently
}

// ---- Test: production mode fails closed at startup ---------------------------

// writeConfig marshals m to a JSON file in dir and returns its path.
func writeConfig(t *testing.T, dir string, m map[string]any) string {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestProductionFailClosedStartup(t *testing.T) {
	t.Setenv("ENDPOINT_GUARD_TEST_KEY", "test-api-key")

	tests := []struct {
		name       string
		cfg        map[string]any // nil → run without --config (built-in defaults)
		wantStderr string
	}{
		{
			name:       "default config has placeholder salt",
			cfg:        nil,
			wantStderr: "Identity.Salt",
		},
		{
			name: "example placeholder salt refused",
			cfg: map[string]any{
				"Identity": map[string]any{"Salt": "replace-with-a-secret-random-string"},
			},
			wantStderr: "Identity.Salt",
		},
		{
			name: "PoW enabled without secret",
			cfg: map[string]any{
				"Identity": map[string]any{"Salt": "real-salt"},
				"PoW":      map[string]any{"Enabled": true, "Bits": 18, "TTLSeconds": 300},
			},
			wantStderr: "PoW.Secret",
		},
		{
			name: "PoW enabled with too-short secret",
			cfg: map[string]any{
				"Identity": map[string]any{"Salt": "real-salt"},
				// base64 of "abc" — 3 bytes, far below proofofwork.MinSecretLen
				"PoW": map[string]any{"Enabled": true, "Secret": "YWJj", "Bits": 18, "TTLSeconds": 300},
			},
			wantStderr: "PoW.Secret",
		},
		{
			name: "missing LLM model",
			cfg: map[string]any{
				"Identity": map[string]any{"Salt": "real-salt"},
				"PoW":      map[string]any{"Enabled": false, "Bits": 1, "TTLSeconds": 60},
				"LLM":      map[string]any{"APIKeyEnv": "ENDPOINT_GUARD_TEST_KEY"},
			},
			wantStderr: "LLM.Model",
		},
		{
			name: "API key env does not resolve",
			cfg: map[string]any{
				"Identity": map[string]any{"Salt": "real-salt"},
				"PoW":      map[string]any{"Enabled": false, "Bits": 1, "TTLSeconds": 60},
				"LLM":      map[string]any{"Model": "test-model", "APIKeyEnv": "ENDPOINT_GUARD_TEST_NO_SUCH_VAR_12345"},
			},
			wantStderr: "ENDPOINT_GUARD_TEST_NO_SUCH_VAR_12345",
		},
		{
			name: "embedding enabled without base url",
			cfg: map[string]any{
				"Identity":  map[string]any{"Salt": "real-salt"},
				"PoW":       map[string]any{"Enabled": false, "Bits": 1, "TTLSeconds": 60},
				"LLM":       map[string]any{"Model": "test-model", "APIKeyEnv": "ENDPOINT_GUARD_TEST_KEY"},
				"Embedding": map[string]any{"Enabled": true, "Model": "embed-model"},
			},
			wantStderr: "Embedding.BaseURL",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			args := []string{
				"--addr", randomAddr(t),
				"--state-file", filepath.Join(dir, "state.json"),
				// NO --demo flag → production mode.
			}
			if tc.cfg != nil {
				args = append(args, "--config", writeConfig(t, dir, tc.cfg))
			}

			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			if code == 0 {
				t.Fatal("expected non-zero exit (fail closed)")
			}
			if !strings.Contains(stderr.String(), "refusing to start") {
				t.Errorf("stderr should say the server refuses to start; got: %s", stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr should name %q; got: %s", tc.wantStderr, stderr.String())
			}
		})
	}
}

// ---- Test: run --demo starts server and /healthz returns 200 ----------------

func TestRunDemoHealthz(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	// Find a free port.
	addr := randomAddr(t)

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan int, 1)
	go func() {
		code := run([]string{
			"--demo",
			"--addr", addr,
			"--state-file", stateFile,
		}, &stdout, &stderr)
		done <- code
	}()

	// Wait for server to be ready.
	if err := waitForAddr(addr, 3*time.Second); err != nil {
		cancel()
		t.Fatalf("server did not start: %v", err)
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /healthz, got %d", resp.StatusCode)
	}
	_ = ctx
}

// ---- Test: run --demo /ask returns answer -----------------------------------

func TestRunDemoAskAnswers(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	addr := randomAddr(t)

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		code := run([]string{
			"--demo",
			"--addr", addr,
			"--state-file", stateFile,
		}, &stdout, &stderr)
		done <- code
	}()

	if err := waitForAddr(addr, 3*time.Second); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"question": "hello"})
	resp, err := http.Post("http://"+addr+"/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /ask: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var ar askResponse
	_ = json.Unmarshal(raw, &ar)
	// In demo mode the answerer has no scripted entries → returns fallback "mock: no more entries"
	// which is still a valid answer.  Status must be 200.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d; body=%s", resp.StatusCode, raw)
	}
	if ar.Action != "Answered" {
		t.Errorf("expected Answered, got %s (reason=%s)", ar.Action, ar.Reason)
	}
}

// ---- Test: /metrics endpoint ------------------------------------------------

func TestMetricsEndpoint(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	guard := buildDemoGuard(t, stateFile)

	srv := &server{
		guard:     guard,
		startedAt: time.Now().Add(-5 * time.Minute),
		stateFile: stateFile,
		stderr:    io.Discard,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "uptime_seconds") {
		t.Errorf("metrics missing uptime_seconds; got: %s", s)
	}
	if !strings.Contains(s, "request_count") {
		t.Errorf("metrics missing request_count; got: %s", s)
	}
}

// ---- Test: POST /ask wrong method -------------------------------------------

func TestAskWrongMethod(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	guard := buildDemoGuard(t, stateFile)

	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: stateFile,
		stderr:    io.Discard,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ask")
	if err != nil {
		t.Fatalf("GET /ask: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// ---- Test: --config flag loads file -----------------------------------------

func TestConfigFlagLoadsFile(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	// Write a minimal valid config with PoW disabled.
	minCfg := aggressiveDefaults()
	minCfg.PoW.Enabled = false
	data, _ := json.Marshal(minCfg)
	cfgFile := filepath.Join(dir, "config.json")
	_ = os.WriteFile(cfgFile, data, 0o600)

	addr := randomAddr(t)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		code := run([]string{
			"--demo",
			"--config", cfgFile,
			"--addr", addr,
			"--state-file", stateFile,
		}, &stdout, &stderr)
		done <- code
	}()

	if err := waitForAddr(addr, 3*time.Second); err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ---- Test: PoW challenge flow over HTTP (server mux) --------------------------

func TestPoWChallengeFlowOverHTTP(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	// Demo-style guard, but with the PoW gate ON and a configured secret —
	// mirrors a production run (which refuses to start without one; --demo
	// disables PoW entirely).
	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test", TrustedProxyHops: 0},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 1000, Burst: 1000},
		GlobalRatePerSec: 1000,
		GlobalBurst:      1000,
		Breaker:          circuitbreaker.Config{FailureThreshold: 100, OpenCooldown: time.Second, HalfOpenProbes: 1},
		Budget:           budgetguard.Config{MaxReqPerMin: 1000, MaxReqPerDay: 10000, MaxUSDPerDay: 100.0},
		Strikes:          strikeledger.Config{Threshold: 100, BaseBlock: time.Minute, MaxBlock: time.Hour},
		Topic: topicgate.Config{
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    -1.0,
			HighSim:   0.0,
		},
		Cache:             answercache.Config{TTL: 5 * time.Minute, SimThreshold: 0.99, MaxEntries: 64},
		PoW:               endpointguard.PoWConfig{Enabled: true, Bits: 8, TTL: time.Minute, Secret: []byte("0123456789abcdef0123456789abcdef")},
		SystemPrompt:      "test system",
		EstCostPerCallUSD: 0.001,
	}

	embedder := embeddingclient.NewMockEmbedder(0)
	detector, err := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	if err != nil {
		t.Fatalf("NewDetector: %v", err)
	}
	answerer := llmclient.NewMockClient(llmclient.MockEntryOK("pow-gated answer"))

	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  cryptoRandHex,
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}

	srv := &server{guard: guard, startedAt: time.Now(), stateFile: stateFile, stderr: io.Discard}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// Step 1: no solution → 402 with a signed challenge.
	body, _ := json.Marshal(map[string]string{"question": "hello world"})
	resp1, err := http.Post(ts.URL+"/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST 1: %v", err)
	}
	var out struct {
		Challenge *proofofwork.Challenge `json:"challenge"`
	}
	if err := json.NewDecoder(resp1.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusPaymentRequired || out.Challenge == nil {
		t.Fatalf("want 402 with challenge, got %d challenge=%v", resp1.StatusCode, out.Challenge)
	}
	if out.Challenge.MAC == "" {
		t.Fatal("issued challenge must carry a MAC")
	}

	// Step 2: forged challenge (Bits=0, no MAC) must NOT answer.
	forged, _ := json.Marshal(proofofwork.Challenge{
		Nonce: "forged", Bits: 0, ExpiresAt: time.Now().Add(1000 * time.Hour),
	})
	reqForged, _ := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader(body))
	reqForged.Header.Set("Content-Type", "application/json")
	reqForged.Header.Set("X-PoW-Challenge", string(forged))
	reqForged.Header.Set("X-PoW-Solution", "anything")
	respForged, err := http.DefaultClient.Do(reqForged)
	if err != nil {
		t.Fatalf("POST forged: %v", err)
	}
	respForged.Body.Close()
	if respForged.StatusCode == http.StatusOK {
		t.Fatal("forged challenge must not be answered")
	}

	// Step 3: solve the genuine challenge and resubmit → 200.
	solution := proofofwork.Solve(*out.Challenge)
	chJSON, _ := json.Marshal(out.Challenge)
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/ask", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-PoW-Challenge", string(chJSON))
	req2.Header.Set("X-PoW-Solution", solution)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("POST 2: %v", err)
	}
	defer resp2.Body.Close()
	raw, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("solved challenge: want 200, got %d body=%s", resp2.StatusCode, raw)
	}
}

// ---- helpers ----------------------------------------------------------------

// randomAddr returns a free TCP address on localhost (port chosen by OS).
func randomAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot get free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// waitForAddr polls addr until a TCP connection succeeds or timeout elapses.
func waitForAddr(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("addr %s not ready after %s", addr, timeout)
}

// ---- Test: config parsing — seconds canonical, legacy nanoseconds accepted ----

func TestLoadConfigDurations(t *testing.T) {
	defaults := aggressiveDefaults()

	tests := []struct {
		name             string
		json             string
		wantOpenCooldown time.Duration
		wantBaseBlock    time.Duration
		wantMaxBlock     time.Duration
		wantCacheTTL     time.Duration
		wantPoWTTL       time.Duration
	}{
		{
			name: "canonical seconds shape",
			json: `{
				"Breaker": {"FailureThreshold": 5, "OpenCooldownSeconds": 45, "HalfOpenProbes": 2},
				"Strikes": {"Threshold": 3, "BaseBlockSeconds": 120, "MaxBlockSeconds": 7200},
				"Cache":   {"TTLSeconds": 600, "SimThreshold": 0.9, "MaxEntries": 10},
				"PoW":     {"Enabled": false, "Bits": 1, "TTLSeconds": 90}
			}`,
			wantOpenCooldown: 45 * time.Second,
			wantBaseBlock:    120 * time.Second,
			wantMaxBlock:     7200 * time.Second,
			wantCacheTTL:     600 * time.Second,
			wantPoWTTL:       90 * time.Second,
		},
		{
			name: "legacy nanosecond shape still accepted",
			json: `{
				"Breaker": {"FailureThreshold": 5, "OpenCooldown": 45000000000, "HalfOpenProbes": 2},
				"Strikes": {"Threshold": 3, "BaseBlock": 120000000000, "MaxBlock": 7200000000000},
				"Cache":   {"TTL": 600000000000, "SimThreshold": 0.9, "MaxEntries": 10},
				"PoW":     {"Enabled": false, "Bits": 1, "TTL": 90000000000}
			}`,
			wantOpenCooldown: 45 * time.Second,
			wantBaseBlock:    120 * time.Second,
			wantMaxBlock:     7200 * time.Second,
			wantCacheTTL:     600 * time.Second,
			wantPoWTTL:       90 * time.Second,
		},
		{
			name: "both forms present: seconds wins",
			json: `{
				"Breaker": {"FailureThreshold": 5, "OpenCooldownSeconds": 45, "OpenCooldown": 999000000000, "HalfOpenProbes": 2},
				"Strikes": {"Threshold": 3, "BaseBlockSeconds": 120, "BaseBlock": 999000000000, "MaxBlockSeconds": 7200, "MaxBlock": 999000000000},
				"Cache":   {"TTLSeconds": 600, "TTL": 999000000000, "SimThreshold": 0.9},
				"PoW":     {"Enabled": false, "Bits": 1, "TTLSeconds": 90, "TTL": 999000000000}
			}`,
			wantOpenCooldown: 45 * time.Second,
			wantBaseBlock:    120 * time.Second,
			wantMaxBlock:     7200 * time.Second,
			wantCacheTTL:     600 * time.Second,
			wantPoWTTL:       90 * time.Second,
		},
		{
			name:             "absent durations fall back to defaults",
			json:             `{"GlobalBurst": 42}`,
			wantOpenCooldown: defaults.Breaker.OpenCooldown,
			wantBaseBlock:    defaults.Strikes.BaseBlock,
			wantMaxBlock:     defaults.Strikes.MaxBlock,
			wantCacheTTL:     defaults.Cache.TTL,
			wantPoWTTL:       defaults.PoW.TTL,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _, err := loadConfig([]byte(tc.json))
			if err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
			if cfg.Breaker.OpenCooldown != tc.wantOpenCooldown {
				t.Errorf("OpenCooldown = %v, want %v", cfg.Breaker.OpenCooldown, tc.wantOpenCooldown)
			}
			if cfg.Strikes.BaseBlock != tc.wantBaseBlock {
				t.Errorf("BaseBlock = %v, want %v", cfg.Strikes.BaseBlock, tc.wantBaseBlock)
			}
			if cfg.Strikes.MaxBlock != tc.wantMaxBlock {
				t.Errorf("MaxBlock = %v, want %v", cfg.Strikes.MaxBlock, tc.wantMaxBlock)
			}
			if cfg.Cache.TTL != tc.wantCacheTTL {
				t.Errorf("Cache.TTL = %v, want %v", cfg.Cache.TTL, tc.wantCacheTTL)
			}
			if cfg.PoW.TTL != tc.wantPoWTTL {
				t.Errorf("PoW.TTL = %v, want %v", cfg.PoW.TTL, tc.wantPoWTTL)
			}
		})
	}
}

func TestLoadConfigProviderDefaultsAndSecret(t *testing.T) {
	// Empty config → provider defaults.
	_, prov, err := loadConfig(nil)
	if err != nil {
		t.Fatalf("loadConfig(nil): %v", err)
	}
	if prov.LLM.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("default APIKeyEnv = %q, want ANTHROPIC_API_KEY", prov.LLM.APIKeyEnv)
	}
	if prov.LLM.MaxTokens != defaultLLMMaxTokens {
		t.Errorf("default MaxTokens = %d, want %d", prov.LLM.MaxTokens, defaultLLMMaxTokens)
	}
	if prov.MaxBodyBytes != defaultMaxBodyBytes {
		t.Errorf("default MaxBodyBytes = %d, want %d", prov.MaxBodyBytes, defaultMaxBodyBytes)
	}
	if prov.Embedding.Enabled {
		t.Error("Embedding must default to disabled")
	}

	// Provider sections parse.
	_, prov2, err := loadConfig([]byte(`{
		"LLM": {"Model": "test-model", "BaseURL": "http://example.invalid", "APIKeyEnv": "MY_KEY", "MaxTokens": 64},
		"Embedding": {"Enabled": true, "BaseURL": "http://emb.invalid/v1", "Model": "embed-model", "APIKeyEnv": "EMB_KEY"},
		"MaxBodyBytes": 1024
	}`))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if prov2.LLM.Model != "test-model" || prov2.LLM.BaseURL != "http://example.invalid" || prov2.LLM.APIKeyEnv != "MY_KEY" || prov2.LLM.MaxTokens != 64 {
		t.Errorf("LLM section mis-parsed: %+v", prov2.LLM)
	}
	if !prov2.Embedding.Enabled || prov2.Embedding.BaseURL != "http://emb.invalid/v1" || prov2.Embedding.Model != "embed-model" || prov2.Embedding.APIKeyEnv != "EMB_KEY" {
		t.Errorf("Embedding section mis-parsed: %+v", prov2.Embedding)
	}
	if prov2.MaxBodyBytes != 1024 {
		t.Errorf("MaxBodyBytes = %d, want 1024", prov2.MaxBodyBytes)
	}

	// PoW secret: valid base64 decodes; invalid base64 errors with guidance.
	cfg3, _, err := loadConfig([]byte(`{"PoW": {"Enabled": true, "Secret": "MDEyMzQ1Njc4OWFiY2RlZg==", "Bits": 8, "TTLSeconds": 60}}`))
	if err != nil {
		t.Fatalf("loadConfig with base64 secret: %v", err)
	}
	if string(cfg3.PoW.Secret) != "0123456789abcdef" {
		t.Errorf("decoded secret = %q, want 0123456789abcdef", cfg3.PoW.Secret)
	}
	_, _, err = loadConfig([]byte(`{"PoW": {"Enabled": true, "Secret": "not!!!base64", "Bits": 8, "TTLSeconds": 60}}`))
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Errorf("invalid secret: want base64 guidance error, got %v", err)
	}
}

// ---- Test: oversized body → 413 templated refusal, zero LLM calls -------------

func TestOversizedBodyRefused413(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	guard, answerer := buildDemoGuardWithAnswerer(t)

	srv := &server{
		guard:        guard,
		startedAt:    time.Now(),
		stateFile:    stateFile,
		stderr:       io.Discard,
		maxBodyBytes: 1024,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// Build a syntactically valid JSON body well over the 1 KiB cap.
	big, _ := json.Marshal(map[string]string{"question": strings.Repeat("a", 8*1024)})
	resp, err := http.Post(ts.URL+"/ask", "application/json", bytes.NewReader(big))
	if err != nil {
		t.Fatalf("POST /ask: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d; body=%s", resp.StatusCode, raw)
	}
	var refusal struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &refusal); err != nil {
		t.Fatalf("413 body is not the templated JSON refusal: %v; body=%s", err, raw)
	}
	if refusal.Action != "Refused" || !strings.Contains(refusal.Reason, "1024") {
		t.Errorf("unexpected refusal payload: %+v", refusal)
	}
	if c := answerer.CallCount(); c != 0 {
		t.Fatalf("oversized body must NEVER reach the LLM; got %d calls", c)
	}

	// A request under the cap still works end to end.
	status, ar := postAsk(t, ts, "hello world")
	if status != http.StatusOK || ar.Action != "Answered" {
		t.Fatalf("small body after oversized one: want 200 Answered, got %d %s", status, ar.Action)
	}
}

// ---- Test: hardened edge timeouts on the http.Server ---------------------------

func TestServerTimeoutsConfigured(t *testing.T) {
	s := newHTTPServer(http.NewServeMux())
	if s.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", s.ReadHeaderTimeout)
	}
	if s.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", s.ReadTimeout)
	}
	if s.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %v, want 60s", s.WriteTimeout)
	}
	if s.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", s.IdleTimeout)
	}
	if s.Handler == nil {
		t.Error("handler must be wired")
	}
}

// ---- Test: /metrics golden after scripted demo traffic -------------------------

func TestMetricsGoldenAfterTraffic(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	registry := metrics.NewRegistry()

	// Demo-style guard with a deterministic gauntlet: everything on-topic
	// (thresholds at -2), per-key burst of exactly 3 with negligible refill.
	cfg := endpointguard.Config{
		Identity:         identity.Config{Salt: "test"},
		KeyRate:          endpointguard.KeyRateConfig{RatePerSec: 0.000001, Burst: 3},
		GlobalRatePerSec: 1000,
		GlobalBurst:      1000,
		Breaker:          circuitbreaker.Config{FailureThreshold: 100, OpenCooldown: time.Second, HalfOpenProbes: 1},
		Budget:           budgetguard.Config{MaxReqPerMin: 1000, MaxReqPerDay: 10000, MaxUSDPerDay: 100.0},
		Strikes:          strikeledger.Config{Threshold: 100, BaseBlock: time.Minute, MaxBlock: time.Hour},
		Topic: topicgate.Config{
			Exemplars: []string{"hello world"},
			MaxLen:    500,
			LowSim:    -2.0,
			HighSim:   -2.0,
		},
		Cache:             answercache.Config{TTL: 5 * time.Minute, SimThreshold: 0.99, MaxEntries: 64},
		PoW:               endpointguard.PoWConfig{Enabled: false},
		SystemPrompt:      "test system",
		EstCostPerCallUSD: 0.001,
	}
	embedder := embeddingclient.NewMockEmbedder(0)
	detector, err := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	if err != nil {
		t.Fatalf("NewDetector: %v", err)
	}
	answerer := llmclient.NewMockClient(
		llmclient.MockEntryOK("answer one"),
		llmclient.MockEntryOK("answer two"),
	)
	guard, err := endpointguard.New(cfg, endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  func() string { return "testrand" },
		Metrics:  registry,
	})
	if err != nil {
		t.Fatalf("endpointguard.New: %v", err)
	}

	srv := &server{
		guard:     guard,
		startedAt: time.Now(),
		stateFile: stateFile,
		stderr:    io.Discard,
		metrics:   registry,
	}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	// Scripted traffic: answered, cache_hit (exact repeat), answered, rate_limited.
	postAsk(t, ts, "hello world")
	postAsk(t, ts, "hello world")
	postAsk(t, ts, "another question")
	postAsk(t, ts, "one too many")

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	got := string(body)

	golden := "# TYPE endpoint_guard_requests_total counter\n" +
		`endpoint_guard_requests_total{outcome="answered"} 2` + "\n" +
		`endpoint_guard_requests_total{outcome="cache_hit"} 1` + "\n" +
		`endpoint_guard_requests_total{outcome="rate_limited"} 1` + "\n"
	if !strings.Contains(got, golden) {
		t.Errorf("/metrics missing golden outcome block.\nwant block:\n%s\ngot:\n%s", golden, got)
	}
	// The legacy lines must survive for existing scrapers.
	if !strings.Contains(got, "uptime_seconds") || !strings.Contains(got, "request_count 4") {
		t.Errorf("/metrics missing legacy lines; got:\n%s", got)
	}
}

// ---- Test: production wiring smoke (httptest-backed providers) -----------------

// fakeAnthropic returns an httptest server speaking just enough of the
// Anthropic Messages API for the smoke tests. answerFor inspects the raw
// request body and returns the text to answer with.
func fakeAnthropic(t *testing.T, answerFor func(body string) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		text, _ := json.Marshal(answerFor(string(raw)))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"msg_test","type":"message","role":"assistant","model":"test-model","content":[{"type":"text","text":%s}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":3,"output_tokens":7}}`, text)
	}))
}

// fakeEmbeddings returns an httptest server speaking the OpenAI-compatible
// embeddings wire format, answering the same unit vector for every input so
// all cosine similarities are 1.0.
func fakeEmbeddings(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type datum struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		}
		out := struct {
			Data []datum `json:"data"`
		}{}
		for i := range req.Input {
			out.Data = append(out.Data, datum{Index: i, Embedding: []float64{0.6, 0.8, 0.0}})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
}

func TestRunProductionWiringSmoke(t *testing.T) {
	t.Setenv("ENDPOINT_GUARD_SMOKE_KEY", "test-api-key")

	anth := fakeAnthropic(t, func(string) string { return "prod answer" })
	defer anth.Close()
	emb := fakeEmbeddings(t)
	defer emb.Close()

	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, map[string]any{
		"Identity":         map[string]any{"Salt": "smoke-test-salt"},
		"KeyRate":          map[string]any{"RatePerSec": 1000, "Burst": 1000},
		"GlobalRatePerSec": 1000,
		"GlobalBurst":      1000,
		"Budget":           map[string]any{"MaxReqPerMin": 1000, "MaxReqPerDay": 10000, "MaxUSDPerDay": 100.0},
		"Topic": map[string]any{
			"Exemplars": []string{"hello world"},
			"MaxLen":    500,
			"LowSim":    0.35,
			"HighSim":   0.65,
		},
		"PoW":       map[string]any{"Enabled": false, "Bits": 1, "TTLSeconds": 60},
		"LLM":       map[string]any{"Model": "test-model", "BaseURL": anth.URL, "APIKeyEnv": "ENDPOINT_GUARD_SMOKE_KEY", "MaxTokens": 64},
		"Embedding": map[string]any{"Enabled": true, "BaseURL": emb.URL, "Model": "test-embed"},
	})

	addr := randomAddr(t)
	var stdout, stderr bytes.Buffer
	go func() {
		run([]string{
			"--config", cfgPath,
			"--addr", addr,
			"--state-file", filepath.Join(dir, "state.json"),
			// NO --demo: real provider wiring against the httptest backends.
		}, &stdout, &stderr)
	}()
	if err := waitForAddr(addr, 3*time.Second); err != nil {
		t.Fatalf("server did not start: %v\nstderr: %s", err, stderr.String())
	}

	body, _ := json.Marshal(map[string]string{"question": "hello world"})
	resp, err := http.Post("http://"+addr+"/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /ask: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ar askResponse
	_ = json.Unmarshal(raw, &ar)
	if resp.StatusCode != http.StatusOK || ar.Action != "Answered" || ar.Answer != "prod answer" {
		t.Fatalf("want 200 Answered %q, got %d %s %q (reason=%s)\nstderr: %s",
			"prod answer", resp.StatusCode, ar.Action, ar.Answer, ar.Reason, stderr.String())
	}

	mResp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer mResp.Body.Close()
	mBody, _ := io.ReadAll(mResp.Body)
	if !strings.Contains(string(mBody), `endpoint_guard_requests_total{outcome="answered"} 1`) {
		t.Errorf("metrics missing answered outcome; got:\n%s", mBody)
	}
	if !strings.Contains(string(mBody), `endpoint_guard_llm_errors_total{role="answerer"} 0`) {
		t.Errorf("metrics missing pre-declared llm error counter; got:\n%s", mBody)
	}
}

func TestRunProductionJudgeOnlySmoke(t *testing.T) {
	t.Setenv("ENDPOINT_GUARD_SMOKE_KEY", "test-api-key")

	// Judge prompts contain the on-topic yes/no question; everything else is
	// the paid answer call.
	anth := fakeAnthropic(t, func(body string) string {
		if strings.Contains(body, "on-topic") {
			return "yes"
		}
		return "judge-only prod answer"
	})
	defer anth.Close()

	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, map[string]any{
		"Identity":         map[string]any{"Salt": "smoke-test-salt"},
		"KeyRate":          map[string]any{"RatePerSec": 1000, "Burst": 1000},
		"GlobalRatePerSec": 1000,
		"GlobalBurst":      1000,
		"Budget":           map[string]any{"MaxReqPerMin": 1000, "MaxReqPerDay": 10000, "MaxUSDPerDay": 100.0},
		"PoW":              map[string]any{"Enabled": false, "Bits": 1, "TTLSeconds": 60},
		"LLM":              map[string]any{"Model": "test-model", "BaseURL": anth.URL, "APIKeyEnv": "ENDPOINT_GUARD_SMOKE_KEY", "MaxTokens": 64},
		"Embedding":        map[string]any{"Enabled": false},
	})

	addr := randomAddr(t)
	var stdout, stderr bytes.Buffer
	go func() {
		run([]string{
			"--config", cfgPath,
			"--addr", addr,
			"--state-file", filepath.Join(dir, "state.json"),
		}, &stdout, &stderr)
	}()
	if err := waitForAddr(addr, 3*time.Second); err != nil {
		t.Fatalf("server did not start: %v\nstderr: %s", err, stderr.String())
	}

	body, _ := json.Marshal(map[string]string{"question": "can you help me with my account"})
	resp, err := http.Post("http://"+addr+"/ask", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /ask: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ar askResponse
	_ = json.Unmarshal(raw, &ar)
	if resp.StatusCode != http.StatusOK || ar.Action != "Answered" || ar.Answer != "judge-only prod answer" {
		t.Fatalf("want 200 Answered via judge-only path, got %d %s %q (reason=%s)\nstderr: %s",
			resp.StatusCode, ar.Action, ar.Answer, ar.Reason, stderr.String())
	}
	if !strings.Contains(stderr.String(), "judge-only") {
		t.Errorf("startup log should state judge-only mode; stderr: %s", stderr.String())
	}
}

// ---- Test: the shipped config.example.json parses and converts ----------------

func TestShippedExampleConfigParses(t *testing.T) {
	data, err := os.ReadFile("config.example.json")
	if err != nil {
		t.Fatalf("read config.example.json: %v", err)
	}
	cfg, prov, err := loadConfig(data)
	if err != nil {
		t.Fatalf("the shipped example must load: %v", err)
	}

	// Seconds fields converted to durations.
	if cfg.Breaker.OpenCooldown != 30*time.Second {
		t.Errorf("OpenCooldown = %v, want 30s", cfg.Breaker.OpenCooldown)
	}
	if cfg.Strikes.BaseBlock != 5*time.Minute || cfg.Strikes.MaxBlock != 24*time.Hour {
		t.Errorf("strike blocks = %v/%v, want 5m/24h", cfg.Strikes.BaseBlock, cfg.Strikes.MaxBlock)
	}
	if cfg.Cache.TTL != 15*time.Minute {
		t.Errorf("Cache.TTL = %v, want 15m", cfg.Cache.TTL)
	}
	if cfg.PoW.TTL != 5*time.Minute {
		t.Errorf("PoW.TTL = %v, want 5m", cfg.PoW.TTL)
	}

	// Provider sections.
	if prov.LLM.Model != "claude-haiku-4-5" || prov.LLM.APIKeyEnv != "ANTHROPIC_API_KEY" || prov.LLM.MaxTokens != 1024 {
		t.Errorf("LLM section mis-parsed: %+v", prov.LLM)
	}
	if !prov.Embedding.Enabled || prov.Embedding.Model != "nomic-embed-text" {
		t.Errorf("Embedding section mis-parsed: %+v", prov.Embedding)
	}
	if prov.MaxBodyBytes != 65536 {
		t.Errorf("MaxBodyBytes = %d, want 65536", prov.MaxBodyBytes)
	}
	// The kit-level economics/cap knobs flow through.
	if cfg.TopicJudgeCostUSD != 0.001 {
		t.Errorf("TopicJudgeCostUSD = %v, want 0.001", cfg.TopicJudgeCostUSD)
	}
	if cfg.MaxBodyBytes != 65536 {
		t.Errorf("kit cfg.MaxBodyBytes = %d, want 65536 (HTTPHandler self-cap must agree with the cmd cap)", cfg.MaxBodyBytes)
	}

	// The example is honest about production: its placeholder salt (and
	// missing PoW secret) must FAIL the production checks until replaced.
	err = validateProductionConfig(cfg, prov)
	if err == nil {
		t.Fatal("the example as-shipped must not pass production validation (placeholder salt)")
	}
	if !strings.Contains(err.Error(), "Identity.Salt") {
		t.Errorf("first refusal should be the placeholder salt; got: %v", err)
	}
}
