// Command endpoint-guard is a self-contained, in-pod HTTP server that puts the
// endpoint-guard kit in front of an LLM-answer backend.
//
// Usage:
//
//	endpoint-guard [--config path] [--addr :8787] [--state-file workspace/state/endpoint-guard.json] [--demo]
//
// Flags:
//
//	--config      path to a JSON config file (see config.example.json; uses
//	              sane aggressive defaults when absent). Canonical duration
//	              fields are integer seconds (*Seconds); the legacy nanosecond
//	              shape is still accepted.
//	--addr        TCP address to listen on (default ":8787")
//	--state-file  path for durable Guard snapshots (default
//	              "workspace/state/endpoint-guard.json")
//	--demo        wire MockEmbedder + MockClient so the server runs with no
//	              external services or API keys
//
// Production mode (no --demo) wires the real providers: the Anthropic
// Messages API (foundations/llmclient/anthropic) as both answerer and
// topic-gate judge, and an OpenAI-compatible embeddings backend
// (foundations/embedding/http-embedder) when the Embedding config section is
// enabled. Without an embedding backend the topic gate runs judge-only (an
// LLM judge call for every question surviving the prefilters) and the answer
// cache degrades to exact-match-only.
//
// Production mode FAILS CLOSED at startup: a placeholder identity salt, an
// enabled PoW gate without an adequate secret, a missing LLM model, or an
// unresolvable API key environment variable each refuse to start with an
// error naming exactly what to set. Demo mode skips these checks.
//
// Routes:
//
//	POST /ask    — Guard.HTTPHandler(); body: {"question":"..."} (capped at
//	               MaxBodyBytes, default 64 KiB; oversized → templated 413,
//	               never an LLM call)
//	GET  /healthz — always 200 "ok"
//	GET  /metrics — Prometheus text format: uptime_seconds, request_count,
//	               endpoint_guard_requests_total{outcome=...} and
//	               endpoint_guard_{llm,embedding}_errors_total
//
// Snapshot durability: the mutable Guard state (budget, strikes, cache) is
// written to --state-file every snapshotInterval seconds and on graceful
// shutdown (SIGINT / SIGTERM).
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	endpointguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/endpoint-guard"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
	httpembedder "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/http-embedder"
	answercache "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/answer-cache"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	circuitbreaker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/circuit-breaker"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/identity"
	strikeledger "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/strike-ledger"
	topicgate "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/topic-gate"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/anthropic"
	injectiondetector "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/injection-detector"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/metrics"
)

// snapshotInterval is how often the Guard state is persisted to disk.
const snapshotInterval = 30 * time.Second

// rateLimiterGCInterval is how often the KeyedLimiter's idle-bucket GC runs.
// Buckets idle for more than one minute (the limiter's hard-coded threshold)
// are evicted; sweeping every minute is sufficient and cheap.
const rateLimiterGCInterval = time.Minute

// Edge timeouts applied to the http.Server (see newHTTPServer). They bound
// slow-loris header dribbling, slow request bodies, slow readers of large
// responses, and idle keep-alive connections respectively.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
)

// aggressiveDefaults returns a conservative, high-security Config suitable
// for a public-facing endpoint with no tuning file present.
func aggressiveDefaults() endpointguard.Config {
	return endpointguard.Config{
		Identity: identity.Config{
			Salt:             "change-me-in-production",
			TrustedProxyHops: 1,
		},
		KeyRate: endpointguard.KeyRateConfig{
			// ~20 req/min per identity → ≈0.33 req/s with a short burst
			RatePerSec: 0.333,
			Burst:      3,
		},
		// Global: a little headroom above per-key to allow several distinct users
		GlobalRatePerSec: 1.0,
		GlobalBurst:      10,
		Breaker: circuitbreaker.Config{
			FailureThreshold: 5,
			OpenCooldown:     30 * time.Second,
			HalfOpenProbes:   2,
		},
		Budget: budgetguard.Config{
			// ~20 req/min, ~500 req/day, ~$2/day
			MaxReqPerMin: 20,
			MaxReqPerDay: 500,
			MaxUSDPerDay: 2.00,
		},
		Strikes: strikeledger.Config{
			Threshold: 3,
			BaseBlock: 5 * time.Minute,
			MaxBlock:  24 * time.Hour,
		},
		Topic: topicgate.Config{
			// Placeholder exemplars — operators must replace these with
			// domain-specific on-topic examples.
			Exemplars: []string{
				"What is the capital of France?",
				"How do I reverse a string in Python?",
				"Explain the difference between TCP and UDP.",
			},
			MaxLen:  500,
			LowSim:  0.35,
			HighSim: 0.65,
		},
		Cache: answercache.Config{
			TTL:          15 * time.Minute,
			SimThreshold: 0.92,
			MaxEntries:   256,
		},
		PoW: endpointguard.PoWConfig{
			// Secret is intentionally nil here: production runs refuse to
			// start without a configured PoW.Secret (validateProductionConfig
			// fails closed), and --demo disables PoW entirely — there is no
			// silent fallback.
			Enabled: true,
			Bits:    18,
			MinBits: 18,
			TTL:     5 * time.Minute,
			MaxSeen: 65536,
		},
		SystemPrompt:      "You are a helpful, concise assistant. Stay on topic.",
		EstCostPerCallUSD: 0.004,
		TopicJudgeCostUSD: 0.001,
	}
}

// cryptoRandHex returns 16 random bytes encoded as a hex string.
// Used as the RandHex dep in production.
func cryptoRandHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to timestamp string.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// server bundles the Guard and ancillary request telemetry.
type server struct {
	guard     *endpointguard.Guard
	startedAt time.Time
	reqCount  atomic.Int64
	stateFile string
	stderr    io.Writer

	// metrics, when non-nil, is rendered on GET /metrics after the legacy
	// uptime/request_count lines. The kit and the provider wrappers record
	// into the same registry.
	metrics *metrics.Registry

	// maxBodyBytes caps the /ask request body. Zero or negative falls back
	// to defaultMaxBodyBytes.
	maxBodyBytes int64
}

// saveSnapshot writes Guard.Snapshot() to s.stateFile as JSON.
func (s *server) saveSnapshot() {
	snap := s.guard.Snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(s.stderr, "endpoint-guard: snapshot marshal error: %v\n", err)
		return
	}
	dir := filepath.Dir(s.stateFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(s.stderr, "endpoint-guard: cannot create state dir %q: %v\n", dir, err)
		return
	}
	tmpPath := s.stateFile + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		fmt.Fprintf(s.stderr, "endpoint-guard: cannot write snapshot tmp file: %v\n", err)
		return
	}
	if err := os.Rename(tmpPath, s.stateFile); err != nil {
		fmt.Fprintf(s.stderr, "endpoint-guard: cannot rename snapshot file: %v\n", err)
	}
}

// loadSnapshot reads and restores a previously saved snapshot, if present.
func (s *server) loadSnapshot() {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(s.stderr, "endpoint-guard: cannot read state file %q: %v\n", s.stateFile, err)
		}
		return
	}
	var snap endpointguard.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		fmt.Fprintf(s.stderr, "endpoint-guard: cannot parse state file %q: %v\n", s.stateFile, err)
		return
	}
	s.guard.Restore(snap)
	fmt.Fprintf(s.stderr, "endpoint-guard: restored state from %q\n", s.stateFile)
}

// mux builds the HTTP mux for the server.
func (s *server) mux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(s.startedAt).Round(time.Second)
		reqs := s.reqCount.Load()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "uptime_seconds %d\n", int(uptime.Seconds()))
		fmt.Fprintf(w, "request_count %d\n", reqs)
		if s.metrics != nil {
			io.WriteString(w, s.metrics.Expose())
		}
	})

	askHandler := s.guard.HTTPHandler()
	mux.Handle("/ask", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.reqCount.Add(1)

		// Cap the body BEFORE any JSON decoding. An oversized body gets a
		// templated 413 refusal here — the kit handler (and therefore any
		// LLM call) is never reached.
		limit := s.maxBodyBytes
		if limit <= 0 {
			limit = defaultMaxBodyBytes
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				var mbe *http.MaxBytesError
				if errors.As(err, &mbe) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					fmt.Fprintf(w, `{"action":"Refused","reason":"request body exceeds the %d-byte limit"}`+"\n", limit)
					return
				}
				http.Error(w, "bad request: cannot read body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
		}

		askHandler.ServeHTTP(w, r)
	}))

	return mux
}

// newHTTPServer wraps handler in an http.Server hardened with edge timeouts:
// ReadHeaderTimeout 5s, ReadTimeout 30s, WriteTimeout 60s, IdleTimeout 120s.
func newHTTPServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// run is the testable entry point. It parses args, builds the Guard, and
// serves until a signal or context cancellation.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("endpoint-guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath = fs.String("config", "", "path to JSON config file (uses defaults if absent)")
		addr       = fs.String("addr", ":8787", "TCP address to listen on")
		stateFile  = fs.String("state-file", "workspace/state/endpoint-guard.json", "path for Guard snapshot durability")
		demo       = fs.Bool("demo", false, "use MockEmbedder + MockClient (no external services required)")
	)
	if err := fs.Parse(args); err != nil {
		// flag already printed the error
		return 2
	}

	// ---- Load config ----------------------------------------------------------
	var raw []byte
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "endpoint-guard: cannot read config %q: %v\n", *configPath, err)
			return 1
		}
		raw = data
	}
	cfg, prov, err := loadConfig(raw)
	if err != nil {
		fmt.Fprintf(stderr, "endpoint-guard: cannot parse config %q: %v\n", *configPath, err)
		return 1
	}

	// ---- Build deps -----------------------------------------------------------
	registry := metrics.NewRegistry()
	answererErrs := registry.Counter(metricLLMErrors, "role", "answerer")
	judgeErrs := registry.Counter(metricLLMErrors, "role", "judge")
	embeddingErrs := registry.Counter(metricEmbeddingErrors)

	var embedder embeddingclient.Embedder
	var answerer, judge llmclient.LLMClient

	if *demo {
		embedder = &countingEmbedder{inner: embeddingclient.NewMockEmbedder(0), errs: embeddingErrs}
		// MockClient with no scripted entries returns "mock: no more entries" forever.
		answerer = &countingLLM{inner: llmclient.NewMockClient(), errs: answererErrs}
		fmt.Fprintf(stderr, "endpoint-guard: running in DEMO mode — MockEmbedder + MockClient active\n")
	} else {
		// Production mode: FAIL CLOSED on placeholder salt, inadequate PoW
		// secret, missing model, or unresolvable API key.
		if err := validateProductionConfig(cfg, prov); err != nil {
			fmt.Fprintf(stderr, "endpoint-guard: refusing to start: %v\n", err)
			return 1
		}

		client, err := anthropic.New(anthropic.Config{
			APIKeyEnv:        prov.LLM.APIKeyEnv,
			BaseURL:          prov.LLM.BaseURL,
			DefaultModel:     prov.LLM.Model,
			DefaultMaxTokens: prov.LLM.MaxTokens,
		})
		if err != nil {
			fmt.Fprintf(stderr, "endpoint-guard: cannot build Anthropic client: %v\n", err)
			return 1
		}
		// One Anthropic client serves both roles; separate wrappers keep the
		// error counters distinguishable.
		answerer = &countingLLM{inner: client, errs: answererErrs}
		judge = &countingLLM{inner: client, errs: judgeErrs}

		if prov.Embedding.Enabled {
			emb, err := httpembedder.New(httpembedder.Config{
				BaseURL:   prov.Embedding.BaseURL,
				Model:     prov.Embedding.Model,
				APIKeyEnv: prov.Embedding.APIKeyEnv,
			})
			if err != nil {
				fmt.Fprintf(stderr, "endpoint-guard: cannot build embedding client: %v\n", err)
				return 1
			}
			embedder = &countingEmbedder{inner: emb, errs: embeddingErrs}
			fmt.Fprintf(stderr, "endpoint-guard: production mode — Anthropic model %q + embedding backend %s (model %q)\n",
				prov.LLM.Model, prov.Embedding.BaseURL, prov.Embedding.Model)
		} else {
			// embedder stays nil: the kit runs the topic gate judge-only and
			// the answer cache exact-match-only.
			fmt.Fprintf(stderr, "endpoint-guard: production mode — Anthropic model %q, NO embedding backend: topic gate runs judge-only (one LLM judge call for EVERY question passing the prefilters; configure the Embedding section to make most decisions free) and the answer cache matches exact normalized questions only (near-duplicate matching disabled)\n",
				prov.LLM.Model)
		}
	}

	detector, err := injectiondetector.NewDetector(injectiondetector.DefaultRules())
	if err != nil {
		fmt.Fprintf(stderr, "endpoint-guard: cannot build injection detector: %v\n", err)
		return 1
	}

	// In demo mode, relax constraints that require real embedding similarity or PoW
	// so that any question passes the topic gate without tuning exemplars.
	if *demo {
		cfg.PoW.Enabled = false
		// Accept every question: set LowSim below any possible cosine score.
		// LowSim and HighSim both at -2.0: since cosine ∈ [-1,1], every score
		// satisfies score >= -2.0 (HighSim) → immediate Allow.
		cfg.Topic.LowSim = -2.0
		cfg.Topic.HighSim = -2.0
	}

	deps := endpointguard.Deps{
		Embedder: embedder,
		Detector: detector,
		Judge:    judge,
		Answerer: answerer,
		Now:      time.Now,
		RandHex:  cryptoRandHex,
		Metrics:  registry,
	}

	guard, err := endpointguard.New(cfg, deps)
	if err != nil {
		fmt.Fprintf(stderr, "endpoint-guard: cannot initialise guard: %v\n", err)
		return 1
	}

	// ---- Restore snapshot -----------------------------------------------------
	srv := &server{
		guard:        guard,
		startedAt:    time.Now(),
		stateFile:    *stateFile,
		stderr:       stderr,
		metrics:      registry,
		maxBodyBytes: prov.MaxBodyBytes,
	}
	srv.loadSnapshot()

	// ---- Start listener -------------------------------------------------------
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(stderr, "endpoint-guard: cannot listen on %q: %v\n", *addr, err)
		return 1
	}
	fmt.Fprintf(stdout, "endpoint-guard: listening on %s\n", ln.Addr())

	httpSrv := newHTTPServer(srv.mux())

	// ---- Periodic snapshot goroutine ------------------------------------------
	snapCtx, snapCancel := context.WithCancel(context.Background())
	defer snapCancel()
	go func() {
		ticker := time.NewTicker(snapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				srv.saveSnapshot()
			case <-snapCtx.Done():
				return
			}
		}
	}()

	// ---- Periodic rate-limiter GC goroutine ------------------------------------
	// The KeyedLimiter's per-key bucket map grows without bound unless idle
	// entries are swept. GCRateLimiter evicts buckets idle for > 1 minute;
	// sweeping every rateLimiterGCInterval keeps the map bounded at the cost
	// of one cheap mutex acquisition per interval.
	go func() {
		ticker := time.NewTicker(rateLimiterGCInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				guard.GCRateLimiter()
			case <-snapCtx.Done():
				return
			}
		}
	}()

	// ---- Signal handling ------------------------------------------------------
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Serve in background.
	serveErr := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
		close(serveErr)
	}()

	// Wait for signal or serve error.
	select {
	case sig := <-sigCh:
		fmt.Fprintf(stdout, "endpoint-guard: received %s, shutting down\n", sig)
	case err := <-serveErr:
		if err != nil {
			fmt.Fprintf(stderr, "endpoint-guard: serve error: %v\n", err)
			snapCancel()
			return 1
		}
	}

	// Graceful shutdown.
	snapCancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		fmt.Fprintf(stderr, "endpoint-guard: shutdown error: %v\n", err)
	}

	// Final snapshot.
	srv.saveSnapshot()
	fmt.Fprintf(stdout, "endpoint-guard: snapshot saved to %s\n", *stateFile)

	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
