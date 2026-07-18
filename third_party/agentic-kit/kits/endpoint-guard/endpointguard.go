// Package endpointguard is a composition kit that wires the agentic-kit
// governance primitives into a single, hardened HTTP endpoint guard.
//
// It enforces a layered gauntlet (cheapest first):
//
//  1. Circuit breaker — fail if downstream is unhealthy
//  2. Identity extraction — privacy-preserving hashed IP
//  3. Strike check — block repeat violators
//  4. Rate limiting — per-key and global token-bucket limits
//  5. Proof-of-work (optional) — HMAC-signed hashcash challenge/response;
//     only server-issued challenges verify (clients cannot mint their own).
//     Verifying a solution is one HMAC + one SHA-256 — cheaper than any
//     embedding or judge call — so when enabled it runs BEFORE the topic
//     gate: an un-PoW'd request can never trigger paid topic evaluation
//  6. Topic gate — injection detection + embedding similarity. Judge LLM
//     calls are charged to the budget guard BEFORE the judge fires
//     (Config.TopicJudgeCostUSD); a refused charge means no judge call and
//     a templated refusal, with the breaker force-opened exactly like the
//     paid-call budget-trip path
//  7. Answer cache — return cached response without calling the model
//  8. Paid LLM call — call Answerer; record cost; trip breaker on budget
//
// All paths that reject (steps 1-7 misses) make zero Answerer calls, and the
// only LLM spend a rejected request can cause — a topic-gate judge call — is
// itself budget-charged before it happens.
// The Guard is concurrency-safe and uses injected clock + rand for
// deterministic testing.
//
// Degraded modes: when Deps.Embedder is nil, a Judge is required — the topic
// gate then runs judge-only (prefilters → judge; an LLM judge call for every
// surviving question, see topicgate.Config.JudgeOnly for the cost trade-off)
// and the answer cache degrades to exact-match-only (near-duplicate matching
// needs an embedder).
//
// Observability: when Deps.Metrics is set, Handle increments one
// MetricRequests counter per call, labeled outcome=<see Deps.Metrics doc>,
// plus MetricBudgetTrips when a call trips the daily budget.
package endpointguard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	answercache "github.com/geoffmilleraz/agentic-kit/foundations/governance/answer-cache"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	circuitbreaker "github.com/geoffmilleraz/agentic-kit/foundations/governance/circuit-breaker"
	"github.com/geoffmilleraz/agentic-kit/foundations/governance/identity"
	proofofwork "github.com/geoffmilleraz/agentic-kit/foundations/governance/proof-of-work"
	ratelimiter "github.com/geoffmilleraz/agentic-kit/foundations/governance/rate-limiter"
	strikeledger "github.com/geoffmilleraz/agentic-kit/foundations/governance/strike-ledger"
	topicgate "github.com/geoffmilleraz/agentic-kit/foundations/governance/topic-gate"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/agentic-kit/foundations/security/injection-detector"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/metrics"
)

// Metric family names emitted when Deps.Metrics is configured. Exported so
// binaries serving GET /metrics can assert on (and document) the exact names.
const (
	// MetricRequests is incremented once per Handle call, labeled
	// outcome=<one of the outcome* constants below>.
	MetricRequests = "endpoint_guard_requests_total"

	// MetricBudgetTrips is incremented when the budget trips: either a paid
	// call exhausts the daily budget (the answer is still returned; the
	// breaker force-opens) or the budget refuses a topic-judge charge (the
	// judge is never called; the request is refused and the breaker
	// force-opens).
	MetricBudgetTrips = "endpoint_guard_budget_trips_total"
)

// Outcome label values for MetricRequests, derived from the Action vocabulary
// plus the gauntlet branch that produced it.
const (
	outcomeAnswered          = "answered"
	outcomeCacheHit          = "cache_hit"
	outcomeRefusedTopic      = "refused_topic"
	outcomeRefusedInjection  = "refused_injection"
	outcomeBlockedStrike     = "blocked_strike"
	outcomeRateLimited       = "rate_limited"
	outcomeBreakerOpen       = "breaker_open"
	outcomeChallengeRequired = "challenge_required"
	outcomePoWFailed         = "pow_failed"
	outcomeAnswererError     = "answerer_error"
)

// KeyRateConfig holds per-key token-bucket parameters.
// The rate-limiter package exposes only constructor functions, so we define
// our own config struct here and pass its fields to ratelimiter.NewKeyed.
type KeyRateConfig struct {
	// RatePerSec is the token refill rate per second for each identity key.
	RatePerSec float64

	// Burst is the maximum burst capacity for each identity key.
	Burst int
}

// ---- Configuration ----------------------------------------------------------

// defaultPoWMaxSeen is the replay seen-set cap applied when PoWConfig.MaxSeen
// is zero.
const defaultPoWMaxSeen = 65536

// defaultTopicJudgeCostUSD is the estimated USD cost charged per topic-gate
// judge call when Config.TopicJudgeCostUSD is zero or negative. It is an
// honest small estimate (a tiny yes/no completion), not a metered price.
const defaultTopicJudgeCostUSD = 0.001

// defaultMaxBodyBytes caps the HTTPHandler request body (64 KiB) when
// Config.MaxBodyBytes is zero or negative.
const defaultMaxBodyBytes = 64 * 1024

// PoWConfig controls proof-of-work challenge behaviour.
type PoWConfig struct {
	// Enabled turns on the proof-of-work gate.
	Enabled bool

	// Secret is the HMAC key used to sign issued challenges so that Verify
	// can prove a submitted challenge was server-issued (an unsigned or
	// tampered challenge — e.g. Bits=0 with a far-future expiry — is
	// rejected). Required when Enabled; must be at least
	// proofofwork.MinSecretLen bytes. Generate one with
	// proofofwork.NewSecret(). In JSON configs this is base64-encoded
	// (standard encoding/json []byte handling); generating a fresh secret on
	// each process start is fine — it only invalidates challenges issued
	// before the restart, and clients simply request new ones.
	Secret []byte

	// Bits is the required number of leading zero bits in SHA-256(Nonce+":"+solution).
	// 1-4 is suitable for testing; 16+ for production anti-bot protection.
	Bits int

	// MinBits is the verification difficulty floor: challenges with Bits
	// below it are rejected even when validly signed. Defaults to Bits when
	// zero.
	MinBits int

	// TTL is how long an issued challenge remains valid.
	TTL time.Duration

	// MaxSeen caps the replay seen-set. Entries are evicted once their
	// challenge expires; if the cap is reached with no expired entries to
	// evict, verifications FAIL CLOSED (rejected). Defaults to 65536 when
	// zero. Size it to at least the expected number of successful
	// verifications per TTL window.
	MaxSeen int
}

// Config is the complete configuration for a Guard.
type Config struct {
	// Identity controls how the caller's IP is extracted and hashed.
	Identity identity.Config

	// KeyRate is the per-identity token-bucket configuration.
	KeyRate KeyRateConfig

	// GlobalRatePerSec is the global rate limit in tokens per second.
	GlobalRatePerSec float64

	// GlobalBurst is the global burst capacity.
	GlobalBurst int

	// Breaker controls the circuit breaker thresholds.
	Breaker circuitbreaker.Config

	// Budget controls per-minute and per-day quota limits.
	Budget budgetguard.Config

	// Strikes controls the escalating-backoff blocklist.
	Strikes strikeledger.Config

	// Topic controls the topic gate (exemplars, similarity thresholds).
	Topic topicgate.Config

	// Cache controls the answer cache TTL and similarity matching.
	Cache answercache.Config

	// PoW controls the optional proof-of-work challenge gate.
	PoW PoWConfig

	// SystemPrompt is prepended to every Answerer call.
	SystemPrompt string

	// EstCostPerCallUSD is the estimated USD cost charged to budget per call.
	EstCostPerCallUSD float64

	// TopicJudgeCostUSD is the estimated USD cost charged to the budget guard
	// for EACH topic-gate judge LLM call, immediately BEFORE the judge is
	// invoked (fail-closed: when the budget refuses the charge, the judge is
	// never called and the request gets a templated refusal while the breaker
	// force-opens, exactly like the paid-call budget-trip path). It is an
	// ESTIMATE fed to budget-guard — not a metered price. Zero or negative
	// means the default 0.001.
	//
	// NOTE: every charge also counts one request toward Budget.MaxReqPerMin
	// and Budget.MaxReqPerDay, so a judged-then-answered request consumes TWO
	// request charges (judge + answer). Conservative by design — consistent
	// with the kit's aggressive posture. Embedding similarity calls remain
	// uncharged (they are an order of magnitude cheaper than a judge call).
	TopicJudgeCostUSD float64

	// MaxBodyBytes caps the request body read by HTTPHandler
	// (http.MaxBytesReader). Oversized bodies get a templated 413 refusal
	// before any JSON decoding, gauntlet processing, or LLM call. Zero or
	// negative means the default 64 KiB.
	MaxBodyBytes int64
}

// Deps holds external dependencies injected into a Guard.
type Deps struct {
	// Embedder is used by both the topic gate and the answer cache.
	// Optional when a Judge is provided: the topic gate then runs judge-only
	// (an LLM judge call for every question that passes the prefilters — the
	// embedding layer exists to make most decisions free) and the answer
	// cache degrades to exact-match-only.
	Embedder embeddingclient.Embedder

	// Detector is used by the topic gate for injection pre-screening.
	Detector *injectiondetector.Detector

	// Judge is an optional LLM client for resolving ambiguous topic scores.
	// If nil, ambiguous queries are SoftDeny (fail-closed). Required when
	// Embedder is nil (judge-only topic gating).
	Judge llmclient.LLMClient

	// Answerer is the paid model client called when all gates pass.
	Answerer llmclient.LLMClient

	// Now is the injected clock function.
	Now func() time.Time

	// RandHex is a function returning caller-supplied hex entropy for PoW nonces.
	// Use crypto/rand hex in production; a fixed string in tests.
	RandHex func() string

	// Metrics is an optional registry. When set, Handle increments
	// MetricRequests{outcome=...} once per call with one of: answered,
	// cache_hit, refused_topic, refused_injection, blocked_strike,
	// rate_limited, breaker_open, challenge_required, pow_failed,
	// answerer_error — plus MetricBudgetTrips on budget exhaustion.
	// Nil disables instrumentation (zero overhead).
	Metrics *metrics.Registry
}

// ---- Action & Decision types -----------------------------------------------

// Action is the outcome of a Handle call.
type Action int

const (
	// Answered means the question was answered (possibly from cache).
	Answered Action = iota
	// Refused means the request was rejected for policy reasons (e.g. injection).
	Refused
	// OffTopic means the question is out of scope; no penalty.
	OffTopic
	// RateLimited means the caller exceeded rate limits.
	RateLimited
	// Blocked means the caller is in the strike blocklist.
	Blocked
	// Unavailable means the circuit is open or the budget is exhausted.
	Unavailable
	// ChallengeRequired means a PoW challenge must be solved before proceeding.
	ChallengeRequired
)

// String implements fmt.Stringer.
func (a Action) String() string {
	switch a {
	case Answered:
		return "Answered"
	case Refused:
		return "Refused"
	case OffTopic:
		return "OffTopic"
	case RateLimited:
		return "RateLimited"
	case Blocked:
		return "Blocked"
	case Unavailable:
		return "Unavailable"
	case ChallengeRequired:
		return "ChallengeRequired"
	default:
		return "Unknown"
	}
}

// Request is the input to Handle.
type Request struct {
	// XFF is the X-Forwarded-For header value (comma-separated IPs).
	XFF string

	// RemoteAddr is the TCP remote address (host:port).
	RemoteAddr string

	// Question is the user's query.
	Question string

	// Challenge is the PoW challenge issued in a previous response, returned
	// by the client verbatim. It is HMAC-signed at issue time, so tampering
	// with any field (or minting a challenge client-side) fails verification.
	// Nil if no challenge has been issued yet.
	Challenge *proofofwork.Challenge

	// PoWSolution is the client's answer to the PoW challenge.
	PoWSolution string
}

// Decision is the output of Handle.
type Decision struct {
	// Action is the outcome.
	Action Action

	// Answer is the model's answer (non-empty only when Action == Answered).
	Answer string

	// Cached is true when the answer came from the cache.
	Cached bool

	// Reason is a human-readable explanation (safe to log; not for end-users).
	Reason string

	// Challenge is the issued PoW challenge (non-nil only when Action == ChallengeRequired).
	Challenge *proofofwork.Challenge

	// HTTPStatus is the suggested HTTP response status code.
	HTTPStatus int
}

// ---- Snapshot / Restore -----------------------------------------------------

// Snapshot is a serialisable copy of the Guard's mutable state.
type Snapshot struct {
	// Budget is the budget-guard runtime state.
	Budget budgetguard.Snapshot

	// Strikes is the strike-ledger state.
	Strikes strikeledger.Snapshot

	// Cache is the answer-cache state.
	Cache answercache.Snapshot
}

// ---- Judge budget charging ----------------------------------------------------

// BudgetRefusedError is returned by the judge-charging middleware when the
// budget guard refuses the topic-judge charge. The judge LLM is never invoked.
// The topic gate maps it — like any judge error — to a fail-closed SoftDeny,
// so the caller gets a templated refusal that cost zero LLM dollars.
type BudgetRefusedError struct {
	// Reason is the budget-guard dimension that refused the charge.
	Reason string
}

// Error implements the error interface, reporting the budget-guard
// dimension that refused the charge.
func (e *BudgetRefusedError) Error() string {
	return "endpointguard: topic-judge charge refused by budget guard: " + e.Reason
}

// chargingJudge wraps the topic-gate judge so every judge LLM call is charged
// to the budget guard BEFORE it fires. On a refused charge it force-opens the
// circuit breaker (and records MetricBudgetTrips), mirroring the paid-call
// budget-trip path, and returns *BudgetRefusedError without calling the inner
// judge — a refused request never silently becomes a free judge call.
//
// Each successful charge counts one request toward the budget's req/min and
// req/day dimensions in addition to the USD estimate (see
// Config.TopicJudgeCostUSD). Safe for concurrent use: all state lives in the
// concurrency-safe budget guard and breaker.
type chargingJudge struct {
	inner   llmclient.LLMClient
	budget  *budgetguard.Guard
	breaker *circuitbreaker.Breaker
	costUSD float64
	metrics *metrics.Registry
}

// charge attempts the budget charge; on refusal it trips the breaker and
// returns the typed error.
func (c *chargingJudge) charge() error {
	d := c.budget.Charge(c.costUSD)
	if !d.Tripped {
		return nil
	}
	c.breaker.ForceOpen()
	if c.metrics != nil {
		c.metrics.Counter(MetricBudgetTrips).Inc()
	}
	return &BudgetRefusedError{Reason: d.Reason}
}

func (c *chargingJudge) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	if err := c.charge(); err != nil {
		return llmclient.Response{}, err
	}
	return c.inner.Complete(ctx, req)
}

func (c *chargingJudge) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	if err := c.charge(); err != nil {
		return nil, err
	}
	return c.inner.Stream(ctx, req)
}

// ---- Guard ------------------------------------------------------------------

// Guard is the fully wired endpoint protection gate.
// Construct via New; the zero value is not usable.
// All methods are concurrency-safe.
type Guard struct {
	cfg        Config
	deps       Deps
	breaker    *circuitbreaker.Breaker
	keyedRate  *ratelimiter.KeyedLimiter
	globalRate *ratelimiter.Limiter
	budget     *budgetguard.Guard
	strikes    *strikeledger.Ledger
	gate       *topicgate.Gate
	cache      *answercache.Cache
	pow        *proofofwork.Issuer // nil unless cfg.PoW.Enabled
}

// New constructs a Guard, wiring all primitives from cfg and deps.
// Returns an error if any primitive fails to initialise (e.g. bad exemplars).
func New(cfg Config, deps Deps) (*Guard, error) {
	if deps.Embedder == nil && deps.Judge == nil {
		return nil, newConfigError("Embedder is required (or provide a Judge to run the topic gate judge-only)")
	}
	if deps.Detector == nil {
		return nil, newConfigError("Detector is required")
	}
	if deps.Answerer == nil {
		return nil, newConfigError("Answerer is required")
	}
	if deps.Now == nil {
		return nil, newConfigError("Now is required")
	}
	if deps.RandHex == nil {
		return nil, newConfigError("RandHex is required")
	}

	// Circuit breaker.
	breaker := circuitbreaker.New(cfg.Breaker, deps.Now)

	// Per-key rate limiter (no global cap here; global Limiter handles it separately).
	keyedRate := ratelimiter.NewKeyed(cfg.KeyRate.RatePerSec, cfg.KeyRate.Burst, deps.Now, 0)

	// Global rate limiter.
	globalRate := ratelimiter.New(cfg.GlobalRatePerSec, cfg.GlobalBurst, deps.Now)

	// Budget guard.
	budget := budgetguard.New(cfg.Budget, deps.Now)

	// Strike ledger.
	strikes := strikeledger.New(cfg.Strikes, deps.Now)

	// Topic gate. With no embedder the gate runs judge-only: prefilters →
	// judge, an LLM call per surviving question (see topicgate.Config.JudgeOnly
	// for the cost trade-off — the embedding layer makes most decisions free).
	topicCfg := cfg.Topic
	if deps.Embedder == nil {
		topicCfg.JudgeOnly = true
	}
	// Wrap the judge in the budget-charging middleware so a judge call can
	// never be free for an attacker: the charge happens fail-closed before
	// the judge fires, whether the gate is judge-only (every non-prefiltered
	// question) or embedding-backed (only the ambiguous band).
	judgeCost := cfg.TopicJudgeCostUSD
	if judgeCost <= 0 {
		judgeCost = defaultTopicJudgeCostUSD
	}
	var judge llmclient.LLMClient
	if deps.Judge != nil {
		judge = &chargingJudge{
			inner:   deps.Judge,
			budget:  budget,
			breaker: breaker,
			costUSD: judgeCost,
			metrics: deps.Metrics,
		}
	}
	gate, err := topicgate.New(topicCfg, topicgate.Deps{
		Embedder: deps.Embedder,
		Detector: deps.Detector,
		Judge:    judge,
	})
	if err != nil {
		return nil, err
	}

	// Answer cache. A nil embedder is supported by answercache directly:
	// near-duplicate matching is skipped, exact (normalized) matching remains.
	cache := answercache.New(cfg.Cache, deps.Now, deps.Embedder)

	// PoW issuer — holds the HMAC secret and a bounded replay seen-set.
	var pow *proofofwork.Issuer
	if cfg.PoW.Enabled {
		minBits := cfg.PoW.MinBits
		if minBits == 0 {
			minBits = cfg.PoW.Bits
		}
		maxSeen := cfg.PoW.MaxSeen
		if maxSeen == 0 {
			maxSeen = defaultPoWMaxSeen
		}
		issuer, err := proofofwork.NewIssuer(cfg.PoW.Secret, proofofwork.IssuerConfig{
			Bits:    cfg.PoW.Bits,
			MinBits: minBits,
			TTL:     cfg.PoW.TTL,
			MaxSeen: maxSeen,
		}, deps.Now)
		if err != nil {
			return nil, newConfigError("PoW: " + err.Error())
		}
		pow = issuer
	}

	return &Guard{
		cfg:        cfg,
		deps:       deps,
		breaker:    breaker,
		keyedRate:  keyedRate,
		globalRate: globalRate,
		budget:     budget,
		strikes:    strikes,
		gate:       gate,
		cache:      cache,
		pow:        pow,
	}, nil
}

// record increments the per-outcome request counter when a Metrics registry
// is configured. A nil registry makes this a no-op.
func (g *Guard) record(outcome string) {
	if g.deps.Metrics == nil {
		return
	}
	g.deps.Metrics.Counter(MetricRequests, "outcome", outcome).Inc()
}

// Handle runs the full gauntlet and returns a Decision.
// It is concurrency-safe.
func (g *Guard) Handle(ctx context.Context, req Request) Decision {
	// Step 1: circuit breaker check — fail-closed if open.
	if !g.breaker.Allow() {
		g.record(outcomeBreakerOpen)
		return Decision{
			Action:     Unavailable,
			Reason:     "circuit breaker open",
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	// Step 2: extract identity.
	id := identity.Extract(g.cfg.Identity, req.XFF, req.RemoteAddr)

	// Step 3: blocklist check.
	if blocked, _ := g.strikes.Blocked(id); blocked {
		g.record(outcomeBlockedStrike)
		return Decision{
			Action:     Blocked,
			Reason:     "identity is blocked",
			HTTPStatus: http.StatusForbidden,
		}
	}

	// Step 4: rate limiting — per-key AND global.
	if !g.keyedRate.Allow(id) || !g.globalRate.Allow() {
		g.record(outcomeRateLimited)
		return Decision{
			Action:     RateLimited,
			Reason:     "rate limit exceeded",
			HTTPStatus: http.StatusTooManyRequests,
		}
	}

	// Step 5: proof-of-work (optional). Runs BEFORE the topic gate:
	// verifying a solution is one HMAC + one SHA-256, cheaper than any
	// embedding or judge call, so an un-PoW'd request can never trigger
	// paid topic evaluation (cheapest-first).
	if g.cfg.PoW.Enabled {
		if req.Challenge == nil || req.PoWSolution == "" {
			// Issue a new HMAC-signed challenge.
			ch := g.pow.Issue(g.deps.RandHex())
			g.record(outcomeChallengeRequired)
			return Decision{
				Action:     ChallengeRequired,
				Reason:     "proof-of-work required",
				Challenge:  &ch,
				HTTPStatus: http.StatusPaymentRequired,
			}
		}
		// Verify the supplied solution. Verify authenticates the challenge's
		// MAC, enforces the difficulty floor and expiry, and rejects replays
		// — a client-minted challenge cannot pass.
		ok, reason := g.pow.Verify(*req.Challenge, req.PoWSolution)
		if !ok {
			g.strikes.Strike(id, "bad-pow")
			g.record(outcomePoWFailed)
			return Decision{
				Action:     Refused,
				Reason:     "invalid proof-of-work: " + reason,
				HTTPStatus: http.StatusBadRequest,
			}
		}
	}

	// Step 6: topic gate. Any judge LLM call inside Evaluate is charged to
	// the budget guard first (see chargingJudge); a refused charge surfaces
	// here as a fail-closed SoftDeny while the breaker force-opens for
	// subsequent callers — never a free judge call.
	td := g.gate.Evaluate(ctx, req.Question)
	switch td.Verdict {
	case topicgate.HardDeny:
		// Injection detected → strike.
		status := g.strikes.Strike(id, "injection-violation")
		if status.Blocked {
			g.record(outcomeBlockedStrike)
			return Decision{
				Action:     Blocked,
				Reason:     "blocked after injection violation: " + td.Reason,
				HTTPStatus: http.StatusForbidden,
			}
		}
		g.record(outcomeRefusedInjection)
		return Decision{
			Action:     Refused,
			Reason:     "injection detected: " + td.Reason,
			HTTPStatus: http.StatusBadRequest,
		}
	case topicgate.SoftDeny:
		// Off-topic → no strike.
		g.record(outcomeRefusedTopic)
		return Decision{
			Action:     OffTopic,
			Reason:     "off-topic: " + td.Reason,
			HTTPStatus: http.StatusUnprocessableEntity,
		}
	}
	// topicgate.Allow — continue.

	// Step 7: cache lookup — answered for free.
	if ans, hit := g.cache.Get(ctx, req.Question); hit {
		g.breaker.Record(true)
		g.budget.Charge(0) // free — count request but no USD
		g.record(outcomeCacheHit)
		return Decision{
			Action:     Answered,
			Answer:     ans,
			Cached:     true,
			Reason:     "cache hit",
			HTTPStatus: http.StatusOK,
		}
	}

	// Step 8: paid LLM call.
	resp, err := g.deps.Answerer.Complete(ctx, llmclient.Request{
		System: g.cfg.SystemPrompt,
		Prompt: req.Question,
	})
	if err != nil {
		g.breaker.Record(false)
		g.record(outcomeAnswererError)
		return Decision{
			Action:     Unavailable,
			Reason:     "answerer error: " + err.Error(),
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	// Store in cache.
	g.cache.Put(ctx, req.Question, resp.Text)

	// Charge budget; trip breaker if exhausted.
	d := g.budget.Charge(g.cfg.EstCostPerCallUSD)
	if d.Tripped {
		g.breaker.ForceOpen()
		if g.deps.Metrics != nil {
			g.deps.Metrics.Counter(MetricBudgetTrips).Inc()
		}
	}
	g.breaker.Record(true)

	g.record(outcomeAnswered)
	return Decision{
		Action:     Answered,
		Answer:     resp.Text,
		Cached:     false,
		Reason:     "answered",
		HTTPStatus: http.StatusOK,
	}
}

// GCRateLimiter evicts per-key token buckets that have been idle for longer
// than one minute (the KeyedLimiter's hard-coded idle threshold). Call it
// periodically — once a minute is sufficient — to bound the size of the
// per-key map. Without periodic calls the map grows without bound for the
// lifetime of the process.
//
// GCRateLimiter is concurrency-safe; it acquires the KeyedLimiter's internal
// lock and returns promptly.
func (g *Guard) GCRateLimiter() {
	g.keyedRate.GC(g.deps.Now())
}

// Snapshot returns a serialisable copy of the Guard's mutable runtime state.
func (g *Guard) Snapshot() Snapshot {
	return Snapshot{
		Budget:  g.budget.Snapshot(),
		Strikes: g.strikes.Snapshot(),
		Cache:   g.cache.Snapshot(),
	}
}

// Restore replaces the Guard's mutable state from a Snapshot.
// It does not affect the circuit breaker or rate limiters.
func (g *Guard) Restore(snap Snapshot) {
	g.budget.Restore(snap.Budget)
	g.strikes.Restore(snap.Strikes)
	g.cache.Restore(snap.Cache)
}

// ---- HTTP handler -----------------------------------------------------------

// httpRequest is the JSON body for HTTPHandler POST requests.
type httpRequest struct {
	Question string `json:"question"`
}

// httpResponse is the JSON body returned by HTTPHandler.
type httpResponse struct {
	Action    string                 `json:"action"`
	Answer    string                 `json:"answer,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Cached    bool                   `json:"cached,omitempty"`
	Challenge *proofofwork.Challenge `json:"challenge,omitempty"`
}

// HTTPHandler returns an http.Handler that wraps Handle.
//
// Request format:
//
//	POST /  Content-Type: application/json
//	{"question": "..."}
//
// The request body is capped via http.MaxBytesReader at Config.MaxBodyBytes
// (default 64 KiB): oversized bodies get a templated JSON 413 refusal before
// any JSON decoding, gauntlet processing, or LLM call.
//
// Headers read:
//
//	X-Forwarded-For   — passed as XFF to identity.Extract
//	X-PoW-Challenge   — JSON-encoded proofofwork.Challenge, exactly as issued
//	                    (it is HMAC-signed; any tampering fails verification)
//	X-PoW-Solution    — raw string solution
//
// Response: JSON body with the Decision fields; HTTP status from Decision.HTTPStatus.
func (g *Guard) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Cap the body BEFORE any decoding: oversized bodies must never
		// reach the gauntlet (and therefore never an LLM call).
		limit := g.cfg.MaxBodyBytes
		if limit <= 0 {
			limit = defaultMaxBodyBytes
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)

		// Decode body.
		var body httpRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(httpResponse{
					Action: Refused.String(),
					Reason: "request body exceeds the " + strconv.FormatInt(limit, 10) + "-byte limit",
				})
				return
			}
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Parse optional PoW headers.
		var ch *proofofwork.Challenge
		if raw := r.Header.Get("X-PoW-Challenge"); raw != "" {
			ch = new(proofofwork.Challenge)
			if err := json.Unmarshal([]byte(raw), ch); err != nil {
				ch = nil // ignore malformed
			}
		}
		solution := r.Header.Get("X-PoW-Solution")

		req := Request{
			XFF:         r.Header.Get("X-Forwarded-For"),
			RemoteAddr:  r.RemoteAddr,
			Question:    body.Question,
			Challenge:   ch,
			PoWSolution: solution,
		}

		dec := g.Handle(r.Context(), req)

		resp := httpResponse{
			Action:    dec.Action.String(),
			Answer:    dec.Answer,
			Reason:    dec.Reason,
			Cached:    dec.Cached,
			Challenge: dec.Challenge,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(dec.HTTPStatus)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// ---- config validation type -------------------------------------------------

type configError struct{ msg string }

func newConfigError(msg string) *configError { return &configError{msg: msg} }
func (e *configError) Error() string         { return "endpointguard: " + e.msg }
