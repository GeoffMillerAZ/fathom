# Phase B — Falsifiability (vision 11 §3.B)

Measure or retract. B1 is the program keystone and the only
workload-independent measurement while consumers are paused (vision 11 §6).

Design (normative):
- **B1 context-bench.** 10–15 tasks drawn from real fleet history (find
  callers of X, blast radius of Y, where is Z configured, what governs file
  F), each defined as {prompt, ground-truth answer, scoring rule} in
  `workspace/artifacts/context-bench/tasks/` (fixtures committed under
  `testdata/contextbench/` where repo-stable). Runner executes each task
  twice via Sonnet subagents — arm A with the fathom agent surface
  available, arm B without (Grep/Read only) — recording task success
  (ground-truth match), total tokens, wall time. Output: one
  machine-readable results file + a summary table. The ×10 tokenomics claim
  is retracted from all docs if the fathom arm shows <5× token reduction or
  worse success; the retraction is itself a tracked row.
- **B2 instrumentation only.** The 30-day fold-or-kill clock starts at law
  8's workload precondition, not now. This phase ships only the clock's
  preconditions: the connected-client existence check (A1-1's
  surface_traffic consumed by a check script) and the t0 stamp convention.
- **B3 verify latency SLO.** verify_plan p50 2.2s / p95 4.5s is the one
  expensive companion tool. Budget: explicit p95 ≤ 5s SLO recorded in
  coding-agent-excellence.md §2's ledger; the Stop-hook caller gets a
  fail-open timeout consistent with the 2s hook convention (§5.3) so a slow
  verify degrades to a warning, never a stall.

## Tasks

- [ ] context-bench task corpus: 10–15 tasks with ground truth + scoring rules, committed and documented {id: B1-1, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "benchmark corpus is data, not code; the runner (B1-2) is the tested surface"}
- [ ] context-bench runner: two-arm subagent execution, token/success/time capture, machine-readable results + summary; deterministic scoring unit-tested {id: B1-2, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "runner shells subagents (non-deterministic LLM arms) — scoring functions are the unit-tested core; anchored to the verify CLI it reuses for ground-truth checks"}
- [ ] First bench run recorded; ×10 claim either substantiated with numbers or retracted everywhere it appears {id: B1-3, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "measurement row; evidence is the committed results artifact + doc diffs"}
- [ ] Existence-check script consuming surface_traffic + t0 stamp convention; clock explicitly NOT started (law 8) {id: B2-1, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "thin script over A1-1's tested block; law-8 gating is procedural, recorded in vision 11"}
- [x] verify_plan p95 SLO recorded + Stop-hook fail-open timeout implemented and tested {id: B3-1, symbol: cmdVerify, package: cmd/fathom}
