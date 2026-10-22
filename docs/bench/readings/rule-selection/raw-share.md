# RS-M4: raw share of served rows over 14 days of edit-hook serving (2026-10-21, $0)

Plan row `RS-M4` in `docs/plans/rule-selection.md`, ADR-077's third revisit trigger: raw rows (ADR-037's fallback, served uncompiled) above 50% of served rows.

## Verdict

RS-M4 raw share: 38.9% (an upper bound)

At most 4,775 of the 12,249 rule rows the edit hook served from 2026-10-08 02:17Z to 2026-10-22 02:17Z were raw. The trigger has not fired. The naive classification (every row whose citation names no stored claim counts as raw) reads 50.6%, but serving prepends at most two raw rows per call (`RawFallbackReserve`), so every unresolved row past two in one call is a compiled row whose claim has since been recompiled away; capping each call at two gives the bound.

| Service | Compiled (resolved) | Unresolved | Unresolved share |
|---|---:|---:|---:|
| fathom | 3,040 | 3,821 | 55.7% |
| app-kit | 2,827 | 1,762 | 38.4% |
| platform-engineering | 58 | 290 | 83.3% |
| workstation_as_code | 3 | 185 | 98.4% |
| agentic-kit | 106 | 148 | 58.3% |
| conjure | 5 | 4 | 44.4% |
| all | 6,039 | 6,210 | 50.7% |

Unresolved rows per call: 0 in 446 calls, 1 in 167, 2 in 1,773, 3 to 7 in 531. A call with three or more holds at least one compiled row the store no longer names, so the per-service shares above overstate raw.

## Method (re-computable)

1. Where the record lives: `~/.fathom/hooks.jsonl` (the hook log) records each edit-hook answer's claim-bearing row count (`meta.claims`) but neither the total rows nor their tier, so it cannot give the share. The daemon's call log (`workspace/logs/mcp-calls.jsonl` of the deployed checkout) records every `obligations` call with `origin: edit-hook` and its served rows' citations (`pointers`, after the edited file itself). Both were copied read-only; nothing was written to either, or to the daemon.
2. Window: the 14 days ending at the log's last `obligations` row: 2,917 edit-hook calls, 2,491 with rows, 12,249 rows.
3. Classification: a row is compiled when its citation equals a `cites` link of a stored claim (any status) in the read-only store copy RS-M1 used, else unresolved. A raw row's citation is a section window (`citeWindow`), which no compiled claim carries. Bound: raw rows in a call are at most min(unresolved, 2).
4. Computed by a short script over the copied call log and store ($0, no model call). What would make it exact: the call log naming each served row's tier (a log field, no table).
