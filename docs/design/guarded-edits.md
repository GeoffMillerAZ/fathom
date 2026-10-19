# Guarded edits — pre-edit guidance for files where the first mistake is expensive

**Status:** adopted design, owner-commissioned 2026-07-29 ("pre tool hooks
that when editing certain files (maybe configurable per project) you get
extra hooks to inject guidance… maybe too much for all files but maybe
some"). Companion to `docs/design/platform-modules.md`; both implement as
phase O of `docs/vision/13-expert-suite.md`. Closes the intent of the
long-unclaimed GI-8 row (docs/plans/archive/gap1-instrumentation.md, "PreToolUse
revival") with a sharper mechanism.

## 1. The economics, stated honestly

The PostToolUse channel (obligations injected after every Edit/Write,
p50 13 ms) is the proven default: cheap, universal, and correct for most
files, where seeing the rules immediately after an edit is early enough to
fix. Firing rich guidance BEFORE every edit would roughly double the
hot-loop token cost for marginal benefit on ordinary files.

But a minority of files invert the economics: the first wrong edit is
already the incident (store DDL and migrations, security-sensitive paths,
public API surfaces, spec/schema files, CI/release plumbing). For those,
guidance must arrive **before the tool runs**. Hence: **guarded files** —
a per-repo, explicitly configured set of scopes where a PreToolUse hook
injects the governing rules ahead of the edit.

## 2. Design (normative)

### 2.1 Configuration: `.fathom/guard.cue` (schema: `schema/guard/`)

```cue
guards: [{
    scopes: ["internal/store/**", "**/migrations/**"]  // consumer-tree globs
    note?:     "Store schema is spec-governed; read docs/spec first."
    max_rules?: 4          // pre-edit payload cap (default 4)
    include_modules?: true // also serve bound platform-module rules (phase O)
}]
```

- Per-project, committed, reviewable — same CUE discipline as every config
  surface (dual-mode loader per the modspec precedent).
- The optional `note` is the author's own one-liner, served verbatim above
  the rules — the cheapest possible "why this file is special."

### 2.2 The hook: inject-only, silent-by-default, budget-capped

- `fathom edit-hook --pre` (same binary, same service-identity resolution,
  same daemon lookup as the PostToolUse path — one code path, two moments).
  Registered as a Claude Code PreToolUse(Edit|Write) hook and the Codex
  equivalent by `fathom init`.
- **Non-matching files: instant silence.** The guard match is a local glob
  check before any daemon call — the universal cost of the hook is one
  file-pattern test.
- Matching files: serve the same obligations payload the post-hook would
  (repo claims ∪ bound modules when `include_modules`), capped at
  `max_rules`, module-tagged, tier + staleness flagged, `note` first.
  Same ≤2 s hook budget; same fail-open silence on daemon absence (O8
  doctrine unchanged).
- **Inject-only, never blocking — AMENDED by ADR-038 for a checkable
  rule.** PreToolUse can deny a tool call; this hook did not, until
  guards-pilot-8's own reading became the documented incident the
  paragraph below asked for: the governing rule reached the agent before
  the edit on 4 of 8 tasks and was violated anyway on two of them, tying
  the control arm 4-for-4 — an obligation the agent can read and still
  contradict is not the same instrument as a check the tool call cannot
  get past. A guard or layer rule marked `enforce: true`
  (`internal/guardspec`'s deterministic pre-write checks — forbidden
  path, org token, secret literal, workspace-output convention,
  doc-shape, cgo purity, import direction/boundary) now DENIES a
  violating PreToolUse call outright — `hookSpecificOutput.
  permissionDecision:"deny"`, a single-line reason, never a wall of
  text — instead of only injecting the advisory payload above.
  `enforce: false` (the default, and every guard authored before this
  amendment) is unchanged: inject-only, exactly as below. Blocking still
  belongs to push/CI gates for everything a deterministic pre-write check
  cannot express; `FATHOM_ENFORCE=off` / `[guards] enforce = false` is
  the repo-wide escape hatch back to advisory-only. Full account: ADR-038.
- **The original escalation trigger, for the record.** "An edit-time
  deny would train agents to route around the channel. Revisit only with
  a documented incident a warning demonstrably failed to prevent" — the
  trigger this amendment fired.

### 2.3 Dedup with the post-hook (no double-billing)

When the pre-hook served rules for a file, the post-hook for the SAME file
within the same session serves only the delta: rules newly matched by the
edit's actual content plus any staleness change — otherwise a one-line
"guarded: rules served pre-edit" marker. Mechanism: the pre-hook writes a
short-lived local stamp (path + rule-set hash, session-scoped tmp);
the post-hook consults it. Never silent: the marker line always names the
suppression.

### 2.4 Doctor + instrumentation

- Doctor: guard file invalid (loud); guard scope matching zero indexed
  files (warn — dead guard); `include_modules` naming no bound module
  (warn).
- Injection stamps gain a `pre|post` dimension; `research report` reports
  guarded-edit coverage separately. Law 8 applies: a guard that never
  fires across a qualifying window is a prune candidate, reported not
  auto-removed.

### 2.5 Guarded commands (same shape, same slice)

`guards[].command_scopes?: ["terraform apply*", "kubectl *", "*/migrate *"]`
extends the same mechanism to PreToolUse(Bash): a command matching a
guard's pattern gets the guard's `note` + rules injected before it runs.
Identical budget, dedup, and inject-only doctrine (ADR-038 §2.2's
amendment applies here too: `enforce: true` on a command guard denies a
matching command outright — a match against `command_scopes` IS the
violation, so there is no separate content check to run) — this
complements the permission system (which decides IF a command may run)
with governance context (what the org knows about running it). Doctor
warns on dead command guards like dead file guards.

`guards[].requires?: ["-race"]` (2026-10-19, `docs/design/readouts.md`
§18) names the argv tokens a command matching the guard's
`command_scopes` must carry. It is optional and additive: it needs
`command_scopes`, is project scope only, and a guard without it behaves
exactly as before. Its one reader is the rich readout, which names a
missing token, the guard's `.fathom/guard.cue:N` and its note beside the
command's output; the PreToolUse path never reads it, so it never denies.

## 3. Non-goals (v1)

No auto-derived guards (an LLM proposing "this file looks important" is
phase-L advisor territory, eval-gated, and lands only as a *suggested*
`guard.cue` diff); no blocking for an `enforce: false` guard (the default);
`enforce: true` blocking is bounded to ADR-038's five deterministic checks,
never an LLM judgment call; per-user guards ONLY as narrowly amended by
`docs/design/scoped-config.md` law 13 (notes and command scopes at user
scope; every governance field stays project-committed); no new MCP tools.

## 4. Deliverables (rows land in the phase-O plan)

1. `schema/guard/` + loader; `fathom init` scaffolds a commented example
   and registers the PreToolUse hook (Claude Code + Codex variants).
2. `edit-hook --pre` with local glob fast-path, payload assembly, stamp
   write; post-hook delta/dedup read.
3. Doctor checks (2.4); stamps + research-report dimension.
4. Docs: use-case + adoption guides updated (claude-code.md, codex.md),
   coding-agent-excellence.md §5 channel table row, fathom-onboard /
   fathom-doctor skills mention guards; GI-8 row closed with a pointer
   here.
