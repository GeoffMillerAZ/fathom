# Test certifications — the plan (vision 33, ADR-043)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/test-certifications.md`
proves it. Op rows carry a `skip` naming the operator decision; they are never
silently skipped. Design: `docs/design/test-certifications.md`.

## A. Stage 0 — facts only (no vocabulary, no spend)

- [x] test-kit's manifest and result schemas are mirrored under `schema/testkit/` at a pinned commit and drift-gated like `third_party/` {id: TC-1, oracle: lexical, path: schema/testkit/manifest.cue, pattern: "#TestManifest"}
- [x] `fathom tests ingest --manifest <dir> --results <file|dir>` loads `tests/*.cue` and attested `#TestResult` envelopes into additive store tables (`test_suites`, `test_cases`, `test_runs`), each row carrying the service watermark {id: TC-2, symbol: IngestTestEvidence, package: internal/testevidence, reachable_from: TestIngestTestEvidence_ManifestAndEnvelope}
- [x] Every envelope case is joined to a `Test*` symbol by exact name (`TestX/sub` joins to `TestX`); the join rate is a reported fact {id: TC-3, symbol: JoinCasesToSymbols, package: internal/testevidence, reachable_from: TestJoinCasesToSymbols_ExactNameAndSubtest}
- [x] `fathom tests status` prints, per suite, class, tier, the last run's verdict and totals, and `joined N of M cases`; "no manifests found" is a named skip, never a clean table {id: TC-4, symbol: cmdTestsStatus, package: cmd/fathom, reachable_from: TestCmdTestsStatus_JoinRateAndNoManifests}
- [x] The Go extractor reads a `// certifies:` marker above a `Test*` function into a declared certification fact; a malformed marker is an ingest warning naming file and line {id: TC-5, symbol: ParseCertifiesMarker, package: internal/extractor, reachable_from: TestParseCertifiesMarker_GrammarAndMalformed}
- [ ] (op) TC-M1: envelope-to-symbol join rate measured on fathom's own repo and on test-kit's `examples/`; bar ≥ 80 %, else the envelope needs a case-to-symbol contract before stage 1 {id: TC-M1, skip: "measured: docs/bench/readings/test-certifications-stage0.md — 100% (9/9) on test-kit examples/runnable, 94.76% (4542/4793) on fathom ./internal/...; both clear the 80% bar, verdict pass"}

## B. Stage 1 — the certifications layer and the gate

- [x] The `certifications` layer compiles deterministically from suite declarations plus markers times the joins (exists, reaches, covers, kills, last attested run): one claim per (target, test) with the tier ladder and the state ladder from the design {id: TC-6, symbol: CompileCertifications, package: internal/certify, reachable_from: TestCompileCertifications_TierAndStateLadders}
- [x] Targets resolve deterministically (invariant, incident, spec, rule, port) or the certificate is `contradicted`; a `behaviour` target never exceeds doc-claimed {id: TC-7, symbol: ResolveTarget, package: internal/certify, reachable_from: TestResolveTarget_EveryKindAndUnknown}
- [x] The post-ingest sweep moves a certificate to `stale` when its test or a reached symbol moved, `contradicted` when the test is gone, and updates state from a newly ingested run {id: TC-8, symbol: SweepCertifications, package: internal/certify, reachable_from: TestSweepCertifications_MovedGoneAndNewRun}
- [x] `fathom verify` accepts `certified_by: <Test>` (repeatable, optional `strength`) and returns `certified` only for a live certificate from that test for the row's symbol; `certificate-unproven`, `certificate-broken`, `certificate-unreachable` and `certificate-absent` never prove a row {id: TC-9, symbol: EvaluateCertifiedBy, package: internal/verify, reachable_from: TestEvaluateCertifiedBy_EveryVerdict}
- [x] `review_diff` and `fathom review` gain the `certifications` dimension: certificates per changed symbol with strength and state, targets that lost a live certificate, and `checked: false` with a reason when nothing is declared {id: TC-10, symbol: DimCertifications, package: internal/analysis/review, reachable_from: TestReview_CertificationsDimension}
- [x] `fathom certify status` and `fathom certify gaps [--kind] [--json]` list every declared target with no live certificate, each with the remedy on the line; the JSON shape is versioned for the Review Kit adapter {id: TC-11, symbol: cmdCertifyGaps, package: cmd/fathom, reachable_from: TestCmdCertifyGaps_RemedyLinesAndJSON}
- [x] `fathom certify manifest [--out] [--check]` regenerates the certifications artifact from the layer and fails `--check` when the committed file differs; a hand edit is a defect {id: TC-12, symbol: cmdCertifyManifest, package: cmd/fathom, reachable_from: TestCmdCertifyManifest_RegeneratedAndCheck}
- [ ] Receipts gain kind `certification`; `fathom receipt verify` re-derives reach with go/types and reports `pass: unverified (no run)` unless `--run-tests` runs that one test {id: TC-13, symbol: verifyCertificationReceipt, package: cmd/fathom, reachable_from: TestReceiptVerify_CertificationKind}
- [x] `fathom doctor` row `certifications`: declared, live, join rate; "no manifests found" is a skip {id: TC-14, symbol: checkCertifications, package: cmd/fathom, reachable_from: TestDoctorCertifications_Row}
- [ ] fathom's own repo declares certificates for its deny-severity invariants and its open incidents' named tests, in `tests/fathom.cue` and markers {id: TC-15, oracle: lexical, path: tests/fathom.cue, pattern: "certifies"}
- [ ] `docs/configuration.md` documents `[tests] results_dir` and `artifacts_dir`; `docs/usage.md` documents the verbs; `docs/mcp-tools.md` documents the review dimension {id: TC-16, oracle: lexical, path: docs/configuration.md, pattern: "\[tests\]"}
- [ ] (op) TC-M2: two weeks after stage 1 lands, ≥ 50 % of fathom's deny-severity invariants carry a live certificate, else the declaration path changes {id: TC-M2, skip: "operator measurement: two-week dogfood window"}

## C. Stage 2 — the judge, the standard, the projections

- [ ] test-kit (platform-engineering): `#Certifies` and `#Suite.certifies` land as an ADR-0042 amendment, `surveyor validate` accepts them, one example manifest carries one {id: TC-17, skip: "lands in platform-engineering; proven by that repository's own gate, recorded here by commit"}
- [ ] Review Kit (platform-engineering): requirement `testing.certified-targets` with an evidence check through adapter `fathom-certify`, a finding rule with a remedy, and the adapter calling `fathom certify gaps --json` through the fathom bridge {id: TC-18, skip: "lands in platform-engineering; proven by that repository's own gate, recorded here by commit"}
- [ ] An incident `Outcome:` line may name `certificate <Test>`; `certify gaps --kind incident` treats a live one as closed and everything else as a gap {id: TC-19, symbol: IncidentCertificates, package: internal/certify, reachable_from: TestIncidentCertificates_OutcomeGrammar}
- [ ] `certify gaps --kind port [--fleet]` lists every exported interface with implementations and no `conformance` certificate, keeping each implementation's evidence tier {id: TC-20, symbol: PortGaps, package: internal/certify, reachable_from: TestPortGaps_ImplementationsWithoutConformance}
- [ ] (op) TC-M3: the Review Kit requirement fires `deny` on a real change within a month of stage 2, else it is theatre and comes out {id: TC-M3, skip: "operator measurement: one-month window"}

## D. Stage 3 — adjacent, each with its own bar

- [ ] Flakiness: a flake in the last three attested runs makes the certificate `flaky`, served at `suspect` tier, never silently live {id: TC-21, symbol: CertificateState, package: internal/certify, reachable_from: TestCertificateState_FlakyFromLastThreeRuns}
- [ ] `fathom certify orphans`: `Test*` symbols that certify nothing, reach no symbol changed in 180 days, and never failed in any ingested run {id: TC-22, symbol: cmdCertifyOrphans, package: cmd/fathom, reachable_from: TestCmdCertifyOrphans_ThreeConditions}
- [ ] `fathom tests select` ranks tests that certify a target in the changed files first, then the existing reach ranking; advisory {id: TC-23, symbol: rankCertifyingFirst, package: cmd/fathom, reachable_from: TestTestsSelect_CertifyingFirst}
- [ ] `fathom certify suggest` asks the decision port one batched request per unregistered test (strength choice, asserts noul); proceed-band answers become suggestions in `workspace/artifacts/certify-suggestions.jsonl`, rendered in `certify gaps`; never a claim; `provider = none` prints one line and exits 0 {id: TC-24, symbol: cmdCertifySuggest, package: cmd/fathom, reachable_from: TestCmdCertifySuggest_BandsAndNone}
- [ ] (op) TC-M4: suggestions accepted ≥ 50 % of the time over the first twenty, else `certify suggest` is removed {id: TC-M4, skip: "operator measurement: needs twenty suggestions reviewed"}

## Deferred (trigger on file)

- [-] Contract certificates on cross-repo `service_ref` edges (an HTTP client call or a Helm route paired with the contract test that certifies both sides) {id: TC-25, disposition: deferred, reason: "single-repo layer first", trigger: "TC-M2 passes and two services in the fleet declare port certificates"}
- [-] The `// certifies:` marker for TypeScript, Python, Java, Kotlin extractors {id: TC-26, disposition: deferred, reason: "Go carries the typed reach today; other languages reach via SCIP import", trigger: "a non-Go service ingests a test-kit envelope"}
- [-] A docs-kit projection of the manifest through test-kit's doc-source contract {id: TC-27, disposition: deferred, reason: "the consumer owns the port it reads; needs docs-kit's schema pinned here", trigger: "docs-kit publishes the contract version this manifest would target"}
- [-] A one-line "no live certificate governs this file" in the edit hook {id: TC-28, disposition: deferred, reason: "the edit payload is over its 150-token budget on half of edits (Q21)", trigger: "Q21 answered and a bench row measures the line"}
