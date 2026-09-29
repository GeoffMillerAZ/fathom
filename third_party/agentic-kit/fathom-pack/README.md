# agentic-kit fathom pack

> The kit ships its law with its code: agentic-kit's hard rules as
> delivered configuration — pre-edit guards for the kit repo itself and
> for consuming repos, plus a compiled-obligations layer — so no adopter
> re-derives the doctrine and no session quietly violates it.

## What's in it

- `guard-pack.cue` — two labeled groups: **kit-self** (working inside
  agentic-kit: stdlib-only, layering, package shape, determinism seams,
  TRACKING discipline, `_reference/` porting) and **consumer** (using the
  kit from another repo: extend-in-kit-not-app, mock-is-an-adapter,
  clock seams at the kit's boundaries, org-agnostic data-not-code).
  Guards are deterministic and need no compiled layer.
- `layers/agentic-kit-standards/` — the compile pack (prompt + kill gate +
  manifest) turning AGENTS.md/VISION.md/docs into cited obligations.

## Install

Kit repo: copy the **kit-self** group into `.fathom/guard.cue`.
Consumer repo: copy the **consumer** group, narrow scopes; then
`fathom layers install <agentic-kit>/fathom-pack/layers/agentic-kit-standards`
and compile when an LLM profile is configured. Keep the provenance line on
every copied entry — `/fathom:activate` re-syncs by it. Prove by
demonstration: touch `go.mod` in the kit and confirm the stdlib-only guard
appears.

## Push gates (`scripts/`)

`push-check.sh` refuses a push from a clone whose pre-push hook is not
lefthook's; `push-http.sh` derives a smart-HTTP URL from the push remote and
sends the pack over it, so no transport session is held open across a long
pre-push gate. Both carry a `_test.sh` that runs over temp repositories and
never touches the network. Why each exists, and the third rule that is a
property of the gate itself: `docs/project-intent/operating.md` §5. Adopt by
calling the vendored scripts from the repo's `push` task.

## Authoring guards responsibly

Guards spend attention as well as tokens; the discipline that keeps them
net-positive:

1. **The bar**: a guard exists because the mistake *already happened* —
   the note cites the incident or the decision. Speculative guards are the
   failure mode: fatigue teaches agents to skim the channel, which is
   worse than having no channel.
2. **Scope hygiene**: globs as narrow as the law allows; consumers narrow
   the shipped globs further on install. A guard firing on files it does
   not govern is spam with a citation.
3. **Budgets**: `max_rules` ≤ 4 per entry, notes under ~150 words. The
   injection channel is token-capped and deduped (repeat edits to the same
   file do not re-inject), but attention is not capped — write for the
   agent that has seen this note before.
4. **Staleness is worse than silence**: every note cites its source, packs
   version with their kit, and `/fathom:activate` re-syncs by provenance
   marker — a lagging pack is drift, and a wrong injected rule actively
   misleads.
5. **Measure or retire**: the hook beats record fire rates; reviews record
   catches. A guard that fires often and catches nothing in a quarter is
   folded into docs or killed — the KILL.md discipline applies to guards
   as much as layers.
6. **Guards warn; gates refuse**: hooks fail open by design (a down daemon
   must never block an edit), so anything that must *hold* belongs in a
   blocking gate (lefthook/CI/verify) — the guard is the warning at the
   moment of the edit, not the enforcement.
