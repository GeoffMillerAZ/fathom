---
name: intent-doctor
description: >
  Run structural checks of a repo's Project Intent wiring: router present and
  within budget, living docs within budget, decisions index matches the records, every
  record has Status and Revisit when, CHANGELOG shape and Release impact line,
  compatibility surface, autonomy and incidents present, the stanzas in AGENTS.md
  alone (a CLAUDE file only imports it), intent-* skills resolvable for Claude Code and Codex, installed tool
  versions vs the router's Verified-against line, cross-scope ids and the AGENTS.md chain
  budget in monorepos. Reports a table; never edits.
---

# intent-doctor

Part of the `intent-*` family. Checks structure and wiring only; `intent-audit` checks
whether the content is still true. Output: one table, `PASS | FAIL | SKIP` per check
with the evidence line. Never edit. Suggest the fixing skill per failure.

Paths: read the router first; it names the decisions directory (`docs/adr/` or
`docs/intent/decisions/`) and any relocated vision doc. `D` below is that directory.

Select scope from the requested paths, not only cwd. Check root plus affected
components; enumerate all owned routers only for a repository-wide check. Exclude
vendored practice snapshots, templates and examples. Component omissions pass when
the router explicitly inherits or points elsewhere. Read root autonomy, never require
a duplicate. Count locally owned records separately from external pointers, accept
the repository's documented Status header style, and check parent links relative to
their files. The first paragraph may link to root after the title.

## Checks

| # | Check | How | Pass |
|---|---|---|---|
| 1 | Router exists | `test -f docs/intent/README.md` | exists |
| 2 | Router budget | `wc -l docs/intent/README.md` | ≤ 80 |
| 3 | Living-doc budgets | `wc -l docs/intent/*.md` | each ≤ 150 (`futures.md` ≤ 180) |
| 4 | Required docs | `principles invariants domain scope futures questions autonomy incidents` present | all present, or router says why one is omitted |
| 5 | Index membership | compare the record-ID column's local link targets with files; exclude external pointers and cross-references in other columns | no missing/duplicate local records |
| 6 | Record shape | Status and Revisit when in the documented local format; append-only legacy exceptions identified | all or explicit exception |
| 7 | Trigger registry non-empty | `grep -rcE "(Revisit when\|Promote when\|Drop if\|Done when)" docs/intent D` | > 0 |
| 8 | Every goal has `Done when`; every future has `Promote when` and `Drop if` and a confidence and horizon | grep in `scope.md`, `futures.md` | all |
| 9 | Compatibility surface | `## Compatibility surface` in `scope.md` with ≥ 1 item | present |
| 10 | Sensitive surfaces carry evidence | router or explicitly linked surface table has a "Done needs" column | no empty evidence cells |
| 11 | CHANGELOG shape | `CHANGELOG.md` exists, has `## [Unreleased]` and `Release impact:` as its first line | present |
| 12 | Unreleased budget | lines in `[Unreleased]` block | ≤ 60 (else `intent-prune`) |
| 13 | Autonomy policy | `autonomy.md` has the three tiers and a branch model | present |
| 14 | Incidents | `incidents.md` present; every open incident has an `Outcome:` line or is under 30 days old | ok |
| 15 | Stanza in one file | the `## Project intent` block (routing, change records, autonomy paragraphs) is in `AGENTS.md`; no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` exists, or each carries `@AGENTS.md` (`@../AGENTS.md` from `.claude/`) and no second copy of the block | one copy, reachable |
| 16 | Skills resolve, Claude Code | inspect declared project-local or personal skill path | each declared skill resolves; identify source and shadowing |
| 17 | Skills resolve, Codex | inspect declared project-local or personal skill path | same bodies as Claude Code; loading remains a separate host check |
| 18 | Tool versions | `claude --version`, `codex --version` vs the router's `Verified against` line | equal, else WARN (re-verify integration) |
| 19 | Links | every relative link in `docs/intent/*.md` and `D/README.md` resolves | all |
| 20 | Dead pointers | every path named in the router exists (ignore placeholders containing `NNN`, `<`, or `*`) | all |
| 21 | Charter (monorepo only: `docs/intent/repo.md` exists or > 1 `docs/intent/README.md`) | `repo.md` present; its component map lists every directory that has its own `docs/intent/`; every evolution cell uses the fixed vocabulary | all |
| 22 | Nested stanzas (monorepo only) | every component with its own intent has the nested stanza in `<component>/AGENTS.md`; a nested CLAUDE file only imports it | all |
| 23 | Component hygiene (monorepo only) | inherited authority, local document budgets, first paragraph links to root, goals have one owner | all |
| 24 | Cross-scope citations (monorepo only) | ids (`F1`, `G1`, `I3`, `ADR-n`, `NNNN`) cited bare in one scope but defined in another map component or repository | each written `<scope> <id>` |
| 25 | Instruction chain (nested `AGENTS.md` present) | `wc -c` of the root `AGENTS.md` plus every `AGENTS.md` down to the deepest component; the root routing table | chain ≤ 32 KiB (Codex stops reading there); the root table names every component |

Use a repository's deterministic checker when available; name its coverage. Link and
budget checks are suitable for scripts. Do not claim a full doctor from that subset.
Report installed, configured, invoked, enforced and adopted independently. A version
command is not a host integration test. Reuse existing authorization for fixes outside
this read-only skill; a FAIL does not itself require another permission prompt.

## Output

```
intent-doctor <repo> — 18 PASS, 1 FAIL, 1 WARN
FAIL  5  index has 10 rows, 11 records          → intent-decide (add the row)
WARN 18  router says Codex 0.146.0, installed 0.147.0 → re-verify integration.md
```

Finish with the single most useful next action.
