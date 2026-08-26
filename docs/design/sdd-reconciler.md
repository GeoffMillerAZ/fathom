# Minimal-theater SDD: the reconciler v0 (normative spec)

Status: **adopted 2026-07-15** (graduates docs/vision/09; ADR-018).
Scope: plan-grammar v2, the wave gate, the CUE shadow, and the Stop hook —
the smallest set that makes dev-process state *derived from evidence*
instead of self-reported. Everything here extends `internal/verify` +
`schema/plan` in place. No engine, no lifecycle enum, no new repo.

## 0. Doctrine (binding)

1. **State is computed, never stored.** `verify.VerifyPlan` stays a pure
   function of (plan, evidence): no clock, no I/O, deterministic order.
   Nothing in v2 adds persisted process state.
2. **Markdown is authored; CUE is generated and validated; Go evaluates.**
   No human or agent hand-writes plan CUE. `gen-plan-cue` output is
   generated-only; `schema/plan` validates shapes.
3. **The honesty boundary is untouched.** `test_reaches` remains static;
   `runtime_pass` remains Unknown; exit codes remain 0 (pass) / 1 (a
   claimed task failed) / 2 (operational, fail-open at pre-push,
   fail-closed in CI).
4. **Dispositions are the one authored state** — intent is not derivable
   from evidence. Everything else (verdicts, readiness, summaries) is
   derived.
5. **Hooks never block on operational failure.** The Stop hook is silent
   (exit 0, no output) on daemon-down, no-git, no-plans, parse errors.

## 1. Plan grammar v2 (additive; v1 plans parse unchanged)

### Checkbox states

| Box | Meaning | Gates? |
|---|---|---|
| `[ ]` | open | no (except `--require-all`) |
| `[x]`/`[X]` | **claimed** — the author asserts done | yes |
| `[~]` | **in-progress** (matches the fleet TRACKING legend) | no (except `--require-all`) |
| `[-]` | **skipped** — exempt, with optional typed disposition | no |

### Annotation keys (one `{key: value, …}` block per checkbox line)

v1 keys unchanged: `symbol`, `package`, `reachable_from`, `min_tier`,
`skip_test`, `allow_contradiction`. New in v2:

| Key | Meaning | Convention (linted, not fatal) |
|---|---|---|
| `id` | author-owned stable task ID, overrides positional `md-NNN` | required for plans that will be diffed/gated across edits; must be unique in the file |
| `disposition` | `refused` \| `deferred` — refines *why* a `[-]` line is skipped | only on `[-]` lines; requires `reason` |
| `reason` | free-text justification (quote it if it contains commas) | required with `disposition` and recommended on every `[-]`/`[~]` |
| `trigger` | the condition under which a `deferred` task revives | recorded and surfaced in status; **not machine-evaluated in v0** |
| `skip` | justification for `skip_test: true` | required whenever `skip_test` is set |

Annotation **values may be double-quoted**; a quoted value may contain
commas (`reason: "kit-side done, donor NOT deleted"`). Quotes are
stripped. Unquoted values still must not contain commas.

`verify.LintPlan(plan)` returns the (deterministic, task-ordered) list of
convention violations. `fathom verify` prints them to stderr as warnings;
they never change the exit code in v0.

### Task identity

Positional `md-NNN` remains the default. When `id:` is present it becomes
`Task.ID` verbatim. Duplicate explicit IDs are a lint violation (first
occurrence wins nothing — both are reported).

## 2. The wave gate: `fathom verify --require-all`

`--require-all` is the wave-advance oracle: in addition to normal gate
semantics (claimed tasks must verify), the gate **fails if any task is
open or in-progress** — a wave may only advance when every task is either
claimed-and-verified or explicitly skipped (`[-]`, ideally with a typed
disposition). Implemented as `verify.GateRequireAll(report, strict)`;
exit-code contract unchanged. Recorded usage (the exit-0 command output
pasted into the advancing spec/ledger) is the transition record — there is
no transition engine.

## 3. The CUE shadow: `fathom gen-plan-cue`

`fathom gen-plan-cue --plan X.md --service S [--output F]` emits a
generated-only CUE document conforming to `schema/plan` `#Plan` (v2
fields included), then **round-trips**: it re-parses its own emission and
compares task-for-task with the markdown parse. Any loss is an error
(exit 2). This proves the typed shadow is lossless without changing what
anyone authors. CUE is output, never input: the legacy hand-authored
`.cue`/`.json` plan inputs to `gen-status-cue` remain for compatibility
but are documented as deprecated; `schema/plan`'s "hand-edited #Plan"
language is retired.

## 4. The Stop hook: `fathom stop-hook`

Claude Code `Stop` hook. Contract:

1. Read the Stop JSON from stdin; if `stop_hook_active` is true, exit 0
   silently (no re-entry loops).
2. Resolve the project root (`CLAUDE_PROJECT_DIR`, then `cwd`) and its
   service identity exactly like the edit-hook (candidates + aliases).
3. For every `docs/plans/*.md` (excluding README): compute **newly
   claimed** tasks = claimed in the working tree, not claimed at `HEAD`
   (matched by task ID — stable `id:` keys make this exact; positional
   IDs make it best-effort, which is acceptable for an advisory channel).
4. If none: silent exit 0. Otherwise verify those plans via the daemon
   (`verify_plan`, gate computed locally) and collect newly-claimed tasks
   whose status is **Fail** (Unknown never fires the hook — fail-open).
5. Output: **advisory by default** — `hookSpecificOutput.additionalContext`
   listing each unproven claim with its first failing criterion citation.
   With `--block`: `{"decision":"block","reason":…}` so the session cannot
   end on a disproven claim. Blocking is opt-in per repo and only after
   one clean advisory week (pilot discipline).
6. Any operational failure at any step: silent exit 0.

## 5. Oracle classes (forward registry — v0 ships `index` only)

A gate check is a conjunction of criteria answered by named oracles; the
moment logic wants OR/negation, that logic becomes a Go oracle. Classes:
`index` (shipped: symbol_present, tier, test_reaches, edge_wired,
no_contradiction, spec_covered), `cmd` (future: pinned command exit →
`runtime_pass`), `lexical` (agentic-kit evidence-signature ABSENT_PROVEN),
`eval` (agentic-kit eval-runner behavioral verdicts), `human` (sme-stated
ratification). Only `index` is wired in v0; the others land strictly
behind named consumers (docs/vision/09 §3).

*Amendment (2026-08-26, ADR-026).* Two classes join the registry for annotated
intent conditions (`docs/design/evidence-plane.md` §3), each with a named consumer
in `docs/intent/`: `plan` (a named plan's rows all proven, or one row proven —
the consumer is every goal's `Done when` that cites a plan) and `date` (after a
date — the consumer is every dated `Drop if`). They evaluate only in
`fathom intent audit`, never in a hook; `cmd` stays future.

## 6. Pilot metrics and kill criteria

As adopted from docs/vision/09 §5: five metrics (checkbox-drift = 0;
wave advances by recorded `--require-all` exit codes; Stop hook catches
≥1 real over-claim; authoring tax ≤10 min/spec; skips 100% reasoned with
share ≤ the 75% control baseline and falling post-O6), four kill criteria
(authoring tax >15 min/spec or ≥3 inexpressible honest states; round-trip
loss; >25% false-fail escape-hatching; zero catches + ≥2 spurious blocks,
or decorative after-the-fact gate runs). Decide keep/kill at week 4 in
`docs/dogfood-observations.md`.

## Maintained

CLI: `fathom verify --require-all`, `fathom gen-plan-cue`,
`fathom stop-hook`. Grammar reference: `docs/plans/README.md`. Schema:
`schema/plan/schema.cue`. Gated plan for this work:
`docs/plans/sdd-reconciler-v0.md`.
