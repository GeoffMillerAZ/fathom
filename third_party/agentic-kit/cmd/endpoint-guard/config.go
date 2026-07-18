package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	endpointguard "github.com/geoffmilleraz/agentic-kit/kits/endpoint-guard"

	answercache "github.com/geoffmilleraz/agentic-kit/foundations/governance/answer-cache"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	circuitbreaker "github.com/geoffmilleraz/agentic-kit/foundations/governance/circuit-breaker"
	"github.com/geoffmilleraz/agentic-kit/foundations/governance/identity"
	proofofwork "github.com/geoffmilleraz/agentic-kit/foundations/governance/proof-of-work"
	strikeledger "github.com/geoffmilleraz/agentic-kit/foundations/governance/strike-ledger"
	topicgate "github.com/geoffmilleraz/agentic-kit/foundations/governance/topic-gate"
)

// Defaults for the cmd-level (non-kit) config sections. The CUE contract in
// schema.cue mirrors these values.
const (
	// defaultAPIKeyEnv is the environment variable consulted for the
	// Anthropic API key when LLM.APIKeyEnv is unset.
	defaultAPIKeyEnv = "ANTHROPIC_API_KEY"

	// defaultLLMMaxTokens caps answerer responses when LLM.MaxTokens is
	// unset. Deliberately small: this binary fronts a budget-guarded public
	// endpoint, not a long-form writing assistant.
	defaultLLMMaxTokens = 1024

	// defaultMaxBodyBytes caps the /ask request body (64 KiB) when
	// MaxBodyBytes is unset. Oversized bodies get a templated 413 refusal
	// before any JSON decoding or LLM call.
	defaultMaxBodyBytes = 64 * 1024
)

// LLMConfig configures the production Anthropic Messages API client used for
// BOTH the paid answerer and the topic-gate judge. The API key itself never
// lives in the config file — only the NAME of the environment variable that
// holds it.
type LLMConfig struct {
	// Model is the Anthropic model id (e.g. "claude-haiku-4-5").
	// Required in non-demo mode.
	Model string

	// BaseURL overrides the API origin (default https://api.anthropic.com).
	// Point it at a proxy or test server; leave empty for production.
	BaseURL string

	// APIKeyEnv names the environment variable holding the API key.
	// Default "ANTHROPIC_API_KEY". The variable must be set and non-empty in
	// non-demo mode — the server refuses to start otherwise.
	APIKeyEnv string

	// MaxTokens caps each model response. Default 1024.
	MaxTokens int
}

// EmbeddingConfig configures the OpenAI-compatible embedding backend
// (POST {BaseURL}/embeddings — OpenAI, Ollama, vLLM, OpenRouter, ...) used by
// the topic-gate similarity layer and the answer-cache near-duplicate
// matching. When Enabled is false the topic gate runs JUDGE-ONLY (one LLM
// judge call for every question that passes the prefilters — the embedding
// layer exists to make most decisions free) and the answer cache degrades to
// exact-match-only.
type EmbeddingConfig struct {
	// Enabled turns the embedding backend on.
	Enabled bool

	// BaseURL is the root of the OpenAI-compatible API. Required when Enabled.
	BaseURL string

	// Model is the embedding model id (e.g. "nomic-embed-text"). Required
	// when Enabled.
	Model string

	// APIKeyEnv names the environment variable holding the bearer token.
	// Empty means no Authorization header (fine for a local Ollama).
	APIKeyEnv string
}

// providerConfig groups the cmd-level settings that live in the config file
// outside the kit's endpointguard.Config.
type providerConfig struct {
	LLM          LLMConfig
	Embedding    EmbeddingConfig
	MaxBodyBytes int64
}

// ---- file config (the JSON shape) --------------------------------------------
//
// The canonical JSON config shape uses integer SECONDS fields (matching the
// CUE contract in schema.cue and kits/endpoint-guard/schema.cue). The
// pre-seconds shape — time.Duration fields serialised as int64 NANOSECONDS —
// is still accepted during the transition; when both forms are present the
// *Seconds field wins. Pointers distinguish "absent" from "zero".

// fileBreakerConfig is the JSON shape of the circuit-breaker section.
type fileBreakerConfig struct {
	FailureThreshold    int
	OpenCooldownSeconds *int64
	// OpenCooldown is the legacy nanosecond form of OpenCooldownSeconds.
	OpenCooldown   *time.Duration
	HalfOpenProbes int
}

// fileStrikesConfig is the JSON shape of the strike-ledger section.
type fileStrikesConfig struct {
	Threshold        int
	BaseBlockSeconds *int64
	// BaseBlock is the legacy nanosecond form of BaseBlockSeconds.
	BaseBlock       *time.Duration
	MaxBlockSeconds *int64
	// MaxBlock is the legacy nanosecond form of MaxBlockSeconds.
	MaxBlock *time.Duration
}

// fileCacheConfig is the JSON shape of the answer-cache section.
type fileCacheConfig struct {
	TTLSeconds *int64
	// TTL is the legacy nanosecond form of TTLSeconds.
	TTL          *time.Duration
	SimThreshold float64
	MaxEntries   int
}

// filePoWConfig is the JSON shape of the proof-of-work section.
type filePoWConfig struct {
	Enabled bool
	// Secret is the standard-base64-encoded HMAC key (the same wire form the
	// legacy []byte field produced via encoding/json). Empty means absent.
	Secret     string
	Bits       int
	MinBits    int
	TTLSeconds *int64
	// TTL is the legacy nanosecond form of TTLSeconds.
	TTL     *time.Duration
	MaxSeen int
}

// fileConfig mirrors the JSON config file (CUE: #FileConfig in schema.cue).
type fileConfig struct {
	Identity          identity.Config
	KeyRate           endpointguard.KeyRateConfig
	GlobalRatePerSec  float64
	GlobalBurst       int
	Breaker           fileBreakerConfig
	Budget            budgetguard.Config
	Strikes           fileStrikesConfig
	Topic             topicgate.Config
	Cache             fileCacheConfig
	PoW               filePoWConfig
	SystemPrompt      string
	EstCostPerCallUSD float64
	TopicJudgeCostUSD float64
	LLM               LLMConfig
	Embedding         EmbeddingConfig
	MaxBodyBytes      int64
}

// pickDuration resolves the seconds-canonical / legacy-nanosecond duration
// pair: an explicit *Seconds value wins, then an explicit legacy value, then
// the built-in default.
func pickDuration(seconds *int64, legacy *time.Duration, def time.Duration) time.Duration {
	if seconds != nil {
		return time.Duration(*seconds) * time.Second
	}
	if legacy != nil {
		return *legacy
	}
	return def
}

// loadConfig parses data (a JSON config file body; nil/empty means no file)
// over the aggressive defaults and returns the kit Config plus the cmd-level
// provider settings. It accepts both the canonical seconds shape and the
// legacy nanosecond shape (see the fileConfig doc).
func loadConfig(data []byte) (endpointguard.Config, providerConfig, error) {
	base := aggressiveDefaults()

	fc := fileConfig{
		Identity:          base.Identity,
		KeyRate:           base.KeyRate,
		GlobalRatePerSec:  base.GlobalRatePerSec,
		GlobalBurst:       base.GlobalBurst,
		Breaker:           fileBreakerConfig{FailureThreshold: base.Breaker.FailureThreshold, HalfOpenProbes: base.Breaker.HalfOpenProbes},
		Budget:            base.Budget,
		Strikes:           fileStrikesConfig{Threshold: base.Strikes.Threshold},
		Topic:             base.Topic,
		Cache:             fileCacheConfig{SimThreshold: base.Cache.SimThreshold, MaxEntries: base.Cache.MaxEntries},
		PoW:               filePoWConfig{Enabled: base.PoW.Enabled, Bits: base.PoW.Bits, MinBits: base.PoW.MinBits, MaxSeen: base.PoW.MaxSeen},
		SystemPrompt:      base.SystemPrompt,
		EstCostPerCallUSD: base.EstCostPerCallUSD,
		TopicJudgeCostUSD: base.TopicJudgeCostUSD,
		LLM:               LLMConfig{APIKeyEnv: defaultAPIKeyEnv, MaxTokens: defaultLLMMaxTokens},
		MaxBodyBytes:      defaultMaxBodyBytes,
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &fc); err != nil {
			return endpointguard.Config{}, providerConfig{}, fmt.Errorf("parsing config: %w", err)
		}
	}

	cfg := endpointguard.Config{
		Identity:         fc.Identity,
		KeyRate:          fc.KeyRate,
		GlobalRatePerSec: fc.GlobalRatePerSec,
		GlobalBurst:      fc.GlobalBurst,
		Breaker: circuitbreaker.Config{
			FailureThreshold: fc.Breaker.FailureThreshold,
			OpenCooldown:     pickDuration(fc.Breaker.OpenCooldownSeconds, fc.Breaker.OpenCooldown, base.Breaker.OpenCooldown),
			HalfOpenProbes:   fc.Breaker.HalfOpenProbes,
		},
		Budget: fc.Budget,
		Strikes: strikeledger.Config{
			Threshold: fc.Strikes.Threshold,
			BaseBlock: pickDuration(fc.Strikes.BaseBlockSeconds, fc.Strikes.BaseBlock, base.Strikes.BaseBlock),
			MaxBlock:  pickDuration(fc.Strikes.MaxBlockSeconds, fc.Strikes.MaxBlock, base.Strikes.MaxBlock),
		},
		Topic: fc.Topic,
		Cache: answercache.Config{
			TTL:          pickDuration(fc.Cache.TTLSeconds, fc.Cache.TTL, base.Cache.TTL),
			SimThreshold: fc.Cache.SimThreshold,
			MaxEntries:   fc.Cache.MaxEntries,
		},
		PoW: endpointguard.PoWConfig{
			Enabled: fc.PoW.Enabled,
			Bits:    fc.PoW.Bits,
			MinBits: fc.PoW.MinBits,
			TTL:     pickDuration(fc.PoW.TTLSeconds, fc.PoW.TTL, base.PoW.TTL),
			MaxSeen: fc.PoW.MaxSeen,
		},
		SystemPrompt:      fc.SystemPrompt,
		EstCostPerCallUSD: fc.EstCostPerCallUSD,
		TopicJudgeCostUSD: fc.TopicJudgeCostUSD,
	}

	if fc.PoW.Secret != "" {
		secret, err := base64.StdEncoding.DecodeString(fc.PoW.Secret)
		if err != nil {
			return endpointguard.Config{}, providerConfig{}, fmt.Errorf(
				"parsing config: PoW.Secret must be standard base64 (generate one with: openssl rand -base64 32): %w", err,
			)
		}
		cfg.PoW.Secret = secret
	}

	prov := providerConfig{LLM: fc.LLM, Embedding: fc.Embedding, MaxBodyBytes: fc.MaxBodyBytes}
	if prov.LLM.APIKeyEnv == "" {
		prov.LLM.APIKeyEnv = defaultAPIKeyEnv
	}
	if prov.LLM.MaxTokens <= 0 {
		prov.LLM.MaxTokens = defaultLLMMaxTokens
	}
	if prov.MaxBodyBytes <= 0 {
		prov.MaxBodyBytes = defaultMaxBodyBytes
	}
	// The kit's HTTPHandler self-caps with the same limit (defense in depth:
	// the cmd-level cap in mux() fires first, but the kit must agree).
	cfg.MaxBodyBytes = prov.MaxBodyBytes
	return cfg, prov, nil
}

// placeholderSalts are identity salts that must never reach production: the
// built-in default and the value shipped in config.example.json.
var placeholderSalts = map[string]bool{
	"":                                    true,
	"change-me-in-production":             true,
	"replace-with-a-secret-random-string": true,
}

// validateProductionConfig fails closed for non-demo runs. Every error names
// exactly what to set. Demo mode never calls this.
func validateProductionConfig(cfg endpointguard.Config, prov providerConfig) error {
	if placeholderSalts[cfg.Identity.Salt] {
		return errors.New(`Identity.Salt is missing or still a placeholder; set Identity.Salt in the config file to a long random secret (e.g. the output of: openssl rand -hex 32)`)
	}
	if cfg.PoW.Enabled && len(cfg.PoW.Secret) < proofofwork.MinSecretLen {
		return fmt.Errorf(
			"PoW is enabled but PoW.Secret is missing or shorter than %d bytes; set PoW.Secret in the config file to a base64-encoded secret of at least %d bytes (e.g. the output of: openssl rand -base64 32), or set PoW.Enabled to false",
			proofofwork.MinSecretLen, proofofwork.MinSecretLen,
		)
	}
	if prov.LLM.Model == "" {
		return errors.New(`LLM.Model is not set; set LLM.Model in the config file to the Anthropic model id to call (e.g. "claude-haiku-4-5")`)
	}
	if os.Getenv(prov.LLM.APIKeyEnv) == "" {
		return fmt.Errorf(
			"no API key: environment variable %s is unset or empty; export %s=<your Anthropic API key> (or point LLM.APIKeyEnv at the variable that holds it)",
			prov.LLM.APIKeyEnv, prov.LLM.APIKeyEnv,
		)
	}
	if prov.Embedding.Enabled {
		if prov.Embedding.BaseURL == "" {
			return errors.New(`Embedding.Enabled is true but Embedding.BaseURL is not set; set Embedding.BaseURL to the root of an OpenAI-compatible API (e.g. "http://localhost:11434/v1"), or set Embedding.Enabled to false to run the topic gate judge-only`)
		}
		if prov.Embedding.Model == "" {
			return errors.New(`Embedding.Enabled is true but Embedding.Model is not set; set Embedding.Model to the embedding model id (e.g. "nomic-embed-text"), or set Embedding.Enabled to false to run the topic gate judge-only`)
		}
	}
	return nil
}
