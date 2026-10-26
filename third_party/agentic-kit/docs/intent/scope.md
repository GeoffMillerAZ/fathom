# Scope

Where the edges are right now. `docs/roadmap.md` holds the capability backlog;
`TRACKING.md` holds every capability's checkbox; this file is the current position with
exit tests. Done goals are deleted, not archived.

## Current goals

Owner-directed strategy and design uplift (2026-10-22), under 0025; cross-cutting G15 under 0034:

| Goal | Done when |
|---|---|
| G9. Executable practice and assurance catalogs | Versioned patterns, antipatterns and applicable checks compose through Catalog Kit; an independent expected-scope check catches omitted criteria; two workflow reviews retain findings and evidence. |
| G10. Evidence and data quality through the lifecycle | A real workflow traces source versions through claims, decisions and effects; conflict, access, freshness, deletion and changed-input cases exercise the controlled path. |
| G11. Portable, economical decisions and investigations | Capability-aware OpenAI/Anthropic/Google conformance and native exceptions are explicit; a bounded investigation escalates on declared signals and reports cost per accepted outcome against a baseline. |
| G12. Human and agent workbench | A consumer proves propose/review/commit/reconcile with scoped authority, accessible evidence inspection and stale-consent refusal; attended-task effort is measured. |
| G13. Governed prose harness | A consumer corrects source facts and annotations, rebuilds a declared dependency closure, compares variants and cannot publish a blocked or stale artifact. |
| G14. Reviewed improvement flywheel | A bounded, deduplicated issue/journal intake yields a sanitized lesson, an independent regression and a second consumer's explicit acceptance; host configuration coverage is measured separately. |
| G15. Governed open-ecosystem interoperability | The same controlled task and required interface profile preserve evidence, refusals, budgets, authorization and recovery across two independently implemented peers/hosts; profiles disclose loss and unsupported cases, and upstream review records a receiver-reviewed contribution or reasoned no-upstream disposition. |

Designs and exits: [uplift program](../designs/agentic-engineering/README.md); [inventory/evidence/review contracts](../designs/inventory-evidence-review.md), 0037, refine G9..G14 without new runtime admission.
G12/G13 are anchor experiences under [0038](decisions/0038-prose-and-hitl-anchor-the-uplift.md); these goals authorize design, not simultaneous implementation incubations.
Implementation admission and the overdue-review disposition are recorded by 0025.

**G1. Ratify D4: does the kit go public, and is it a 1.0 candidate?** Done when: Geoff
records the decision (`decisions/`), which unblocks H8 Stage B
(`docs/plans/handoff-h8-public-candidacy.md`, 5 open rows). Status: blocked on Geoff
since the uplift plan closed at v0.6.0. This does not block the workflow uplift;
the 2026-09-12 strategy review does not authorize public release or a tag.

Sequence and scenarios: [governed workflows](../designs/governed-workflows.md).
Incubation does not itself authorize promotion, deployment, paid runs, gate weakening
or consumer mutation. The owner's separate 2026-09-13 authorization covers necessary
consumer-a/consumer-c changes, their project-local Claude/Codex configuration
and synthetic live verification capped at $10 total; no global configuration changes.
Maintenance and review owner: the kit maintainer (currently Geoff); agents execute
bounded work under `autonomy.md`. Use 0004's promote / revise / retire criteria at review.

## Current boundaries

[0034](decisions/0034-open-ecosystem-interoperability.md) accepts AAIF alignment design,
not ten integrations. [Profiles and queued work](../designs/aaif-alignment.md) retain
0003's MCP revision clock and the two-slot admission ceiling; G15 is not yet met.

- [0032](decisions/0032-admit-offline-practice-review-and-local-diagnostics.md) admits W1's second slot; 0033 adds decision sites, 0035 command qualification, 0036 assignments, 0039 inspection/compact views, 0040 Git reads and [0041](decisions/0041-harness-config-authenticates-selected-sources.md) signed harness configuration. Independent coverage, five engineering days, zero paid runs, <=64 MiB corpus, review 2026-11-21/next release. [GT delivery](../plans/governed-command-tools.md) retains remaining command/hook work; no third slot, installation, whole AE closure, production adoption or measured benefit follows. Local quality-gate diagnosis retains its slot.

- [0042](decisions/0042-concern-sites-require-independent-disclosure-pins.md) adds W1's file-output concern receiver, explicitly built with `command_classifier`, over independently pinned public/synthetic inputs and exact policy bytes. The ordinary artifact rejects selected controls before source/scanner work. Its bounded loopback transport is not qualified local inference or private-input permission. Same W1 budget; no paid/native model call or installation follows. [MC](../designs/command-concern-classifier.md) keeps native quality/residency/calibration work open under GT12.

- [0043](decisions/0043-maintenance-grants-bind-native-approval-to-receiver-state.md) admits [HMT](../designs/harness-maintenance.md) within W1: exact native-bound advisory plans and protected receiver transactions, with a build-only macOS signing helper. No key creation, live prompt, hook installation, third slot or full GT9..11 qualification follows. Reboot/clock/crash ambiguity holds; existing mandatory floors remain.

- [0044](decisions/0044-governed-spawn-checks-preserve-required-controls.md) admits [HSE](../designs/harness-spawn-economics.md) within W1: required spawn evaluation at full coverage, exact pinned policy/evaluator, private HC8/HMT integration and stable advice-observation sampling. Legacy hook behavior stays unchanged. No installation, global mutation, journal, native consent or actual coding-host interception is claimed.

- [0045](decisions/0045-prove-session-governed-prose-headlessly-before-ui.md) makes [HP](../designs/prose-headless-proof.md) the active reference in the existing second slot: immutable session goals/document types, headless prose/HITL behavior and bounded real Anthropic/Jev proof before UI. The first paid stage is <=$25 with synthetic inputs; prior command-tool no-inference limits remain. Command expansion waits, its full goal stays open, and no third incubation or UI implementation is admitted.
- [0046](decisions/0046-separate-document-sufficiency-from-length-targets.md) follows HP's retained failed first cell: the next study distinguishes advisory length from justified required ranges, keeps independent sufficiency and a required long-artifact case, and never rewrites original failures. HP-10 implements the offline policy/shape; required long-document and independent sufficiency proof remain open.
- [0047](decisions/0047-qualify-oracle-semantics-before-prompt-tuning.md) follows HP-17's retained failed paired trial: admit only the command-local offline oracle packet/report and independent whole-floor adjudication under0045. Preserve original labels/results; freeze any revised semantic floor before new calibration. No new paid study, library, UI or third slot follows.

- Command-local `quality-gate` is a bounded offline reference admitted by [0030](decisions/0030-quality-boundaries-compose-hard-rules-and-calibrated-holds.md): exact Git snapshots, hard rules and optional calibrated holds. No consumer/global hook installation or live model quality is implied. [0031](decisions/0031-bind-working-methods-to-deterministic-boundaries.md) refines runtime/context trust, required coverage and invocation budgets; broader working methods remain composition designs under the existing uplift work order. Review by 2026-11-21 or next release; F18 stays unpromoted.

- Experimental Catalog Kit composes the existing catalog and decision foundations under
  [0023](decisions/0023-catalog-workflow-composes-existing-foundations.md). Offline resolution
  and preparation include profiles, exact definition binding and discovery tools.
  Isolated Review Kit/Env Kit pilots preserve domain laws; production adoption and live
  model quality remain separately gated in [the work order](../plans/catalog-kit-uplift.md).
  Identity, independent adoption and safe developer workflows follow
  [0024](decisions/0024-independent-kits-share-identity-not-authority.md) and its
  [acceptance specification](../designs/kit-interoperability.md).

- Five library roots: `foundations`, `kits`, `runtimes`, `surfaces`, `toolchains`.
  Counts and classifications live in `docs/catalog.md` / `docs/compat.md` rather than
  being duplicated here. `cmd/`, `examples/`, `scenarios/`, `specs/` are outside that surface.
- Zero external dependencies; two nested modules (`kits/docextract/{pdf,xlsx}`) with
  their own tags. Go 1.26, CUE 0.16.
- Four inspected consumers: fathom, consumer-a, consumer-b, consumer-c.
  They use a mix of sibling workspaces, replacement modules and vendored snapshots.
  The declared downstream task covers all four; current prepared-snapshot results
  and prerequisites are in the [integration evidence](../evidence/catalog-kit/2026-10-21/downstream.md).
- LLM adapters: anthropic, bedrock (+awscred), openai (+Azure), claudecli, codexcli.
- CLI providers are text-completion lanes; they reject caller-defined tools. Native
  host adapters are a separate installed-policy boundary, with no universal parity claim.
  G5's bounded setup exit passed in 142 seconds with tested version/scoped receipts;
  [native evidence](../evidence/native-hosts/2026-09-13/README.md) leaves strict Claude
  editing and Codex live coverage unproven. Human usability remains unmeasured.
- G7's bounded transfer exit passed in kit and consumer-c `7a6c56a`: actual
  code-byte provenance, independent negative receivers, a bound adoption receipt and
  tested revocation. Removing only the classification check makes the seeded false-pass
  test fail. This is retrospective synthetic evidence, not measured recurrence reduction.
  [Delivery](../assessments/2026-09-13-governed-workflows.md); review under 0009 remains due.
- Consumer-a's experimental guardrails upgrade is explicitly pinned and accepted
  under [0010](decisions/0010-pinned-experimental-adoption.md), not promoted to stable.
  F8/F9 run through its actual default-vendor gate; H8-1 is an explicit opt-in MCP
  dispatch policy with tested drift refusal and documented remote race limits.
- Evidence-comparison incubation [0011](decisions/0011-workload-selection-evidence.md)
  extends the existing evaluator; [0012](decisions/0012-composable-engineering-assessments.md)
  adds domain assessment profiles to the fitness registry and configurable prompt
  screening to guardrails. [Design](../designs/engineering-assessments.md): two bounded
  reference domains, hard/soft separation, unknown evidence and held execution.
  Live screening quality, model selection and broad profile adoption remain unproven.
  The 2026-10-16 review fired; 0025 retains offline maintenance and parks further
  expansion pending a bounded admission. This portfolio opens no new implementation
  incubation; the two-slot ceiling still applies.
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
