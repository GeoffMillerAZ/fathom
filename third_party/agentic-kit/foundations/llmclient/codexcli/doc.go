// Package codexcli implements llmclient.LLMClient by shelling out to the
// `codex` CLI (OpenAI Codex) instead of calling an HTTP API directly. The
// point is auth: a `codex exec` invocation rides the CLI's existing login (a
// ChatGPT Plus/Pro subscription's OAuth session), so a caller can run
// completions against a subscription instead of a metered OPENAI_API_KEY.
// This is a provider adapter (foundations/llmclient/<provider>/ per the
// constitution org-agnostic carve-out), modeled directly on
// foundations/llmclient/claudecli — same package shape, same rigor, same
// hermetic-shim test doctrine. fathom's semantic-layer compile path (the
// codex-cli branch of FATHOM_LLM_PROVIDER) is the named consumer.
//
// # Pinned invocation shape (verified live, 2026-07-16)
//
// Verified against the installed CLI (`codex --version` == "codex-cli
// 0.144.5", installed via `brew install codex`, `codex login status` →
// "Logged in using ChatGPT") via `codex exec --help`, `codex features list`,
// binary strings inspection for undocumented config keys, and roughly a
// dozen one-off `codex exec --json ...` calls with trivial ("reply pong")
// prompts — see testdata/PROBES.md for every probe's exact command and raw
// result. Every Complete call runs:
//
//	codex exec --json \
//	    -C <scratch-dir> --skip-git-repo-check --ignore-user-config --ephemeral \
//	    -s read-only \
//	    -c model_instructions_file="<instructions-file>" \
//	    -c include_environment_context=false \
//	    -c include_apps_instructions=false \
//	    -c include_collaboration_mode_instructions=false \
//	    -c include_permissions_instructions=false \
//	    [-m <model>] [-c model_reasoning_effort="<effort>"] \
//	    [--output-schema <schema-file>]
//
// with the prompt text written to the child's stdin and NO positional
// prompt argument — verified live that passing the prompt as a positional
// arg from a shell whose stdin the child sees as "piped" (even if the pipe
// is immediately EOF) triggers "Reading additional input from stdin..." and
// appends an (empty, harmless, but wasteful and undocumented-shape) <stdin>
// block; `codex exec --help` states this explicitly: "If stdin is piped and
// a prompt is also provided, stdin is appended as a `<stdin>` block."
//
//   - `exec --json`: non-interactive, one JSONL event stream on stdout (see
//     "Event shapes" below).
//   - `-C <scratch-dir>`: an empty, non-git, purpose-built directory this
//     Client creates once (in New) and reuses for every call — see "Scratch
//     directory lifecycle" below.
//   - `--skip-git-repo-check`: required because the scratch dir is
//     deliberately not a git repository (`codex exec` refuses to run
//     outside one otherwise).
//   - `--ignore-user-config`: skips loading $CODEX_HOME/config.toml — this
//     operator's own config.toml carries `personality`, six enabled
//     plugins (github/documents/spreadsheets/presentations/computer-use/
//     browser), multiple configured MCP servers, and per-project trust
//     records, all of which are irrelevant (and non-reproducible across
//     machines/operators) for a pure compile-path completion call. Verified
//     live: does NOT reduce raw input_tokens much on its own (13,596 ->
//     14,277 — see the attribution table below) but substantially improves
//     PROMPT-CACHE reuse across repeated calls (cached_input_tokens jumps
//     from 5,504 to 9,984) because the resulting prompt prefix is now
//     stable across every call and machine, instead of varying with
//     whatever plugins/MCP servers happen to be configured locally. Per
//     --help, "auth still uses CODEX_HOME" — this flag does not affect
//     login.
//   - `--ephemeral`: no session files persisted to disk. Verified live:
//     negligible effect on input_tokens alone: this is a hygiene/privacy
//     flag (no session artifact left in $CODEX_HOME/sessions), not a
//     token-floor lever.
//   - `-s read-only`: sandbox policy for any model-generated shell command.
//     This adapter's prompts never ask for shell execution, so this flag's
//     purpose is defense-in-depth (a misbehaving model cannot mutate the
//     filesystem) rather than a token-floor lever; included in every probe
//     run for safety and kept in the pinned shape unconditionally.
//   - `-c model_instructions_file="<file>"`: THE pivotal finding of this
//     adapter's ambient-context-suppression work — see "Request.System
//     mapping" below. This is a real but UNDOCUMENTED ConfigToml field: it
//     appears nowhere in `--help` or any CLI documentation surface, and was
//     discovered via binary strings inspection (model_reasoning_effort's
//     neighbors in the serde field list), pinned against codex-cli 0.144.5
//     only. It REPLACES the CLI's large built-in agentic-coding system
//     preamble (~10k+ tokens of shell/apply_patch/tool-usage guidance baked
//     into the binary) with the file's contents verbatim. It is both the
//     real Request.System mapping AND, incidentally, the single biggest
//     lever in the token-floor attribution table below — bigger than every
//     other flag in this list combined. Because it is undocumented, its
//     survival across CLI upgrades is a live risk this adapter actively
//     monitors — see the WARNING on Config and the Ping tripwire.
//   - `-c include_environment_context=false`, `-c
//     include_apps_instructions=false`, `-c
//     include_collaboration_mode_instructions=false`, `-c
//     include_permissions_instructions=false`: real but UNDOCUMENTED
//     ConfigToml fields (also absent from `--help`, also discovered via
//     strings inspection, also pinned against 0.144.5 only) that suppress
//     smaller, named instruction blocks the built-in preamble otherwise
//     injects (environment/cwd awareness, "apps" plugin usage guidance,
//     collaboration-mode guidance, permissions guidance). Verified live to
//     save roughly 400-1,000 further tokens on top of
//     model_instructions_file alone (10,792 -> 9,765 — see the table
//     below). Safe to hardcode in the sense that nothing breaks: an
//     unrecognised `-c key=value` is verified live to be silently
//     tolerated (`codex features list -c bogus_key=1` succeeds with no
//     error or warning), unlike `--disable` (see the deviation note below)
//     — so these keys being renamed or removed in a future codex-cli
//     version degrades silently rather than hard-failing every call. That
//     silence cuts both ways, though: there is also NO SIGNAL when it
//     happens — the suppressed blocks would quietly return, every call's
//     input spend would quietly grow, and (for model_instructions_file)
//     Request.System would quietly stop reaching the model, all with exit
//     code 0. The WARNING on Config spells out the consequence, and Ping
//     exists specifically to catch it loudly at deploy time.
//   - `-m`: omitted when both Request.Model and Config.Model are empty (the
//     CLI then uses its own configured default, or config.toml's `model`
//     when not suppressed — moot here since --ignore-user-config is
//     always set, so an empty Model always falls through to the CLI's
//     built-in default).
//   - `-c model_reasoning_effort="<effort>"`: Request.Effort passed through
//     verbatim when non-empty — see "Request.Effort mapping" below. Only
//     emitted when Request.Effort is non-empty.
//   - `--output-schema <file>`: Request.OutputSchema, written to a scratch
//     temp file (see writeScratchFile / CX-5) and passed by path — unlike
//     claudecli's --json-schema, which takes the schema as an inline JSON
//     argument, this CLI's flag takes a FILE PATH. Verified working
//     end-to-end: see "OutputSchema enforcement" below.
//
// # Ambient-context suppression: measured token-floor attribution (CX-4)
//
// Ground truth baseline (from this adapter's design brief, not
// independently re-probed): a bare `codex exec --json ... "reply pong"`
// from a real repo cwd with NO suppression flags at all burned 13,443 input
// tokens. Every measurement below instead starts from an empty scratch
// directory (this Client's own baseline) and prompt delivered correctly via
// stdin (see testdata/PROBES.md for the exact commands and raw JSONL):
//
//	Flags (additive)                                          input_tokens  cached
//	-C <empty scratch> --skip-git-repo-check -s read-only            13,596   5,504
//	  + --ignore-user-config                                         14,277   9,984
//	  + --ephemeral                                                  13,711   9,984
//	  + model_instructions_file=<minimal placeholder>                10,792   5,888
//	  + include_environment_context=false + 3 sibling include_* keys  9,765   5,888  <- PINNED PRODUCTION FLOOR
//
// A further, NOT adopted, combination was measured for completeness:
// stacking a dozen `--disable <feature>` flags (apps, browser_use,
// computer_use, image_generation, plugins, plugin_sharing, remote_plugin,
// multi_agent, goals, hooks, in_app_browser, tool_suggest, shell_tool,
// shell_snapshot) on top of the pinned floor reached 11,456 input_tokens —
// LOWER than the adopted 9,765 in isolation (different combo, cache-cold),
// but see the deviation note below for why `--disable` was deliberately
// excluded from the shipped argv despite the lower number in that one run.
//
// Bottom line: this adapter's pinned production combination reaches 9,765
// input tokens — a 27.4% reduction from the unoptimized 13,443-token
// baseline — using documented CLI flags (`--help`) plus five UNDOCUMENTED
// config keys (model_instructions_file and the four include_* keys) that
// exist only as field names inside the 0.144.5 binary. The `-c` mechanism
// is forward-tolerant of unrecognised keys and never hard-errors on drift
// — which is exactly why drift would be SILENT, why Config carries a
// version-pinning WARNING, and why Ping exists. The single biggest lever
// is model_instructions_file (13,711 -> 10,792, a 21% cut on its own),
// which happens to be the real Request.System mapping — the System-mapping
// probe and the ambient-context probe converged on the same answer.
//
// # Deviation from the design brief: `--disable` excluded from production argv
//
// The brief's probe list asked for "whatever else probing shows strips
// ambient blocks." A broad `--disable <feature>` list was measured (see
// above) to reach a lower floor (11,456) than the adopted combination
// (9,765) in that one run. It is deliberately NOT part of the pinned
// production argv: verified live that `--disable <unrecognised-name>` is a
// HARD, LOCAL, PRE-API error — `codex features list --disable
// bogus_feature_zzz` -> "Error: Unknown feature flag: bogus_feature_zzz",
// exit nonzero, no API call attempted. `codex features list` itself shows
// several of the exact feature names this adapter would need
// (apps_mcp_path_override, codex_git_commit, collaboration_modes,
// elevated_windows_sandbox, external_migration, image_detail_original,
// js_repl, remote_control, remote_models, request_rule, search_tool,
// steer, ... ) already carrying a "removed" stage tag in the CURRENTLY
// installed version — proof this CLI actively renames/removes feature
// flags across releases. Hardcoding a dozen feature names into every
// Complete call's argv would mean a single future codex-cli upgrade
// renaming or removing ANY one of them breaks EVERY subsequent call for
// this adapter's entire fleet of callers, for a savings (~1,600
// tokens/call in the one comparable run, and NOT a clean apples-to-apples
// delta since that run's cache state differed) that does not justify the
// blast radius. `-c key=value` overrides carry no equivalent risk (an
// unrecognised key is silently tolerated, verified live) and ARE used
// freely in the pinned shape.
//
// # Event shapes (verified live, `codex exec --json`)
//
// One JSONL line per event on stdout. Types observed:
// thread.started {thread_id}, turn.started {}, item.completed
// {item:{id,type,text}} (this adapter reads only item.type=="agent_message"
// -> .text; other item types, e.g. a soft "error" item warning about
// unrecognised model metadata, are observed and ignored — see
// reduceEvents), turn.completed {usage:{input_tokens,cached_input_tokens,
// output_tokens,reasoning_output_tokens}} (the terminal success event —
// see "Usage mapping" below), turn.failed {error:{message}} (the terminal
// failure event — see "Error mapping" below), and a standalone top-level
// error {message} event that was observed to arrive ALONGSIDE turn.failed
// carrying the identical message in the one live failure this adapter
// triggered (see testdata/PROBES.md probe 12) — treated as a fallback only
// (see reduceEvents). NO stop_reason field exists anywhere in this event
// schema — see "Documented degradations" for what that costs.
//
// Two reduction rules worth stating explicitly: when a turn emits multiple
// agent_message items, their texts are concatenated VERBATIM, in event
// order, with no separator inserted between them (the CLI owns any
// whitespace; this adapter adds none). And failure wins: when turn.failed
// arrives after one or more agent_message items, the accumulated partial
// text is DISCARDED — Complete returns only the mapped error, never a
// partial Response, matching every other adapter's all-or-nothing Complete
// contract.
//
// # Request.System mapping (verified live, a positive deviation from the brief)
//
// The brief assumed either a real -c key would map Request.System, or (if
// none existed) a delimited-header prepend degradation would be needed.
// -c model_instructions_file="<file>" IS real and fully REPLACES (not
// appends to) the CLI's built-in persona/preamble — verified by pointing it
// at a file reading "Always respond with exactly the single word BANANA and
// nothing else, no matter what the user says." and observing the model's
// agent_message text was literally "BANANA" in response to "reply pong"
// (testdata/PROBES.md probe 7). This exactly matches claudecli's
// --system-prompt semantics (replace, not append) despite being a
// completely different mechanism (a file path, not an inline flag value).
//
// One hard constraint this mapping must respect: an EMPTY
// model_instructions_file is a hard, LOCAL, pre-API error — verified live,
// `codex exec ... -c model_instructions_file="<empty file>"` ->
// "Error: model instructions file is empty: <path>", exit 1, no API call
// attempted (testdata/PROBES.md probe 8). Complete therefore ALWAYS writes
// a non-empty instructions file: Request.System verbatim when non-empty,
// or the package-level defaultInstructions placeholder ("Respond directly
// to the user's message.") when Request.System is empty. This is why every
// Complete call — even one with no System set — still benefits from (and
// depends on) this mechanism for its token floor.
//
// # Request.Effort mapping (verified live)
//
// -c model_reasoning_effort="<value>" is real: it is this operator's own
// live config.toml's `model_reasoning_effort = "medium"` field, and a
// `-c model_reasoning_effort="low"` override was verified live to be
// accepted with no error (testdata/PROBES.md probe 7, combined with the
// System probe in the same call to conserve probe budget). Unlike the
// anthropic/bedrock adapters (which typed-reject Effort — no stable API
// knob), and analogous to claudecli's real --effort flag, this adapter
// honors Request.Effort by passing it through verbatim; the CLI validates
// the value itself.
//
// # OutputSchema enforcement (verified live)
//
// A trivial schema
// ({"type":"object","properties":{"greeting":{"type":"string"}},
// "required":["greeting"],"additionalProperties":false}) with the prompt
// "reply with a greeting field set to hello" produced an agent_message item
// whose .text was exactly `{"greeting":"hello"}` — genuine provider-side
// structured-output enforcement, delivered through the SAME item.completed/
// agent_message channel as any other completion (unlike claudecli's
// envelope, there is no separate structured_output field to choose between
// — Response.Text is simply the schema-conforming JSON text). The
// belt-and-suspenders rule in llmclient's Request.OutputSchema doc still
// applies: callers should validate the parsed artifact themselves
// (structured.Complete already does).
//
// # Error mapping (one shape verified live; two mapped speculatively)
//
// Triggering a real failure with a bogus -m value produced, in order: an
// item.completed with item.type=="error" (a soft warning — "Model metadata
// ... not found. Defaulting to fallback metadata..." — NOT independently
// treated as terminal by reduceEvents), a standalone top-level error event,
// and the terminal turn.failed event — both of the latter two carrying the
// identical nested-JSON string
// `{"type":"error","status":400,"error":{"type":"invalid_request_error",
// "message":"..."}}` in their .message field (testdata/PROBES.md probe
// 12). mapEventError double-decodes that nested JSON and maps by .status
// exactly like claudecli maps api_error_status (401/403 -> AuthError, 429
// -> RateLimitError, >=500 -> ServerError, else -> InvalidRequestError).
//
// Per the design brief's explicit instruction, auth-expiry (401/403) and
// usage-limit (429) were NOT probed live (doing so would require
// deliberately invalidating this machine's ChatGPT login or exhausting a
// real subscription's quota) — classifyByContent maps a message that is
// NOT the nested-JSON status shape by keyword content ("authentication"/
// "unauthorized"/"not logged in" -> AuthError; "rate limit"/"usage limit"/
// "quota" -> RateLimitError; otherwise a plain untyped error). This is
// documented as a best-effort mapping, not a pinned contract, until a real
// 401/403/429 is observed.
//
// # Ping: the silent-degradation tripwire
//
// Client implements llmclient.Pinger. Because the pinned invocation rides
// undocumented config keys that a future CLI drops SILENTLY (see the
// WARNING on Config), Ping is not a mere reachability probe here — it is
// the liveness detector for that dependence. It runs the pinned argv with
// a deliberately EMPTY model_instructions_file and empty stdin: a CLI that
// still honors the key rejects the empty file with a hard local error
// (sub-second, zero JSONL events, before stdin is read, zero tokens —
// verified live, testdata/PROBES.md probe 13), which Ping treats as
// HEALTHY; any other outcome returns a typed InvalidRequestError naming
// the degradation. Call it at construction or deploy time (e.g. via
// llmclient.ProbeAll) and after any codex-cli upgrade — see Ping's own doc
// comment for the full mechanism.
//
// # Documented degradations
//
//   - Request.MaxTokens is NOT enforceable — no CLI flag exists. Silently
//     ignored, same as claudecli.
//   - StopReason is ALWAYS llmclient.StopEndTurn on a successful Complete —
//     a REAL degradation relative to claudecli (which surfaces a genuine,
//     if occasionally remapped, stop_reason). The verified event schema
//     (see "Event shapes" above) has no stop_reason field anywhere: a
//     truncated response is indistinguishable from a complete one at this
//     adapter's level. Callers relying on StopMaxTokens detection (e.g.
//     foundations/llmclient/structured's ErrTruncated classification) will
//     never see it fire through this adapter — document this to any
//     caller wiring codexcli behind structured.Complete.
//   - Request.Tools / Request.ToolChoice: not supported — codex exec's own
//     agentic tool surface cannot be replaced or extended with
//     caller-declared tools over a text-pipe shell-out. Typed
//     InvalidRequestError.
//   - Request.Thinking, Request.Temperature, Request.StopSequences: no CLI
//     flag exists for any of these (Effort is the one reasoning-depth knob
//     that IS real — see above). Each returns a typed InvalidRequestError.
//   - Request.ProviderParams: validated for JSON-object shape then typed
//     InvalidRequestError'd — there is no outbound wire body for a CLI
//     shell-out to merge into, identical to claudecli's rationale.
//   - Request.SystemCacheHint / ContentBlock.CacheHint: silently ignored.
//     codex manages its own prompt caching internally (visible as
//     cached_input_tokens in the usage event) with no per-block control
//     surface exposed.
//   - Request.Messages: supported only when every block is BlockText,
//     flattened to a plain "role: text" transcript on stdin — identical
//     rationale and behavior to claudecli.
//   - Request.System: real support (see above), but note the token-floor
//     interaction — a caller supplying a very long System prompt forgoes
//     the ~3,000-token savings this adapter's short defaultInstructions
//     placeholder achieves when System is empty. That is the honest wire
//     tradeoff of setting System, not a bug.
//
// # Cost accounting
//
// Unlike claudecli's envelope (which carries an Anthropic API-equivalent
// total_cost_usd), the codex exec --json event stream carries NO cost
// field of any kind — only raw token counts (see "Usage mapping" below).
// This adapter surfaces zero dollar figure; callers wanting cost
// visibility apply their own rate tables via
// foundations/llmclient/instrument.WithCost over Response.Usage, same as
// every other adapter, keeping in mind the real marginal cost of a call
// riding a ChatGPT Plus/Pro subscription is $0 against metered API
// pricing.
//
// The decision-relevant framing of the measured floor: even fully
// optimized, every call through this adapter carries roughly 10k input
// tokens of CLI overhead REGARDLESS of prompt size — a batch consumer like
// a 31-section obligations compile pays on the order of 300k input tokens
// of overhead against the subscription's plan limits per run (though in
// steady state roughly 5.9k of the 9.8k are served from the provider's
// prompt cache at a discounted rate — the stable prefix the pinned flags
// buy is what makes that cache hit reliable). Size batch workloads against
// plan limits with that overhead in mind.
//
// # Usage mapping
//
// turn.completed.usage.cached_input_tokens -> Usage.CacheReadInputTokens
// (a cache-HIT discount on input, read-only from this adapter's vantage —
// there is no cache_creation-equivalent counter in the event schema, so
// Usage.CacheCreationInputTokens is always left at zero, never
// synthesized). turn.completed.usage.reasoning_output_tokens is folded
// INTO Usage.OutputTokens (output_tokens + reasoning_output_tokens) rather
// than dropped or exposed as a third field the shared llmclient.Usage type
// has no room for: reasoning tokens are a real, separately-billed
// component of a GPT-5-class reasoning turn, and a caller applying a
// per-output-token rate table needs them included for an honest total. See
// resultEventUsage.toUsage's doc comment for the full reasoning.
//
// # Scratch directory lifecycle
//
// New creates (or, given Config.ScratchDir, validates) ONE scratch
// directory for the Client's entire lifetime and reuses it as -C's target
// on every call — cheaper than a fresh mkdir/rmdir per call, and the
// directory stays empty in normal operation (every call runs with -s
// read-only and --ephemeral, so codex itself should never write into it).
// It is never removed: Client, like claudecli's Client, has no Close
// method — the OS's normal temp-directory lifecycle applies, exactly as
// for any other os.MkdirTemp caller that does not explicitly clean up
// (e.g. long-lived daemon processes commonly rely on OS-level tmp
// reaping). Per-call instructions and OutputSchema temp files ARE written
// fresh (with unique names, safe for concurrent calls) and removed within
// each Complete call — see writeScratchFile / CX-5.
//
// # Scope note
//
// This adapter is for personal/dev tooling riding a local ChatGPT login —
// it deliberately never touches ~/.codex/auth.json or any keychain; it
// only ever invokes the `codex` binary as a subprocess and lets the CLI
// resolve its own credentials via CODEX_HOME (verified: --ignore-user-config's
// help text explicitly states "auth still uses CODEX_HOME").
//
// # Composition
//
// Subprocess lifetime, output-size bounding, and process-tree cleanup on
// timeout/cancellation are delegated to the kit's own
// foundations/agent/bounded-exec runner rather than a second hand-rolled
// exec.CommandContext — see Client's doc comment for why its environment
// deliberately passes os.Environ() through verbatim (the opposite of
// bounded-exec's typical scrubbed-allowlist caller), exactly like
// claudecli.
//
// # Concurrency
//
// Client is immutable after New and safe for concurrent use: every
// Complete (and Stream) call spawns its own subprocess, writes its own
// uniquely-named temp files under the shared scratch directory, and shares
// no other mutable state between concurrent calls.
package codexcli
