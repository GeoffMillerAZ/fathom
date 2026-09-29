# AG-OP1: the AGENTS.md operator reading

Statistics and row states only. The reading is one line that begins with the row id, which the row's lexical
oracle matches (`docs/plans/agents-md.md`); the sections under it say what was read. Taken 2026-09-29 at
`9ea6da5f`, with no model spend.

AG-OP1 reading: clause 1 holds (`stanza_present` PASS, "native (no CLAUDE file, default mode)"), clause 2 is not read (no transcript line of a fresh session showing the AGENTS.md rules), and clause 3 does not hold (`obligations` with no path served 6 root rules, 6 from ADR.md and 0 from AGENTS.md, which is one of 7 uncompiled scopes), so the row stays open.

## What was read

- **Clause 1, doctor.** `fathom doctor --service fathom`, read-only, from a clean clone at `9ea6da5f`: 53 checks, 40 pass, 4 warn, 0 fail, 9 skipped. `stanza_present` PASS, label "AGENTS.md: fathom stanza well-formed; Claude Code loads it: native (no CLAUDE file, default mode); Claude Code 2.1.285". The label carries the row's quoted text; the version is the Claude Code installed today (2.1.285), one patch above the 2.1.284 the row and the EW-S1 reading name. The EW-S1 reading (`docs/bench/readings/efficiency-wave/stops.md`) took the same row on the main checkout after `task deploy` exited 0 with the daemon serving `087fb5ab`: PASS, native, 2.1.284. `binary_fresh` and `daemon_fresh` also pass here (both at `9ea6da5f`).
- **Clause 2, session.** Not read. Doctor's `stanza_loaded` SKIPs by design: Claude Code from 2.1.277 loads AGENTS.md through the `agents-md` setting without an InstructionsLoaded event, so "no harness-side evidence exists here". No session transcript line showing an AGENTS.md rule was available to this reading: none of the 8 most recent transcripts for this project carries an AGENTS.md instruction block. Seven carry the CLAUDE.md block and one carries none. The newest, the session that dispatched this wave (entries through Claude Code 2.1.284), carries the CLAUDE.md block, and the main checkout it ran in now has AGENTS.md alone (8,338 bytes, no CLAUDE file), so that transcript is not evidence of an AGENTS.md load.
- **Clause 3, obligations.** The `obligations` tool with no path (MCP, service `fathom`, indexed at `9ea6da5f`, layer compiled at watermark `fa8c8ed8`) served 6 root rules, all six sourced from ADR.md (`ADR.md#L220-L221`, `#L1189-L1191`, `#L221-L221`, `#L219-L221`, `#L1419-L1424`, `#L2172-L2174`) and none from AGENTS.md. The layer is partially compiled: 32 of 89 sections, `budget_stopped` false, `corpus_grew` 42, 223 rules displaced. `uncompiled_scopes` names 7: ADR.md, the root AGENTS.md, the four nested AGENTS.md (cmd/fathom, docs, internal, schema) and docs/plans/README.md. Doctor's `obligations_served` WARNs on the same partial compile.

## What closes it

- **Clause 2** closes with one line from a fresh Claude Code session (2.1.284 or later) opened in a checkout that carries AGENTS.md and no CLAUDE file, quoting a rule the transcript shows it loaded. That reading is the operator's; it was not produced here.
- **Clause 3** closes when the obligations layer compiles the root AGENTS.md, so that `obligations` with no path serves at least one AGENTS.md-sourced rule. The recompile is model spend, which the budget hold refuses; doctor's remedy line names the command (`fathom compile --service fathom --layer obligations --dry-run=false --continue`). Nothing was compiled here.

## What this reading does not say

- It does not close AG-OP1: the row's three clauses are joined, and two of them are open.
- It does not say the served rules are wrong. ADR.md-sourced root rules are the rules the compiled part of the layer holds; the reading says only that the AGENTS.md rules are not in it yet.
