# Live probe log — codexcli adapter (2026-07-16)

Every probe below ran against the real, installed CLI on this machine:
`codex-cli 0.144.5` (`brew install codex`), `codex login status` ->
"Logged in using ChatGPT". No probe touched a real API-key-metered surface;
all calls ride a ChatGPT Plus/Pro subscription. Every prompt is a few words
in/out, per the probe budget. `$SCRATCH` below is an empty, non-git
directory created fresh for these probes; `$EMPTY` = `$SCRATCH/codex-probe-empty`.

This log is the audit trail for every claim pinned in `../doc.go`. Where a
probe's raw JSONL is reproduced, it is copied verbatim from the actual run.

## Preliminary: help text + config-key discovery (no LLM cost)

```
$ codex --version
codex-cli 0.144.5

$ codex login status
Logged in using ChatGPT

$ codex exec --help
```

`codex exec --help` confirmed the flags the design brief's ground truth
named (`--json`, `-m/--model`, `--output-schema <FILE>`,
`-o/--output-last-message`, `-s read-only`, `--ephemeral`,
`--ignore-user-config`, `--skip-git-repo-check`, `-C/--cd <DIR>`,
`-c/--config <key=value>`), and that the prompt argument doc string reads:
"If not provided as an argument (or if `-` is used), instructions are read
from stdin. If stdin is piped and a prompt is also provided, stdin is
appended as a `<stdin>` block" — confirming the plan's "never pass the
prompt as argv" rule from the CLI's own docs, not just the design brief.

`codex features list -c totally_bogus_config_key_zzz=1` (no LLM cost) exits
0 with the normal feature table and no warning — an UNRECOGNISED `-c
key=value` is silently tolerated.

`codex features list --disable bogus_feature_zzz` (no LLM cost) ->
`Error: Unknown feature flag: bogus_feature_zzz`, nonzero exit — an
UNRECOGNISED `--disable <feature>` is a HARD local error. This asymmetry is
why the production argv uses `-c key=value` overrides freely but excludes
`--disable` entirely — see doc.go's deviation note.

`strings -a <codex binary>` located several ConfigToml field names not
documented in `--help` (discovered near `model_reasoning_effort`, which
IS documented indirectly via this operator's own `~/.codex/config.toml`
already setting `model_reasoning_effort = "medium"`):
`model_instructions_file`, `experimental_compact_prompt_file`,
`include_permissions_instructions`, `include_apps_instructions`,
`include_collaboration_mode_instructions`, `include_environment_context`.
These four `include_*` keys plus `model_instructions_file` are the config
keys probed below.

## Ambient-context suppression (CX-4) — flag-by-flag attribution

All commands below deliver the prompt via stdin (`printf 'reply pong' |
codex exec --json ...`, no positional prompt arg) unless noted as the
discovery run that used a positional arg by mistake.

**Probe 1 (discovery — positional arg, wrong method):**
```
$ codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only "reply pong"
```
stderr: `Reading additional input from stdin...` (the CLI treated the
Bash tool's non-TTY stdin as "piped" and appended an empty `<stdin>`
block — exactly the hazard `--help` warns about). Result:
`{"type":"turn.completed","usage":{"input_tokens":14015,"cached_input_tokens":5504,"output_tokens":20,"reasoning_output_tokens":13}}`.
This run is why every subsequent probe switched to stdin-only delivery.

**Probe 1b (baseline — stdin delivery, scratch dir, no user-config/ephemeral flags):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only
```
stderr: `Reading prompt from stdin...` (confirms correct delivery mode).
```
{"type":"thread.started","thread_id":"019f6e83-8222-7962-ac4e-ade09c2c8b0f"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pong"}}
{"type":"turn.completed","usage":{"input_tokens":13596,"cached_input_tokens":5504,"output_tokens":16,"reasoning_output_tokens":9}}
```
**input_tokens = 13,596** — barely different from the design brief's
repo-cwd baseline of 13,443. Being in an empty scratch dir alone buys
almost nothing.

**Probe 2 (+ `--ignore-user-config`):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config
```
```
{"type":"turn.completed","usage":{"input_tokens":14277,"cached_input_tokens":9984,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 14,277** (went UP, not down) but **cached_input_tokens
jumped to 9,984** (from 5,504) — this operator's real config.toml (6
enabled plugins, several MCP servers, per-project trust records) makes the
prompt prefix vary run to run, defeating caching; suppressing it makes the
prefix stable and highly cacheable, which matters far more for a
repeated-caller compile path than the one-shot raw count.

**Probe 3 (+ `--ephemeral`):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral
```
```
{"type":"turn.completed","usage":{"input_tokens":13711,"cached_input_tokens":9984,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 13,711** — negligible further change. `--ephemeral` is a
session-persistence hygiene flag, not a token-floor lever.

**Probe 4 (+ four `include_*=false` config overrides):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c include_environment_context=false -c include_apps_instructions=false \
    -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false
```
```
{"type":"turn.completed","usage":{"input_tokens":13298,"cached_input_tokens":9984,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 13,298** — a small (~3%) cut. At this point it was clear
the dominant cost (~10-13k tokens) is the CLI's built-in agentic-coding
instructions_template (visible via binary strings inspection as a large
embedded prompt describing shell/apply_patch/tool-usage conventions),
which none of the documented suppression flags touch.

**Probe 5 (+ a dozen `--disable <feature>` flags — NOT adopted, see below):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c include_environment_context=false -c include_apps_instructions=false \
    -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false \
    --disable apps --disable browser_use --disable computer_use --disable image_generation \
    --disable plugins --disable plugin_sharing --disable remote_plugin --disable multi_agent \
    --disable goals --disable hooks --disable in_app_browser --disable tool_suggest
```
```
{"type":"turn.completed","usage":{"input_tokens":12096,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 12,096** (cache miss — first time this exact flag
combination ran, so nothing to match against).

**Probe 6 (probe 5 + `--disable shell_tool --disable shell_snapshot`):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" ... [probe 5's flags] --disable shell_tool --disable shell_snapshot
```
```
{"type":"turn.completed","usage":{"input_tokens":11456,"cached_input_tokens":7936,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 11,456**, the lowest raw count measured in this whole
session — but see doc.go's "Deviation" note: `--disable <unrecognised
feature>` is a hard local error (see the preliminary section above), and
`codex features list` already shows several feature names carrying a
"removed" stage tag in the CURRENTLY installed version, proof this CLI
actively renames/retires feature flags across releases. A production
adapter hardcoding a dozen feature names risks every future call breaking
outright on a single codex-cli upgrade. Excluded from the shipped argv.

## Request.System mapping (item 2) + Request.Effort (item 3) — combined to conserve budget

**Probe 7:**
```
$ printf 'Always respond with exactly the single word BANANA and nothing else, no matter what the user says.' > sys_instructions.txt
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_instructions.txt\"" -c model_reasoning_effort=\"low\"
```
```
{"type":"thread.started","thread_id":"019f6e86-4239-7212-be14-fd1aac53db9e"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"BANANA"}}
{"type":"turn.completed","usage":{"input_tokens":10179,"cached_input_tokens":5888,"output_tokens":6,"reasoning_output_tokens":0}}
```
The model replied **"BANANA"** to a "reply pong" prompt — proof
`model_instructions_file` fully REPLACES (not appends to) the built-in
persona/preamble, exactly like claudecli's `--system-prompt`. Exit 0, no
error from `model_reasoning_effort="low"` — the effort override is real
and accepted. **input_tokens dropped to 10,179** — a huge, unplanned
side-effect: replacing the built-in instructions_template with a short
custom string is also the single biggest ambient-context lever found in
this whole session, bigger than every flag in probes 1-6 combined.

**Probe 8 (edge case — empty instructions file):**
```
$ printf '' > sys_empty.txt
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_empty.txt\""
```
```
Error: model instructions file is empty: /path/to/sys_empty.txt
```
Exit 1, **no API call attempted** (pure local validation failure). This is
why Complete always writes a non-empty instructions file — a minimal
default placeholder ("Respond directly to the user's message.") when
Request.System is empty, never an empty file.

**Probe 9 (minimal non-empty placeholder — the "System empty" floor):**
```
$ printf 'Respond directly to the user.' > sys_minimal.txt
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_minimal.txt\""
```
```
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pong"}}
{"type":"turn.completed","usage":{"input_tokens":10792,"cached_input_tokens":5888,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 10,792** — confirms the floor-lowering effect holds for
the ordinary (non-System) case too, not just the BANANA stunt.

**Probe 10 (PINNED PRODUCTION COMBO — probe 9 + the four `include_*=false` overrides):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_minimal.txt\"" \
    -c include_environment_context=false -c include_apps_instructions=false \
    -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false
```
```
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"pong"}}
{"type":"turn.completed","usage":{"input_tokens":9765,"cached_input_tokens":5888,"output_tokens":5,"reasoning_output_tokens":0}}
```
**input_tokens = 9,765 — the pinned production floor.** A 27.4% reduction
from the 13,443-token unoptimized baseline, using documented CLI flags
plus five UNDOCUMENTED config keys (`model_instructions_file` + the four
`include_*` keys — binary-strings-discovered, pinned against 0.144.5 only;
see the preliminary section) that the `-c` mechanism tolerates silently
when unrecognised (also verified there — which is why drift would be
silent and why the adapter ships a Ping tripwire, probe 13). This exact
combination (with a per-call instructions file, defaulting to a short
placeholder) is what `buildArgs` emits on every Complete call.

## `--output-schema` enforcement (item 4)

**Probe 11:**
```
$ cat > schema.json <<'EOF'
{"type":"object","properties":{"greeting":{"type":"string"}},"required":["greeting"],"additionalProperties":false}
EOF
$ printf 'reply with a greeting field set to hello' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_minimal.txt\"" \
    -c include_environment_context=false -c include_apps_instructions=false \
    -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false \
    --output-schema "$SCRATCH/schema.json"
```
```
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"{\"greeting\":\"hello\"}"}}
{"type":"turn.completed","usage":{"input_tokens":9783,"cached_input_tokens":0,"output_tokens":16,"reasoning_output_tokens":0}}
```
The agent_message text is exactly `{"greeting":"hello"}` — genuine
schema-conforming structured output, delivered through the same
item.completed/agent_message channel as a normal completion (no separate
structured-output field to choose between, unlike claudecli's envelope).

## Failure shape (item 5)

**Probe 12 (bogus `-m` value — cheap, real, local failure):**
```
$ printf 'reply pong' | codex exec --json -C "$EMPTY" --skip-git-repo-check -s read-only --ignore-user-config --ephemeral \
    -c model_instructions_file="\"$SCRATCH/sys_minimal.txt\"" -m "this-model-does-not-exist-xyz"
```
```
{"type":"thread.started","thread_id":"019f6e87-5e96-7fc1-880d-7875dd1a025b"}
{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Model metadata for `this-model-does-not-exist-xyz` not found. Defaulting to fallback metadata; this can degrade performance and cause issues."}}
{"type":"turn.started"}
{"type":"error","message":"{\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'this-model-does-not-exist-xyz' model is not supported when using Codex with a ChatGPT account.\"}}"}
{"type":"turn.failed","error":{"message":"{\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'this-model-does-not-exist-xyz' model is not supported when using Codex with a ChatGPT account.\"}}"}}
```
Exit 1. Three distinct signals for one failure: a soft, non-terminal
`item.completed` warning item (type "error", NOT type "agent_message" —
this adapter ignores it, since a well-formed turn can apparently recover
from it), a standalone top-level `error` event, and the authoritative
terminal `turn.failed` event — the latter two carry an IDENTICAL
nested-JSON string in their `.message` field:
`{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"..."}}`.
`reduceEvents`/`mapEventError` double-decode that nested string and map by
`.status`, mirroring claudecli's `api_error_status` mapping. Only this 400
shape is live-verified; 401/403/429 are mapped speculatively by keyword
content (see doc.go and events.go's classifyByContent doc comments) per
the design brief's explicit instruction not to probe those live.

## Ping tripwire ordering (post-review addition, 2026-07-16)

**Probe 13 (Ping's healthy shape — full pinned argv, EMPTY instructions
file, EMPTY stdin; zero LLM cost, local error only):**
```
$ : > ping_empty_instructions.txt
$ codex exec --json -C "$EMPTY" --skip-git-repo-check --ignore-user-config --ephemeral -s read-only \
    -c model_instructions_file="\"$SCRATCH/ping_empty_instructions.txt\"" \
    -c include_environment_context=false -c include_apps_instructions=false \
    -c include_collaboration_mode_instructions=false -c include_permissions_instructions=false \
    < /dev/null
```
Exit 1, elapsed 0s (sub-second), stdout completely EMPTY (not even
`thread.started` — no session was created), stderr:
```
Error: model instructions file is empty: /path/to/ping_empty_instructions.txt
```
Notably there is no `Reading prompt from stdin...` line, which every
probe above emitted — the empty-file validation fires before stdin is
even read, let alone before any network activity. This pins the ordering
`Ping` depends on: with the exact production argv, an installed CLI that
still honors `model_instructions_file` can never reach an API call during
a Ping. Probe 8 established the same error with a partial flag set;
this probe re-verifies it under the full pinned combination and empty
stdin, which is Ping's exact invocation.

**End-to-end confirmation through the Go method (same zero-cost local
error, exercised via `codexcli.New(Config{}) + Client.Ping` against the
real binary):** `Ping err=<nil> elapsed=316ms` — the healthy path returns
nil through the full adapter plumbing (scratch-file write, bounded-exec
run, stderr-marker match, temp-file cleanup).

The degraded shapes (a future CLI that ignores the key and proceeds,
fails differently, or hangs) cannot be produced by the installed 0.144.5
binary and are pinned by hermetic shim fixtures instead
(`TestPing_Degraded_*` in codexcli_test.go).

## Probe budget note

The brief suggested "maybe 6-10" live calls. This session used 12
API-touching invocations (probe 8's empty-instructions-file check was a
local, pre-API validation error — no LLM cost) plus two free
(`codex features list ...`) local checks. The overrun is deliberate: the
ambient-context attribution table (item 1) is only honest if each flag's
marginal contribution is measured additively rather than asserted, and
discovering `model_instructions_file` live (probe 7) — which turned out to
be both the real System mapping AND the dominant token-floor lever —
justified two further confirmation probes (9, 10) to pin the actual
production floor rather than leaving it as a one-off anecdote. Every call
stayed within the "a few words in/out" budget. The post-review Ping
probes (probe 13 and its Go end-to-end confirmation) are, like probe 8,
zero-cost local validation errors — no API is ever reached, which is the
entire point of the tripwire's design.
