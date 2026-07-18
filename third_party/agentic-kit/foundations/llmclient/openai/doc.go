// Package openai implements llmclient.LLMClient against the OpenAI Chat
// Completions API (POST /v1/chat/completions). It is a provider adapter that
// maps the platform's common llmclient.Request/Response types to OpenAI's wire
// format, with full SSE streaming support.
//
// Composition: imports only the standard library and the parent
// foundations/llmclient package (for the interface, request/response types,
// and the shared typed errors). No SDK, no external dependencies.
//
// Local compatibility: the default BaseURL is https://api.openai.com/v1. Set
// BaseURL to http://localhost:11434/v1 (Ollama) or another OpenAI-compatible
// endpoint to use local models. APIKey is optional for local endpoints.
//
// Authentication: the API key is sent in the "Authorization: Bearer <key>"
// header. The credential is resolved once at construction and never appears in
// any error string, wrapped error, or response artifact.
//
// Ping (GET /models) is the credential-validity probe: it proves the
// credential works and the endpoint is reachable while consuming zero
// inference tokens.
//
// Mapping notes (the live-API contract this client enforces):
//
//   - Request.System → a {"role":"system","content":"..."} message prepended
//     to the messages array before any user/assistant messages.
//   - ContentBlock text → {"type":"text","text":"..."} content part.
//   - ContentBlock tool_use → assistant message with tool_calls array.
//   - ContentBlock tool_result → {"role":"tool","tool_call_id":...,"content":...}
//     message.
//   - ContentBlock image → {"type":"image_url","image_url":{"url":"data:{MediaType};base64,{Data}"}}
//     content part (inline base64, no upload required).
//   - max_completion_tokens is the modern field (post-Nov-2024); a comment in
//     the source notes the older max_tokens for compatibility with pre-Nov-2024
//     deployments.
//   - Temperature is sent only when non-zero (zero = model default per the
//     llmclient contract; explicit zero can be rejected by some frontier models).
//   - Thinking (ThinkingConfig): there is no standard OpenAI field for
//     budget_tokens thinking. String Effort values ("low", "medium", "high") map
//     to reasoning_effort. A non-empty Thinking.Type with no Effort equivalent
//     returns a typed InvalidRequestError (not silently dropped).
//   - OutputSchema → response_format: {type:"json_schema", json_schema: {name:"response",
//     strict:true, schema: <OutputSchema>}}.
//   - ProviderParams merged VERBATIM last; its keys override adapter-set keys.
//   - StopSequences → the "stop" array field.
//
// Streaming: the client requests stream_options:{include_usage:true} so the
// final data chunk carries usage statistics. SSE events are consumed
// incrementally; tool_call deltas are accumulated by index and emitted as
// complete BlockToolUse blocks. Text arrives as TextDelta chunks; the terminal
// chunk carries StopReason and Usage.
//
// Typed errors: HTTP 400 → InvalidRequestError, 401/403 → AuthError, 429 →
// RateLimitError (with Retry-After header parsing), 5xx → ServerError. The
// API key never appears in error messages.
//
// Concurrency contract: Client is safe for concurrent use — all fields are
// set at construction and never mutated afterwards, and the underlying
// http.Client is safe for concurrent use. Each channel returned by Stream is
// owned by exactly one producer goroutine and is closed exactly once by that
// producer; producers select on ctx.Done() and terminate without goroutine
// leaks.
//
// # Azure OpenAI
//
// Setting Config.AzureDeployment switches the client into Azure mode. Three
// things change from the vanilla path above; everything else — wire
// request/response mapping, streaming, typed error mapping, key redaction —
// is unchanged, since Azure OpenAI matches the OpenAI wire format and error
// envelope byte-for-byte at the endpoints this client uses.
//
//   - Endpoint shape: chat completions (Complete and Stream both) route to
//     {BaseURL}/openai/deployments/{AzureDeployment}/chat/completions
//     instead of {BaseURL}/chat/completions, and Ping routes to
//     {BaseURL}/openai/models instead of {BaseURL}/models. BaseURL is the
//     Azure resource endpoint (https://<resource>.openai.azure.com), not
//     the OpenAI public API base — it must be set explicitly in Azure mode
//     (New rejects an empty BaseURL when AzureDeployment is set, rather
//     than silently defaulting to the OpenAI public API).
//   - api-version: every Azure request — chat completions, streaming, and
//     Ping alike — carries a mandatory api-version query parameter.
//     Config.AzureAPIVersion sets it explicitly; empty defaults to
//     DefaultAzureAPIVersion. That constant is pinned to a known GA
//     (never preview) version and is expected to lag Azure's own latest
//     release by design — see its doc comment for the full pinning
//     doctrine. Callers who need a newer GA version, or an explicit
//     preview version for a feature the pinned constant predates, set
//     Config.AzureAPIVersion themselves; the override always wins.
//   - Auth: the credential travels in the "api-key" header by default
//     (Config.AzureAuthScheme = AzureAuthSchemeAPIKey, the zero value).
//     Setting Config.AzureAuthScheme to AzureAuthSchemeBearer sends
//     "Authorization: Bearer <credential>" instead — Entra-ID token
//     passthrough. Config.APIKey then holds a caller-minted managed-identity
//     or client-credential access token rather than an Azure resource API
//     key. The vanilla (non-Azure) path is unaffected by this field and
//     always uses Bearer.
//
// Routing reality: Azure routes purely by deployment name. Request.Model /
// Config.DefaultModel are still sent in the wire body when set — harmless,
// and it keeps Rung.Apply/model-catalog behavior uniform across every
// provider this platform supports — but they play no part in routing; only
// AzureDeployment does.
//
// Explicit non-goals: this package does not mint, refresh, or cache
// Entra-ID tokens — AzureAuthSchemeBearer expects the caller to supply an
// already-valid token obtained however their environment mints one
// (managed identity, client-credential flow, or any other out-of-kit
// source). It does not implement region failover or multi-deployment
// routing: one Client is pinned to one deployment on one resource endpoint;
// composing multiple Clients (one per deployment/region) behind a
// failover-capable wrapper is the caller's responsibility.
package openai
