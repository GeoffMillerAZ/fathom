// Package claudecli implements llmclient.LLMClient by shelling out to the
// `claude` CLI (Claude Code) instead of calling an HTTP API directly. The
// point is auth: a `claude -p` invocation rides the CLI's existing login
// (Claude Pro/Max subscription OAuth), so a caller can run completions
// against a subscription instead of a metered ANTHROPIC_API_KEY. This is a
// provider adapter (foundations/llmclient/<provider>/ per the constitution
// org-agnostic carve-out) — fathom's semantic-layer compile path is the named
// consumer.
//
// # Pinned invocation shape (verified against the installed CLI, 2026-07-16)
//
// Verified manually against `claude --version` == "2.1.211 (Claude Code)"
// (`claude --help`, then several one-off `claude -p --output-format json
// ...` calls with trivial prompts — see the package's git history for the
// transcript). The installed build has NO `--max-turns` flag (the flag named
// in this adapter's design brief does not exist in this CLI version) and NO
// `--temperature` / `--stop-sequences` / thinking-budget flag of any kind.
// Every Complete call runs:
//
//	claude -p --output-format json --safe-mode --tools "" \
//	    [--model <model>] [--system-prompt <system>] [--effort <effort>] \
//	    [--json-schema <schema>]
//
// with the prompt text written to the child's stdin (never as a trailing CLI
// argument, which would leak the prompt into shell history / process
// listings and is unnecessary — stdin works with no positional prompt arg).
//
//   - `-p --output-format json`: non-interactive, single JSON envelope on
//     stdout.
//   - `--safe-mode`: disables CLAUDE.md (project AND user-scope),
//     skills/plugins/hooks/MCP servers — chosen deliberately over `--bare`,
//     which looks similar but its help text states it forces
//     "Anthropic auth is strictly ANTHROPIC_API_KEY or apiKeyHelper... OAuth
//     and keychain are never read", which would silently defeat this
//     adapter's entire reason to exist. `--safe-mode`'s help text instead
//     promises "Auth, model selection, built-in tools, and permissions work
//     normally" — verified: running from inside a repo whose CLAUDE.md is
//     large and fathom-specific, `--safe-mode` produced a response with
//     cache_creation_input_tokens: 0 and no awareness of the repo at all;
//     the same prompt without `--safe-mode` (even with a fully custom
//     `--system-prompt`, and even run from an empty scratch directory)
//     leaked the operator's user-scope CLAUDE.md and connected MCP server
//     names into the completion. Ambient-context leakage was the single
//     biggest risk to this adapter's suitability for programmatic,
//     reproducible compiles, and `--safe-mode` closes it.
//   - `--tools ""`: declares zero available tools, which is this CLI
//     version's only way to guarantee the model cannot request a tool round
//     trip (no `--max-turns`, see above). Verified: a plain (no-OutputSchema)
//     call always returns num_turns: 1.
//   - `--model`: omitted when both Request.Model and Config.Model are empty
//     (the CLI then uses its own configured default). Accepts an alias
//     ("haiku", "sonnet", "opus", "fable") or a full model ID — both verified
//     working.
//   - `--system-prompt` (not `--append-system-prompt`): chosen because it
//     REPLACES the CLI's default persona/preamble rather than appending to
//     it, matching every other llmclient adapter's Request.System semantics
//     ("an optional system prompt" that IS the system prompt, not additive
//     to a hidden one). Only emitted when Request.System is non-empty.
//   - `--effort`: Request.Effort passed through verbatim when non-empty.
//     Unlike the anthropic/bedrock adapters (which typed-reject Effort — no
//     stable API knob), the installed CLI has a real `--effort <level>` flag
//     (low/medium/high/xhigh/max), so this adapter honors it instead of
//     rejecting it. The CLI validates the value itself; an unrecognised
//     value surfaces through the normal is_error envelope path.
//   - `--json-schema`: Request.OutputSchema, marshaled verbatim, passed as a
//     single JSON-text argument. Verified working end-to-end: the response
//     envelope's `result` field is compact JSON conforming to the schema
//     (and a redundant `structured_output` field carries the same content
//     pre-parsed — this adapter reads `result` only, consistent with how
//     every other adapter surfaces Response.Text). This is a genuine
//     provider-enforced structured-output mechanism, not merely prompt-side
//     instructions as this adapter's design brief assumed before
//     verification — see "OutputSchema" below for the one caveat it
//     introduces.
//
// # Response envelope (verified shape, `type":"result"`)
//
// Top-level fields this adapter reads: `is_error` (bool), `api_error_status`
// (nullable int — present even for some non-HTTP-looking failures, e.g. an
// unknown --model value surfaced api_error_status: 404), `result` (string —
// the final text, or a human-readable error message when is_error is true),
// `stop_reason` (string — verified to reuse the Messages API vocabulary
// llmclient.StopReason already models: "end_turn", "tool_use",
// "stop_sequence", ...), `usage.{input_tokens,output_tokens,
// cache_creation_input_tokens,cache_read_input_tokens}`. Fields parsed but
// NOT surfaced on llmclient.Response (no mapping exists / cost accounting is
// the caller's job — see "Cost accounting" below): `subtype`, `duration_ms`,
// `num_turns`, `session_id`, `total_cost_usd`, `terminal_reason`,
// `structured_output`.
//
// # Error surface (verified — a deviation from this adapter's design brief)
//
// The brief assumed provider-side errors would need to be read from stderr
// with a non-zero exit code. Verified behavior is different and better: an
// is_error:true result (e.g. an invalid --model value) exits 1 with an EMPTY
// stderr — the full error detail (human-readable message, and often an
// api_error_status) is in the stdout JSON envelope exactly like a success
// response. This adapter therefore parses stdout FIRST regardless of exit
// code; it only falls back to "non-zero exit + stderr tail" when stdout is
// not valid envelope JSON at all (subprocess crash, spawn failure, a killed
// process). When api_error_status is present, it is mapped to the shared
// typed errors the same way the anthropic adapter maps HTTP status
// (401/403 → AuthError, 429 → RateLimitError, >=500 → ServerError, otherwise
// → InvalidRequestError) so WithRetry classifies claudecli failures exactly
// like every other adapter's. When is_error is true with no
// api_error_status, a plain (untyped) error carries the envelope's result
// text.
//
// # Documented degradations
//
//   - Request.MaxTokens is NOT enforceable — the installed CLI has no
//     max-tokens flag. It is silently ignored; the model runs to its own
//     default output cap (`modelUsage.<model>.maxOutputTokens` in the raw
//     envelope, e.g. 32000 for claude-haiku-4-5), which may exceed a
//     caller's intended budget. Callers relying on a tight MaxTokens to
//     bound spend should not assume this adapter enforces it.
//   - StopReason IS surfaced (a positive deviation from the brief, which
//     assumed it would not be) — with one remapping: when Request.OutputSchema
//     is set, the CLI resolves it via an internal structured-output tool call
//     (verified: num_turns becomes 2 and the raw stop_reason is "tool_use"
//     even though the call is already fully resolved and Result is the final
//     answer). Surfacing that raw "tool_use" verbatim would wrongly tell a
//     caller a tool result is owed before the turn can complete, so this
//     adapter remaps it to StopEndTurn whenever OutputSchema was requested.
//     Consumers using structured.Complete therefore still see StopMaxTokens →
//     ErrTruncated correctly classified (contrary to the brief's assumption
//     that truncation would misclassify as ErrMalformed) whenever the model's
//     own cap is hit.
//   - Request.Tools / Request.ToolChoice: not supported. Every call runs with
//     `--tools ""`, so no caller-declared tool can ever be exposed to the
//     model; a non-empty Request.Tools returns a typed InvalidRequestError
//     rather than silently discarding the tool declarations.
//   - Request.Thinking, Request.Temperature, Request.StopSequences: no CLI
//     flag exists for any of these. Each returns a typed InvalidRequestError
//     when set (fail loud, matching the kit's Effort-rejection precedent in
//     the other adapters) rather than silently ignoring the caller's request.
//   - Request.ProviderParams: validated for JSON-object shape
//     (llmclient.ValidateProviderParams) but never applied — there is no
//     outbound wire body for a CLI shell-out to merge into. A non-empty value
//     returns a typed InvalidRequestError instead of being silently dropped,
//     since silently ignoring it would violate the "verbatim merge, never
//     silently shadowed" promise the field carries on every other adapter.
//   - Request.SystemCacheHint / ContentBlock.CacheHint: silently ignored (not
//     an error). These are optimization hints on every other adapter too;
//     the CLI manages its own prompt caching internally (visible as
//     cache_creation_input_tokens / cache_read_input_tokens in the envelope)
//     with no per-block control surface exposed.
//   - Request.Messages: supported only when every block is BlockText — this
//     adapter flattens the conversation to a plain "role: text" transcript on
//     stdin. A Messages entry containing a tool_use / tool_result / image /
//     unknown block returns a typed InvalidRequestError: the CLI shell-out
//     cannot round-trip provider-native tool state or attach binary content
//     via a text pipe. (fathom's compile path, the named consumer, uses
//     Request.Prompt/System only — Messages support here is best-effort.)
//   - OutputSchema is genuinely provider-enforced via --json-schema (see
//     above) rather than merely prompt-side, better than the brief assumed —
//     but the belt-and-suspenders rule in llmclient's Request.OutputSchema
//     doc still applies: callers should validate the parsed artifact
//     themselves (structured.Complete already does).
//
// # Cost accounting
//
// The envelope's total_cost_usd is Anthropic's API-equivalent accounting of
// what the call would have cost on a metered key — it is NOT what this call
// actually cost the caller, which is $0 marginal spend against a Claude
// Pro/Max subscription's plan limits. This adapter does not surface
// total_cost_usd on llmclient.Response (no such field exists on the shared
// type); callers wanting cost visibility apply their own rate tables via
// llmclient/instrument.WithCost over Response.Usage, exactly as with every
// other adapter — the fathom-side wiring documents that its reported dollars
// under this provider are the same API-equivalent accounting, not real spend.
//
// # Scope note
//
// This adapter is for personal/dev tooling riding a local Claude Code login —
// it deliberately never touches ~/.claude/.credentials.json, the OS keychain,
// or any stored OAuth token; it only ever invokes the `claude` binary as a
// subprocess and lets the CLI resolve its own credentials.
//
// # Composition
//
// Subprocess lifetime, output-size bounding, and process-tree cleanup on
// timeout/cancellation are delegated to the kit's own
// foundations/agent/bounded-exec runner rather than a second hand-rolled
// exec.CommandContext — see Client's doc comment for why its environment
// deliberately passes os.Environ() through verbatim (the opposite of
// bounded-exec's typical scrubbed-allowlist caller).
//
// # Concurrency
//
// Client is immutable after New and safe for concurrent use: every Complete
// (and Stream) call spawns its own subprocess with no shared mutable state
// between concurrent calls.
package claudecli
