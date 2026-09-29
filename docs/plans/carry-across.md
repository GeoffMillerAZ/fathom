# Carry-across after compaction: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/carry-across.md` proves it. Design:
`docs/design/carry-across.md` (section numbers below are its). Goal G31.

A hook experiment with a stop rule, not a feature. Wave 1 verifies the hook facts and runs Reading 0, a
transcript scan that needs no product change; the build rows carry `needs: CA-M1` and start only if
Reading 0 shows waste. If it passes its A/B, ADR-056 records the reversal of `cmdThreadBrief`'s deliberate
silence on compact (invariant I12 is amended in the same change). No row needs a provider. Beside
`docs/plans/receipts.md` and the continuity-threads plan.

## Wave 1: verify, then read (sections 2 and 3)

- [ ] (op) CA-V1: SessionStart(source=compact) fires in the current Claude Code and accepts additionalContext {id: CA-V1, oracle: lexical, path: docs/bench/readings/carry-across/hook-facts.md, pattern: "^CA-V1 reading:"}
- [x] (op) CA-M1: Reading 0 over 14 days of transcripts: for each compaction the files read or edited in the 40 turns before it, the re-reads in the 20 turns after (split into re-read then edited within 3 turns, and re-read only), and the tokens and turns to the next proof event; the stop rule fires if the median compaction has fewer than 2 re-reads of the pre set or re-reads are under 10 percent of post-compaction reads {id: CA-M1, oracle: lexical, path: docs/bench/readings/carry-across/reading0.md, pattern: "^CA-M1 verdict: (BUILD|STOP)", needs: "CA-V1", weight: 3}

## Wave 2: the injection, only if CA-M1 reads BUILD (sections 4 and 5)

- [ ] At SessionStart(source=compact) inject a computed working set of at most 1,200 bytes with no model and no egress: dirty files with their changed symbols, plan rows touched with their verify state now, the last test run's failing ids, rules served on touched files and not yet resolved, the latest handoff pointer, active map ids {id: CA-1, symbol: carryAcrossBrief, package: cmd/fathom, reachable_from: TestCarryAcrossBriefWithin1200BytesNoModel, needs: "CA-M1"}
- [ ] The working set carries the last test run's failing ids from the test-run hook's sensor when it is enabled, and omits the line when it is not {id: CA-2, symbol: carryAcrossFailingIDs, package: cmd/fathom, reachable_from: TestCarryAcrossFailingIDsOnlyWithSensor, needs: "CA-1,test-run-hook.md#TR-7"}
- [ ] `[hooks] carry_across` is the opt-in switch, default off; PreCompact keeps writing its pointer {id: CA-3, symbol: Load, package: internal/config, reachable_from: TestConfig_HooksCarryAcrossDefaultsOff, needs: "CA-1"}
- [ ] The A/B assignment is a seeded hash of the session id, half the sessions on and half off {id: CA-4, symbol: carryAcrossArm, package: cmd/fathom, reachable_from: TestCarryAcrossArmSeededBySessionHash, needs: "CA-1"}
- [ ] (op) CA-M2: the A/B on the owner's real traffic, at least 150 compactions per arm and at least 7 days: re-reads of the pre set in the 20 turns after compaction drop at least 30 percent with a bootstrap 95 percent CI excluding 0, and tokens-to-proof is not worse; otherwise the injection is removed and thread.go's decision is kept, citing the reading {id: CA-M2, oracle: lexical, path: docs/bench/readings/carry-across/ab.md, pattern: "^CA-M2 verdict: (PASS|FAIL)", needs: "CA-3,CA-4", weight: 5}
- [ ] If CA-M2 passes, `cmdThreadBrief`'s silence on compact and invariant I12's compact clause are amended together, and ADR-056 records the reversal {id: CA-5, symbol: cmdThreadBrief, package: cmd/fathom, reachable_from: TestBriefBudgetAndSourceGating, needs: "CA-M2"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] When-to-compact advice at UserPromptSubmit: a noul over the previous prompt's head and the current prompt, through a daemon endpoint under the deadline, shown to the user only, site `compact_advice` {id: CA-D1, disposition: deferred, reason: "without carry-across it is a generic token heuristic Claude Code already shows, and it is the route site's prompt-classification shape", trigger: "CA-M2 reads PASS and the route site is re-admitted under judgment-foundations.md#JF-8 (F38)"}
- [-] A level 7 port (turn-end compaction questions with token lines) in a harness that owns compaction {id: CA-D2, disposition: deferred, reason: "only a harness that owns compaction can act on the decision; it belongs to agentic-kit, outside fathom", trigger: "agentic-kit's harness adopts it; fathom supplies the carry-across through a CLI"}
- [-] Fathom triggering or performing compaction {id: CA-X1, disposition: refused, reason: "a hook cannot trigger compaction, and the dispatch boundary keeps fathom a store and server"}
