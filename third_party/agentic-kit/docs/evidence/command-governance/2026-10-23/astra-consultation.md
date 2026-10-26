# Astra architecture consultation

2026-10-23, one fresh Herdr Codex session. Requested and observed model: GPT-6-Astra,
high effort. Session `01a169c3-a058-7db0-9614-60ea89ff0cb3`; temporary pane `wC:p3`.
The consultation was read-only, with no implementation, providers/services, tests,
installation, consumer mutation or subagents. The response is design advice, not proof.

## Recommendations adopted

- Extend W1's second offline slot; do not turn quality-gate into arbitrary execution
  or economic fail-open agent-hooks into a safety boundary.
- Audit by minimization: closed categories and bounded counters, not raw commands or
  optimistic redaction. Do not publish arbitrary basenames or unsalted command hashes.
- Typed read/search/immutable-Git operations precede shell authorization. Root handles,
  exact revisions, protected paths and actual effect/disclosure gates need runtime proof.
- Policy layers combine authority/delegation, not filename precedence. Candidate files
  are review subjects; org floors, exact exceptions and independent scope remain intact.
- Existing decision ports support none/replay/local/advanced profiles, but every model
  output remains evidence. Localhost is not proof of privacy or ownership.
- Fathom contributes revision/freshness-bound context, never permission or runtime proof.
- One-approved hook tuning is a scoped expiring maintenance transaction with rollback
  and independent evaluation. Mandatory controls remain active; no blanket disable.
- Preserve secret-sentinel, opaque-shell, omitted-scope, stale-approval, lease/crash/clock
  and concurrent-tuning negative cases. Do not build another interpreter/policy engine.

Root decision: CG-1..7 ships offline qualification first. CG-8..16 specify typed execution,
scanner/model/engine adapters, delegated policy, hook maintenance and harness review as
separate queued work. The consultation did not read the requested daily brief;
the root subsequently located today's scheduled run and verified linked primary sources.
Final independent review and gates belong
to the implementation delivery, not to the consultant's recommendation.
