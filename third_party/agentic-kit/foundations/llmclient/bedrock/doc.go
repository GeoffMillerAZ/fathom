// Package bedrock implements llmclient.LLMClient against the AWS Bedrock
// runtime API for Claude (Anthropic-on-Bedrock). It supports both commercial
// and GovCloud regions. Complete maps a llmclient.Request to one blocking
// InvokeModel call (POST
// https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke) and
// Stream maps it to InvokeModelWithResponseStream
// (/model/{modelId}/invoke-with-response-stream), surfacing chunks per the
// llmclient v2 streaming contract.
//
// Composition: imports ONLY the standard library and the parent
// foundations/llmclient package. No aws-sdk, no external dependencies. AWS
// SigV4 request signing is hand-rolled on crypto/hmac + crypto/sha256 (see
// sigv4.go); the binary AWS event-stream response framing is hand-decoded on
// encoding/binary + hash/crc32 (see eventstream.go).
//
// Authentication: requests are signed with AWS Signature Version 4 for the
// "bedrock" service in Config.Region. Credentials are resolved through the
// full AWS default chain (foundations/llmclient/bedrock/awscred): explicit
// Config.AccessKeyID/SecretAccessKey/SessionToken, then the conventional
// AWS_* environment variables, then the shared config/credentials files
// (static keys, credential_process, or SSO — `aws sso login`'s modern
// sso_session form and the legacy inline form), then IMDSv2 last. Explicit
// credentials never fall through to ambient state: a complete explicit pair
// is used verbatim without any file/SSO/IMDS access, and a partial pair
// (one half set) is a typed error rather than a silent fall-through to the
// shared files' identity. The secret
// access key is used ONLY to derive the per-request signing key; it never
// travels on the wire and never appears in any error or artifact. A session
// token (temporary STS / instance-role credentials) travels in the
// X-Amz-Security-Token header and is part of the signed header set. GovCloud is
// first-class: set Region to "us-gov-west-1" (or a FIPS BaseURL) and the
// credential scope and host follow; Region itself resolves independently
// (AWS_REGION, then AWS_DEFAULT_REGION, then the shared profile's "region"
// key) when Config.Region is empty.
//
// Credentials that carry an expiry (anything but explicit/env/shared-static)
// are not frozen at construction: Complete, Stream, and Ping re-resolve
// through the same chain, under a mutex-guarded cache, whenever the cached
// credential is within 5 minutes of expiry — so a Client backed by
// `aws sso login` or an instance role keeps working across a token refresh
// without reconstruction. A refresh failure (e.g. an SSO token that expired
// between calls) surfaces as a typed, non-retryable *llmclient.AuthError
// whose message names the fix (`aws sso login --profile <name>`).
//
// Body mapping (Anthropic-on-Bedrock): the request body is the public Anthropic
// Messages body with two differences — the model is the {modelId} URL path
// segment, NOT a body field; and "anthropic_version" is in the BODY (the fixed
// Bedrock token "bedrock-2023-05-31"), not a header. max_tokens is required and
// always sent (Request.MaxTokens, else Config.DefaultMaxTokens, default 4096).
// temperature is omitted when zero. messages, system, tools, tool_choice, and
// stop_sequences map exactly as the public API. CacheHint / SystemCacheHint map
// to cache_control {"type":"ephemeral"} with the same 4-breakpoint local limit.
// BlockUnknown content round-trips verbatim. The model is Request.Model, else
// Config.DefaultModel; both empty fails with a typed InvalidRequestError before
// any HTTP.
//
// Streaming: InvokeModelWithResponseStream returns AWS binary event-stream
// frames (vnd.amazon.eventstream), each "chunk" event wrapping a base64
// {"bytes":...} envelope whose decoded payload is one Anthropic streaming event
// (message_start, content_block_delta, message_stop, ...). The decoder unwraps
// the binary frame and dispatches the inner event through the same
// accumulation logic the public-SSE adapter uses, so the v2 streaming contract
// (incremental TextDelta, whole tool_use blocks, a terminal StopReason+Usage
// chunk, mid-stream errors as a terminal Err chunk) is identical. A Bedrock
// "exception" frame ends the stream with a typed error.
//
// Error mapping: non-200 responses map through the shared typed errors —
// 429/ThrottlingException → RateLimitError, 401/403 → AuthError, 5xx →
// ServerError (retryable), other 4xx → InvalidRequestError. Transport failures
// are typed retryable ServerError; context cancellation/deadline pass through
// untyped so WithRetry never retries a caller-chosen stop.
//
// Timeouts: when Config.HTTPClient is nil, Complete uses a 10-minute overall
// timeout and Stream uses none (ctx governs a stream's lifetime). An injected
// client is used verbatim for both paths; do not set an overall Timeout on it
// if you call Stream.
//
// Concurrency contract: Client is safe for concurrent use — every field
// except the credential cache is set at construction and never mutated, and
// the credential cache is itself internally mutex-guarded (see
// credentialCache), so a re-resolution triggered by one goroutine never
// races a concurrent request's read of the active credential. Each channel
// returned by Stream is owned by exactly one producer goroutine, closed
// exactly once; producers select on ctx.Done() and never leak, and the HTTP
// response body is always closed.
//
// Offline-verifiability caveat: every mapping, the SigV4 canonicalization and
// signature DERIVATION, and the event-stream decode are deterministic and fully
// tested against an httptest.Server with golden vectors (including the AWS SigV4
// reference test vector). What CANNOT be checked offline is acceptance by the
// live Bedrock service — that the GovCloud endpoint accepts the produced
// signature and the model id / inference-profile resolves. Run a live-AWS
// integration test (real credentials, real region) before relying on this in
// production.
package bedrock
