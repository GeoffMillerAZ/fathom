// Package endpointguardcmd defines the CUE contract for the endpoint-guard
// binary's JSON config file (config.example.json → config.json). It composes
// the kit's section definitions and adds the cmd-level provider sections
// (LLM, Embedding) and edge settings (MaxBodyBytes). Evaluated with
// `cue vet ./...`.
//
// The canonical duration shape is integer SECONDS (*Seconds fields), exactly
// as the kit schema defines. The Go loader additionally accepts the legacy
// nanosecond fields (Breaker.OpenCooldown, Strikes.BaseBlock/MaxBlock,
// Cache.TTL, PoW.TTL) during the transition; that legacy shape is NOT part of
// this contract and new configs must not use it.
package endpointguardcmd

import kit "github.com/geoffmilleraz/agentic-kit/kits/endpoint-guard:endpointguard"

// #LLMConfig configures the Anthropic Messages API client used for BOTH the
// paid answerer and the topic-gate judge. The API key itself never appears in
// config data — only the NAME of the environment variable holding it.
#LLMConfig: {
	// Model is the Anthropic model id. Required in production (non --demo)
	// runs — the server refuses to start without it.
	Model: string & !=""

	// BaseURL overrides the API origin (proxies, test servers). Omitted
	// means https://api.anthropic.com.
	BaseURL?: string & !=""

	// APIKeyEnv names the environment variable holding the API key.
	// Defaults to ANTHROPIC_API_KEY. The variable must resolve non-empty in
	// production runs — the server refuses to start otherwise.
	APIKeyEnv?: string & !=""

	// MaxTokens caps each model response. Defaults to 1024.
	MaxTokens?: >=1 & int
}

// #EmbeddingConfig configures the OpenAI-compatible embeddings backend used
// by the topic-gate similarity layer and answer-cache near-duplicate
// matching. COST TRADE-OFF when Enabled is false: the topic gate runs
// judge-only (one LLM judge call for EVERY question passing the prefilters —
// the embedding layer exists to make most decisions free) and the answer
// cache serves exact repeats only. BaseURL and Model are required when
// Enabled is true; the Go loader fails closed with a named error when they
// are missing.
#EmbeddingConfig: {
	// Enabled turns the embedding backend on.
	Enabled: bool

	// BaseURL is the root of the OpenAI-compatible API
	// (POST {BaseURL}/embeddings). Required when Enabled.
	BaseURL?: string & !=""

	// Model is the embedding model id. Required when Enabled.
	Model?: string & !=""

	// APIKeyEnv names the environment variable holding the bearer token.
	// Omitted means no Authorization header (fine for a local Ollama).
	APIKeyEnv?: string & !=""
}

// #FileConfig is the full JSON config file shape: the kit's sections plus the
// cmd-level provider sections. config.example.json mirrors this (with
// _comment keys, which the Go loader ignores).
#FileConfig: {
	// Identity controls IP extraction and hashing. Production runs refuse to
	// start while Salt is a known placeholder.
	Identity: kit.#IdentityConfig

	// KeyRate is the per-identity token-bucket configuration.
	KeyRate: kit.#KeyRateConfig

	// GlobalRatePerSec is the global token refill rate. Must be > 0.
	GlobalRatePerSec: >0 & number

	// GlobalBurst is the global burst capacity. Must be >= 1.
	GlobalBurst: >=1 & int

	// Breaker controls the circuit breaker (OpenCooldownSeconds is seconds).
	Breaker: kit.#BreakerConfig

	// Budget controls per-minute and per-day quotas.
	Budget: kit.#BudgetConfig

	// Strikes controls the escalating-backoff blocklist (seconds fields).
	Strikes: kit.#StrikesConfig

	// Topic controls the topic gate.
	Topic: kit.#TopicConfig

	// Cache controls the answer cache (TTLSeconds is seconds).
	Cache: kit.#CacheConfig

	// PoW controls the optional proof-of-work gate (TTLSeconds is seconds).
	// In production runs an enabled gate REQUIRES a configured Secret.
	PoW: kit.#PoWConfig

	// SystemPrompt is prepended to every Answerer call.
	SystemPrompt: string

	// EstCostPerCallUSD is the estimated USD cost per paid LLM call.
	EstCostPerCallUSD: >0 & number

	// TopicJudgeCostUSD is the estimated USD cost charged to the budget
	// guard for each topic-gate judge LLM call, BEFORE the judge fires (a
	// refused charge means no judge call — a templated refusal instead). An
	// estimate, not a metered price; each charge also counts one request
	// toward the req/min and req/day quotas. Optional; defaults to 0.001.
	TopicJudgeCostUSD?: >0 & number

	// LLM configures the Anthropic client. Required in production runs;
	// optional here because --demo runs need no provider sections.
	LLM?: #LLMConfig

	// Embedding configures the embeddings backend. Omitted (or Enabled
	// false) runs the topic gate judge-only and the cache exact-match-only.
	Embedding?: #EmbeddingConfig

	// MaxBodyBytes caps the /ask request body; oversized bodies get a
	// templated 413 refusal before any JSON decoding or LLM call.
	// Defaults to 65536 (64 KiB).
	MaxBodyBytes?: >=1024 & int
}
