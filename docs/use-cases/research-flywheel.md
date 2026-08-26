# Research Flywheel: Proving (or Disproving) Fathom's Own Impact

Use fathom to aggregate its own local instrumentation into one honest report
on demand, and to produce a sanitized, review-first bundle you can choose to
share — never automatically, never over the network.

---

## When to use this

- You want to know, over a real window (a week, a sprint), whether fathom's
  forced channels (the edit-hook obligations injection) are actually firing
  in this repo, not just wired up.
- You are about to write a claim like "fathom caught N bugs" or "fathom
  saved us tokens" in a PR description, standup update, or a
  `docs/plans/*.md` status line, and want the number backed by a committed
  artifact instead of memory.
- Someone outside your local machine wants evidence that fathom is working —
  and you want to hand them something sanitized, not raw logs that might
  carry repo names, file paths, or free-text friction notes.
- You want a periodic gut-check on the flywheel itself: is the gate catching
  anything, are observations getting filed and closed, is a bench result on
  file to back the tokenomics story.

This is a **CLI-only** feature — no MCP tool exists for it (see
`docs/design/research-flywheel.md` §8 for why). Run it from a terminal or a
CI-adjacent script, not from an agent session's tool calls.

---

## Tools

| Command | Kind | What it gives you |
|---|---|---|
| `fathom research report` | CLI | A deterministic, honest aggregate of local evidence over a `--since` window: call-log traffic, injection coverage, verify-gate catches, staleness signal, observation counts, bench-artifact token/turn deltas, and (phase P, `docs/design/production-tokenomics.md`) a payload **cost** ledger ("what fathom SPENT"), a discovery-share estimate, and a service-identity cache hit rate. Every metric it cannot compute prints `"not measurable: <reason>"` instead of a fabricated zero. |
| `fathom research bundle` | CLI | The sanitized, shareable twin: aggregate counts, rates, tool-name frequencies, and verdict distributions only — never file paths, repo/service names, symbol names, source code, or free text. Service identities appear only as salted hashes. `--dry-run` previews the exact bundle without writing or transmitting anything. |
| `fathom observe` | CLI | File one structured friction observation (`--symptom` required; `--tool`, `--expected`, `--actual`, `--repro`) into `.fathom/observations.md` — the input side of the flywheel; the report counts them |
| `fathom instrument --adoption` | CLI | The flywheel gauge: injection coverage vs the ≥90% gate, read-error classification, surface-scoped zero-call tool list, open observations |
| `fathom flywheel` | CLI | Each feedback loop reported CLOSED/OPEN/IDLE with its evidence and the remedy (`--json`) |
| `fathom rate` | CLI | Rate this session 1–5 with a note: the OUTCOME half of the A/B criterion (fewer tokens is not a win if less got done) — an optional human override, **experimental** |

> Both commands are local-file-only: they read `workspace/logs/mcp-calls.jsonl`,
> `~/.fathom/hooks.jsonl`, `.fathom/observations.md`, and (if present)
> `workspace/artifacts/context-bench/results-*.json`. Neither ever queries
> the live daemon or opens a store — see the design doc §3 for why that's a
> feature, not a limitation.

---

## How it enhances Claude Code

Without this, "is fathom helping" is answered by memory, or by manually
running `instrument --adoption` and separately reading `.fathom/observations.md`
and eyeballing a bench artifact, if you remember one exists. `fathom
research report` does the same aggregation once, deterministically, and adds
signals `instrument --adoption` doesn't cover on its own — the hooks beat
log (injection coverage, obligations served) and bench-artifact token/turn
deltas. Phase P (`docs/design/production-tokenomics.md`) closed the one gap
this doc used to describe as structurally unmeasurable: a counting-only
PreToolUse observer now DOES see an agent's own `Grep`/`Glob`/`Read` calls
and grep-like `Bash` invocations (never their content, one coarse-kind beat
per call), giving `discovery_share_percent` a real denominator for the
first time. It is reported CORRELATIONAL, not causal — see "Production
tokenomics" below.

`fathom research bundle` turns "prove it's working" from a screenshot of
raw logs (which might leak a repo name or a symptom's free text) into a
reviewable file you can actually hand to someone else with confidence about
what's in it.

---

## Worked example

**Scenario:** a week after wiring fathom into a repo, you want to know
whether it's actually doing anything, and you want to share a sanitized
summary with a teammate.

**Step 1 — Collect.**

```sh
fathom research report --service my-service --since 168h
```

```
fathom research report
window: 2026-07-21T00:00:00Z -> 2026-07-28T00:00:00Z (generated 2026-07-28T00:00:00Z)
service: my-service

call log: workspace/logs/mcp-calls.jsonl (214 calls in window)

surface traffic:
  agent        calls=180
  hook         calls=34

top tools:
  find_symbol              62
  verify_plan              21
  ...

injection coverage (obligations, per-user):
  seen=40 answered=38 obligations_served=35 rate=95%
  note: per-user, all repos this user's hooks fired in — ...

verify gate:
  claimed=21 verified=18 failing=3 review_active_findings=0
  21 verify_plan/review call(s) instrumented this window

...

context-bench:
  no context-bench results artifact found under the bench directory
```

Two files land under `workspace/artifacts/research/`: `report.md` (readable)
and `report.json` (the same data, machine-readable). Both are gitignored —
this is local evidence, not something committed.

**Step 2 — Decide what to say.** The report shows a real 95% injection
coverage and 3 gate catches this week — citable. It also shows no
context-bench artifact — so a tokenomics claim ("fathom saves N% tokens")
is **not** supported by this repo's own evidence yet, and shouldn't be made
here even if it's true elsewhere (`fathom-impact` skill's rule 1: report only
what was measured).

**Step 3 — Share (optional).**

```sh
fathom research bundle --service my-service --since 168h --dry-run
```

Prints the exact JSON that would be written — review it. Once satisfied:

```sh
fathom research bundle --service my-service --since 168h
```

```
wrote workspace/artifacts/research/bundle/bundle.json
wrote workspace/artifacts/research/bundle/bundle.md

These are plain local files. Nothing was transmitted anywhere — fathom
never phones home. Review them yourself before sharing; sharing is
entirely your own manual choice (email, paste, upload — whatever you pick).

Share only these two files (or this bundle/ subdirectory in isolation) —
never the parent workspace/artifacts/research/ directory, which also
contains report.md (that file is NOT sanitized: it carries plaintext
service names and the local hooks-log path).
```

`bundle.json` carries the same injection-coverage rate and gate-catch counts
as the report — under a `call_traffic`/`injection`/`verify_gate` grouping
that's absent entirely when its underlying log was never read (never a
fabricated zero) — plus fathom's own version/commit and coarse platform. But
`my-service` never appears in it; only a salted hash of it does, and every
tool/surface/bench-arm name in it is checked against fathom's own
`^[a-z0-9_]{1,64}$` label shape (anything else lands in an `"unrecognized"`
bucket instead). You choose how (or whether) to send the two files in
`workspace/artifacts/research/bundle/` onward — but never the parent
directory.

---

## Production tokenomics (phase P)

`docs/design/production-tokenomics.md` closed the gap between "fathom is
firing" (the coverage numbers above) and "what did that actually cost, and
is it being chosen over the alternative" — three new report sections, plus
a per-session ledger and a kill switch for the one causal instrument that
answers the second question.

- **"what fathom SPENT"** — a **COST** ledger, never a savings ledger (law
  12: no saving is claimed without a counterfactual). Every injected
  payload (session brief, post-edit, pre-edit guard, module-tagged rules)
  records its rendered bytes and an explicit token **estimate**
  (`bytes/4`, always labeled "estimate" — never "measured"), summed
  per-channel plus a per-edit p50/p95 in the report.
- **Discovery share** — `fathom research report` now states, from real
  local evidence, fathom's own index-tool calls against a counting-only
  PreToolUse observer's `grep`/`glob`/`read`/`shell-search` beats. `read`
  is always its own bucket (dual-use — reading a file you already located
  isn't necessarily discovery) and the whole section is labeled
  **correlational**: a rising share is not proof of substitution until the
  pre-registered A/B protocol (design §5) runs.
- **Service-identity cache hit rate** — a **latency/load** signal
  (`identcache.go`), never a token figure; reported separately so it can
  never be summed into a cost total by mistake.
- **Session ledger** — the Stop hook now appends one line per session to
  `~/.fathom/session-ledger.jsonl` (override: `$FATHOM_SESSION_LEDGER`):
  session id, repo, duration/turn-count/token-totals **if the transcript
  exposes them** (an honest `unavailable` note otherwise — never a
  fabricated zero), plus that session's payload-cost and discovery-action
  totals. `session_id` is the join key across all three instruments.
- **`FATHOM_CHANNELS=off`** — one env var that silences every fathom hook
  (session brief, both edit-hook phases, the discovery observer, the Stop
  hook) completely: the OFF arm of the pre-registered A/B. `fathom doctor`
  reports loudly (`channels_env`, `warn`) whenever it finds this set — a
  silent kill switch would repeat the exact O15 failure class.

None of this licenses a production savings claim on its own — that is
what the pre-registered A/B protocol (design §5) exists to test, not this
report. **Status (experimental):** the counterfactual is one-armed today —
the ON arm is collecting and each arm needs ≥20 admissible units before a
comparison is admissible (law 12), so every savings claim stays
inadmissible until then; `fathom rate` supplies the optional human outcome
score per session, and `fathom flywheel` names which loops are CLOSED,
OPEN or IDLE with the evidence.

---

## Setup & notes

**Windowing:** `--since` is a Go duration (e.g. `168h` for 7 days, `720h`
for 30) — matches `fathom instrument`'s existing `--window` flag
convention. There is no calendar-day/week shorthand.

**Per-user vs per-repo:** injection coverage (`injection_seen`,
`injection_answered`, `obligations_served`) reflects **every repo this
user's hooks have fired in**, not just the one you ran the command from —
the underlying hooks beat log (`~/.fathom/hooks.jsonl`) carries no
repo/service field today. Every report says this explicitly next to the
number. Everything else (call-log-derived: surface traffic, verify gate,
staleness, latency) *is* scoped by `--service` when given.

**Observation counts are not time-windowed.** `.fathom/observations.md`
entries carry no timestamp field, so `total_observations`/
`open_observations` always cover the file's entire history, regardless of
`--since`. The report says this explicitly too.

**Bench artifacts:** if `scripts/context-bench.sh` has never been run in
this repo, the report and bundle both say so honestly rather than omitting
the section or printing a zero. When a results file exists, the most recent
one is used, with its own `generated_at` surfaced so you can judge
staleness against your report window yourself.

**The salt:** the first `fathom research bundle` run in a repo generates
`workspace/state/research.salt` (gitignored, `0o600`, **outside** the
shareable `workspace/artifacts/research/` tree on purpose) and reuses it on
every later run — this is what keeps a given service's hash stable across
bundles (so repeated counts of the same service are recognizable as the
same service without naming it). Deleting the salt file just starts a new,
uncorrelated hash lineage; it does not leak anything. An existing repo's
salt from before this location changed is migrated automatically (value
preserved, old file removed) the next time you run `fathom research bundle`.

**Sharing discipline:** share the two files under
`workspace/artifacts/research/bundle/` (or that subdirectory as a whole) —
**never** the parent `workspace/artifacts/research/` directory. The parent
also holds `report.md`, which is deliberately unsanitized (plaintext service
names, the local hooks-log path). The CLI repeats this warning after every
non-`--dry-run` write.

**No MCP tool, no scheduling:** run these from a terminal, by hand or from
your own script/cron if you want a recurring snapshot — fathom itself never
schedules or transmits either command's output.

---

## Maintained

This doc covers: `fathom research report`, `fathom research bundle`,
`fathom observe` (and `.fathom/observations.md`), `fathom instrument
--adoption`, `fathom flywheel`, `fathom rate`. Update it when any command's
flags, output shape, or sanitization guarantees change — see also `docs/design/research-flywheel.md` for the full design
rationale and `internal/research/bundle`'s doc comments for the allowlist
implementation.
