// Package anthropic defines the CUE schema for the Anthropic provider client
// configuration. These constraints mirror config.go (Config and the defaults
// applied by New) and are evaluated with `cue vet ./...`. Only data-shaped
// fields appear here — Config.HTTPClient is a Go-only injection point with no
// data representation.
package anthropic

// #OutputConfig mirrors the adapter's outbound wire object. Empty typed
// fields omit the corresponding wire property. Model support is checked by
// the caller's CapabilityCheck, not a hardcoded model catalog.
#OutputConfig: {
	effort?: "low" | "medium" | "high" | "xhigh" | "max"
	format?: {
		type: "json_schema"
		schema: {...}
	}
}

// #Config is the data shape of anthropic.Config. Defaults match New.
#Config: {
	// APIKey is the literal API key. Optional: when omitted or empty, the key
	// is resolved at construction from the environment variable named by
	// APIKeyEnv. Fixtures must NEVER contain a real key.
	APIKey?: string

	// APIKeyEnv names the environment variable consulted when APIKey is empty.
	APIKeyEnv: *"ANTHROPIC_API_KEY" | (string & !="")

	// AuthScheme selects the header that carries the credential:
	// "x-api-key" (the default, for API keys) or "bearer" (sends
	// "Authorization: Bearer <credential>", for OAuth access tokens).
	AuthScheme: *"x-api-key" | "bearer"

	// BetaHeaders are anthropic-beta header values, one header line per
	// entry (e.g. "oauth-2025-04-20", required by the API for Claude Code
	// OAuth tokens). Optional; entries must be non-empty.
	BetaHeaders?: [...string & !=""]

	// BaseURL is the API origin (scheme + host, no trailing path).
	BaseURL: *"https://api.anthropic.com" | (string & =~"^https?://")

	// DefaultModel is used when Request.Model is empty. Optional: a client may
	// rely on per-request models, but a call with neither fails before HTTP.
	DefaultModel?: string & !=""

	// DefaultMaxTokens is used when Request.MaxTokens is zero. The Messages
	// API requires max_tokens (no server default), so it must be positive.
	DefaultMaxTokens: *4096 | (int & >0)

	// Version is the anthropic-version header value.
	Version: *"2023-06-01" | (string & !="")
}
