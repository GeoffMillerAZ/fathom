# Eval waves vs D7 — decision record (H5 Stage 3)

**Question:** does the single-wave eval-runner suffice, or does multi-wave QA
(the Update-2 wave-manager design) fire D7's worker-pool trigger?

**Observed (first real `task eval`, 2026-07-15):** 6 scenarios, replay mode,
one judge lane, K=3 determinism probe — completes in seconds, deterministic,
regressions=0. Single-wave is sufficient for the current corpus size and the
current subscriber (this repo's own CI loop). No fan-out pressure exists yet.

**Recommendation (owner decides):** do NOT fire D7 now. The doctrine stands
verbatim: "Wave *policy* (counts, variation schedule, termination predicate)
is CUE data + a pure planner regardless; execution rides D7" (kits/
worker-pool, only when a real fan-out workload subscribes). Re-open when the
corpus grows past what a serial run completes in interactive time, or when
cross-family multi-wave agreement (parked P1–P3) gains a real subscriber.
