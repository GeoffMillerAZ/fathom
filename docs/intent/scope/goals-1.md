# Scope: current goals, G2 to G25

Part of the scope router, `../scope.md` (the goal index, the compatibility surface and every section's member).

**G2. The A/B becomes admissible.** Done when: `fathom flywheel` reports ≥20
arm-homogeneous units per arm (MI-5) and a keep/retract verdict is recorded under law 12.
Every production savings claim stays inadmissible until then (0 admissible ON-arm units
on 2026-08-21, `docs/vision/24-work-frontier.md` §7).
{bet: arm-bench, instrument: "fathom flywheel", threshold: "≥20 arm-homogeneous units per arm (MI-5), then a keep/retract verdict under law 12", review: 2026-10-25}

**G3. The continuity-thread bet reaches its sample.** Done when: `threadBetCheck`
reports 20 units and the flywheel's continuity loop states keep or kill
(`docs/vision/21-continuity-threads.md`; 4 of 20 on 2026-08-21).
{bet: thread-bet, instrument: "fathom flywheel", threshold: "20 fresh-with-brief units (threadBetCheck), then the continuity loop states keep or kill", review: 2026-10-25}

**G4. Ambient fathom closes.** Done when: `fathom plans status` shows
`docs/plans/ambient-fathom.md` with 0 open and 0 claimed-unproven rows. Open (4 of 93 on
2026-09-29, 0 claimed-unproven): SH-2 (the owned-keys ledger), SP-10 (the Scope panel's
purge half), RG-9 (the app-kit viz wiring row) and OP-1 (the A/B, G2); BA-6 landed with
wave E. DO-3 and IR-3/4/5 (the assess lanes) are
designed (`docs/design/assess-lanes.md`, ADR-046) and proven 2026-09-25.
{bet: ambient-closes, instrument: "fathom plans status", threshold: "docs/plans/ambient-fathom.md at 0 open and 0 claimed-unproven rows", review: 2026-10-25}

**G9. The bench is honest, cheap to re-run, and the 2026-09-13 reading is the baseline.**
BP-1..25 proved (L0 index oracle, L1 payload replay, L2 corpus gate, per-key cell memoization,
H2-3 registered). The first replicated home run (run-20260913T002832Z) is FAIL on cost
either way: fathom passed 31/33 to grep's 28/33 but returned 1.90× the bytes (O31), and
every headless cell paid a removable ToolSearch turn (O32). Done when: the next L4 (BP-44)
borrows every grep/none cell by key, re-runs only fathom's, and records payload ≤ 1.0× or
why not, after BP-41..43 land. Publication stays an operator decision (Q4, F2).
{bet: bench-payload, instrument: "fathom plans status", threshold: "BP-44 proven in docs/plans/bench-pyramid.md with fathom's payload ≤ 1.0× grep's, or the reading names why not", review: 2026-10-25}

**G13. The Observatory console closes.** Done when: `fathom plans status` shows
`docs/plans/observatory-console.md` with 0 open non-op rows, UX-M1 and UX-M2 are measured
PASS, and every contribution row names its app-kit delivery (vision 35, ADR-045).
{bet: console-closes, instrument: "fathom plans status", threshold: "docs/plans/observatory-console.md at 0 open non-op rows; UX-M1 and UX-M2 read PASS", review: 2026-10-25}

**G14. Fathom is a responsible tenant (ADR-047, vision 36).** The owner's 2026-09-26 priority:
the daemon keeps to a declared `phys_footprint` budget, refuses what cannot fit with a named
reason, never pays repository-sized costs on a request, and cannot relaunch or leak its way
back into the machine. Done when: `docs/plans/responsible-tenant.md` has 0 open non-op rows
and RT-M1 (seven days within budget, no exit rung) and RT-M3 (the stress harness on the
wave-B and post-wave-D binaries) read PASS.
{bet: responsible-tenant, instrument: "fathom plans status", threshold: "docs/plans/responsible-tenant.md at 0 open non-op rows; RT-M1 and RT-M3 read PASS", review: 2026-10-26}

**G15. Language breadth and depth, measured (ADR-048, vision 37).** The owner's 2026-09-27
directive: excellent, measured support for the languages and tool formats people use, most
common first. A language counts at a level only when its committed reading proves it.
Done when: the foundation rows of `docs/plans/language-breadth.md` are proven and LB-M6
reads milestone M1 (20 languages at L1 excellent, 12 at L2, 8 at L3, 6 at L4; 15 tool
formats at target) with LB-M3 (no footprint rise) PASS.
{bet: language-breadth, instrument: "fathom plans status", threshold: "docs/plans/language-breadth.md LB-F rows proven; LB-M6 reads milestone M1; LB-M3 PASS", review: 2026-11-27}
Named 2026-09-27 (ADR-049, vision 38 §9.3): after M1 the ladder continues to vision 37 §5's M2
(45 languages at L1 with 35 excellent, 25 at L2, 14 at L3, 10 at L4; 30 tool formats at
target; LB-M6-M2) and M3 (60+ at L1, 35 at L2, 20 at L3, 12 at L4; 40+ tool formats;
LB-M6-M3). M2 and M3 are the next bets once M1 is read; this goal closes on M1. Today no
language has a committed reading and only Go (and partly SQL) fill `Signature`, so every
current level is an unmeasured estimate until LB-F3 and LB-M1 land.
Observability-as-code formats (Prometheus rules, OpenTelemetry collector configs, the telemetry
identities a deploy declares) are G15 dialects at tier T7, measured as dialects like any other
(ADR-062, 2026-10-09: Stage A identity joins only; `docs/plans/telemetry-links.md`).

**G16. Reach: fathom installs in one step and serves the clients people use (ADR-049,
vision 38).** The owner's 2026-09-27 wave 1. Done when: `docs/plans/reach.md` and the wave-1
rows of `docs/plans/governance-learning.md` are proven and RH-M1 (a stranger reaches a first
proven answer in at most 5 minutes, timed by `stranger-check`), RH-M2 (the plugin installs in
Claude Code and one other client), RH-M3 (default agent surface at most 8 tools), GL-M1
(rules read from at least 6 client formats) and GL-M4 (hooks live in at least 3 clients) read
PASS.
{bet: reach, instrument: "fathom plans status", threshold: "RH-M1 at most 5 minutes install to first proven answer; RH-M2 plugin installs in 2 clients; RH-M3 default surface at most 8 tools; GL-M1 at least 6 rules formats; GL-M4 hooks live in at least 3 clients", review: 2026-11-27}

**G17. The system graph: proof crosses service boundaries (ADR-050).** Done when: the
wave-2 and wave-3 rows of `docs/plans/system-graph.md` are proven and SG-M1 (route, rpc, gql,
topic, table and env links on the fixture fleet at precision at least 0.95 and recall at least
0.85 for the spec and typed tiers), SG-M2 (`contract_drift` catches at least 90% of seeded
breaking changes) and SG-M3 (`system_impact` answered on the home fleet with every link's
tier named) read PASS.
{bet: system-graph, instrument: "fathom plans status", threshold: "SG-M1 precision at least 0.95 and recall at least 0.85 per kind for spec+typed tiers; SG-M2 drift catches at least 90% of seeded breaks; SG-M3 home-fleet reading", review: 2027-01-15}

**G18. Dependencies: upgrade impact from what is already on disk (ADR-050).** Done when: the
dependency rows of `docs/plans/system-graph.md` are proven and SG-M4 (upgrade impact for Go,
npm, PyPI, Maven and Cargo at precision at least 0.9 on the seeded-upgrade corpus) and SG-M5
(the dependency API index is shared fleet-wide, one index per package version, within the
memory budget, with no network access) read PASS.
{bet: dependencies, instrument: "fathom plans status", threshold: "SG-M4 upgrade-impact precision at least 0.9 in 5 ecosystems; SG-M5 one shared index per package version, footprint within budget, zero dials", review: 2027-01-31}

**G19. Spec interop and the certified proof tier (ADR-049).** Done when:
`docs/plans/spec-interop.md` is proven through wave 4 and SI-M1 (5 spec dialects verify in
place), SI-M2 (the converge report on a real Spec Kit or OpenSpec project), SI-M3 (a plan
under `Proof: certified` verifies only on live certificates) and SI-M4 (Jev anchor inference
passes calibration, or is dropped with the reading) read PASS or record the drop.
{bet: spec-interop, instrument: "fathom plans status", threshold: "SI-M1 5 dialects verify in place; SI-M2 converge report read; SI-M3 certified tier live; SI-M4 calibration at least 9 in 10 or the drop recorded", review: 2026-12-15}

**G20. Knowledge retrieval: doc questions answered at grep's rate or better (ADR-049).**
Done when: `docs/plans/knowledge-retrieval.md` wave 2 is proven and KR-M1 (the doc-question
bench slice passes at least grep's rate at no more than 1.0x grep's bytes) and KR-M2 (doc drift
reported by `staleness_report` on seeded symbol changes, recall at least 0.9) read PASS.
{bet: knowledge-retrieval, instrument: "fathom plans status", threshold: "KR-M1 pass rate at least grep's at no more than 1.0x bytes; KR-M2 doc-drift recall at least 0.9", review: 2026-12-15}

**G21. Governance that learns and reaches reviewers (ADR-049).** Done when:
`docs/plans/governance-learning.md` is proven and GL-M2 (promotions and retirements under
the published thresholds, fewer than 1 in 10 promotions refuted within 30 days), GL-M3
(`as_of` answers across claims, memories and learned-rule states) and GL-M5 (SARIF or the
check payload consumed by one external reviewer) read PASS.
{bet: governance-learning, instrument: "fathom plans status", threshold: "GL-M2 fewer than 1 in 10 promotions refuted in 30 days; GL-M3 as_of reading; GL-M5 one external reviewer consumes the output", review: 2027-01-15}

**G22. Progress and economics, computed (ADR-049).** Done when:
`docs/plans/progress-and-economics.md` is proven and PE-M1 (every number on the progress
and economics pages traces to an instrument), PE-M2 (the proof history ledger reconstructs
burn-up and proof regressions for 30 days), PE-M3 (tokens per proven row joined for Claude
Code and Codex sessions) and PE-M4 (the pages measured like G13's UX-M1) read PASS, and PE-M5 (the ledger-pollution
reading: rows under unregistered scratch roots counted and every figure recomputed without them) is published. The plan
gained cost per proven weight point (PE-22) and registered-roots ledger hygiene (PE-23 to PE-29) on 2026-09-29
(ADR-056 decision 10, observation O121).
{bet: progress-economics, instrument: "fathom plans status", threshold: "PE-M1 every number traced; PE-M2 30-day ledger reconstruction; PE-M3 cost per proven row for 2 harnesses; PE-M4 UX reading PASS; PE-M5 ledger-pollution reading published", review: 2026-12-31}

**G23. Test intelligence: the tests that matter, ranked and selected (ADR-051).** Done when:
`docs/plans/test-intelligence.md` rows for T1 to T5 are proven and TQ-M1 (risk-weighted
coverage ranked on the home fleet), TQ-M2 (certifying-first selection re-proves every touched
certificate while running at most 40% of the suite on a replayed month of diffs) and TQ-M3
(coverage ingested for at least 5 languages) read PASS.
{bet: test-intelligence, instrument: "fathom plans status", threshold: "TQ-M1 home-fleet ranking read; TQ-M2 every touched certificate re-proved at no more than 40% of the suite; TQ-M3 coverage for at least 5 languages", review: 2027-01-15}

**G24. Work intelligence: effort, friction and cost joined to proof (ADR-051).** Done when:
`docs/plans/work-intelligence.md` is proven and WI-M1 (U1 to U6 on the console, every number
from an instrument), WI-M2 (U2's interest rate computed on at least 3 repositories with at
least 30 proven rows each) and WI-M4 (U6's preflight A/B read, keep or drop) read PASS or
record the drop.
{bet: work-intelligence, instrument: "fathom plans status", threshold: "WI-M1 U1-U6 traced to instruments; WI-M2 interest rate on at least 3 repos with at least 30 proven rows each; WI-M4 preflight A/B keep-or-drop recorded", review: 2027-01-31}

**G25. Change and compliance receipts (ADR-051).** Done when:
`docs/plans/change-and-compliance.md` is proven and CX-M1 (the risk receipt on every PR in at
least 2 repositories with SARIF or check output), CX-M2 (an evidence pack re-verifies
byte-identically), CX-M3 (one real migration tracked to proven completion), SG-M6 (the
catalog covers every fleet service) and CX-M4 (the drift radar live) read PASS.
{bet: change-compliance, instrument: "fathom plans status", threshold: "CX-M1 receipts on every PR in at least 2 repos; CX-M2 byte-identical re-verify; CX-M3 one migration proven complete; SG-M6 catalog for every fleet service; CX-M4 radar live", review: 2027-01-31}
