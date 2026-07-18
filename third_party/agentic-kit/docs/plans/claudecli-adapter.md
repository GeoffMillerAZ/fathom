# claudecli adapter — subscription-backed llmclient provider

**Mission** — fathom wants its semantic-layer compile path to ride Geoff's
Claude Pro/Max subscription instead of a metered `ANTHROPIC_API_KEY`. Claude
Code's headless print mode (`claude -p`) uses the CLI's existing OAuth login
and is the supported way to script it. This plan lands
`foundations/llmclient/claudecli`, a new `llmclient.LLMClient` implementation
that shells out to the `claude` binary instead of calling an HTTP API
directly. fathom's compile path is the named consumer (wired in a companion
change on the fathom side); throughline is a plausible second. This plan is
gated by `fathom verify` at pre-push, same as every other `docs/plans/*.md`
in this repo.

**Grounding note (2026-07-16):** the fathom fleet-index daemon was timing out
on every `ingest`/`find_symbol`/`staleness_report` call while this plan was
authored (likely load/lock contention unrelated to this change — see
CLAUDE.md's single-writer-lock note). Every `{symbol, package}` below names a
real, already-built-and-tested identifier in this change-set (confirmed via
`go build`/`go vet`/`go test -race`, all green — see "Acceptance = proof"),
but was NOT separately confirmed against the live fathom index before the
first `[x]` was written here, contrary to this repo's normal grounding
practice. **Re-run `fathom verify --service agentic-kit --plan
docs/plans/claudecli-adapter.md` once the index has ingested this commit** and
fix any claim it disputes — the pre-push gate will do this automatically; if
it reports an operational error (exit 2) rather than a real verdict, re-run it
by hand once the daemon is healthy.

## Design-brief deviations (verified against the installed CLI, 2.1.211)

The original design brief assumed a `--max-turns` flag and stderr-carried
errors; the installed CLI (verified via `claude --help` plus one-off manual
`claude -p --output-format json ...` calls, transcribed in the package doc)
differs in ways this implementation follows instead of the brief:

- **No `--max-turns` flag exists.** `--tools ""` (declaring zero available
  tools) is this CLI's only way to guarantee no tool round-trip; verified via
  `num_turns: 1` on a plain call.
- **Errors surface on stdout, not stderr.** An `is_error:true` result (e.g. an
  invalid `--model`) exits 1 with an EMPTY stderr; the full error detail
  (`result` text, often `api_error_status`) is in the stdout JSON envelope
  exactly like a success response. `Complete` parses stdout first and only
  falls back to exit-code+stderr-tail when stdout isn't valid envelope JSON.
- **`--safe-mode`, not `--bare`, suppresses ambient context.** `--bare`'s help
  text states it forces API-key-only auth (OAuth/keychain never read) —
  wiring it would silently defeat this adapter's entire reason to exist.
  `--safe-mode` disables CLAUDE.md/MCP/skills while explicitly preserving
  OAuth auth; verified empirically (`cache_creation_input_tokens: 0`, zero
  project awareness, even run from inside a repo with a large project-specific
  CLAUDE.md).
- **`--json-schema` and `--effort` are real, working flags** the brief didn't
  anticipate: `--json-schema` genuinely enforces `Request.OutputSchema`
  (verified: a conforming JSON `result` plus a redundant `structured_output`
  field), and `--effort` honors `Request.Effort` — unlike the anthropic/bedrock
  adapters, which reject `Effort` outright (no stable API knob there).
  `--json-schema`'s one wrinkle: it resolves via an internal structured-output
  tool call, surfacing raw `stop_reason:"tool_use"` even though the turn is
  already fully resolved — `Complete` remaps that specific case to
  `StopEndTurn` so a caller is never told a tool result is owed.
- **`stop_reason` IS surfaced** (the brief assumed it would not be), so
  `structured.Complete` on the fathom side correctly classifies `StopMaxTokens`
  as `ErrTruncated` through this adapter too, contrary to the brief's
  assumption that truncation would misclassify as `ErrMalformed`.

Full detail, including every field this adapter cannot honor
(`Tools`/`Thinking`/`Temperature`/`StopSequences`/`ProviderParams`/non-text
`Messages` blocks — each a typed `InvalidRequestError`, never a silent drop —
and `MaxTokens`, silently ignored since no CLI flag exists for it) lives in
`foundations/llmclient/claudecli/doc.go`'s package doc, which is the source of
truth; this plan summarizes it for gating purposes only.

## CA — build now (named consumer: fathom's compile provider seam)

- [x] `Config{Bin, Model, Timeout}` + `New(cfg Config) (*Client, error)` — resolves `Bin` (default `"claude"`) via `exec.LookPath` at construction (fails fast, not on first `Complete`), builds a `foundations/agent/bounded-exec` `Runner` (reused rather than a second hand-rolled `exec.CommandContext`), passes `os.Environ()` through verbatim so the CLI can find its own OAuth session. {id: CA-1, symbol: New, package: foundations/llmclient/claudecli}
- [x] `func (c *Client) Complete(ctx, req) (llmclient.Response, error)` — builds argv (`-p --output-format json --safe-mode --tools ""` plus conditional `--model`/`--system-prompt`/`--effort`/`--json-schema`), writes the prompt to stdin, runs it through the bounded-exec runner, parses the JSON result envelope, and maps `result`/`stop_reason`/`usage` onto `llmclient.Response`. Covered by `TestComplete_HappyPath_ArgvStdinAndResponseMapping`, `TestComplete_RequestModelOverridesConfigModel`, `TestComplete_NoModel_OmitsModelFlag`, `TestComplete_SystemPromptFlagOmittedWhenEmpty`, `TestComplete_SystemPromptPassedThrough`, `TestComplete_EffortPassthrough` (all hermetic, fake-shim argv/stdin assertions — no network). {id: CA-2, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `Complete` maps an `is_error:true` envelope to the shared typed errors by `api_error_status` (401/403→`AuthError`, 429→`RateLimitError`, ≥500→`ServerError`, else→`InvalidRequestError`; absent status→plain error), mirroring how the anthropic adapter maps HTTP status so `llmclient.WithRetry` classifies claudecli failures identically to every other adapter. Covered by `TestComplete_IsErrorMappedByAPIErrorStatus` (table-driven over all four buckets, each subtest calling `c.Complete` directly) and `TestComplete_IsErrorWithoutAPIErrorStatus_PlainError`. {id: CA-3, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `Complete` genuinely enforces `Request.OutputSchema` via `--json-schema` (verified provider-side enforcement, not merely prompt-side instructions) and remaps the resulting internal `stop_reason:"tool_use"` to `StopEndTurn` so callers are never told a tool result is owed for an already-resolved turn; `stop_reason` otherwise passes through the open `llmclient.StopReason` set verbatim (e.g. `max_tokens`→`StopMaxTokens`, unaffected by the remap). Covered by `TestComplete_OutputSchema_PassesJSONSchemaFlagAndRemapsToolUse` and `TestComplete_StopReason_PassthroughWithoutOutputSchema`. {id: CA-4, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `Complete` rejects `Request.Tools`, `Request.Thinking`, `Request.Temperature`, `Request.StopSequences`, and a non-empty `Request.ProviderParams` with a typed `InvalidRequestError` (fail loud — no CLI knob exists for any of them, so silently dropping the caller's request would be dishonest), and restricts `Request.Messages` to `BlockText`-only content, flattening to a plain transcript on stdin. Covered by `TestComplete_UnsupportedFeaturesRejected` (table-driven, one subtest per field, each calling `c.Complete` directly), `TestComplete_Messages_TextOnlyFlattened`, and `TestComplete_Messages_NonTextBlockRejected`. {id: CA-5, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `Complete` maps a malformed (non-JSON) stdout envelope to an error carrying the exit code and a bounded stderr tail, regardless of exit code — the CLI's own error surface (see "Design-brief deviations") puts provider errors on stdout, not stderr, so this path is reserved for subprocess-level failures (crash, spawn failure). Covered by `TestComplete_NonZeroExit_NonJSONStdout_UsesStderrTail` and `TestComplete_MalformedJSON_ExitZero`. {id: CA-6, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `Complete` honors ctx cancellation and its own `Config.Timeout` bound: a timeout returns an error wrapping `context.DeadlineExceeded` and a caller-side `cancel()` returns one wrapping `context.Canceled`, both via the composed bounded-exec runner's process-tree kill (not a bare `Process.Kill` on just the immediate child). Covered by `TestComplete_CtxTimeoutKillsChild` and `TestComplete_CallerCtxCancelKillsChild` (both assert prompt return, not just eventual return). {id: CA-7, symbol: Complete, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `func (c *Client) Stream(ctx, req) (<-chan llmclient.StreamChunk, error)` — satisfies the `llmclient.LLMClient` interface's Stream half (required since `pkg/client.WithLLM` on the fathom side takes the full interface, not just `Complete`) by delegating to `Complete` and delivering the result as one `TextDelta` chunk followed by the terminal `StopReason`+`Usage` chunk; a `Complete` failure arrives as the terminal chunk's `Err`, per the llmclient streaming contract's pre-stream-vs-in-stream error split. Covered by `TestStream_DelegatesToComplete_HappyPath` and `TestStream_DelegatesToComplete_Error`. {id: CA-8, symbol: Stream, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Complete/Stream is defined by every llmclient adapter; ambiguous method-call edges are the documented O9 residue test_reaches honestly refuses even package-qualified — the named hermetic fake-shim tests in this row call the method directly (claudecli client_test.go)"}
- [x] `New` fails fast (returns a non-nil error, not a panic or a lazily-broken Client) when `Config.Bin` cannot be resolved via `exec.LookPath`. Covered by `TestNew_ResolvesBin` and `TestNew_BinNotFound`. {id: CA-9, symbol: New, package: foundations/llmclient/claudecli}
- [x] Repo hygiene for the new package, same change-set: `cmd/constitution-lint/waivers.txt` (schema-presence waiver — behavioral-only, no data contract of its own), `TRACKING.md` §1.2 bullet, `docs/compat.md` (new `experimental` row + a post-audit drift note, mirroring the existing `vaulttest` precedent), `docs/catalog.md` regenerated (`go run ./cmd/kit-catalog`, 182 packages), `README.md`'s package-count sentence (181→182). Ops/doc actions, not call-graph-reachable from a test. {id: CA-10, symbol: New, package: foundations/llmclient/claudecli, skip_test: true, skip: "doc/config housekeeping (waivers.txt, TRACKING.md, compat.md, catalog.md, README.md) — no Test* calls these; anchored to CA-1's symbol since it's the same change-set the housekeeping documents"}

## Design constraints

- **No network in tests, ever.** Every `claudecli` test uses a fake `claude`
  shell script written to `t.TempDir()` (recorded argv/stdin, canned
  JSON/exit-code/stderr, all controlled via environment variables) — never the
  real CLI, never a real credential.
- **Never touch stored credentials.** This package never reads
  `~/.claude/.credentials.json` or the OS keychain; it only invokes the
  `claude` binary as a subprocess and lets the CLI resolve its own auth.
- **Compose, don't reinvent.** Subprocess timeout/output-bounding/process-tree
  cleanup reuses `foundations/agent/bounded-exec` rather than a second
  hand-rolled `exec.CommandContext` + kill-on-cancel implementation.
- **Behavioral-only package shape.** No `schema.cue` — this package wraps the
  `claude` CLI's own JSON envelope; it defines no CUE-owned data contract of
  its own (same class as `foundations/llmclient/structured`).

## Acceptance = proof

1. `task validate` green (fmt-check, vet, cue-vet, `constitution-lint`,
   `go test -race ./...` — all confirmed passing on this change-set, including
   the pre-existing suite unaffected by this addition).
2. `foundations/llmclient/claudecli`'s own suite: 21 test funcs, `-race` clean,
   zero network calls, zero real credentials touched.
3. `constitution-lint` clean on this package specifically: schema-presence
   (waived), tracking-coverage, compat-coverage, catalog-fresh, readme-count,
   doc-coverage (every exported identifier documented), org-agnostic (no
   hardcoded org names; the provider-adapter carve-out already covers any
   model-ID literal defaults in this directory).
4. This plan verifies via the pre-push gate (`fathom verify --service
   agentic-kit --plan docs/plans/claudecli-adapter.md`) once the fleet index
   has ingested this commit — see the grounding note above.
