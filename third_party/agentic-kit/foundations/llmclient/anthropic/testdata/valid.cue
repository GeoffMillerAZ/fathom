// Package anthropic — valid test data that must unify with schema.cue.
// Keys in fixtures are placeholders only; NEVER a real key.
package anthropic

import "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/anthropic"

// validMinimal relies entirely on defaults; the key comes from the
// ANTHROPIC_API_KEY environment variable at construction time.
validMinimal: anthropic.#Config & {}

// validEnvKey resolves the key from a custom environment variable.
validEnvKey: anthropic.#Config & {
	APIKeyEnv:    "MY_TEAM_ANTHROPIC_KEY"
	DefaultModel: "claude-sonnet-4-6"
}

// validExplicit pins every data-shaped field.
validExplicit: anthropic.#Config & {
	APIKey:           "test-placeholder-key-not-real"
	APIKeyEnv:        "ANTHROPIC_API_KEY"
	AuthScheme:       "x-api-key"
	BetaHeaders:      ["context-1m-2025-08-07"]
	BaseURL:          "https://anthropic-gateway.internal.example"
	DefaultModel:     "claude-opus-4-8"
	DefaultMaxTokens: 8192
	Version:          "2023-06-01"
}

// validBearer authenticates with an OAuth access token (resolved by the
// caller — e.g. from Claude Code's local credential store — and passed in
// via APIKey) and opts into the OAuth beta the API requires for such tokens.
validBearer: anthropic.#Config & {
	APIKey:      "test-placeholder-oauth-token-not-real"
	AuthScheme:  "bearer"
	BetaHeaders: ["oauth-2025-04-20"]
}
