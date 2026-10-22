# Context bill — unchanged hook context once per session

Grammar: v2. Prefix: CB-. Baseline reading: `docs/bench/readings/context-bill/2026-10-22.md`.
Scope: hook-written files only; no daemon tables, caches, jobs, tenancy policy or model calls.

- [x] (op) Record the $0 baseline reading. {id: CB-M1, oracle: lexical, path: docs/bench/readings/context-bill/2026-10-22.md, pattern: "^CB-M1 bill: [0-9]+ bytes"}
- [ ] Post-edit dedupe: unchanged obligations for a file emit nothing after their first injection in an agent session; changed or new content emits in full. SessionStart compact or resume resets the record because earlier context is lost. {id: CB-1, symbol: dedupeHookInjection, package: cmd/fathom, reachable_from: TestPostEditObligationsDedupeWithinSession}
- [ ] Cwd-changed dedupe: key unchanged primers by repository root and content digest within an agent session; returning to a repository emits nothing new. {id: CB-2, symbol: cwdChangedPrimerForSession, package: cmd/fathom, reachable_from: TestCwdChangedDedupeSameRepo}
- [ ] Measure SubagentStart before and after; retain one primer per fresh subagent and cut only sections demonstrably carried by its own SessionStart or instructions. {id: CB-3, oracle: lexical, path: docs/bench/readings/context-bill/2026-10-22.md, pattern: "^CB-3 measurement:"}
- [ ] (op) Re-read after seven days of use: hook bytes per edit-heavy session down at least 40% against CB-M1’s per-session distribution (p95 896458 B and maximum 1657805 B; two largest sessions approximately 0.90 MB and 1.66 MB); no rule missed at first touch (first injection for a file unchanged); dedupe state errors fail open. Record method and evidence before writing the verdict. {id: CB-M2, oracle: lexical, path: docs/bench/readings/context-bill/after-dedupe.md, pattern: "^CB-M2 verdict: (PASS|FAIL)"}

Dedupe state: at most one size-bounded file per session under the existing fathom home hook-state mechanism; existing GC prunes files older than seven days. Read/write errors and absent SessionStart evidence emit the full injection. `[hooks] dedupe = false` disables the optimization (default on). Suppressed-byte beats remain distinct from emitted-byte accounting.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/context-bill.md`.

## Decision models

None applies; dedupe is a digest comparison.
