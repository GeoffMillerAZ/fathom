# Rewire & activate: the two prompts

> Date: 2026-08-07 · Companion to vision 14 §7 (reconcile + self-healing).
> Until `fathom fleet reconcile` ships, these prompts ARE the reconcile:
> paste Prompt 1 into a session in any drifted repo; paste Prompt 2 into a
> repo that's wired but hasn't activated the newer features (guards, layer
> packs, bindings). Both are written for an agent, with the known false
> signals called out so it doesn't chase doctor's wrong-repo bugs.

## Prompt 1 — rewire this repo (hooks, binary, stanza, index)

```
Run a fathom rewire pass on this repo. The fathom binary is `fathom` on
PATH (a shim at ~/.local/bin/fathom → the deployed daemon binary at
~/.local/libexec/fathom/fathom; if the shim is missing, create it with
ln -sf). The daemon is launchd `com.geoffmilleraz.fathom-http` at
http://localhost:7575.

1. Run `fathom doctor` and capture the output — but do NOT chase these
   KNOWN FALSE SIGNALS: (a) `binary_fresh` compares against THIS repo's
   git HEAD (wrong-repo bug — ignore it); (b) `hooks_firing` looks for a
   call log inside this repo, but the shared daemon writes one log at its
   own workspace — ignore it; (c) a `binding_file` FAIL on a
   fully-commented .fathom/modules.cue scaffold means add `bindings: []`
   (and `guards: []` in guard.cue) beneath the comments — the config is
   not broken; (d) `hooks_installed` may cite hook-beat "evidence" that
   actually came from OTHER repos (the beat log is user-global) — trust
   your own read of settings.json below over it.
2. Read .claude/settings.json (project scope) AND ~/.claude/settings.json
   (user scope). The CURRENT hook template is: SessionStart
   (matcher startup|resume|clear|compact) → `fathom session-hook`,
   timeout 10; PostToolUse (Edit|Write|MultiEdit) → `fathom edit-hook`,
   timeout 5; PreToolUse three registrations, each timeout 5 —
   (Edit|Write|MultiEdit), (Bash), (Grep|Glob|Read) — all →
   `fathom edit-hook --pre`; Stop (no matcher) → `fathom stop-hook`,
   timeout 10. Reconcile IN PLACE at whichever ONE scope already has
   fathom hooks (project scope if neither does; never both): add missing
   events/matchers, remove duplicate registrations, and rewrite every
   hook command to invoke plain `fathom` instead of any absolute
   /Volumes/.../sextant/bin/fathom path.
3. Ensure .fathomignore exists and covers secrets, user data, vendored
   trees, and build output — BEFORE any ingest if this repo has never
   been indexed.
4. Regenerate the CLAUDE.md stanza: `fathom gen-claude-md` and merge —
   the stanza must say fathom (not sextant), name the correct service id,
   and give the verify command in shim form (`fathom verify ...`, no
   absolute paths).
5. Index or refresh via the MCP ingest tool with BOTH fields:
   {"service":"<plain-repo-name>","path":"<canonical /Volumes/... path>"}
   — the service id is a logical name, NEVER a path or ../ form; the path
   is the real volume path, not the ~/code symlink.
6. Smoke-test by demonstration: call list_services (this repo present,
   canonical path); make a trivial edit and within seconds confirm a beat
   landed: `tail -5 ~/.fathom/hooks.jsonl`.
7. `fathom observe` anything that deviated from this checklist, then
   report a table: hooks (before → after, per scope), stanza, ignore
   file, index row, smoke results.
```

## Prompt 2 — activate the newer features (guards, layer packs, bindings, skills, gates)

```
Run a fathom feature-activation pass on this repo (`fathom` is on PATH;
run Prompt 1's rewire first if doctor shows wiring drift).

1. GUARDS (.fathom/guard.cue): author 3–7 real guards from this repo's
   own binding docs (CLAUDE.md rules, ADRs) — each guard cites the
   decision it enforces. Prefer refusals for things that are "a build
   failure by policy" (e.g. a package.json under a no-Node surface, a
   raw CIDR in a rule file). If the scaffold is fully commented out,
   ensure a concrete `guards: []` (or your real list) exists so the
   loader parses it.
2. LAYER PACK (.fathom/layers/<repo>-standards/): create a rule pack
   holding the 5–10 most load-bearing, glob-scoped rules from this
   repo's doctrine — these are injected at edit time on matching files.
   Follow the working example's format exactly:
   platform-engineering/.fathom/layers/platform-standards/ (KILL.md
   rule-file convention). UI-active repos: scope UI globs (ui/**,
   **/*.templ, **/*.css, portal/**) to the house UI pipeline — search
   the appkit MCP corpus before writing markup, kit role tokens never
   raw palette colors, the @source line, no Node. Check
   `fathom layers list` for installable shipped packs first.
3. BINDINGS (.fathom/modules.cue): bind any expert modules actually
   mounted on the daemon; otherwise a concrete `bindings: []`.
4. SKILLS: ensure the six fathom skills exist under .claude/skills/
   (fathom-plan/verify/doctor/feedback/impact/onboard) — rerun
   `fathom init` scoped to skills if absent.
5. GATES: if this repo uses lefthook and has docs/plans/*.md, wire the
   pre-push verify loop using plain `fathom` (shim), and make the gate
   LOUD on a missing binary (`command -v fathom || { echo "fathom
   missing — gate NOT run"; exit 1; }`) — a gate that can't run must
   fail, not pass.
6. Prove each activation by demonstration, not assertion: trip one guard
   on purpose (expect the refusal), touch one rule-scoped file (expect
   the injection in the hook output / a beat), run
   `fathom verify --service <svc> --plan <a-plan>` once if plans exist.
7. HONESTY RAIL: `fathom narrate`, `fathom fleet reconcile`, `fathom arm`
   and an installable `ui-appkit` pack are vision items (docs/vision/14,
   15) that do NOT exist yet — do not invent flags; if you reach for one,
   note it as pending and move on.
8. Report what you activated, what you demonstrated, and file
   `fathom observe` for any friction.
```

## Why prompts and not a flag (yet)

These two prompts are the manual edition of vision 14 §7's reconcile tiers:
Prompt 1 is tier-1 detection + apply done by an agent; Prompt 2 is the
activation catalog done by an agent. When `fleet reconcile` ships, Prompt 1
collapses to one command and the session-hook's self-check names it; the
prompts then remain useful as the AUDIT edition (reconcile explains, the
prompt verifies by demonstration). Keep them versioned here so the template
in Prompt 1 is updated in the same commit that changes the init template —
a drifted prompt is the same bug class as a drifted hook.

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
