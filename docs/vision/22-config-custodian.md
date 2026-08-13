# 22 — The config custodian: instructed means true

> Owner ask, 2026-08-13: *"fathom is really good at understanding your
> project — can it keep CLAUDE.md/AGENTS.md DRY, thin routers but very
> capable? Hooks to keep them updated? CUE config making fathom a
> continuous custodian of the agentic configuration? Concerns registered
> globally, brought in locally — global instruction about how to set up
> the local config? Make sure it's AI editing, not pasting, so you don't
> destroy the user's own concerns. Audit the config against my concerns as
> the project grows and call my attention to it. Frame it better, step up
> the ambition, or reject it."*

## 1. Accepted, with a sharper frame

Agent configuration is a pile of **claims about a codebase**, and
codebases move. Config therefore rots — not into absence, but into
*confident misdirection*, which is strictly worse: an agent believes its
config with more conviction than it believes the code. This program has
measured the failure twice in its own repo: a vision-range instruction
that silently went stale, and a deploy instruction whose rot produced a
fleet-wide daemon outage (O25). Nobody re-reads their config. Every agent
obeys it.

The sharpened thesis: **fathom made plans verifiable ("done means
proven"); it now makes configuration verifiable ("instructed means
true").** The config plane joins the evidence plane. This is
fathom-shaped work because an auditor needs exactly the three things
fathom already holds:

1. **The index** — the only way to fact-check "the store is pure Go" or
   "run `task validate`" against what the repo actually contains today;
2. **The gate discipline** — claims checked by machine, absences named,
   confidence capped by verification depth (laws 3 and 14);
3. **The ownership mechanics** — marker-owned regions, drift that stops
   and reports, receipts (law 15).

## 2. The gate on the asks

| Ask | Verdict |
|---|---|
| DRY, thin-router config | **ACCEPTED as an audited PROPERTY, not an autofix.** The compositional restructure (this repo, 2026-08-13: 239→84 lines, path-scoped files, pointer rows) is the reference shape; the custodian measures every repo against it — router thinness, pointer validity, duplication — and reports. |
| Hooks keep config updated | **REFRAMED: hooks keep config AUDITED.** Fathom's own regions stay auto-refreshed (reconcile already does this, including content drift as of today). The user's prose is **never auto-edited** — see the refusal below. Audit runs at session start (one cached line) and on demand; never per-edit. |
| "AI editing, not pasting, so you don't destroy the user's concerns" | **REJECTED in favour of something stricter.** Even careful AI editing of a human's prose is fathom exceeding its ownership. The custodian's verbs are AUDIT (mechanical, index-backed), PROPOSE (a diff the owner or an explicitly-invoked agent applies), and REFRESH (only inside fathom's own markers). Law 15 is not softened for good intentions — good intentions are how config gets destroyed politely. |
| Concerns in global fathom config, instantiated locally | **ACCEPTED — and it is law 13's exact shape, already built once.** `~/.fathom/concerns.cue` = user scope: "what my agentic config must guarantee, everywhere, unless a repo says otherwise." `<repo>/.fathom/concerns.cue` governs. The distinction the owner reached for is precisely the registry's: the harness's global config is a global *instruction*; fathom's global config is a global *contract the local config is audited against*. Loader, scope resolution, closed schemas, `?:` refusal — all reused from the capability registry. |
| Continuous custodian / watcher | **ACCEPTED with the tokenomics gate.** Mechanical checks first and mostly-only: path-exists, command-exists (parsed from Taskfile/lefthook), symbol-exists (the index), pointer-valid, section-present, stanza-current, duplication (normalized-hash), staleness-dated claims. Zero LLM calls in the standing loop. An LLM lane for prose-level truth ("does this paragraph still describe the architecture?") is deferred behind the FATHOM_METRICS tier system and priced before it exists. |

## 3. The concern model

```cue
// ~/.fathom/concerns.cue — the contract, user scope (repo scope governs)
concerns: {
    deploy_safety: {
        want:  "no raw-cp deploy instruction anywhere in agent config"
        check: {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom", where: "agent-config"}
        why:   "O25: an unsigned binary took the daemon down fleet-wide"
    }
    gate_documented: {
        want:  "every repo's config names its validate command, and the command exists"
        check: {kind: "command_documented_and_exists", command: "task validate"}
    }
    router_thin: {
        want:  "the root CLAUDE.md stays a router"
        check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
    }
    pointers_hold: {
        want:  "every relative path in agent config resolves"
        check: {kind: "pointers_resolve", where: "agent-config"}
    }
}
```

A concern is a **falsifiable want**: each `check` is a machine-evaluable
kind, and a concern whose check cannot be stated mechanically is refused
at load (the anchored-memory refusal, applied to governance — an
uncheckable concern is a wish, and wishes rot into the same misdirection
this exists to remove). The audit output is per-concern
value-or-named-reason: `holds` / `violated (file:line, evidence)` /
`unable-to-check (reason)` — never a silent skip.

## 3b. The delivery loop — an instrument, not a hand

Owner-confirmed 2026-08-13: *"instead of editing files like CLAUDE.md
directly, fathom would just provide the feedback to the code editing
session to take action on it."* Exactly — and this is the design's load-
bearing asymmetry, stated once so nothing later softens it:

**Fathom is the instrument; the session is the hand.** A violation is
delivered as feedback — the session-brief attention line, the doctor row,
the audit output — to an agent that has conversational context and a human
present. That agent edits the prose it is entitled to edit; fathom's own
writes stay inside its markers (law 15). The division is not a limitation
to work around later; it is why the custodian can run continuously without
ever being the thing that destroyed someone's config: the component with
standing access has no write authority, and the component with write
authority acts only inside a session where a human can see it.

## 4. Surfaces

- `fathom config audit [--repo R]` — the verb; JSON for the observatory.
- One **doctor row** (the capability-registry pattern: absence of concerns
  is not a failure; a violated concern is).
- One **flywheel loop**: custodian CLOSED = audits running AND violations
  being consumed (attended within N days); OPEN = violations accumulating
  unread — a conveyor moving nothing, one layer up.
- One line in the **session brief** when violations exist for the repo
  being opened: `config: 2 instruction(s) no longer true — fathom config
  audit`. Calling attention is the custodian's entire delivery mechanism;
  it edits nothing it does not own.
- The fleet page's realization ladder gains no new rung — a violated
  concern surfaces through the existing config panel, priced against the
  same honesty rules.

## 5. What this is NOT

- Not an autofixer of prose. Not an LLM rewriting your instructions. Not
  a second obligations system (obligations govern *code* edits; concerns
  govern the *config* itself). Not per-edit hooks. Not shared or hosted
  (law 13). Not built until its plan rows exist — this document is the
  vision; the design doc and rows come with the build slice.

## 7. Corpora and packs (owner follow-up, same day)

> *"Should we make it extensible to other config types — lefthook,
> Taskfiles? Ship generic packs for the coding agents, or build the agent
> custodians in and allow extensible configs too? Lefthook has an
> excellent agentic story: it pushes deterministic validation."*

**Accepted — and it sharpens the architecture rather than widening it.**

- **The engine is corpus-generic already.** Every v1 check kind operates
  on "a named set of files plus the repo"; `where:` is the seam. New
  corpora (`lefthook`, `taskfile`, `ci`) are new resolvers behind the same
  engine — never a parallel system, never a per-tool custodian (CC-11).
- **Knowledge ships as packs, not built-ins.** A pack is a curated,
  versioned set of concerns — the layer-pack precedent, reapplied. Fathom
  ships packs (claude-code, codex, lefthook, taskfile); a repo enables
  them; `.fathom/concerns.cue` overrides or disables per entry (law 13
  unchanged). Hardcoding agent custodians into Go would make fathom's
  releases the bottleneck for what is plainly data (CC-12).
- **The lefthook story is real and it is OURS, precisely because it is not
  linting.** Other tools validate a config's *syntax*; the custodian
  validates its *honesty* — cross-artifact truth: the pre-push hook still
  runs the gate the docs promise, the Taskfile target the hook names still
  exists, the chain from CLAUDE.md → lefthook → Taskfile → scripts is
  unbroken. Deterministic validation is the spine of agentic coding; the
  custodian's job is to notice when the spine quietly disconnects — the
  O25 failure class, which was exactly a documented command outliving
  reality.
- **The convergence this buys (O27's structural resolution):** several of
  doctor's bespoke checks — stanza present, lefthook gates, binary on
  path — are concerns in Go clothing. As packs mature, doctor's rows
  render what the concern audit knows instead of re-deriving it (law 10:
  one evaluation engine, many surfaces). Direction, not a rewrite mandate:
  each check migrates only when the pack expresses it fully.
- **Refused:** re-implementing yamllint/shellcheck/schema validation for
  third-party tools; a "custodian for all configs" ambition with no
  failure class behind it. A corpus earns admission with a pack of
  falsifiable concerns that catch a real, named failure — lefthook and
  taskfile qualify today because the deterministic gate chain is what
  agentic coding stands on; the next corpus must make the same case.

## 6. Falsification, pre-registered

- If, 30 days after the audit ships, violations are produced and never
  consumed (the flywheel loop's OPEN state persisting), the attention
  surface is wrong — fix delivery before adding check kinds.
- If mechanical checks cannot express the majority of the owner's actual
  concerns (measured: concerns refused at load vs accepted), the
  falsifiable-want constraint is too strict and the design needs an
  honest LLM lane sooner — priced, tiered, never standing.
