// Package llmreplay provides deterministic record/replay cassettes for
// llmclient.LLMClient: RecordingClient wraps any live client and appends every
// interaction to a JSONL cassette; ReplayClient replays a cassette verbatim,
// so downstream agent tests never need a live API or hand-scripted mocks.
// Record a session once (against a real provider or against MockClient),
// commit nothing by default, and replay it forever — byte-identical responses,
// identical stream chunk segmentation, equivalent errors.
//
// Composition: imports the parent foundations/llmclient (the domain types and
// the LLMClient interface), foundations/persistence/jsonl-log (the cassette's
// append-only JSONL persistence), and foundations/security/pre-llm-dlp-gate
// (the default scrubbing policy). Stdlib otherwise; no network anywhere.
//
// # Cassette format
//
// A cassette is a JSONL file: one Entry per line, in call order. Entries hold
// DOMAIN STRUCTS ONLY — llmclient.Request, llmclient.Response, and recorded
// stream Chunks with their original segmentation. HTTP headers, API keys,
// transport details, and timestamps are never recorded; a cassette captures
// what the model was asked and what it answered, nothing about how the bytes
// travelled. Stream entries preserve chunk boundaries exactly, so replay
// reproduces not just the final text but the incremental delivery shape.
//
// Errors are recorded as strings (Entry.Err for failed Complete calls and
// pre-stream Stream failures; Chunk.Err for mid-stream terminal failures) and
// replayed as *ReplayedError carrying the original message. The original
// error TYPE is not preserved: a recorded *llmclient.RateLimitError replays
// as a *ReplayedError, which llmclient.WithRetry treats as non-retryable.
// Cassettes capture transcripts, not provider failure taxonomy.
//
// A stream entry recorded under cancellation may end without a terminal
// chunk (zero chunks, or a last chunk carrying no StopReason/Usage/Err).
// Replay does NOT reproduce that shape verbatim — closing the channel with
// no terminal chunk would violate the llmclient streaming contract — so
// ReplayClient.Stream re-emits the recorded chunks and then synthesizes a
// terminal chunk whose Err is a *ReplayedError explaining that the cassette
// was recorded under cancellation.
//
// # Cassettes are sensitive artifacts
//
// A cassette is a verbatim transcript of prompts and completions. Treat it
// like a log of everything your agent said and was told: store cassettes
// under workspace/ (gitignored) unless they have been deliberately reviewed
// and committed on purpose.
//
// Scrubbing is ON BY DEFAULT: before each append, a Scrubber runs over the
// entry. DefaultScrubber applies the pre-llm-dlp-gate MustDefaultGate
// redaction policy to all text content — request prompt/system text, text and
// tool_result blocks in messages, response text, chunk text deltas, and
// recorded error strings. Opting out requires the explicit NoScrub option,
// which is DANGEROUS: an unscrubbed cassette preserves credentials verbatim.
//
// Scrubbing is best-effort secret-SHAPE redaction, not a guarantee. Known
// gaps, stated honestly:
//
//   - Stream text deltas are scanned PER DELTA: a secret split across chunk
//     boundaries does not match any rule and survives. The same response
//     recorded via Complete would be redacted.
//   - tool_use Input, BlockUnknown Raw, and ToolDef schemas/descriptions are
//     raw JSON and are NOT scanned — redacting inside structured JSON could
//     corrupt it. Secrets inside tool arguments pass through.
//   - Encoded, truncated, or novel-format secrets pass through (see the
//     pre-llm-dlp-gate honesty contract).
//
// Review every cassette before sharing or committing it.
//
// # Matching
//
// ReplayClient consumes entries ORDINALLY by default: call n gets entry n,
// the entry's Kind must match the called method (Complete vs Stream), and
// exhaustion is a clear error. Opt-in MatchByRequestHash additionally
// verifies, per call, that the replayed request hashes to the same value as
// the recorded one; a mismatch errors with the entry index and both hashes.
//
// RequestHash hashes the V2 CANONICAL JSON of the request: marshal, decode
// into generic values, DROP zero-value fields (empty strings, zero numbers,
// false booleans, nil/empty slices and maps), then re-encode with sorted
// keys. The zero-drop step makes the hash invariant to additive schema
// growth: a new zero-valued field added to Request or ContentBlock does not
// change the hash of any existing request that doesn't populate that field,
// so cassettes survive schema evolution without re-recording.
//
// Cassette entries carry a hash_version field (HashVersion2 = 2 for newly
// recorded cassettes; absent/0 for cassettes recorded before v2). When
// MatchByRequestHash verifies an entry, the LIVE request is always hashed
// with v2. The STORED request is hashed with the version recorded in the
// entry: v1 entries (no zero-dropping) are re-hashed without dropping, so
// a v1 cassette whose stored request JSON already contains all fields
// (including zero-valued ones introduced later) still verifies correctly.
// In practice: if the stored v1 request was recorded when a field did not
// yet exist, that field's zero value is filled by JSON decode, making v1
// re-hashing of the stored request equivalent to v2 hashing of the live
// request (both drop/exclude the zero-valued field, one by absence, one by
// the drop rule). This is the core compatibility guarantee.
//
// Callers who want hash matching must keep requests deterministic — no
// timestamps, random IDs, or other volatile content in prompts. Note also
// that recorded requests are SCRUBBED before being written, so a live
// request containing redactable content will not hash-match its recorded
// form — by design: test requests should not contain real secrets.
//
// # Concurrency contract
//
// RecordingClient and ReplayClient are data-race safe: the cassette writer is
// mutex-guarded (via jsonl-log's Writer) and the replay cursor is
// mutex-guarded. They are NOT meaningfully safe for concurrent interleaved
// use: concurrent recorded calls append entries in lock-acquisition order,
// which ordinal replay cannot reproduce. Record and replay single-goroutine,
// and drain each recorded stream fully before making the next call (a stream
// entry is appended when its terminal chunk arrives, not when Stream
// returns). Channels returned by ReplayClient.Stream follow the llmclient
// streaming contract: one producer goroutine, closed exactly once, honours
// ctx cancellation without leaking.
package llmreplay
