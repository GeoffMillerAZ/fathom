---
name: intent-doctor
description: >
  Deterministic health check of a repo's Project Intent wiring: router present and
  within budget, living docs within budget, decisions index matches the records, every
  record has Status and Revisit when, CHANGELOG shape and Release impact line,
  compatibility surface, autonomy and incidents present, stanzas identical in CLAUDE.md
  and AGENTS.md, intent-* skills resolvable for Claude Code and Codex, installed tool
  versions vs the router's Verified-against line. Reports a table; never edits.
---

# intent-doctor

Part of the `intent-*` family. Checks structure and wiring only; `intent-audit` checks
whether the content is still true. Output: one table, `PASS | FAIL | SKIP` per check
with the evidence line. Never edit. Suggest the fixing skill per failure.

Paths: read the router first; it names the decisions directory (`docs/adr/` or
`docs/intent/decisions/`) and any relocated vision doc. `D` below is that directory.

## Checks

| # | Check | How | Pass |
|---|---|---|---|
| 1 | Router exists | `test -f docs/intent/README.md` | exists |
| 2 | Router budget | `wc -l docs/intent/README.md` | ≤ 80 |
| 3 | Living-doc budgets | `wc -l docs/intent/*.md` | each ≤ 150 (`futures.md` ≤ 180) |
| 4 | Required docs | `principles invariants domain scope futures questions autonomy incidents` present | all present, or router says why one is omitted |
| 5 | Index rows = records | count `^\| ` rows minus header in `D/README.md` vs `ls D/*-*.md \| grep -v README` | equal |
| 6 | Record shape | every record has `**Status:**` and `## Revisit when` | all |
| 7 | Trigger registry non-empty | `grep -rcE "(Revisit when\|Promote when\|Drop if\|Done when)" docs/intent D` | > 0 |
| 8 | Every goal has `Done when`; every future has `Promote when` and `Drop if` and a confidence and horizon | grep in `scope.md`, `futures.md` | all |
| 9 | Compatibility surface | `## Compatibility surface` in `scope.md` with ≥ 1 item | present |
| 10 | Sensitive surfaces carry evidence | router table has a "Done needs" column, no empty cells | all filled |
| 11 | CHANGELOG shape | `CHANGELOG.md` exists, has `## [Unreleased]` and `Release impact:` as its first line | present |
| 12 | Unreleased budget | lines in `[Unreleased]` block | ≤ 60 (else `intent-prune`) |
| 13 | Autonomy policy | `autonomy.md` has the three tiers and a branch model | present |
| 14 | Incidents | `incidents.md` present; every open incident has an `Outcome:` line or is under 30 days old | ok |
| 15 | Stanza identical | the `## Project intent` block (routing, change records, autonomy paragraphs) in `CLAUDE.md` and `AGENTS.md`, compared with blank lines removed | identical |
| 16 | Skills resolve, Claude Code | `ls -L ~/.claude/skills/intent-*/SKILL.md` | 8 files |
| 17 | Skills resolve, Codex | `ls -L ~/.agents/skills/intent-*/SKILL.md` | 8 files |
| 18 | Tool versions | `claude --version`, `codex --version` vs the router's `Verified against` line | equal, else WARN (re-verify integration) |
| 19 | Links | every relative link in `docs/intent/*.md` and `D/README.md` resolves | all |
| 20 | Dead pointers | every path named in the router exists (ignore placeholders containing `NNN`, `<`, or `*`) | all |
| 21 | Charter (monorepo only: `docs/intent/repo.md` exists or > 1 `docs/intent/README.md`) | `repo.md` present; its component map lists every directory that has its own `docs/intent/`; every evolution cell uses the fixed vocabulary | all |
| 22 | Nested stanzas (monorepo only) | every component with its own intent has the nested stanza in both `<component>/CLAUDE.md` and `<component>/AGENTS.md`, identical | all |
| 23 | Component hygiene (monorepo only) | no `<component>/docs/intent/autonomy.md`; each component's living docs within budget; its router's first line points to the root | all |

## Output

```
intent-doctor <repo> — 18 PASS, 1 FAIL, 1 WARN
FAIL  5  index has 10 rows, 11 records          → intent-decide (add the row)
WARN 18  router says Codex 0.146.0, installed 0.147.0 → re-verify integration.md
```

Finish with the single most useful next action.
