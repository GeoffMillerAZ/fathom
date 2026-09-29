// Package anthropic implements llmclient.LLMClient against the Anthropic
// Messages API (POST /v1/messages, anthropic-version 2023-06-01). It is the
// platform's first real provider adapter: Complete maps a llmclient.Request
// to one blocking Messages call, and Stream maps it to the API's SSE stream,
// surfacing chunks per the llmclient v2 streaming contract.
//
// Composition: imports only the standard library and the parent
// foundations/llmclient package (for the interface, request/response types,
// and the shared typed errors — RateLimitError, ServerError,
// InvalidRequestError, AuthError). No SDK, no external dependencies.
//
// Authentication: the credential is resolved exactly once at construction
// (Config.APIKey, else the environment variable named by Config.APIKeyEnv)
// and sent per Config.AuthScheme — x-api-key (the default, for API keys) or
// "Authorization: Bearer <token>" (for OAuth access tokens such as Claude
// Code credentials; pair with Config.BetaHeaders when the API requires an
// opt-in beta like "oauth-2025-04-20" for the token type). Credential
// RESOLUTION is deliberately the caller's concern: reading an OAuth token
// from Claude Code's local store, a keychain, or a secrets manager is
// environment-specific and belongs in a caller-side adapter that passes the
// resolved token in via Config.APIKey. Whatever the scheme, the credential is
// never embedded in any error string, wrapped error, or response artifact;
// server-echoed occurrences are redacted.
//
// Ping (GET /v1/models) is the credential-validity probe: it proves the
// credential works and the endpoint is reachable while consuming zero
// inference tokens, mapping failures through the same typed errors
// (401/403 → AuthError, and so on). Use it after credential entry, at
// startup, or in health checks.
//
// Mapping notes (the live-API contract this client enforces):
//
//   - max_tokens is REQUIRED by the API and always sent: Request.MaxTokens,
//     else Config.DefaultMaxTokens (default 4096).
//   - The model is Request.Model, else Config.DefaultModel; if both are empty
//     the call fails with a typed InvalidRequestError before any HTTP.
//   - temperature is omitted entirely when Request.Temperature is zero (zero
//     means "model default" per the llmclient contract, and explicit sampling
//     values can be rejected by newer models).
//   - BlockUnknown content is serialized from ContentBlock.Raw VERBATIM
//     outbound and captured verbatim inbound, so provider-specific blocks
//     (e.g. thinking blocks) survive multi-turn round-trips. CacheHint is
//     ignored on BlockUnknown blocks — verbatim wins. The verbatim capture
//     holds for Complete ONLY: Stream emits each unmodeled block as a
//     BlockUnknown chunk carrying just the content_block_start JSON, and
//     deltas for unmodeled block types (e.g. thinking_delta) are dropped.
//     Streamed BlockUnknown blocks MUST NOT be round-tripped into history —
//     use Complete for conversations that may carry unmodeled block types.
//   - CacheHint on blocks/tools and Request.SystemCacheHint map to
//     cache_control {"type":"ephemeral"}. More than 4 breakpoints total is
//     rejected locally with a typed InvalidRequestError (the API would reject
//     it anyway).
//   - Effort maps to output_config.effort (low, medium, high, xhigh, max),
//     alongside OutputSchema's output_config.format. Empty effort is omitted.
//     CapabilityCheck can reject values unavailable on the selected model.
//     No beta header is added. Raw ProviderParams.output_config conflicts
//     with either typed field and is rejected before HTTP. Migration: remove
//     that raw override and use Effort + OutputSchema, or unset both typed
//     fields when explicitly owning the complete raw output_config.
//
// Effort wire contract verified against official Anthropic documentation on
// 2026-09-13: https://platform.claude.com/docs/en/build-with-claude/effort.
// Verification is httptest-based, not a live model-availability or billing claim.
//
// Timeouts: when Config.HTTPClient is nil, Complete uses an http.Client with
// a 10-minute overall timeout and Stream uses one with NO overall timeout —
// a stream's lifetime is governed by ctx, not http.Client.Timeout. When you
// inject Config.HTTPClient it is used verbatim for both paths; do not set
// http.Client.Timeout on an injected client if you call Stream (it would
// sever long streams mid-flight) — bound streams with the context instead.
//
// Concurrency contract: Client is safe for concurrent use — all fields are
// set at construction and never mutated afterwards, and the underlying
// http.Client is safe for concurrent use. Each channel returned by Stream is
// owned by exactly one producer goroutine and is closed exactly once by that
// producer; producers select on ctx.Done() and terminate without goroutine
// leaks, and the HTTP response body is always closed.
package anthropic
