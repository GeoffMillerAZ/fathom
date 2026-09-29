# Incident maps: the plan (ADR-058)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/incident-maps.md` proves it. Design:
`docs/design/incident-maps.md` (section numbers below are its). Decision: ADR-058. Futures: F56 (incident maps) and
F57 (probes and MCP next queries). Moves the incident rows of `docs/plans/exploration-maps.md` here (XM-26 to XM-30,
XM-M6, XM-M7), whose ids stay reserved there.

The build is deferred by the owner. Nothing below starts until its own gate reads: wave 0 builds nothing in the
product, wave F fixes two defects in shipped code whenever the owner chooses, wave 1 needs IM-M0 to read BUILD,
and wave 2 needs IM-M2 to read KEEP and a named consumer (IM-V1). Root rows name (op) readings, never a deferred
row: before G2-6 a deferred row satisfied a `needs:` edge by its disposition alone, and since G2-6 it blocks until
it revives. No row here needs a
provider. A reading's pattern accepts either verdict, so a KILL or DROP reading proves the reading row and would
leave the rows behind it eligible: the change that records such a verdict also marks the wave it gated `[-]` with a
refused disposition. The deferred and refused rows at the end are recorded so they are not re-proposed.

## Wave 0: readings that build nothing

- [ ] (op) IM-V1: a named production consumer is recorded (P20): an adopting repository with a production service, postmortems that cite a cause change, commit-to-deploy lag known, and an owner who asks; fathom's own development incidents do not qualify {id: IM-V1, oracle: lexical, path: docs/bench/readings/incident-maps/consumer.md, pattern: "^IM-V1 consumer: [A-Za-z0-9]"}
- [ ] (op) IM-M0: the revert-corpus replay, at least 50 fast reverts across at least 3 public repositories, two arms at equal list length (seed files only; a mechanical or blind footprint), seeds simulated as untouched files beside the cause, the production git flags, bars fixed before the run {id: IM-M0, oracle: lexical, path: docs/bench/readings/incident-maps/replay-proxy.md, pattern: "^IM-M0 verdict: (BUILD|KILL)", weight: 3}
- [ ] (op) IM-M1: the consumer replay, at least 10 change-caused incidents, the same two arms with footprints recorded before the cause is viewed, ref-range windows, commit-to-deploy lag and the git-invisible share recorded {id: IM-M1, oracle: lexical, path: docs/bench/readings/incident-maps/replay-consumer.md, pattern: "^IM-M1 verdict: (KEEP|DROP)", needs: "IM-V1", weight: 5}

## Wave F: defects in shipped code found by this analysis (independent of the deferral)

- [ ] CODEOWNERS matching follows GitHub semantics: a leading slash anchors at the root, a pattern with no slash (or only a trailing one) matches at any depth, a slash inside a pattern anchors it, and unsupported syntax is named with its line rather than approximated {id: IM-F1, symbol: matchPattern, package: internal/ownership, reachable_from: TestMatchPatternGitHubLeadingSlashAndAnyDepth}
  <!-- product wiring open (fathom verify --strict reads it library-only, 2026-09-29): the matcher and Owners.Unsupported are built and tested, but no production code calls ownership.Parse or Owners.For; the design's claim that it feeds review's reviewers is wrong on this base (review.Input.CodeownersMatches has no producer). IM-10's ownersFacet or a review_diff feed is the first caller; flip when one lands. O135. -->
- [x] Every gitro command runs with GIT_NO_LAZY_FETCH=1 and GIT_TERMINAL_PROMPT=0 besides GIT_OPTIONAL_LOCKS=0, so a partial clone never fetches and git never prompts {id: IM-F2, symbol: Command, package: internal/gitro, reachable_from: TestCommandSetsNoLazyFetchAndNoPrompt}

## Wave 1: the suspect list with no declaration, only if IM-M0 reads BUILD

- [ ] A bounded request-time git read lists commits touching given paths inside a window or a previous..deployed range: committer dates, first-parent landing order, merge files, renames, an -n cap with the remainder counted, a timeout, reverts and bulk commits marked, a shallow clone named as a lower bound, and nothing stored {id: IM-1, symbol: SuspectCommits, package: internal/suspects, reachable_from: TestSuspectCommitsBoundedCommitterDateMergesRenamesShallow, needs: "IM-M0,IM-F2"}
- [ ] The freshness line names the index commit, commits behind the remote-tracking default branch and the last fetch age, and never fetches or ingests {id: IM-2, symbol: incidentFreshness, package: internal/suspects, reachable_from: TestIncidentFreshnessNamesBehindCountAndFetchAge, needs: "IM-1"}
- [ ] The localize recency hop lists the window's commits over the seed files through IM-1 and names its horizon, no longer a zero-yield hop (VB-26) {id: IM-3, symbol: localizeRecencyHop, package: internal/mcp, reachable_from: TestLocalizeRecencyHopListsWindowCommitsOverSeeds, needs: "IM-1"}
- [ ] Commit subjects and other contributor text are stripped of control, ANSI and bidi characters, capped at 80 characters, served in a field labeled untrusted, and escaped in any markdown output {id: IM-4, symbol: sanitizeUntrusted, package: internal/suspects, reachable_from: TestSanitizeUntrustedStripsBidiCapsAndEscapesMentions, needs: "IM-1"}
- [ ] A pasted trace that resolves to repo files gets one hook line with the window's commit count and how to expand, preferring frames in indexed repo files, never a subject, once per session, silent on failure {id: IM-5, symbol: suspectPointerLine, package: cmd/fathom, reachable_from: TestSuspectPointerPrefersRepoFramesAndOmitsSubjects, needs: "IM-3,IM-4"}
- [ ] (op) IM-M2: pointer expansion over pointers shown across 30 days, at least 10 percent keeps the pointer (the XM-M3 rule) {id: IM-M2, oracle: lexical, path: docs/bench/readings/incident-maps/pointer.md, pattern: "^IM-M2 verdict: (KEEP|CUT)", needs: "IM-5", weight: 3}

## Wave 2: the declaration, only if IM-M2 reads KEEP and IM-V1 names a consumer

- [ ] `.fathom/incidents.cue` loads with id, purpose, symptoms, footprint, window (duration or since_tag), runbook path, links and blind spots, and refuses a missing purpose, secret-shaped strings, URL userinfo, token-named query keys, short error literals and non-https links, with valid and invalid testdata {id: IM-6, symbol: LoadIncidents, package: internal/incident, reachable_from: TestLoadIncidentsRefusesSecretsShortLiteralsAndUnsafeLinks, needs: "IM-M2,IM-V1"}
- [ ] Links render window placeholders percent-encoded per component on any https host, commit and key placeholders only on configured link hosts, show their host, and are never fetched {id: IM-7, symbol: renderLink, package: internal/incident, reachable_from: TestRenderLinkEncodesPerComponentAndGatesPlaceholdersByHost, needs: "IM-6"}
- [ ] The answer matches by symptom or footprint with the match reason, at most 3, serves the unprepared view when none match, labels the change facet git-visible and not cause with the blind spots, adds computed dependency inputs, and reads every facet as a value, checked none, or not computed {id: IM-8, symbol: Answer, package: internal/incident, reachable_from: TestAnswerUnpreparedViewFirstAndNamesEveryFacet, needs: "IM-6,IM-1"}
- [ ] A deployed SHA (and an optional previous one) turns the suspect list into the deployed range, names a rollback's removed commits, and reads not computed when the SHA is not in the clone {id: IM-9, symbol: deployedRange, package: internal/incident, reachable_from: TestDeployedRangeRollbackAndShaNotInClone, needs: "IM-8"}
- [ ] Owners come from CODEOWNERS with the matching rule and line shown, then the escalation link; no committer is listed as an owner {id: IM-10, symbol: ownersFacet, package: internal/incident, reachable_from: TestOwnersFacetShowsRuleAndNoCommitters, needs: "IM-8,IM-F1"}
- [ ] `localize` gains an incident section and optional deployed and previous arguments, and the default surface keeps its tool count {id: IM-11, symbol: localizeIncidents, package: internal/mcp, reachable_from: TestLocalizeServesIncidentSectionNoNewTool, needs: "IM-8,IM-9"}
- [ ] The answer admits only point and index work per facet, serves a partial answer naming a late facet, and computes once per declaration id under concurrent requests {id: IM-12, symbol: admitIncidentFacets, package: internal/incident, reachable_from: TestIncidentFacetsPartialOnDeadlineAndSingleFlight, needs: "IM-8"}
- [ ] `fathom incident` serves the answer from a stack file or an alert, and with --offline reads the CUE, CODEOWNERS and the bounded git read in-process, naming index facets not computed {id: IM-13, symbol: cmdIncident, package: cmd/fathom, reachable_from: TestCmdIncidentOfflineNamesIndexFacetsNotComputed, needs: "IM-8"}
- [ ] Readiness facts as a doctor row and a flag: anchors resolve, a non-wildcard owner for the footprint and for the declaration file, runbook path age against footprint churn, URL-only runbooks and unchecked symptoms read unverifiable, symptom collisions listed; nothing graded {id: IM-14, symbol: incidentReadiness, package: internal/incident, reachable_from: TestIncidentReadinessRunbookAgeAgainstChurnAndCollisions, needs: "IM-8"}
- [ ] No package reachable from the incident code imports os/exec except through internal/gitro, and no incident path fetches a URL {id: IM-15, symbol: incidentNoExec, package: internal/incident, reachable_from: TestIncidentCodeReachesNoExecButGitro, needs: "IM-8"}
- [ ] Experiment: an incident record's Outcome names the declaration id (XM-29 moved); the intent audit notes a closed incident that names none, never a failure {id: IM-16, symbol: incidentOutcomeNamesDeclaration, package: internal/intent, reachable_from: TestIncidentOutcomeNamesDeclarationNoteNotFailure, needs: "IM-8"}
- [ ] (op) IM-M3: on real incidents at the consumer, whether the answer named the declaration, whether the suspect list held the cause, its position and the list length; bar fixed before the run (XM-M6 moved) {id: IM-M3, oracle: lexical, path: docs/bench/readings/incident-maps/incidents.md, pattern: "^IM-M3 verdict: (KEEP|DROP)", needs: "IM-11,IM-16", weight: 3}
- [ ] (op) IM-M4: whether a readiness gap (no owner, stale runbook, unresolved anchor) was later confirmed by an incident or review and closed; the report is dropped when no named gap mattered (XM-M7 moved) {id: IM-M4, oracle: lexical, path: docs/bench/readings/incident-maps/readiness.md, pattern: "^IM-M4 verdict: (KEEP|DROP)", needs: "IM-14", weight: 3}
- [ ] (op) IM-M5: cold and warm p95 of the answer for a 500-anchor declaration on the consumer's machine right after an ingest; proposed cold under 3 s and warm under 1 s {id: IM-M5, oracle: lexical, path: docs/bench/readings/incident-maps/latency.md, pattern: "^IM-M5 reading:", needs: "IM-12"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] Probes: read-only command templates rendered as argv under the design's section 6.4 contract {id: IM-D1, disposition: deferred, reason: "a command runner in effect under bypass permissions, no meter, duplicates runbooks", trigger: "F56 promoted, Q32 answered and a sensor matches a run argv to a rendered probe id, and a consumer asks (F57)"}
- [-] MCP next queries addressed to the agent's own servers {id: IM-D2, disposition: deferred, reason: "operator-local aliases and vendor schemas in a committed file are not portable and readiness cannot check them", trigger: "IM-D1 promoted and a portable server identity exists"}
- [-] Footprint pre-resolution stored at ingest {id: IM-D3, disposition: deferred, reason: "a durable memo (R5)", trigger: "responsible-tenant.md#RT-M1 reads PASS and IM-M5 reads over target"}
- [-] A declaration's optional scope naming a live map {id: IM-D4, disposition: deferred, reason: "needs live maps", trigger: "exploration-maps.md#XM-D1 promoted"}
- [-] Symptoms checked against alerts-as-code in the index {id: IM-D5, disposition: deferred, reason: "alert-rule extraction is unverified", trigger: "a consumer keeps alert rules in the repo and their extraction is verified"}
- [-] A read-only console incident view {id: IM-D6, disposition: deferred, reason: "must earn readers; editing and timeline forms are refused", trigger: "a consumer asks after wave 2"}
- [-] A postmortem proposes a declaration diff, never writes it {id: IM-D7, disposition: deferred, reason: "needs a machine-readable cited cause in the repo; importing postmortems means a connector or free text", trigger: "that input exists at a consumer"}
- [-] Companion services (at most 3, already indexed) and a capped 1-hop dependency closure in the change facet {id: IM-D8, disposition: deferred, reason: "cost and noise before evidence", trigger: "IM-M1 or IM-M3 shows causes in shared dependencies or companion services"}
- [-] A committed rendered snapshot per declaration with a CI check, and a hook literal match when the daemon is down {id: IM-D9, disposition: deferred, reason: "worth it only if daemon-down incidents happen at the consumer", trigger: "a consumer reading records a daemon-down incident"}
- [-] Fathom-side connectors that fetch, poll or subscribe to ops systems {id: IM-X1, disposition: refused, reason: "domain-connectors and dispatch non-goals, I14, I56, credentials, liability"}
- [-] Attach: external tools push deploy, flag or alert events into fathom's store {id: IM-X2, disposition: refused, reason: "a connector with the call moved out; reopen only by a new ADR when IM-M1 shows git-invisible causes at 20 percent or more that tag windows and deployed SHAs miss"}
- [-] Fathom executes a probe, fetches a link or calls another MCP server {id: IM-X3, disposition: refused, reason: "command execution by fathom is refused (ADR-056 decision 13); I56"}
- [-] Drills and drill receipts as product state, a last-drill time, a deploy, flag and alert timeline, an incident-tool export {id: IM-X4, disposition: refused, reason: "together they rebuild an incident command center (exploration-maps.md XM-X4)"}
- [-] An incident map as a kind in the map family, and the map verb for it {id: IM-X5, disposition: refused, reason: "the declaration uses none of the map machinery and would have F50's gate both ways (ADR-058)"}
- [-] Frequent committers listed as owners {id: IM-X6, disposition: refused, reason: "selects humans by commit count (I58) for little value"}
- [-] Remote or fetched declarations, values bound from env, keychain or outside files, and a static bind copying indexed facts {id: IM-X7, disposition: refused, reason: "I56, Q5; copies rot"}
- [-] The hook auto-runs anything, presents the change facet as complete, or presents no pointer as no map {id: IM-X8, disposition: refused, reason: "I9, I44"}
- [-] A typed model names a cause, picks the match, judges runtime data, sets severity or proposes mitigation {id: IM-X9, disposition: refused, reason: "I62, R1, R7"}
- [-] The unqualified claim that the incident path has no egress {id: IM-X10, disposition: refused, reason: "the answer and anything the agent does with it reach the model provider"}
