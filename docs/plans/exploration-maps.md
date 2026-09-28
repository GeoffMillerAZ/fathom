# Exploration maps: the plan (vision 40, ADR-056)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/exploration-maps.md` proves it. Design:
`docs/design/exploration-maps.md` (section numbers below are its). Goal G30.

Measurement first. Wave 1 is a transcript-scan reading over 14 days; the wave 2 build rows all carry
`needs: XM-M1` and start only if re-derivation is at least 15 percent of sub-agent reads. The design is
binding for how a map is built (composable by reference, live maps beside semantic layers, maps that
arrive rather than being looked up); the reading decides whether it is built. No row here needs a
provider. Beside `docs/plans/receipts.md` (RC-28, the handoff) and the continuity-threads plan
(`docs/design/continuity-threads.md`).

## Wave 1: is there waste to remove? (section 5)

- [ ] (op) XM-V1: the SubagentStart input is checked for the sub-agent's prompt, and RC-28's 591-byte rows for boilerplate {id: XM-V1, oracle: lexical, path: docs/bench/readings/exploration-maps/handoff-facts.md, pattern: "^XM-V1 reading:"}
- [ ] (op) XM-M1: a 14-day transcript scan: the share of sub-agent Read calls that hit files the parent read or edited before spawning, split into followed by an edit (legitimate) and not followed by an edit (re-derivation); below 15 percent of sub-agent reads kills handoff maps {id: XM-M1, oracle: lexical, path: docs/bench/readings/exploration-maps/rederivation.md, pattern: "^XM-M1 verdict: (BUILD|KILL)", needs: "XM-V1", weight: 3}

## Wave 2: the smallest slice (sections 2 and 4), only if XM-M1 reads BUILD

- [ ] A map entry is an anchor (a symbol key or `path#Lx-Ly`, plus a content hash) with a why line, appended to a thread with `completes_when` {id: XM-1, symbol: appendMapEntry, package: cmd/fathom, reachable_from: TestThreadNoteMapEntryAnchoredWithHash, needs: "XM-M1"}
- [ ] `includes` references between maps form a DAG: shared bases allowed, cycles refused, depth capped at 4 {id: XM-2, symbol: ResolveMap, package: internal/explmap, reachable_from: TestResolveMapDAGRefusesCyclesAndCapsDepth, needs: "XM-1"}
- [ ] The handoff carries ONE id: several maps are combined into a minted composite map made only of references, and a sub-agent can pass its own composite down {id: XM-3, symbol: mintCompositeMap, package: internal/explmap, reachable_from: TestMintCompositeMapCarriesOnlyReferences, needs: "XM-2"}
- [ ] Resolution is a union deduplicated by anchor; conflicting annotations keep both with provenance, the nearest include first; freshness is checked once per entry however many maps include it {id: XM-4, symbol: expandMap, package: internal/explmap, reachable_from: TestExpandMapDedupNearestWinsFreshnessOnce, needs: "XM-2"}
- [ ] A drifted entry is re-resolved by symbol key or flagged stale with its diff size, never served silently wrong {id: XM-5, symbol: resolveEntryFresh, package: internal/explmap, reachable_from: TestResolveEntryFlagsStaleWithDiffSize, needs: "XM-4"}
- [ ] Lifetimes follow references: an ephemeral map is pinned while a live map includes it and its idle clock restarts when the last reference drops {id: XM-6, symbol: mapLifetime, package: internal/explmap, reachable_from: TestMapLifetimeFollowsReferences, needs: "XM-2"}
- [ ] A named map without a purpose line (the question it answers) is refused {id: XM-7, symbol: validateMapPurpose, package: internal/explmap, reachable_from: TestNamedMapWithoutPurposeRefused, needs: "XM-1"}
- [ ] SubagentStart carries one map id inside RC-28's existing 1,200-byte cap {id: XM-8, symbol: subagentDiscoveries, package: cmd/fathom, reachable_from: TestSubagentStartCarriesOneMapIDWithin1200Bytes, needs: "receipts.md#RC-28,XM-3", weight: 3}
- [ ] `fathom map show <id> --budget 1200` re-resolves every anchor at read time and expands includes once, within the budget {id: XM-9, symbol: cmdMapShow, package: cmd/fathom, reachable_from: TestCmdMapShowReResolvesWithinBudget, needs: "XM-5"}
- [ ] `map` lives on the extended surface and the CLI only; the default surface stays at its eight tools {id: XM-10, symbol: isDefaultSurfaceTool, package: internal/mcp, reachable_from: TestDefaultSurfaceUnchangedByMap, needs: "XM-9"}
- [ ] Relevance by location: when a prompt or edit touches code inside a map's anchored footprint, the hook injects one line (purpose, size, freshness, how to expand), once per session and within a budget {id: XM-11, symbol: mapPointerLine, package: cmd/fathom, reachable_from: TestMapPointerLineOncePerSessionWithinBudget, needs: "XM-9"}
- [ ] The session brief names at most 3 maps by recent use and overlap with the thread's recent files, never a full list {id: XM-12, symbol: briefMaps, package: cmd/fathom, reachable_from: TestBriefNamesAtMostThreeMaps, needs: "XM-9"}
- [ ] Thread notes may reference maps, the pre-compact pointer carries active map ids, and resuming a thread re-mounts its maps {id: XM-13, symbol: cmdPreCompactHook, package: cmd/fathom, reachable_from: TestPreCompactPointerCarriesActiveMapIDs, needs: "XM-1"}
- [ ] (op) XM-M2: the A/B on identical briefs, with and without a map: at least 25 percent fewer sub-agent tokens, no accuracy loss, under 5 percent stale entries {id: XM-M2, oracle: lexical, path: docs/bench/readings/exploration-maps/handoff-ab.md, pattern: "^XM-M2 verdict: (PASS|FAIL)", needs: "XM-8,XM-11", weight: 5}
- [ ] (op) XM-M3: pointer precision (expanded over shown) at least 10 percent, and the reuse rate (mounted after handoff) reported; pointers expanded under 10 percent are cut {id: XM-M3, oracle: lexical, path: docs/bench/readings/exploration-maps/pointer-precision.md, pattern: "^XM-M3 reading:", needs: "XM-11"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] Live maps (stored queries recomputed on read) that compose the same way, and graduate to a named concern in a semantic layer when many agents reuse them {id: XM-D1, disposition: deferred, reason: "the designed extension; storage stays in existing thread fields until RT-M1 reads PASS (R5)", trigger: "XM-M2 reads PASS and responsible-tenant.md#RT-M1 reads PASS"}
- [-] Set algebra over maps (intersection, difference) {id: XM-D2, disposition: deferred, reason: "union covers the observed handoffs so far", trigger: "a real brief needs base minus what is already done (F44); dropped if union covers every observed handoff for 60 days"}
- [-] `map show --for '<task>'`: one noul per entry on a map over budget {id: XM-D3, disposition: deferred, reason: "the SubagentStart input likely does not carry the sub-agent's prompt, and the planner already decided what matters", trigger: "XM-V1 finds the prompt in the hook input and a map exceeds its budget in practice"}
- [-] A list of map ids in the brief instead of one composite id {id: XM-X1, disposition: refused, reason: "every sub-agent would re-implement merge, dedup and budgeting, and ids could not be handed down a level"}
- [-] Fathom spawning, scheduling or dispatching sub-agents from a map {id: XM-X2, disposition: refused, reason: "the dispatch non-goal: fathom stores and serves, never spawns"}
