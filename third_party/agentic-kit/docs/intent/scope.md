# Scope

Where the edges are right now. `docs/roadmap.md` holds the capability backlog;
`TRACKING.md` holds every capability's checkbox; this file is the current position with
exit tests. Done goals are deleted, not archived.

## Current goals

**G1. Ratify D4: does the kit go public, and is it a 1.0 candidate?** Done when: Geoff
records the decision (`decisions/`), which unblocks H8 Stage B
(`docs/plans/handoff-h8-public-candidacy.md`, 5 open rows). Status: blocked on Geoff
since the uplift plan closed at v0.6.0. This does not block the workflow uplift;
the 2026-09-12 strategy review does not authorize public release or a tag.

Sequence and scenarios: [governed workflows](../designs/governed-workflows.md).
Incubation does not itself authorize promotion, deployment, paid runs, gate weakening
or consumer mutation. The owner's separate 2026-09-13 authorization covers necessary
throughline/resume_as_code changes, their project-local Claude/Codex configuration
and synthetic live verification capped at $10 total; no global configuration changes.
Maintenance and review owner: the kit maintainer (currently Geoff); agents execute
bounded work under `autonomy.md`. Use 0004's promote / revise / retire criteria at review.

## Current boundaries

- Five library roots: `foundations`, `kits`, `runtimes`, `surfaces`, `toolchains`.
  Counts and classifications live in `docs/catalog.md` / `docs/compat.md` rather than
  being duplicated here. `cmd/`, `examples/`, `scenarios/`, `specs/` are outside that surface.
- Zero external dependencies; two nested modules (`kits/docextract/{pdf,xlsx}`) with
  their own tags. Go 1.26, CUE 0.16.
- Four inspected consumers: fathom, throughline, growing-gardens-bi, resume_as_code.
  They use a mix of sibling workspaces, replacement modules and vendored snapshots.
  At the assessment baseline, the committed downstream task covers the first three;
  the fourth consumer's proposed check is an uncommitted, mutating Taskfile change.
- LLM adapters: anthropic, bedrock (+awscred), openai (+Azure), claudecli, codexcli.
- CLI providers are text-completion lanes; they reject caller-defined tools. Native
  host adapters are a separate installed-policy boundary, with no universal parity claim.
  G5's bounded setup exit passed in 142 seconds with tested version/scoped receipts;
  [native evidence](../evidence/native-hosts/2026-09-13/README.md) leaves strict Claude
  editing and Codex live coverage unproven. Human usability remains unmeasured.
- G7's bounded transfer exit passed in kit and resume_as_code `7a6c56a`: actual
  code-byte provenance, independent negative receivers, a bound adoption receipt and
  tested revocation. Removing only the classification check makes the seeded false-pass
  test fail. This is retrospective synthetic evidence, not measured recurrence reduction.
  [Delivery](../assessments/2026-09-13-governed-workflows.md); review under 0009 remains due.
- Throughline's experimental guardrails upgrade is explicitly pinned and accepted
  under [0010](decisions/0010-pinned-experimental-adoption.md), not promoted to stable.
  F8/F9 run through its actual default-vendor gate; H8-1 is an explicit opt-in MCP
  dispatch policy with tested drift refusal and documented remote race limits.
- Evidence-comparison incubation [0011](decisions/0011-workload-selection-evidence.md)
  extends the existing evaluator; [0012](decisions/0012-composable-engineering-assessments.md)
  adds domain assessment profiles to the fitness registry and configurable prompt
  screening to guardrails. [Design](../designs/engineering-assessments.md): two bounded
  reference domains, hard/soft separation, unknown evidence and held execution.
  Live screening quality, model selection and broad profile adoption remain unproven.
  One active implementation incubation; review due 2026-10-16 or next release.
  The owner's operational follow-through is now admitted under
  [0013](decisions/0013-operational-selection-evidence.md), with the required consumer,
  selection/reuse, screening, retrieval and measurement exits in the
  [completion ledger](../plans/2026-09-operational-evidence.md). Live spending remains
  separately gated; no production model switch is implied.
  On 2026-09-21 the owner directs completion without further paid calls:
  [0014](decisions/0014-offline-delivery-live-study-followup.md) defers the remaining
  live study, not implementation, review or offline verification. Operational code and
  offline delivery now pass in the normal vendored consumer; the ledger identifies all
  receipts and remaining fleet-gate failures. Historical evidence remains scoped and
  expired; the consumer's study TODO is delivered, not a passing empirical check.

## Compatibility surface

Changes to these are breaking: `!` on the commit, `### Breaking` in `CHANGELOG.md`, a
migration note in `docs/upgrades/` when the fix exceeds one sentence, and the same-commit
`TRACKING.md` update `compat.md` demands.

- Exported Go API of every `stable` and `internal-ish` package under the five roots.
  Evidence: `docs/compat.md` §Promise.
- Typed-error sentinels of `stable` packages (`errors.Is`/`errors.As` behavior).
- `schema.cue` of `stable` packages (fixtures included).
- Nested-module tags and the `go.work` layout consumers depend on. Evidence: `compat.md`
  §v1 decision record.
- `cmd/constitution-lint` check semantics: consumers inherit the kit's gate expectations
  through `fathom-pack`.

Pre-1.0: a breaking change to `stable` still bumps minor and still gets every line above.
`experimental` may change, rename, or retire at any time; a `CHANGELOG.md` line is still
written.

## Non-goals

Long-lived boundaries live in `VISION.md`. The ones an agent is most likely to re-propose:

- **Guaranteed semantic truth or universal best-practice compliance.** Deterministic
  link validation cannot prove that a source is true or entails a generated claim.
  Publish the scope and quality evidence for each workflow (`assurance.md`).
- **A global memory service or automatic policy propagation.** Lessons travel as
  reviewed, versioned artifacts; local workloads retain authority and private storage.
  Revisit when: two consumers need guarantees file-based distribution cannot supply (0006).

- **A sandbox or isolation claim for `bounded-exec`.** Why: confinement is not
  isolation; stdlib cannot deliver it. Revisit when: never; consumers add isolation.
- **A deletion story for data written outside the tokenization vault.** Why: journals,
  event stores, and cassettes are append-only by design. Revisit when: a consumer's
  retention regulation names a specific store; then it is a decision, not a feature.
- **Runtime per-call capability re-checking for the trifecta mechanism.** Why: a
  deliberate seam, not an oversight (`threat-model.md`). Revisit when: a consumer
  reports a capability changing mid-run.
- **An executor for `coordination/*`.** Why: uplift D7; plan-only until a real fan-out
  workload subscribes. Revisit when: `futures.md` F8's trigger fires.
- **The deprecated MCP HTTP+SSE transport.** Why: removed from the 2025-11-25 spec and
  absent from Claude Code's client. Revisit when: never.
- **AssumeRole / role-chaining in `bedrock/awscred`.** Why: no named consumer routes
  through it (D8). Revisit when: a consumer names the need.
- **A dependency, however small.** Why: I1 is constitution, not marketing (uplift D1).
  Revisit when: never; the WASM lane and the zero-conflict promise depend on it.
- **Porting `_reference/` sources wholesale.** Why: mine, don't port (uplift D8).
  Revisit when: never.
