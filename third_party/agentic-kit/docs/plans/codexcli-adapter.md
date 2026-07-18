# codexcli adapter — ChatGPT-subscription lane via the OpenAI Codex CLI

Mirror of `foundations/llmclient/claudecli`, for the OpenAI side: shell out to
the local `codex` CLI so completions ride a ChatGPT Plus/Pro plan's OAuth
login instead of a metered `OPENAI_API_KEY`. Named consumer: fathom's compile
path (`codex-cli` branch of `FATHOM_LLM_PROVIDER`, fathom-side
docs/plans/provider-matrix.md); owner states fathom will serve Codex-based
customers and ordered this polished, 2026-07-16.

Ground truth pinned live on this machine 2026-07-16 (codex-cli 0.144.5,
installed via `brew install codex`, `codex login status` → "Logged in using
ChatGPT", one real headless turn completed on that session):

- Invocation: `codex exec` reads the prompt from **stdin** when no positional
  arg (NEVER pass the prompt as argv — argv + piped stdin appends stdin as a
  `<stdin>` block, corrupting the prompt). Flags verified in `--help`:
  `--json` (JSONL events), `-m/--model`, `--output-schema <FILE>` (JSON
  Schema for the final response), `-o/--output-last-message <FILE>`,
  `-s read-only` sandbox, `--ephemeral` (no session persistence),
  `--ignore-user-config` (auth still honored), `--skip-git-repo-check`,
  `-C <dir>` working root, `-c key=value` TOML config overrides.
- Event shapes observed live: `thread.started`, `turn.started`,
  `item.completed` with `item.type=="agent_message"` carrying `.text`,
  `turn.completed` carrying
  `usage{input_tokens,cached_input_tokens,output_tokens,reasoning_output_tokens}`.
  No stop_reason field exists.
- **Ambient-context hazard (must fix, this is the polish)**: a trivial
  prompt from a repo cwd consumed 13,443 input tokens — repo AGENTS.md /
  environment context is injected even with `--ephemeral`. The adapter must
  run with `-C` pointed at an empty scratch dir + `--ignore-user-config` +
  `--skip-git-repo-check` (+ whatever else probing shows strips ambient
  blocks) and the package doc records the measured minimal input-token
  floor and which flag removes what. A pure completion adapter that silently
  quadruples every caller's input spend is not shippable.

Design constraints:
- Same package shape and rigor as claudecli: `Config{Bin, Model, Timeout}`
  (+ scratch-dir override), `New` fails fast on unresolvable Bin, `Complete`
  + `Stream` (Stream delegates to Complete exactly as claudecli does — one
  TextDelta then terminal chunk), composed over the bounded-exec runner,
  hermetic fake-shim tests for everything (argv, stdin delivery, JSONL
  reduction, error paths, ctx-cancel kills the process tree). No live codex
  calls in tests.
- JSONL reducer, not envelope parser: concatenate `agent_message` item
  texts (assert single-message case AND multi-item case), map
  `turn.completed.usage` → `llmclient.Usage`
  (`cached_input_tokens`→`CacheReadInputTokens`;
  `reasoning_output_tokens` counted into output — document the choice),
  `turn.completed` → `StopEndTurn`, `turn.failed`/`error` events → typed
  errors (probe the real failure shapes: auth-expired, model-not-found,
  usage-limit; map to AuthError/InvalidRequestError/RateLimitError by
  content, plain error otherwise, and pin whatever you observe as fixtures).
- `Request.OutputSchema` → write schema to a temp file, pass
  `--output-schema`, always remove the file; verify enforcement live once
  while developing, then pin the behavior in shim fixtures.
- `Request.System`: codex has no system-prompt flag. Probe `-c` config keys
  for a supported instructions override; if none exists in 0.144.5, prepend
  under a clearly-delimited header block and DOCUMENT the degradation on the
  Config type (claudecli gets a real flag; codexcli does not — say so).
- `Request.Effort`: probe `-c model_reasoning_effort=...` live; map if real,
  else reject typed.
- Fail loud (typed InvalidRequestError) on: `Tools`, `Thinking`,
  `Temperature`, `StopSequences`, `ProviderParams`, non-text message blocks
  — same doctrine as claudecli.
- Auth: rides `codex login` (ChatGPT OAuth) or the CLI's own env-key
  support; the adapter NEVER reads token files or the keychain. Subscription
  usage-limit caveat in the package doc, mirroring claudecli's.

## Tasks

- [x] Package skeleton: Config/New/bin resolution/scratch workdir, bounded-exec composition {id: CX-1, symbol: New, package: foundations/llmclient/codexcli}
- [x] JSONL event reducer: agent_message accumulation, usage mapping, turn.completed→StopEndTurn, turn.failed→typed errors {id: CX-2, symbol: reduceEvents, package: foundations/llmclient/codexcli, skip_test: true, skip: "reached only through Client.Complete/do, whose method names are defined by every adapter — the documented O9 ambiguous-method-edge residue severs the static test path one hop up; behavior is pinned by this package's direct shim/fixture tests"}
- [x] Argv/stdin contract: exec verb, --json --ephemeral --ignore-user-config --skip-git-repo-check -s read-only -C <scratch>, prompt via stdin only, conditional -m/--output-schema/effort mapping {id: CX-3, symbol: buildArgs, package: foundations/llmclient/codexcli, skip_test: true, skip: "reached only through Client.Complete/do, whose method names are defined by every adapter — the documented O9 ambiguous-method-edge residue severs the static test path one hop up; behavior is pinned by this package's direct shim/fixture tests"}
- [x] Ambient-context suppression proven: measured input-token floor recorded in the package doc, flag-by-flag attribution, regression-pinned via argv assertions {id: CX-4, symbol: buildArgs, package: foundations/llmclient/codexcli, skip_test: true, skip: "the measurement is a live probe recorded as doc + argv pins; the argv itself is asserted by CX-3's shim tests"}
- [x] OutputSchema temp-file plumbing with guaranteed cleanup {id: CX-5, symbol: writeScratchFile, package: foundations/llmclient/codexcli, skip_test: true, skip: "reached only through Client.Complete/do, whose method names are defined by every adapter — the documented O9 ambiguous-method-edge residue severs the static test path one hop up; behavior is pinned by this package's direct shim/fixture tests"}
  (contract repair: the spec anchor was `writeSchemaFile`; implementation generalized the helper to `writeScratchFile` because the live-probed System mapping — `-c model_instructions_file=<file>` — needs the identical write-temp-file-with-guaranteed-cleanup mechanics for the instructions file on every call, so one audited helper serves both)
- [x] Unsupported-field rejection + System/Effort degradation per live probe results, documented on Config {id: CX-6, symbol: validateRequest, package: foundations/llmclient/codexcli, skip_test: true, skip: "reached only through Client.Complete/do, whose method names are defined by every adapter — the documented O9 ambiguous-method-edge residue severs the static test path one hop up; behavior is pinned by this package's direct shim/fixture tests"}
- [x] Stream delegates to Complete per the llmclient streaming contract {id: CX-7, symbol: Stream, package: foundations/llmclient/codexcli, skip_test: true, skip: "method-name Stream is defined by every adapter; ambiguous method-call edges are the documented O9 residue — pinned by direct shim tests in this package"}
- [x] Ping liveness tripwire for the undocumented-key dependence: the pinned argv with a deliberately EMPTY model_instructions_file must hard-error locally pre-API (probe 13: exit 1, sub-second, zero JSONL events, before stdin is read) — that error is HEALTHY (nil); any other outcome (clean exit, unrelated failure, hang killed by the ping bound) returns a typed InvalidRequestError naming the silent-degradation risk; hermetic shim fixtures pin both shapes, one live probe confirmed the healthy path end-to-end {id: CX-10, symbol: Ping, package: foundations/llmclient/codexcli, skip_test: true, skip: "method-name Ping is defined by other adapters too (anthropic, bedrock, openai via the Pinger interface); ambiguous method-call edges are the documented O9 residue — pinned by direct shim tests in this package"}

Verification note: this slice was built in a git worktree the fleet index
does not see — `fathom verify` for these rows runs at merge time
(coordinator: merge, ingest, then verify), not from the worktree.

## Deferred

- [-] True incremental streaming from the JSONL event feed {id: CX-8, disposition: deferred, reason: "claudecli set the delegate-to-Complete precedent; no consumer needs sub-turn deltas from a compile path", trigger: "first interactive consumer that renders partial output"}
- [-] codex `resume`/session reuse for multi-turn {id: CX-9, disposition: deferred, reason: "llmclient is stateless per-request by contract", trigger: "a consumer with a conversational workload names it"}
