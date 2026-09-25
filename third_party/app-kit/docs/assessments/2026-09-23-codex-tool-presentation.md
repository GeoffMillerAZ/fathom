# Bounded Codex tool-presentation comparison

## Prospective contract

R4 continuation from the [current-source host trial](2026-09-23-current-mcp-hosts.md),
under the existing owner-funded adoption work, ADR-0011/0020/0072 and I8/I13/I15/I18.
Both default Codex runs stopped with reported local response-processing errors
despite the required fields existing in retained MCP results. The host processing
code is not in those traces; root cause remains unknown. Do not alter the server
or normalize its valid results merely to fit that narration.

The installed Codex0.155.1 CLI lists `code_mode_host` as a stable feature enabled
by default. Read-only `codex --disable code_mode_host features list` shows it false.
The CLI supports invocation-local `--disable`; no global configuration is written.
Hypothesis: disabling this tool-presentation feature may avoid the processing
failure. A single passing comparison would establish a working tested invocation,
not causation or reliability. Native/direct presentation semantics remain subject
to host behavior, not an assumption from the flag's name.

Reuse the exact `c7e6d44` candidate, source docs, synthetic index, SDK oracle and
explicit-field-map prompt from packet `/Volumes/m2ext/appkit-evidence/appkit-host-current.Uh3VanNH`.
Keep Sol/low, read-only sandbox, disabled shell/apps/plugins/hooks, seven enabled
MCP tools, `env -i` for the MCP child and every nine-call acceptance condition.
Only add `--disable code_mode_host` in a separately frozen launcher/run directory.
The post-acquisition verifier corrections from the prior work remain fixed: they
recognize actual host refusal encoding, reject incomplete calls and unknown tools,
and still reject both failed default traces. No gate or baseline is weakened.

Budget: **one** Codex acquisition, maximum 180 seconds and 2 MiB per stream. No prompt
change, retry, alternative model or policy bypass if it fails. SDK/trace replay
is offline and may be used to interpret the retained result. Verify invocation
argument delta, prompt hash, exact result bodies, refusal/recovery, source retrieval
and owned-process cleanup. Stop after this comparison; installation, global feature
changes, consumer setup, automatic instruction loading, general reliability,
UI certification and G1/G4 acceptance remain outside this proof.

## Result

**Failed: 0/9 calls.** Codex0.155.1 emits an explicit prerequisite error before
the first MCP call: `Code Mode is unavailable because code-mode host is disabled`.
Its diagnostic says the runtime will fail closed and requires `features.code_mode_host`.
The model stops without substitutions. The process nevertheless exits0 naturally
after20.268s, with complete output drain, no forced cleanup and unchanged captured
input/host hashes. No MCP calls appear in the retained trace.

Disabling this feature **does not provide a working direct-MCP fallback** in the
tested runtime. Leave it enabled; do not recommend the flag as an adoption fix.
This rules out the tested workaround, not all alternative presentation mechanisms,
and does not establish why the earlier enabled-host trials mishandled responses.
The original four-call/eight-call failures and Claude's nine-call pass are unchanged.
No further model acquisition, install, update or global configuration change was made.

The same verifier correctly rejected the trace at its unknown-item boundary.
A subsequent synthetic canary now recognizes this explicit host error and reports
its diagnostic instead; it remains a failure even with a successful terminal event.
Offline replay uses no model and must still accept Claude and reject all three
incomplete/error Codex traces. The ordinary runner defaults are unchanged. Its
explicit diagnostic-only presentation parameter preserves this failed experiment,
not a supported configuration recommendation.

Verification: all 21 offline verifier tests pass. Replaying the four retained
traces accepts Claude and rejects all three Codex failures; both invalid
presentation-option cases refuse before filesystem or model work. The runner
passes Node syntax checking, and `go test -mod=readonly ./cmd/appkit-mcp` passes.
These checks do not turn the failed acquisition into a host-adoption pass.
A review canary also found that a top-level host-error event could be ignored in
an otherwise complete synthetic trace. The verifier now refuses both top-level
and item-shaped errors. The failed canary and its exact executed files remain in
the packet's `review-errors.WfWJVA0o/`; this is a synthetic verifier repair, not
another model acquisition or a diagnosis of the original host-processing failure.

## Identity and continuation

The private packet's `codex-presentation/` retains prompt, invocation, raw streams,
input manifest, lifecycle and failure. `fixtures/host-presentation/` preserves
the executed runner/verifier; `presentation-contract.md` is the frozen contract.
Prompt and MCP inputs are unchanged from `codex-field-map/`; the only command-line
delta is `--disable code_mode_host`. The named host/model/approval/sandbox/allowlist
controls remain otherwise identical. One sample cannot establish latency benefit,
model reliability, UI quality, real adoption or completion of G1/G4.

| Artifact | SHA-256 |
|---|---|
| Frozen runner | `36d188a9cdc9ba280eb96452cea8cd850c9193f75404d2c641e89ebfdcd866cf` |
| Frozen verifier | `29d37fdaef10ff2c854c2a21fdcc9754ad9533e9cde7a622ca534cc34b150747` |
| `codex-presentation/stdout.jsonl` | `256b01a9c3068db5f03ddf17265b421e57072976fe17a917b6aead62f2a5617c` |
| `codex-presentation/stderr.log` | `79df216930e8ce103c2943b78596e37ee516e1476ccbaa7a1dd32ae250ca8a9f` |

Do not resume by toggling undocumented feature combinations or repeating prompts.
The next host-specific investigation needs observable processing/return-value
semantics with code-mode host enabled, or a separately reviewed host release with
a relevant fix. Existing pinned local documentation remains the explicit adoption
fallback; no server or user configuration is automatically changed.
