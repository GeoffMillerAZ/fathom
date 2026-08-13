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
- concerns sorted by ID after merge; no map order reaches output.

## 2. Check kinds v1 — mechanical, index-free, offline-safe

All four run from file reads on the local tree; no daemon, no LLM, no
network. `where: "agent-config"` is the only corpus in v1 (the loader
refuses other values rather than silently scanning nothing).

| kind | fields | holds when | violated evidence |
|---|---|---|---|
| `forbidden_pattern` | `pattern` (Go regexp), `where` | no line of any agent-config file matches | `file:line` per match |
| `command_documented_and_exists` | `command` | the command string appears in ≥1 agent-config file AND it exists (`task X` → an `X:` target in `Taskfile.yml`; else `$PATH` lookup of argv[0]) | which half failed, named |
| `max_lines` | `file`, `lines` | the file exists and has ≤ `lines` lines | `<file> is N lines (max M)`; absence is a violation (`absent — the concern expects it`) |
| `pointers_resolve` | `where` | every relative markdown-link target (file-relative, fragment stripped; http/mailto/absolute/`#` skipped) AND every backtick repo-path (this house style's pointer form — ≥2 path-charset segments, no globs; runtime dirs workspace/tmp/ref/bin skipped; `../` escapes skipped; a token whose first segment is not a directory the repo even has is a foreign name — Go module, kit package — not a pointer; root-relative or file-relative resolution both accepted) resolves | `file:line → target` per dead pointer |

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

## 3. Surfaces

- **`fathom config audit [--repo R] [--json]`** — the verb. Per concern:
  `✓ holds` (with what was checked: "no match across 4 agent-config
  file(s)"), `✗ violated` (evidence, want, why, provenance scope),
  `~ unable to check` (reason), `– off (this repo)`. Exit 1 iff any
  violation; **no concerns registered exits 0 with the absence named** and
  the two paths to write. `--json` is the observatory's model (law 10: one
  shape, many surfaces).
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

## 4. Tokenomics

The standing loop is file reads and regexps — zero LLM calls, zero daemon
calls, sub-millisecond per repo. The only context cost is the attention
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

`fathom config propose` (the diff-offering verb — CC-9, after audit has
users); the flywheel loop (CC-8); `stanza_current` as a check kind
(reconcile already owns stanza drift; wiring it in duplicates a working
surface until the audit subsumes it deliberately); ADR assertions and the
doctrine catalog (vision 23, its own plan phase); any LLM lane.
