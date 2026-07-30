# Scoped config — what may live at user scope, and what must never

**Status:** adopted design, owner-commissioned 2026-07-30 ("should we have
global fathom ignore and guards and other such files?"). Amends
`docs/design/guarded-edits.md` §3's "no per-user guards" non-goal (narrowly,
with reasons) and extends the precedence model `internal/config` already
established. Rows land in the phase-P plan.

## 1. The evidence that prompted it

`.fathomignore` in this repo grew five entries **today** — `.playwright-mcp/`,
`*.wasm`, `.DS_Store`, a captured-transcript glob, and a build artifact —
none of which are about this project. They are machine junk, and every repo
in a fleet re-derives the same list by hand, badly, one incident at a time
(the entries were added *after* 4.5 MB of PNGs and prebuilt grammars reached
the index). That is the classic case for a user-scope default, and git
solved it the same way with `core.excludesFile`.

But an ignore file decides **what evidence exists**, and a guard file decides
**what an agent is told before it edits**. Both are governance surfaces, so
"just add a global file" is not automatically right. The rule below is
therefore per-artifact, not blanket.

## 2. The precedence law (new law 13)

13. **User scope may add defaults; only project scope may govern.** A
    user-scope file may contribute *additive, machine-level* defaults and
    *the operator's own notes*. It may never narrow, weaken, or widen a
    project's committed governance: not a tier floor, not a binding, not a
    scope restriction another author wrote. Where both scopes speak, the
    project wins, and every user-scope contribution is **disclosed at the
    point of use** — an undisclosed global is indistinguishable from a bug.

## 3. Per-artifact verdicts

| Artifact | User scope? | Why |
|---|---|---|
| `~/.fathom/ignore` | **YES — additive only** | machine junk is machine-wide; patterns UNION with the repo's `.fathomignore`, and a user-scope pattern can only ever *exclude* (never re-include, so no global file can silently pull something back into an index) |
| `~/.fathom/guard.cue` | **YES — notes and commands only** | the operator's own danger list ("show me the rules before touching a migration", "confirm before an apply") is genuinely user-level. Restricted to `note` and `command_scopes`; may NOT set `min_tier`, `include_modules`, or `max_rules` (those shape what *governance* serves — project business) |
| `~/.fathom/modules.cue` (bindings) | **NO, by design** | binding is the trust boundary: "no org rule reaches a repo that did not bind it." A global binding injects org standards into repos that never opted in — the exact inversion the phase-O review caught in the CUE scaffold. Keep it committed, reviewable, per repo |
| `~/.fathom/config.toml` | **already exists** | precedence flags > env > project > user > defaults; this design follows its merge semantics rather than inventing new ones |
| layer packs / prompts | **NO** | a pack's identity is its content hash; a user-scope prompt would make two machines compile *different* claims under one identity — unreproducible by construction |

## 4. Disclosure (the part that makes this safe)

Every consumer of a merged result must say what came from where:

- `fathom module sources` / `repo_map` / ingest summaries: `excluded: 14
  patterns (9 project, 5 user-scope ~/.fathom/ignore)`.
- The guard payload: a user-scope note renders tagged `[user]`, exactly as a
  module rule renders tagged with its module.
- `fathom doctor`: a check that names each user-scope file in effect, and
  **warns when a user-scope ignore pattern is what's hiding an
  otherwise-indexable tree** (the "why can't fathom see my code" failure
  mode, pre-empted).
- `fathom config --explain` already prints provenance per key; the ignore and
  guard merges get the same treatment.

## 5. Non-goals

No user-scope bindings (§3). No user-scope override of project guards — only
addition. No sync/registry of user config. No implicit creation: `fathom
init` may *offer* to seed `~/.fathom/ignore` with the machine-junk defaults,
printing exactly what it would write, and does nothing silently.

## 6. Deliverables

1. `~/.fathom/ignore` loaded and UNIONed by the ignore resolver
   (`connector.LoadIgnore`'s callers), additive-only enforced in code +
   test (a user-scope `!pattern` re-include is refused loudly).
2. `~/.fathom/guard.cue` loaded by `guardspec`, restricted to
   `note`/`command_scopes` by a strict-mode validation error naming any
   governance field found there; payload tags `[user]`.
3. Disclosure everywhere in §4, including the doctor check and the
   ignore-is-hiding-your-code warning.
4. A seeded machine-junk default list (`.DS_Store`, `*.wasm`, editor and
   tool state, screenshot dirs) shipped as the `init --seed-user-ignore`
   offer, never auto-written.
5. Docs: `docs/configuration.md` gains a scoped-config section; the
   guarded-edits design's non-goal line is amended to point here.
