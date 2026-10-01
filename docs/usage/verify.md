### `verify`

The **done-means-proven gate** (ADR-018): re-parses one or more `docs/plans/*.md`
checklists, queries the live index (HTTP daemon by default, or a local `--store` for
CI/no-daemon), and returns per-task verdicts with file:line evidence. Exit `0` — every
claimed task verifies; exit `1` — a claimed `[x]` task failed verification (block the
push); exit `2` — operational error (daemon unreachable, service unindexed, empty index)
— fail-open, never a false "all claims failed."

```sh
fathom verify --service <id> --plan docs/plans/<slug>.md [--plan <more>...] \
  [--store <path> | --url http://localhost:7575] [--strict] [--require-all] \
  [--concern <id>] [--json | --format text|json|sarif] [--quiet] [--quiet-pass] \
  [--mutation <report>] [--strict-mutation] [--since <ref>] [--dialect <id>] \
  [--ledger <path>] [--converge [--base <ref>] [--write-fixups]]
```

| Flag | Default | Description |
|---|---|---|
| `--plan` | (required, repeatable) | Plan file(s) to verify: a plan-grammar-v2 Markdown checklist, or a foreign spec read in place (ADR-049) — Spec Kit's `specs/<feature>/tasks.md` (`T###` rows), OpenSpec's `openspec/changes/<id>/tasks.md` (numbered rows, requirements linked from the change's spec deltas as `accept:`), Kiro's `.kiro/specs/<name>/tasks.md` (numbered rows, `_Requirements: 1.1, 2.3_` links resolved against the spec's `requirements.md` as `accept:`), Task Master's `.taskmaster/tasks/tasks.json` (tasks and subtasks — dotted ids like `1.1` — `status: "done"` as claimed, anchors from `details`/`testStrategy` text rather than the title), or a GitHub issue/PR task list exported to a file (plain checkbox rows, no id scheme — pass `--dialect gh-tasklist`, since unlike the others it carries no path or shape fathom can detect on its own). A foreign task's contract is an inline fathom annotation, else the first file path it names (a lexical "exists and is non-empty" contract); a task naming nothing verifies `no-contract`. The file is never written; verdicts name its own lines (`(<file>:<line>)` in text, `line` in JSON), and daemon mode sends the plan synthesized into the native grammar |
| `--converge` | `false` | Also print each plan's converge report (ADR-049, `docs/design/spec-interop.md` §5): **claimed-unproven** tasks with their failing criterion, **unanchored** tasks (no contract), and **unclaimed code** — symbols whose definitions overlap a hunk of `git diff -U0 <base>` (working tree included) that no task anchors by `symbol:` or by naming its file. The last section reads the index's symbol ranges for the changed files only and needs `--store`; without it, or outside a git work tree, it says why it was not evaluated. JSON gains a `converge` array. Exit codes stay the gate's |
| `--base` | the spec's creation commit | `--converge` only: the branch range's base ref |
| `--write-fixups` | `false` | `--converge` only: write one open task per claimed-unproven and unclaimed item inside the spec's `<!-- fathom:begin converge -->` … `<!-- fathom:end converge -->` block (appended when absent), in the spec's own checkbox shape (`T###` numbered after the spec's own tasks for Spec Kit, a new top-level number for OpenSpec) and carrying a contract; a re-run replaces the block's contents; text outside the markers never changes (law 15), and a lone marker is refused (exit 2) |
| `--dialect` | detected | Read every `--plan` with this dialect (`fathom-v2`, `speckit`, `openspec`, `kiro`, `taskmaster`, `gh-tasklist`) instead of detecting it by path and content shape; an unknown id is exit 2. `gh-tasklist` is never auto-detected (an exported GitHub issue/PR body carries no path or shape fathom can trust) and always needs this flag |
| `--service` | (required) | Service ID to verify against |
| `--store` | `""` | store path (direct mode, bypasses the daemon; required for CI) |
| `--url` | `defaultFathomURL()` | fathom `serve-http` base URL (HTTP mode; ignored when `--store` is set). The daemon's response budget cuts a long plan's `verify_plan` answer; the cut rows are verified in follow-up calls, and a row whose verdict never arrives reads unknown (`verdict-elided`), never absent from the gate |
| `--strict` | `false` | Treat claimed-but-Unknown tasks as failures too (recommended for CI) |
| `--require-all` | `false` | Wave gate: additionally fail on any open/in-progress task (ADR-018 §2) |
| `--concern` | `""` | Optional concern for contradiction scoping |
| `--json` | `false` | Emit a JSON report + gate result instead of human text (alias of `--format json`) |
| `--format` | `text` | `text`, `json`, or `sarif` — SARIF 2.1.0 of the refusals: one rule per criterion (`symbol_present`, `test_reaches`, `lexical_oracle`, …), each result located at the plan file and row line, so code scanning and editor problem panes render them. `fathom intent audit --format sarif` and `fathom config audit --format sarif` share the emitter. Exit codes are unchanged |
| `--quiet` | `false` | Print only failing rows (and, with `--strict`, unknown rows) plus the summary line -- the gate mode |
| `--quiet-pass` | `false` | Suppress output when the gate passes (exit 0) |
| `--mutation` | `""` (falls back to `$FATHOM_MUTATION_REPORT`) | VB-34: a mutation-testing report (go-mutesting log output, or fathom's own JSON shape — see `docs/use-cases/test-coverage.md`); a claimed row whose symbol is `test_reaches`-covered but has mutation score 0 gets a `covered-not-asserted` note. `--store`/direct mode only — HTTP mode errors (exit 2) if given |
| `--strict-mutation` | `false` | Gate-fail a `covered-not-asserted` finding instead of noting it as advisory |
| `--row` | `""` | Narrow the report and the gate to one row id (the console's row evidence renders `verify … --row ID --json`); every plan is still verified and ledgered in full, and an id no plan holds is exit 2 |
| `--ledger` | `$FATHOM_VERIFY_LEDGER`, else `verify-ledger.jsonl` under `$FATHOM_HOME` or `~/.fathom` | The one verify ledger this run appends to (PE-26): a scratch or test run names its own file so the operator's ledger is never touched; see *Scratch and test runs* |
| `--since` | `""` | ADR-044 §1.4/CV-14 (I45): compare each `--plan` file against its version at this git ref. A claimed row whose `skip_test`/`skip`, `disposition`, `retires:`, `surface_change:`, or a raised `max:` was gained ANYWHERE inside `<ref>`..head (not merely the last edit/commit — the narrower thing no `--since` does) is reported `self-declared` and, for a gained skip, fails the gate exactly as the plain self-skip check does. Text output prints `self-declared: <keys>` under each affected row and, after the summary, `deleted since <ref> (<plan>): <ids>` for rows present at `<ref>` and missing now. Applies only inside a git work tree; a `<ref>` the plan file did not exist at yet counts as every current declaration being self-declared |

A plan row's `certified_by: <Test>` annotation (vision 33 / ADR-043 stage 1,
TC-9, `internal/verify.EvaluateCertifiedBy`) replaces the ordinary
`test_reaches`/`skip_test` requirement: the row verifies only when that test
has a `live` compiled certification (`internal/certify`) scoped to the row's
`symbol`, at or above `strength` (default `asserts`). See
[docs/plans/README.md](../plans/README.md) for the full annotation grammar.

See [docs/plans/README.md](../plans/README.md) for the plan grammar and
[docs/adoption/spec-driven-development.md](../adoption/spec-driven-development.md) for the
full authoring-to-gate loop.

---
