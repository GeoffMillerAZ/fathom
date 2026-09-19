# Governed workflows implementation

Design: [governed workflows](../designs/governed-workflows.md). Owner authorized full
implementation on 2026-09-13; decisions 0004–0006 remain binding. This checklist is the
work order, not a second design. Contracts and adversarial examples belong in tests.

## Acceptance

- [x] GW-01 Regeneration and its correction ledger use the same injected clock. {symbol: NewPipelineWithClock, package: kits/governed-prose-kit, reachable_from: TestRegenerateUsesInjectedClock}
- [x] GW-02 Foundations import downward; harness telemetry wire values stay compatible. {symbol: TestFoundationsImportDownward, package: specs, skip_test: true, skip: direct executable repository regression test}
- [x] GW-03 Guardrails expose structured findings independently of block/advisory policy. {symbol: PreLLMChain.runInjectDetect, package: kits/guardrails-pipeline, reachable_from: TestInjectionFindingsIndependentOfPolicy}
- [x] GW-04 Encrypted token records survive reconstruction with an explicit persistence port; shredding remains fail-closed. {symbol: NewEncryptedVaultWithStore, package: foundations/security/tokenization, reachable_from: TestEncryptedRecordReconstruction}
- [x] GW-05 MCP dispatch offers explicit pre-call metadata pin verification and refuses missing/drifted tools. {symbol: Client.CallToolVerified, package: foundations/agent/mcp-client, reachable_from: TestCallToolVerified}
- [x] GW-06 A strict engineering profile validates wiring, bounds model attempts and reports actual control coverage. {symbol: NewEngineeringProfile, package: kits/agent-harness, reachable_from: TestEngineeringProfileValidation}
- [x] GW-07 Native Claude Code/Codex configuration is inspectable, reversible and version-probed; unsupported paths cannot pass. {symbol: runHost, package: cmd/kit-new, reachable_from: TestHostCLIPlanExpectedDigest}
- [x] GW-08 Provider capability/identity and an OpenAI Responses lane remove duplicated glue; conformance covers tools, refusals, truncation, usage and streaming. {symbol: Client.completeResponses, package: foundations/llmclient/openai, reachable_from: TestResponsesConformance}
- [x] GW-09 Prose resolves versioned sources, facts and exact claim support; malformed, stale and unsupported claims cannot publish. {symbol: SourcedPipeline.CheckDraft, package: kits/governed-prose-kit, reachable_from: TestSourcedReviewCannotOutliveEvidence}
- [x] GW-10 Corrections invalidate dependent prose and approvals; exports retain a verifiable source map without leaking redacted sources. {symbol: SourcedPipeline.Export, package: kits/governed-prose-kit, reachable_from: TestSourcedArithmeticAndTransitiveInvalidation}
- [x] GW-11 Approval binds actor/operation/state/evidence/expiry; guarded commit and durable effect attempts handle interruption and ambiguous outcomes. {symbol: NewCommitter, package: kits/hitl, reachable_from: TestDurableCommitReconstructionAndReplay}
- [x] GW-12 Versioned lesson bundles validate applicability, independent reproduction, review, adoption and revocation without granting authority. {symbol: CheckAdoptionAt, package: kits/lesson-transfer, reachable_from: TestCheckAdoptionRequiresIndependentReceiverBoundaries}
- [x] GW-13 End-to-end scenarios, a non-Go front door and the adoption guide exercise the supported workflow paths. {symbol: Run, package: scenarios/sourced-prose, reachable_from: TestRunSourcedProseApprovalAndRecovery}

Add exact symbol contracts to a row when its implementation exists, before claiming
completion. GW-02 names the executable repository test itself, so no second test caller
is required. `fathom verify` proves static wiring; `task validate` proves execution.

## Operational acceptance — GW-14

Completed 2026-09-13: pinned consumer checks, actual Throughline/resume_as_code
delivery, native observations, setup/cost measurements and intent refresh are recorded
in the [delivery assessment](../assessments/2026-09-13-governed-workflows.md), with
[offline-verifiable native evidence](../evidence/native-hosts/2026-09-13/README.md).
This non-code work is not a symbol-backed checkbox: static reachability cannot prove
an external command ran. No unrelated symbol is substituted for that evidence.
Throughline and resume_as_code gates pass; Fathom and growing-gardens-bi build but
retain the documented pre-existing test failures. Main remains protected; unsupported
native controls and unmeasured production effectiveness remain explicitly unproven.

## Boundaries

Owner approval on 2026-09-13 includes necessary changes in `throughline` and
`resume_as_code`, their project-local Claude/Codex configuration, and synthetic live
verification capped at $10 total. Preserve existing work. No tags/public release,
tier demotion, global configuration changes, or new dependencies are authorized.
Other consumer trees remain read-only; compatibility checks use isolated copies.
Do not replace genuine host/adoption evidence with simulated success. Missing evidence
remains open even when the underlying library implementation passes its tests.
