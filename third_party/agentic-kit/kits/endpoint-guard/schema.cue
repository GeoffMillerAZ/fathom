// Package endpointguard defines the CUE contract for the endpoint-guard kit.
// These constraints mirror the Go types in endpointguard.go and are evaluated
// with `cue vet ./...`.
package endpointguard

// #KeyRateConfig defines the per-identity token-bucket parameters.
#KeyRateConfig: {
	// RatePerSec is the token refill rate per second for each identity key.
	// Must be > 0.
	RatePerSec: >0 & number

	// Burst is the maximum burst capacity. Must be >= 1.
	Burst: >=1 & int
}

// #PoWConfig controls proof-of-work challenge behaviour. Challenges are
// HMAC-signed server-side so clients cannot mint or tamper with them.
#PoWConfig: {
	// Enabled turns on the proof-of-work gate.
	Enabled: bool

	// Secret is the base64-encoded HMAC key ([]byte in Go; encoding/json
	// base64 in JSON configs) used to sign issued challenges. Optional here
	// only because PoW may be disabled: when Enabled is true the Go kit
	// requires it (and the cmd's production startup refuses to run without
	// one; --demo disables PoW entirely). SECURITY: CUE fixtures and
	// committed examples must only ever record the field's PRESENCE — never
	// a real secret value.
	Secret?: string & !=""

	// Bits is the required number of leading zero bits in the PoW hash.
	// Must be >= 1 when Enabled is true. Values 1-8 suit testing; 16+ for
	// production anti-bot protection.
	Bits: >=1 & int

	// MinBits is the verification difficulty floor: challenges below it are
	// rejected even when validly signed. Optional; the Go kit defaults it to
	// Bits when omitted or zero.
	MinBits?: >=1 & int

	// TTLSeconds is the challenge validity period in seconds. Must be >= 1.
	TTLSeconds: >=1 & int

	// MaxSeen caps the replay seen-set; entries are evicted once their
	// challenge expires, and at the cap with nothing evictable new
	// verifications fail closed. Optional; the Go kit defaults it to 65536
	// when omitted or zero.
	MaxSeen?: >=1 & int
}

// #IdentityConfig mirrors identity.Config.
#IdentityConfig: {
	// Salt is mixed into every hash. Must be non-empty.
	Salt: string & !=""

	// TrustedProxyHops is the number of rightmost XFF entries to skip.
	// 0 means ignore XFF entirely.
	TrustedProxyHops: >=0 & int
}

// #BreakerConfig mirrors circuitbreaker.Config.
#BreakerConfig: {
	// FailureThreshold is the consecutive failures needed to open the circuit.
	FailureThreshold: >=1 & int

	// OpenCooldownSeconds is how long the breaker stays open (seconds).
	OpenCooldownSeconds: >=1 & int

	// HalfOpenProbes is consecutive successes needed to close from half-open.
	HalfOpenProbes: >=1 & int
}

// #BudgetConfig mirrors budgetguard.Config.
#BudgetConfig: {
	// MaxReqPerMin is the maximum requests per rolling minute. Must be >= 1.
	MaxReqPerMin: >=1 & int

	// MaxReqPerDay is the maximum requests per rolling day. Must be >= 1.
	MaxReqPerDay: >=1 & int

	// MaxUSDPerDay is the maximum USD spend per rolling day. Must be > 0.
	MaxUSDPerDay: >0 & number
}

// #StrikesConfig mirrors strikeledger.Config.
#StrikesConfig: {
	// Threshold is the number of strikes before a block is applied. Must be >= 1.
	Threshold: >=1 & int

	// BaseBlockSeconds is the duration of the first block in seconds.
	BaseBlockSeconds: >=1 & int

	// MaxBlockSeconds is the upper bound on any single block duration in seconds.
	MaxBlockSeconds: >=1 & int
}

// #TopicConfig mirrors topicgate.Config (subset of CUE-expressible fields).
#TopicConfig: {
	// Exemplars are on-topic example strings. At least one is required.
	// (Unused when the gate runs judge-only — see JudgeOnly — but the
	// contract still requires one so embedding-backed deployments never
	// ship without exemplars.)
	Exemplars: [string & !="", ...string & !=""]

	// MaxLen is the maximum rune length of an accepted input. Must be >= 1.
	MaxLen: >=1 & int

	// LowSim is the lower cosine similarity threshold (SoftDeny below this).
	LowSim: >=0 & <=1 & number

	// HighSim is the upper cosine similarity threshold (Allow at or above this).
	// Must be >= LowSim.
	HighSim: >=LowSim & <=1.1 & number

	// JudgeOnly skips the embedding similarity layer: prefilters → judge.
	// COST TRADE-OFF: every non-prefiltered question then costs an LLM judge
	// call — the embedding layer exists to make most decisions free. The
	// endpoint-guard kit sets this automatically when no Embedder is wired;
	// set it explicitly only for deliberate judge-only deployments.
	JudgeOnly?: bool
}

// #CacheConfig mirrors answercache.Config.
#CacheConfig: {
	// TTLSeconds is the time-to-live for each cached entry in seconds.
	TTLSeconds: >=1 & int

	// SimThreshold is the minimum cosine similarity for a near-duplicate match.
	SimThreshold: >=-1 & <=1 & number

	// MaxEntries is the maximum number of entries. 0 means no limit.
	MaxEntries: >=0 & int
}

// #Config is the full configuration for an endpoint-guard Guard.
#Config: {
	// Identity controls IP extraction and hashing.
	Identity: #IdentityConfig

	// KeyRate is the per-identity token-bucket configuration.
	KeyRate: #KeyRateConfig

	// GlobalRatePerSec is the global token refill rate. Must be > 0.
	GlobalRatePerSec: >0 & number

	// GlobalBurst is the global burst capacity. Must be >= 1.
	GlobalBurst: >=1 & int

	// Breaker controls the circuit breaker.
	Breaker: #BreakerConfig

	// Budget controls per-minute and per-day quotas.
	Budget: #BudgetConfig

	// Strikes controls the escalating-backoff blocklist.
	Strikes: #StrikesConfig

	// Topic controls the topic gate.
	Topic: #TopicConfig

	// Cache controls the answer cache.
	Cache: #CacheConfig

	// PoW controls the optional proof-of-work gate.
	PoW: #PoWConfig

	// SystemPrompt is prepended to every Answerer call.
	SystemPrompt: string

	// EstCostPerCallUSD is the estimated USD cost per paid LLM call.
	EstCostPerCallUSD: >0 & number

	// TopicJudgeCostUSD is the estimated USD cost charged to the budget
	// guard for each topic-gate judge LLM call, BEFORE the judge fires
	// (fail-closed: a refused charge means no judge call). An ESTIMATE fed
	// to budget-guard, not a metered price; each charge also counts one
	// request toward the req/min and req/day quotas, so a judged-then-
	// answered request consumes two request charges. Optional; the Go kit
	// defaults it to 0.001 when omitted or zero.
	TopicJudgeCostUSD?: >0 & number

	// MaxBodyBytes caps the HTTPHandler request body; oversized bodies get
	// a templated 413 refusal before any JSON decoding or LLM call.
	// Optional; the Go kit defaults it to 65536 (64 KiB) when omitted or
	// zero.
	MaxBodyBytes?: >=1024 & int
}
