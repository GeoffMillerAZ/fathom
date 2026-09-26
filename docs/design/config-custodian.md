# Config custodian — mechanics of "instructed means true"

> Binding promotion of `docs/vision/22-config-custodian.md` (rows CC-* in
> `docs/plans/custodian.md`). The refusal boundary — audit/propose/refresh
> only, the user's prose never auto-edited, fathom as instrument not hand
> (vision 22 §3b) — binds everything below and is not repeated per section.

## 1. The concern model (`internal/custodian`)

(Package named `custodian`, not `concern` — `internal/concern` is the
concern-scoped query engine, a different notion that predates this design.)

A concern is a falsifiable want with law-13 scoping:

- `~/.fathom/concerns.cue` — user scope: what my agent config must
  guarantee everywhere, unless a repo says otherwise.
- `<repo>/.fathom/concerns.cue` — project scope: **governs.** A project
  entry with the same name replaces the user entry wholesale; `off: true`
  disables an inherited concern (and the audit *names* it as off — a
  disabled contract is a fact, not a hole).

```cue
concerns: {
    deploy_safety: {
        want:  "no raw-cp deploy instruction anywhere in agent config"
        why:   "O25: an unsigned binary took the daemon down fleet-wide"
        check: {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom", where: "agent-config"}
    }
}
```

Loader discipline is the capability loader's, reused deliberately:
- three states — absent file (normal), empty/scaffold (normal), malformed
  (a loud `LoadError` naming every problem, never a silent partial read);
- `?:` optional fields refused (the phase-O trust-boundary inversion);
- unknown fields refused; unknown check kinds refused **by name** with the
  accepted list — an uncheckable concern is a wish, and the loader is
  where wishes are turned away;
- a check field its kind does not read is refused naming the kinds that do
  (`check.file applies to max_lines, max_bytes, stanza_present only, not
  "required_pattern"`): scoping a `required_pattern` with `file:` would load,
  scan the whole corpus, and say nothing;
- concerns sorted by ID after merge; no map order reaches output.

## 2. Check kinds — mechanical, index-free, offline-safe (one daemon exception)

The four v1 kinds run from file reads on the local tree; no daemon, no LLM,
no network. `where:` names a corpus from the registry (§2b; default
`agent-config`); the loader refuses an unregistered corpus, a corpus the
kind cannot read, and a `where` on a kind that reads none, each by name,
rather than silently scanning something else. The two
kinds added by CC-14/CC-15 keep the no-LLM rule; `stanza_tools_served` is
the **one kind that asks the daemon** (`/rpc/agent` `tools/list`, falling
back to `/rpc`) and degrades to `unable to check` naming the URL — never a
violation — when the daemon is down or the registry is empty/malformed.

| kind | fields | holds when | violated evidence |
|---|---|---|---|
| `forbidden_pattern` | `pattern` (Go regexp), `where` | no line of any agent-config file matches | `file:line` per match |
| `command_documented_and_exists` | `command`, `where` | the command string appears in ≥1 corpus file (over a YAML corpus, on a line that is not a comment) AND it exists (`task X` → a target in the parsed Taskfile graph `commands_resolve` uses — any root spelling, includes, aliases, flattening; else `$PATH` lookup of argv[0]) | which half failed, named; a target behind a remote include is unable-to-check |
| `max_lines` | `file`, `lines` | the file exists and has ≤ `lines` lines | `<file> is N lines (max M)`; absence is a violation (`absent — the concern expects it`) |
| `pointers_resolve` | `where` | every relative markdown-link target (file-relative, fragment stripped; http/mailto/absolute/`#` skipped) AND every backtick repo-path (this house style's pointer form — ≥2 path-charset segments, no globs; runtime dirs workspace/tmp/ref/bin skipped; `../` escapes skipped; a token whose first segment is not a directory the repo even has is a foreign name — Go module, kit package — not a pointer; root-relative or file-relative resolution both accepted) resolves | `file:line → target` per dead pointer |
| `stanza_tools_served` (CC-14) | — | every tool name in the fathom stanza region of CLAUDE.md/AGENTS.md (backticked names, and bare names containing an underscore such as `find_callers`/`pack_context`/`verify_plan`; single-word names only when backticked) is served by the daemon's `/rpc/agent` `tools/list` (falls back to `/rpc`) | `file:line` per pruned/undeployed tool; daemon down or an empty/malformed registry → `unable to check` naming the URL |
| `required_pattern` (CC-12) | `pattern`, `where`, `under` | ≥1 line of the corpus matches — over a YAML corpus never a comment line or a trailing comment (a commented-out step runs nothing), and with `under` only the lines under that top-level key (a lefthook hook) | "no line matches" naming the files searched; "no `pre-push` key in lefthook.yml"; an EMPTY corpus is a violation (nothing there does the required thing); matching lines are the evidence when it holds |
| `stanza_present` (CC-12) | `file` | the file has the fathom begin marker, then the end marker | `<file>: never wired (no fathom markers)` or `<file>: corrupted marker pair (begin=… end=…)` at the marker's line; absent → violated, unreadable → unable (I38) |
| `max_bytes` (CC-12) | `file`, `bytes` | the file exists and is ≤ `bytes` | `<file> is N bytes (max M)`; absence is a violation |
| `commands_resolve` (CC-12) | `where` | every in-repo command the corpus invokes resolves, from the directory it runs in: a `task X` target in the Taskfile Task would find there, or in a local include (namespaced, aliased, flattened); a Taskfile's own `task:` / `deps:` call in its include's namespace (`:x` is the root's); a local include itself; a repo script path | `file:line → task X (no such target)` / `file:line → scripts/x.sh (no such file in web/)`, naming the file's own directory when the path resolves only from there; an empty corpus is unable-to-check naming it; a script that exists and cannot be stat'd is unable-to-check (I38); refs behind a flag, a template, a remote include, outside the repo or in a directory the audit cannot model are counted as not checkable, never guessed |
| `handoff_cites_instruments` (CC-15) | `days` | every handoff thread note filed for this repo in the window that claims shipped/landed/done/fixed work carries a citation — a commit sha (letters + digits), an active `docs/plans` row id, `fathom verify`, or a `workspace/artifacts/retros/` path | the uncited item ids; rows with unparseable timestamps are counted and named; no threads file → holds |

An invalid regexp or unreadable input is `unable-to-check (reason)` —
never a silent skip, never a fake pass (law 3).

**Agent-config discovery:** `git ls-files --cached --others
--exclude-standard` over `CLAUDE.md`, `*/CLAUDE.md`, `AGENTS.md`,
`*/AGENTS.md` — git's ignore rules are the authority on what is part of
the repo (keeping `ref/` clones and `workspace/` out) — then vendored
trees (`third_party/`, `vendor/`, `node_modules/`) filtered out even when
tracked: a vendored snapshot's CLAUDE.md is that project's instruction
surface, not this repo's, and auditing it produces violations nobody here
can fix. Outside a git work tree: a sorted walk with a fixed skip list
(the same dirs plus `.git`, `workspace`, `ref`, `tmp`, `bin`, `testdata`,
`.idea`, `.vscode`). Deterministic either way; the file list is sorted.

## 2b. Corpora — the `where:` seam (CC-11, vision 22 §7)

A corpus is a named file set resolved by one registry entry
(`internal/custodian/corpus.go`); every corpus-reading kind runs over
whatever its `where:` resolves to. New configuration types arrive as new
resolvers behind the same engine — never a per-tool custodian.

| corpus | files | commands live in |
|---|---|---|
| `agent-config` | CLAUDE.md / AGENTS.md at any depth (git's authority, vendored trees out) | code spans and shell fences (unlabelled, `sh`, `bash`, `shell`, `zsh`, `console`) — never prose, never another language's fence |
| `lefthook` | `lefthook.yml`, else `lefthook.yaml` (lefthook loads one) | `run:` |
| `taskfile` | the root Taskfile (Task's own search order) plus the local Taskfiles it includes | `cmd`/`cmds` (shell), `task:` and `deps` (names) |
| `ci` | `.github/workflows/*.yml` and `*.yaml`, `.gitlab-ci.yml`, `.circleci/config.yml`, `azure-pipelines.yml`, `bitbucket-pipelines.yml`, `.buildkite/pipeline.yml` | `run`, `script`, `before_script`, `after_script`, `command(s)` |

`pointers_resolve` and `stanza_tools_served` read `agent-config` only (a
pointer is a markdown notion; the stanza lives only there). A candidate
that exists and cannot be stat'd or listed is a discovery failure —
unable-to-check — never "absent" (I38). Named gaps: lefthook's dot-prefixed
and `.config/` spellings and `lefthook-local.yml` are not resolved
(doctor's `lefthook_gates` row renders this corpus and has always read the
two names above; widening it is its own change), and Taskfile `vars` with
`sh:` are not read as commands.

A command resolves from where it runs, not where it is written (probed
against task 3.52 and lefthook 1.13): a Taskfile include runs in its
includer's directory unless it sets `dir:` (joined to the including file's
directory), and a task's `dir:` is joined to that; lefthook's `root:` and a
GitHub step's `working-directory:` (or its job's or workflow's
`defaults.run`) are joined to the repo root. Inside an included Taskfile a
`task:` / `deps:` call resolves in the include's namespace (`deps: [build]`
in `sub` calls `sub:build`; `:build` calls the root's), while a shell `task
X` starts a new process that resolves X against the Taskfile it finds from
its directory upward. A doc does not say where its reader stands, so an
agent-config command resolves from the repo root or from the doc's own
directory. A templated or absolute directory, and CircleCI's / Azure's
working-directory keys, make a ref not checkable; a folded block (`run: >`)
is read joined and each ref placed back on its source line.

Commands are read for precision, not recall: quoted strings are blanked
and comments cut before a line is read, only the command word of each
simple command is considered (`echo "no task here"` names nothing), and a
YAML corpus is parsed, so `desc:` and comments are never commands. Task
targets come from the parsed Taskfile rather than a `^\s+name:` grep, which
matched any indented key.

## 2c. Packs — deterministic validation shipped as data (CC-12)

A pack is a curated, versioned set of concerns in one CUE file under
`internal/custodian/packs/`, embedded in the binary and parsed by the same
loader (`pack: {name, version, about}` + `concerns`). Hardcoding them in Go
would make fathom's releases the bottleneck for what is plainly content.

| pack | entries | the failure each catches |
|---|---|---|
| `claude-code` | `claude_code_stanza_present`, `agent_config_commands_resolve`, `agent_config_pointers_resolve` | a CLAUDE.md never wired or with a deleted marker; a documented target or script that is gone (O25); a dead pointer |
| `codex` | `codex_stanza_present`, `codex_agents_md_fits` (≤ 32 KiB, `project_doc_max_bytes`), the two `agent_config_*` entries | the same for AGENTS.md; instructions past Codex's read budget silently absent |
| `lefthook` | `lefthook_pre_push_verify` (`under: "pre-push"`), `lefthook_runs_gate` (`task validate`), `lefthook_commands_resolve` | a pre-push hook that no longer runs verify (I17 — a commented-out or pre-commit verify gates nothing) or the gate the docs promise; a hook naming a renamed target or moved script |
| `taskfile` | `taskfile_commands_resolve` | a task calling a missing task, include or script — found at push time otherwise |

Enabling and overriding are law 13's shape: `packs: [...]` in either scope
(the union is enabled; an unknown name is refused with the list); a same-id
entry in the user or project file replaces the pack's wholesale; `<id>:
{off: true}` disables one and the audit names it `off`. Two enabled packs
that ship the same entry (the `agent_config_*` pair) collapse to one
concern naming both; differing definitions are a shipped-data defect,
reported, never an arbitrary winner. Admission (vision 22 §7): every entry
names in `why` the real failure it catches — `TestPacksAreCuratedData`
refuses one that does not. Packs are embedded only; a pack search path
(a kit shipping its own pack) waits for a kit that ships one.

## 3. Surfaces

- **`fathom config audit [--repo R] [--json]`** — the verb. Per concern:
  `✓ holds` (with what was checked: "no match across 4 agent-config
  file(s)"), `✗ violated` (evidence, want, why, provenance scope),
  `~ unable to check` (reason), `– off (this repo)`. Exit 1 iff any
  violation; **no concerns registered exits 0 with the absence named** and
  the two paths to write. `--json` is the observatory's model (law 10: one
  shape, many surfaces).
- **`fathom config propose [--repo R] [--json | --patch]`** (CC-9) — the
  diff-offering verb (vision 22 §2's PROPOSE): for each violated concern a
  unified diff where the mend is mechanical — a dead pointer or dead script
  path whose basename names exactly ONE repo file (a move; zero is "gone",
  two or more "ambiguous", both named) — and the remedy otherwise (prose,
  budgets, renamed targets; fathom's own stanza region goes through `fathom
  init` / `fleet reconcile`, never a hand-applied diff). A line inside any
  fathom marker region (the stanza; the `INTENT.md` playbook) is never
  offered a diff — a hand-edited v3 region reads as drift that reconcile
  refuses (I13), and generated text is overwritten on the next render — so
  it is listed with the regenerating remedy. The relocation index is the
  working tree on git's authority (`git ls-files -z`, kept only when on
  disk): an unstaged move is a move, not an ambiguity. It writes nothing:
  no repo file, no beat, no receipt. `TestConfigPropose_OffersDiffNeverWrites`
  hashes the tree before and after, then applies `--patch` with `git apply`
  and re-audits. Exit 0 whenever it ran; the audit is the gate.
- **Doctor row** (`concerns`) — the capability-row discipline: a load
  error or a violation is the only red; zero concerns is a passing row
  with the absence in its reason.
- **Session-brief line** — on `startup`/`clear` only (the thread-brief
  gate), when violations exist for the repo being opened:
  `⚠️ config: N instruction(s) in this repo's agent config may no longer
  be true — run fathom config audit (fathom reports; this session
  decides).` Its bytes ride the same payload-cost accounting as every
  injected line.
- **Flywheel custodian loop** — deferred to CC-8 (needs an
  attended-within-N-days record to distinguish consumed from accumulating;
  designing that record is the row's work, not a footnote here).

## 3b. Doctor convergence (CC-13, O27)

Doctor rows that a pack fully expresses render the concern audit's
findings instead of re-deriving them (law 10: one evaluation engine, many
surfaces; `cmd/fathom/doctor_packrows.go`):

- `lefthook_gates` ← lefthook `lefthook_pre_push_verify` (its pattern is
  `lefthookInvokes`', held equal by a test), plus the optional review step
  as a doctor-side informational `required_pattern` — optional (C3-1), so
  no pack ships it as a want;
- `stanza_present` ← claude-code `claude_code_stanza_present` + codex
  `codex_stanza_present`, the row joining their file-prefixed details.

The entries are evaluated as shipped (a repo's same-id override governs
its own audit, not these wiring rows). Output is unchanged for users —
`TestDoctorConvergence_RowsUnchanged` runs the retired bespoke checks
beside the converged rows over every fixture and requires identical rows —
with two deliberate differences: a `lefthook.yml` that exists and cannot be
read is named unreadable, where the bespoke check reported it missing
(I38); and a commented-out `fathom verify`, or one only in pre-commit,
fails the row whose reason has always said "in its pre-push commands",
where the bespoke raw-line check passed it (I17,
`TestDoctorLefthookGatesReadsOnlyPrePushCommands`). The capability table's
`VerifyInLefthook` reads the same entry. Not migrated, because no pack expresses them fully: `binary_on_path`
(a PATH fact), `hooks_installed` / `skills_installed` (harness registry,
embedded skills), and every row that asks the daemon.

## 4. Tokenomics

The standing loop is file reads and regexps — zero LLM calls, and zero
daemon calls except `stanza_tools_served`'s one `tools/list` (which
degrades to unable-to-check offline), sub-millisecond per repo otherwise.
The only context cost is the attention
line, which exists only while violations exist, and whose purpose is to
end the far larger cost it flags: an agent obeying a false instruction.
The LLM lane for prose-level truth stays behind FATHOM_METRICS tiers,
priced before it exists (vision 22 §2, unchanged).

## 5. Test obligations (the load-bearing ones)

- loader refuses an unknown check kind naming the concern and the kind;
  refuses `?:` fields; absent file and commented scaffold load as zero
  concerns with no error;
- project scope governs: same-ID replacement, and `off: true` disables an
  inherited concern while the audit output still names it;
- each kind's holds / violated / unable paths, with `file:line` evidence
  asserted (not merely a status);
- the verb's exit codes (0 clean, 0-with-named-absence, 1 violated) and
  the no-concerns text;
- falsification runs: project-governs removed → its test reds; evidence
  collection removed → the evidence test reds.

## 6. Out of scope for v1

The flywheel loop (CC-8); stanza *content* drift as a check kind
(reconcile already owns it; `stanza_tools_served`, CC-14, checks only that
the tools a stanza names are served — a different question); ADR
assertions and the doctrine catalog (vision 23, its own plan phase); any
LLM lane.
