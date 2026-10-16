# Live probe log — claudecli Ping (2026-07-17)

Every probe below ran against the real, installed CLI on this machine:
`claude 2.1.212 (Claude Code)` (`claude auth status` -> `loggedIn: true`,
`authMethod: claude.ai`, `subscriptionType: max`). Every API-touching probe
(only probe E) rides that Claude Max subscription — no metered
`ANTHROPIC_API_KEY` was ever consulted. Probes ran from a scratch
directory outside any repo; probe F additionally re-ran from a freshly
created, empty, non-git directory to rule out CWD-dependent behavior.

This log is the audit trail for the claims pinned in `../ping.go`.

## Preliminary: version, login, help text (no LLM cost)

```
$ claude --version
2.1.212 (Claude Code)

$ claude auth status
{
  "loggedIn": true,
  "authMethod": "claude.ai",
  "apiProvider": "firstParty",
  "email": "<redacted>",
  "orgId": "<redacted>",
  "orgName": "<redacted>",
  "subscriptionType": "<redacted>"
}
```

`claude --help` confirms the installed CLI has NO `--system-prompt-file` (or
any file-based) form of `--system-prompt` — only `--system-prompt <prompt>`,
an inline string. The design brief's candidate "empty/invalid
--system-prompt file" probe therefore does not apply to this CLI version;
the search moved to other degenerate-input candidates below.

## Baseline timing (no LLM cost, for contrast with the probes below)

```
$ time claude --version
2.1.212 (Claude Code)
```
`0.03s user 0.02s system 91% cpu 0.059 total` — pure Node startup + argv
parsing, no CLI-internal work, is ~60ms. Every probe below that returns a
LOCAL error (never touching the network) lands in the 1-3.2s range instead
— slower than a bare `--version` (this CLI does more startup work before
`-p`/`--output-format` specifically, e.g. resolving its own config/auth
state even for a call that will be rejected before sending anything), but
categorically faster and structurally different from the real network round
trip measured in probe E (5.6s wall time with a genuine JSON response
envelope).

## Finding the local pre-API failure signature (item: KP-1)

**Probe A (discovery — `--output-format json` alone, empty stdin, no
prompt argument):**
```
$ claude --output-format json < /dev/null
```
Exit 1, elapsed ~1-2.5s across 3 repeated runs, stdout EMPTY, stderr:
```
Error: Input must be provided either through stdin or as a prompt argument when using --print
```
`--output-format` implies print-mode's input requirement even without an
explicit `-p`. This is a hard, LOCAL, pre-anything error — with `/dev/null`
piped in, stdin yields immediate EOF (zero bytes), so the CLI has no prompt
from either channel and refuses before doing anything else.

**Probe A' (bare `-p`, same empty stdin):**
```
$ claude -p < /dev/null
```
Exit 1, elapsed ~1.06s, identical stderr to probe A. Confirms the signature
is tied to print-mode's input requirement, not specifically
`--output-format`.

**Probe B (PINNED PRODUCTION ARGV SHAPE — `buildArgs(llmclient.Request{})`'s
output, empty stdin):**
```
$ claude -p --output-format json --safe-mode --tools "" < /dev/null
```
Exit 1, elapsed 1.18s, stdout EMPTY, stderr identical to probe A:
```
Error: Input must be provided either through stdin or as a prompt argument when using --print
```
This is the exact argv Ping runs (Client.buildArgs with a zero-valued
Request — no `--model`, `--system-prompt`, `--effort`, `--json-schema`).

**Probe B2 (probe B + `--model sonnet --system-prompt "you are a test"
--effort low` — proves the signature survives every flag a configured
Client might add):**
```
$ claude -p --output-format json --safe-mode --tools "" --model sonnet --system-prompt "you are a test" --effort low < /dev/null
```
Exit 1, elapsed 1.27s, IDENTICAL stderr and empty stdout. (Ping itself never
adds these flags — buildArgs(llmclient.Request{}) omits them all — but this
confirms a Config with a non-empty Model would not change Ping's healthy
signature if a caller reused the same helper for a non-empty Request.)

**Probe C (CAUTIONARY — whitespace-only stdin, NOT empty — why Ping uses
`nil` stdin, not merely blank):**
```
$ printf '   \n' | claude -p --output-format json --safe-mode --tools ""
```
Exit 1, elapsed 0.99s, but a DIFFERENT stderr:
```
Error: No messages returned from query
```
Non-empty (even whitespace-only) stdin is accepted as "input was provided"
and the CLI proceeds PAST the local gate probes A/B trigger — this
different, unrelated failure shape is not proof of anything Ping needs, and
critically is NOT verified to be pre-API (its error text ambiguously
suggests it may have gone further into query setup than probes A/B/D ever
did). This is why `boundedexec.Spec.Stdin` in Ping is left `nil` (immediate
EOF, zero bytes) rather than an empty-looking-but-technically-present
string — only genuinely empty stdin reproduces the pinned healthy
signature.

**Probe D (pinned argv + a deliberately bogus `--model` value, empty
stdin — does the input-presence check win over model validation?):**
```
$ claude -p --output-format json --safe-mode --tools "" --model this-model-does-not-exist-xyz < /dev/null
```
Exit 1, elapsed 1.85s, IDENTICAL stderr to probes A/B/B2:
```
Error: Input must be provided either through stdin or as a prompt argument when using --print
```
Proves ORDERING: the input-presence check runs BEFORE model
resolution/validation — an installed CLI that would otherwise reject an
unknown model name still hits the input gate first, so the gate cannot be
bypassed by anything Ping's zero-valued Request could supply.

**Probe E (REAL completion, for contrast — the only API-touching, token-
costing probe in this session; a few words in/out):**
```
$ printf 'reply with exactly the word pong' | claude -p --output-format json --safe-mode --tools "" --model sonnet
```
Exit 0, elapsed 5.615s wall time (`duration_api_ms: 5642` in the envelope —
essentially the entire wall time was the network round trip), stdout:
```json
{"type":"result","subtype":"success","is_error":false,"api_error_status":null,"duration_ms":4379,"duration_api_ms":5642,"ttft_ms":3969,"ttft_stream_ms":3633,"time_to_request_ms":23,"num_turns":1,"result":"pong","stop_reason":"end_turn","session_id":"0288f79a-98b5-4c31-84e4-0b5ee13986bf","total_cost_usd":0.012482700000000001,"usage":{"input_tokens":2,"output_tokens":4,...}}
```
This is the API-shaped output the healthy Ping path must NEVER produce: a
full result envelope with `total_cost_usd`, a `session_id`, and real
`usage` — a `total_cost_usd` of ~$0.0125 (Anthropic API-equivalent
accounting; $0 marginal spend against the Max subscription plan, per the
package doc's "Cost accounting" section) versus Ping's genuinely zero-token
local rejection.

**Probe F (stability — pinned shape B repeated 3x from a fresh empty
non-git directory, timed with wall-clock start/end rather than the shell's
imprecise `time` builtin):**
```
$ claude -p --output-format json --safe-mode --tools "" < /dev/null
```
| run | exit | elapsed | stderr |
|-----|------|---------|--------|
| 1   | 1    | 1852ms  | `Error: Input must be provided either through stdin or as a prompt argument when using --print` |
| 2   | 1    | 1059ms  | (identical) |
| 3   | 1    | 3234ms  | (identical) |

Stable across every run: exit 1, empty stdout, identical single-line
stderr. The 1.0-3.2s spread is Node cold-start jitter on this machine, not
network variance — `pingTimeout` (15s) gives generous headroom above the
observed worst case.

**Byte-exact confirmation (`xxd` on a captured stderr file):**
```
00000000: 4572 726f 723a 2049 6e70 7574 206d 7573  Error: Input mus
00000010: 7420 6265 2070 726f 7669 6465 6420 6569  t be provided ei
00000020: 7468 6572 2074 6872 6f75 6768 2073 7464  ther through std
00000030: 696e 206f 7220 6173 2061 2070 726f 6d70  in or as a promp
00000040: 7420 6172 6775 6d65 6e74 2077 6865 6e20  t argument when
00000050: 7573 696e 6720 2d2d 7072 696e 740a       using --print.
```
stdout is exactly 0 bytes on every probe A/B/B2/D/F run. `pingHealthyMarker`
in ping.go matches on the substring `"Input must be provided either
through stdin or as a prompt argument"` (dropping the trailing "when using
--print" from the match, which is plausibly the part most likely to drift
across CLI versions) via `bytes.Contains`, not a full-string match.

## End-to-end confirmation through the Go method

`claudecli.New(Config{}) + Client.Ping` against the real binary (Bin
resolved via PATH, no fake shim; run from a throwaway `tmp/pingcheck/main.go`,
removed after this confirmation): `Ping err=<nil> elapsed=1.406313s` — in
the same 1-3s band as probes B/F above — the healthy path returns nil
through the full adapter plumbing (buildArgs, bounded-exec run,
stderr-marker match).

The degraded shapes (a future CLI that accepts empty stdin and proceeds, a
CLI that fails a different LOCAL way, or a CLI that hangs) cannot be
produced by the installed 2.1.212 binary and are pinned by hermetic shim
fixtures instead (`TestPing_Degraded_*` / `TestPing_Healthy_*` in
claudecli_test.go).

## Probe budget note

Six flag/input-shape probes (A, A', B, B2, D, F — F itself 3 repeated runs)
plus one cautionary probe (C) — all zero-cost local errors — and exactly
ONE real, API-touching completion (probe E, "reply pong", a few words
in/out) to establish the contrast a healthy Ping must never produce. No
probe attempted `--json-schema`, `--session-id`, `--mcp-config`, or other
degenerate-flag candidates once probe A's signature was found stable,
reproducible, and ordered ahead of model validation (probe D) — chasing
additional candidates after a verified, stable signature was in hand would
have spent probe budget without changing the design.
