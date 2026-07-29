# Platform modules in the coding loop, and guarded edits

Let an org's own standards, guides, and platform docs — compiled once as a
portable expert module — enhance the coding agent's edit-time guidance in
every repo that opts in, and add a pre-edit channel for the minority of
scopes where the first mistake is already the incident.

Scope: this doc covers two companion phase-O designs together —
[platform-modules.md](../design/platform-modules.md) (binding a mounted
platform module so its rules join a repo's own obligations) and
[guarded-edits.md](../design/guarded-edits.md) (pre-edit guidance for
configured critical scopes). The wiring both ride — `fathom init`,
the edit-hook stack — lives in [coding-agents.md](coding-agents.md).

---

## When to use this

- Your org has standards/guides that today get copy-pasted into every
  repo's `.fathom/` pack, each with its own compile, its own drift clock,
  and zero shared freshness story — and you want ONE canonical corpus that
  every consumer repo can bind instead of copying.
- A repo has a minority of scopes — store schema/migrations,
  security-sensitive paths, public API surfaces, CI/release plumbing —
  where seeing the governing rules AFTER the edit (the default channel) is
  already too late; you want them BEFORE the edit or command runs.
- You want to know, honestly, whether a bound module or a configured guard
  is actually being used — not just that it's configured.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `obligations` | MCP tool | Governing rules for a file — module-tagged and staleness-flagged once a binding exists, unchanged in shape otherwise |
| `list_modules` | MCP tool | Every module store mounted on this daemon (module name, path, per-service watermark) |
| `module_obligations` / `module_query` / `module_staleness` | MCP tool | The full federation surface a bound module's rules ride, scoped to one or every mounted module |
| `fathom init` | CLI | Wires the PreToolUse guard hook and scaffolds commented `.fathom/guard.cue` / `.fathom/modules.cue` |
| `fathom doctor` | CLI | Binding/guard config validity, unmounted bindings, sub-servable bindings, dead guards — see Setup & notes |
| `fathom research report` | CLI | Guarded-edit pre\|post coverage and per-module serving counts (never fabricated when unmeasured) |

---

## How it enhances Claude Code

**Platform modules — the platform stratum's actual delivery vehicle.**
ADR-024 named the platform stratum (org standards, trusted via compilation
+ eval evidence) but its only realization was per-repo `.fathom/` packs: N
repos, N copies, N drift clocks. A platform module compiles the SAME
corpus once; a consumer repo *binds* it via `.fathom/modules.cue` — an
explicit, reviewable, committed opt-in (doctrine: **no org rule ever
reaches a repo that did not bind it**). Once bound, the daemon's
obligations lookup unions the module's rules into the SAME channels the
excellence benches already proved: the edit-hook and the `obligations`
tool. Module rules render **module-tagged** (`[platform-standards]`),
tier- and staleness-flagged exactly like repo-local rules — never
silently blended in unattributed. Repo rules keep priority inside the
shared budget; a binding naming a module this daemon does not have
mounted produces an explicit note, never silence.

**Guarded edits — the pre-edit channel for expensive-to-get-wrong
scopes.** The PostToolUse edit-hook (rules injected AFTER every edit,
~13ms, zero-output-on-clean) is the right default for most files — but a
minority invert the economics: store DDL, migrations, security-sensitive
paths, CI/release plumbing. For those, a `.fathom/guard.cue` scope gets
its governing rules injected BEFORE the edit or command runs, via a
PreToolUse hook that is silent-by-default for everything else: a local
glob/pattern check runs before any daemon call, so a non-guarded file
costs one comparison and nothing more. Inject-only, never blocking —
blocking stays the pre-push/CI gate's job.

---

## Worked example

**1. Bind a platform module** (the module must already be mounted on the
daemon — `serve-http --module <path>`, an operator step). Uncomment
`fathom init`'s scaffolded `.fathom/modules.cue`:

```cue
bindings: [{
    module:  "platform-standards"
    scopes?: ["infra/**", "**/*.go"]
    max_rules?: 2
    min_tier?:  "doc-claimed"
}]
```

Commit it. The next `obligations` call for a matching file now unions in
the module's rules:

```json
obligations { "service": "my-repo", "file": "infra/main.tf" }
```

Returns something like:

```json
{
  "service": "my-repo", "file": "infra/main.tf",
  "rules": [
    { "rule": "Terraform state must be remote.", "src": "CLAUDE.md#L10-L10", "tier": "doc-claimed" },
    { "rule": "Tag every resource with cost-center.", "src": "STANDARDS.md#L4-L4", "tier": "doc-claimed", "module": "platform-standards" }
  ]
}
```

The second rule's `module` field names where it came from — the repo's
own rule keeps the first slot.

**2. Guard a critical scope.** Uncomment `fathom init`'s scaffolded
`.fathom/guard.cue`:

```cue
guards: [{
    scopes: ["internal/store/**", "**/migrations/**"]
    note?:     "Store schema is spec-governed; read docs/spec first."
    max_rules?: 4
    include_modules?: true
}]
```

Commit it. The next time Claude Code is about to `Edit` a file under
`internal/store/`, the PreToolUse hook injects the note plus the same
obligations payload — BEFORE the edit runs. The following PostToolUse
render for that same edit collapses to a one-line "already served
pre-edit" marker instead of repeating the rules.

**3. Guard a command.**

```cue
guards: [{
    command_scopes: ["terraform apply*", "kubectl *"]
    note?: "Infra commands are guarded; check the runbook first."
}]
```

A `Bash` call matching either pattern gets the note injected before it
runs — no obligations lookup for a command (there is no file to scope
one to), no daemon call at all.

---

## Setup & notes

**Nothing is active by default.** `fathom init` scaffolds both
`.fathom/guard.cue` and `.fathom/modules.cue` **entirely commented out** —
binding a module or guarding a scope is an explicit, reviewed, committed
opt-in.

**Doctor checks (`fathom doctor`):** `binding_file`/`guard_file` (loud on
a malformed CUE file); `binding_modules_mounted` (loud — a binding naming
a module this daemon does not have mounted); `binding_module_readiness`
(warn — a bound module below the phase-J readiness ladder's `servable`
level, checked via the module's own local manifest when resolvable, else
honestly "not checkable: module not local"); `guard_dead_scopes` (warn — a
guard's scopes match zero indexed files; command-scoped guards are noted
as not checkable this way, never silently skipped); `guard_include_modules_unbound`
(warn — a guard sets `include_modules:true` with no binding to include).
Warn checks never fail the `doctor` exit code — they are visibility, not a
wiring-broken verdict.

**`include_modules` gates DISPLAY, not the daemon's computation:** the
daemon always computes module rules once a binding exists; a guard's
`include_modules:false` (the default) only hides module-tagged rules from
the PRE-edit preview — they still reach the PostToolUse render, and the
pre/post dedup stamp is built from what was actually shown, so a
module-tagged rule hidden pre-edit is never silently lost, only deferred.

**Non-goals (v1):** no auto-binding, no org-push of bindings, no path
remapping, no module rules in `verify_plan` gating (a plan row is proven
by the repo's own evidence only), no auto-derived guards, no blocking, no
per-user guards.

**Measuring it, honestly:** `fathom research report` reports guarded-edit
`pre`/`post` coverage (seen/served counts, dedup-suppressed count) and
per-module rules-served counts separately — both print an explicit
"not measurable: …" note (never a fabricated zero) when the relevant
channel has never fired in the reporting window. Law 8 applies per binding
and per guard: zero served rules across a qualifying window is a prune/
unbind candidate, reported, never auto-removed.

---

## Maintained

This doc covers: `.fathom/modules.cue` (schema: `schema/binding/`),
`.fathom/guard.cue` (schema: `schema/guard/`), the `obligations` MCP
tool's `module`/`displaced_repo`/`displaced_module`/`module_notes` fields,
`fathom edit-hook --pre`, and the `binding_file`/`guard_file`/
`binding_modules_mounted`/`binding_module_readiness`/`guard_dead_scopes`/
`guard_include_modules_unbound` doctor checks. Update it when these
change.
