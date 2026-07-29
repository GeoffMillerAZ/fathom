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
| `fathom research report` | CLI | A deterministic, honest aggregate of local evidence over a `--since` window: call-log traffic, injection coverage, verify-gate catches, staleness signal, observation counts, and bench-artifact token/turn deltas when one exists. Every metric it cannot compute prints `"not measurable: <reason>"` instead of a fabricated zero. |
| `fathom research bundle` | CLI | The sanitized, shareable twin: aggregate counts, rates, tool-name frequencies, and verdict distributions only — never file paths, repo/service names, symbol names, source code, or free text. Service identities appear only as salted hashes. `--dry-run` previews the exact bundle without writing or transmitting anything. |

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
deltas — while being explicit about the one thing it structurally *can't*
see: an agent's own grep/read tool calls happening alongside fathom's, which
have no fathom-visible trace.

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
wrote workspace/artifacts/research/bundle.json
wrote workspace/artifacts/research/bundle.md

These are plain local files. Nothing was transmitted anywhere — fathom
never phones home. Review them yourself before sharing; sharing is
entirely your own manual choice (email, paste, upload — whatever you pick).
```

`bundle.json` carries the same injection-coverage rate and gate-catch counts
as the report, plus fathom's own version/commit and coarse platform — but
`my-service` never appears in it; only a salted hash of it does. You choose
how (or whether) to send `bundle.md`/`bundle.json` onward.

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
`workspace/artifacts/research/.salt` (gitignored, `0o600`) and reuses it on
every later run — this is what keeps a given service's hash stable across
bundles (so repeated counts of the same service are recognizable as the
same service without naming it). Deleting the salt file just starts a new,
uncorrelated hash lineage; it does not leak anything.

**No MCP tool, no scheduling:** run these from a terminal, by hand or from
your own script/cron if you want a recurring snapshot — fathom itself never
schedules or transmits either command's output.

---

## Maintained

This doc covers: `fathom research report`, `fathom research bundle`. Update
it when either command's flags, output shape, or sanitization guarantees
change — see also `docs/design/research-flywheel.md` for the full design
rationale and `internal/research/bundle`'s doc comments for the allowlist
implementation.
