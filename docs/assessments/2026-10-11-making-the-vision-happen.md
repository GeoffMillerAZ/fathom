# How to make the vision happen

Owner-architect assessment, 2026-10-11, revised after critique; summarized into the repository 2026-10-11. Read at
fathom `c39875f3` and platform-engineering `84f9d41`; adapted to the records at `65e5acfa`. Companion to
`2026-10-11-reviews-and-incidents.md` (reviews, breach prevention and incidents) and
`2026-10-11-index-snapshots.md` (index snapshots and environment daemons).
Decisions it drove: ADR-069 (all fifteen decisions, and decision 10's ports-and-adapters boundary in particular),
vision 44, G37 and its 2027-01-31 kill, the provisional G38, F71 to F74, and the owner rulings D1 to D19 below as
recorded there. The pe-side records (D17) are drafted in platform-engineering, not here.
Conventions: plain paths = fathom; `pe:` = platform-engineering; `rk:` = `pe:kits/review-kit/`; [J] marks a
judgment (a *hypothesis*); sourced rows without [J] are *measured* at the read commit; external facts carry a URL
and are *claimed by a vendor* where they are a vendor's. Measurement ids are the plans' `WAE-` and `IXS-` ids.

## 1. Does fathom belong at the center?

**Partly. Fathom is the center of the evidence plane, not of the platform.**

Arguments for the center:

- Every vision item needs the same thing: what the code and config at commit C do, with source-linked proof.
  Fathom already has the parts: a portable store keyed by commit (ADR-017), dialect parsing of manifests,
  proxies, alerts, IAM and IaC at L3 (docs/languages.md:180-276), routing and alert joins
  (internal/fusion/alertroute.go:47-51), telemetry-to-code (ADR-062 Stage A), git-native anchors
  (internal/receipt/anchor.go:33-50) and offline receipt verification (EVX-6).
- Prevention and incident response become one query at three refs: PR head (review), deploying commit
  (gate), deployed commit (page). Only an engine keyed by commit can serve all three.
- Vision 07 R1 already names the hosted strategy: "the file travels, you never hold the data"
  (docs/vision/07-questions-and-risks.md:26).

Arguments against putting fathom at the center of everything:

- The user-visible product for reviews is Review Kit: catalog, severity, findings, admission, lens export.
  ADR-055 d7 put the well-architected and ops domains there (ADR.md:3222-3229).
- Change-risk scoring already exists in pe: `pdk/risk` scores `pulumi preview --json` and rendered-manifest diffs
  into approval tiers with structural and compliance floors (pe ADR-0080; pe:pdk/risk/model.go:1-15;
  pe:pdk/risk/gates/compliance-below-floor-refused.cue:4-6).
- Index snapshots' transport, retention and freshness enforcement are CI work and the operator's registry. ADR-063
  forbids a second package graph, verifier or result format (ADR.md:3957).
- The incident actor is an external agent. The AI SRE field is funded and crowded (Resolve.ai, reportedly valued at
  $1B, https://siliconangle.com/2025/12/22/incident-remediation-startup-resolve-ai-reportedly-valued-1b-new-funding-round/;
  Datadog Bits GA, https://www.datadoghq.com/about/latest-news/press-releases/datadog-launches-bits-ai-sre-agent-to-resolve-incidents-faster/),
  and vendor agents already read source repositories and run scheduled sweeps
  (https://learn.microsoft.com/en-us/azure/sre-agent/overview).
- Fathom has a bus factor of one, a budget hold on model spend, and a language-excellence program as its main
  line. A hosted service operated by fathom would consume all three.

[J] Fathom's defensible ground is determinism, commit-pinned answers, replayable receipts, cross-layer joins and
self-hosting. It should be the best context source the other actors have, not a fourth actor.

**Semantic layers or a better feature?** (owner vision 2) [J]: the deterministic fact sheet is the better
feature. In an active repository compiled layers fall behind their corpus within days: this repository's
obligations corpus took 106 commits in the 14 days to 2026-10-11, and refreshing costs model spend
(`docs/bench/readings/self-use/2026-10-11-compiled-layer-staleness.md`, hygiene note G10, which also corrects the
first note's "24 of 103 sections stale within the hour"). They stay opt-in judged sites under the spend hold; the
fact sheet is computed, $0 and commit-pinned.

## 2. What to build, and who owns it

| Component | Owner | Status | Notes |
|---|---|---|---|
| Concern facts + joins + coverage (reliability and security) | fathom | partial (joins for alerts ship; facts not stored) | `2026-10-11-reviews-and-incidents.md` sections 3.3, 3.4 |
| Fact receipts (`export receipt --kind fact\|join`, both at `fathom@<version>`) | fathom | EVX-9 open | grows ADR-068 d4's kind table |
| `export snapshot` + cache key printout + `--cache-from` ingest + parity check | fathom | parts exist | `2026-10-11-index-snapshots.md` section 9 |
| Code reads over an imported index snapshot (`import snapshot`) | fathom | **new subsystem**, L, gated | `2026-10-11-index-snapshots.md` change 10; mounted stores federate obligations only |
| `serve-http --read-only` (four-part definition) + answer basis | fathom | new | F8's real shape (its network form awaits U1) |
| Ref-pinned `read_source` | fathom | new | incident and review correctness |
| Reliability catalog, bundles, lens mapping, defaults pack | Review Kit | 3 findings, all off (rk:internal/catalog/assets/data-reliability/pack.cue:6-8) | the critical path |
| `fathom-facts` adapter, `anchors[]`, anchor state, index_commit check | Review Kit | reserved lane (rk:internal/evidence/adapters.go:444-453) | pe-side record needed (section 6, D17) |
| Admission (deploy gate) | Review Kit `admit` | exists (rk:README.md:57) | no fathom gate verb; fail-open on fathom failure |
| Change-risk tiers; concern facts as named reasons | pe `pdk/risk` | shipped (pe ADR-0080) | fathom deltas enter `#ChangeContribution.reasons` (pe:pdk/risk/model.cue:43-51), a pe-side additive change; no new gate |
| Drift attestation (`pulumi preview --json`, Crossplane XR diff, flux status) | pdk/risk parses previews and diffs (pe:pdk/risk/pulumi.go:43, pe:pdk/risk/manifest.go:31); the publisher job attests | partial | fathom only resolves addresses |
| Publish job, retention, the deploy job that writes the lineup, PR bot | the operator's CI: **a plain GitHub Actions job first** | none | not ci-kit: ci-kit is "a library and a set of ports", `cmd/ci` deliberately empty, and adopting it is "standing up an execution plane" (pe:kits/ci-kit/README.md:3-6,14-35); its futures say do not build "an interface for a consumer that does not exist" (pe:kits/ci-kit/docs/intent/futures.md:10). A move into ci-kit needs a pe-side scope record |
| Image digest -> commit, signing, referrers | image-kit | SLSA provenance on the bake path only; apko records none (pe:docs/adr/0115-image-kit-ci-signs-keyless-or-by-key-without-a-public-log.md:51-56) | deploy state without a connector, bake path |
| Toolchain digest, fathom release lock, index snapshot pull in devenv | env-kit | v0.2.0 | cache hits need the release lock |
| Attested runs, incident regression tests | test-kit | v0.1.0 | `#Certifies` joins certify gaps |
| Runbooks as DocSets, alert->runbook coverage | docs-kit | v0.1.0 | |
| Resource and security rules, framework crosswalks, mitigation approval controls | policy-kit (Kyverno interim) | design only (pe:docs/architecture/policy-kit.md:14-22); not among pe's kits (pe:kits/ holds ci, docs, env, image, review and test kits) | not on this quarter's critical path |
| Incident-responder skill, evidence-before-claim doctrine | agentic-kit | | |
| Freshness column in the observatory | fathom (app-kit doctrine) | GKE-MH-6's library exists; its tool was removed by SR-17 | new wiring; no incident view |
| OCI registry, OIDC proxy, GitOps controller | operator | external | flux in pe (pe:docs/adr/0005-cluster-topology.md:18,28) |
| AI SRE / mitigating agent | external | HolmesGPT, vendors, own agent | reads fathom over MCP |
| **A new service** | none | | [J] nothing here needs one; the environment daemon is a recipe the operator runs |

## 3. Exact fathom changes, in order

Each is a plan row in `docs/plans/well-architected-evidence.md` (WAE-) or `docs/plans/index-snapshots.md` (IXS-).

| Order | Change | Code | Why first |
|---|---|---|---|
| 1 | Findings importer hashes at `SourceRevision` (WAE-1) | stamp at internal/findings/external/importer.go:256; disk read at pkg/client/findings.go:56-66 | a live revision-skew defect on the Review Kit path |
| 2 | `version --json` gains epochs, format version, fact-kind versions (WAE-2) | cmd/fathom/version.go:80-90 | cache key and freshness gate inputs; XS |
| 3 | `read_source` takes `ref:` and reads `git show` (WAE-3) | internal/mcp/readsource.go:170-188 | review, incident and receipts read the commit, not the disk |
| 4 | Machine-local fields: spec and code agree (IXS-0) | docs/spec/store-format-v0.md:160-163; internal/store/ingeststate.go:49-52; internal/store/derived.go:165-171 | a defect to fix before any index snapshot is exported |
| 5 | FG0: concern facts stored as observations of their own file and served by `symbol_context` (WAE-4, WAE-5) | DocFacts read only in-pass today (internal/fusion/alertroute.go:397,515); precedent internal/fusion/logtemplate_pass.go:7-10 | every concern row depends on it |
| 6 | FG2: Prometheus and Alertmanager fact lines (WAE-6) | internal/extractor/dialects/prometheus/doc.json:23-26; internal/extractor/dialects/alertmanager/doc.json:39-42 | cheapest, joins already ship |
| 7 | FG1: LB-F451 widened, env names and `valueFrom` only, arg flag names only, sentinel test (WAE-7, WAE-13), under an ADR-067 d12 amendment | internal/extractor (k8sContainerFacts, docs/plans/language-breadth.md:993) | most incident-frequent manifest facts |
| 8 | TL-15/TL-16 observed-by | docs/plans/telemetry-links.md:35-36 | alert-per-workload |
| 9 | `ingest --cache-from <file>` with a parity test (IXS-1) | cmd/fathom ingest; ingest-diff is a full ingest today (cmd/fathom/ingestdiff.go:110-120) | IXS-M0b and IXS-M1 need a product path |
| 10 | Joins J1 (probe->handler->callee), J2 (workload->alert->receiver), security S1 to S3 and S6 (WAE-8, WAE-9, WAE-14) | request-time, "computed, never stored" (ADR-058 d2) | the value no linter has |
| 11 | `export receipt --kind fact\|join` (WAE-10) | EVX-9 (docs/plans/portable-receipts.md:40) + ADR-068 d4 table | Review Kit transport |
| 12 | `serve-http --read-only` + answer basis (IXS-3, IXS-2) | store.MountReadOnly (internal/store/conformance.go:201); autorefresh off (internal/mcp/autorefresh.go:31,227); bare `/rpc` gated (internal/mcp/dispatch.go:376-381) | a read-only sidecar for agents and review runners; index-snapshot phase 3, parked |
| 13 | FG4 Envoy/Istio policy facts; J3 retry x timeout chain (WAE-11, WAE-12) | internal/extractor/dialects/envoy/doc.json:14-18; kubernetes doc.json:65-72 | the owner's flagship, config side |
| 14 | FG7 code client construction (Go first), only past WAE-M1 | internal/extractor/goextract.go:591-598 | the flagship, code side |
| 15 | `rollback` review dimension, only past IM-M0 BUILD | cmd/fathom/review.go | mitigation input |
| 16 | `import snapshot` (IXS-7), only for a named cross-repo consumer and past IXS-M1 KEEP | cmd/fathom, internal/store | code reads over imported index snapshots |
| 17 | Lineup-keyed `$HOME` state (IXS-5) | internal/store/scoped.go:16-24 | only before a second team uses an environment daemon |

## 4. Missing today, covered by nothing

1. **Resilience code facts.** No record mentions retry, backoff, jitter, timeouts, breakers or pool
   reservation; the nearest seams are F31 (capability reach) and F52 (concern scope).
2. **Security and compliance joins.** The dialects exist (IAM, Kyverno, Gatekeeper, Rego, Azure Policy under
   internal/extractor/dialects/) but no record joins them to workloads and code (`2026-10-11-reviews-and-incidents.md` section 3.4).
3. **Coverage on absence facts as a contract field.** ADR-068 d4 says absence facts carry read sets, but no
   fact kind for concerns exists to carry it.
4. **A library-defaults pack** (`pkg@version` plus env facts -> behaviour). Nobody owns it yet; it should be
   Review Kit's.
5. **Rendered per-environment config.** Helm values have no scalars (internal/extractor/helmextract.go:233-241);
   Kustomize overlays are not applied. No row.
6. **An index snapshot lifecycle**: export step, cache key, invalidation, retention, parity. F27 refused part of
   it until ADR-069.
7. **Code reads over an imported index snapshot.** Mounted stores federate obligations only
   (`2026-10-11-index-snapshots.md` section 1).
8. **A calibration corpus of incidents** that proves a bundle would have caught them. F4's corpus is
   postmortem import; IM-M0 is a revert replay. Neither tests concern bundles.
9. **Alert investigative value** as a judged property (runbook, owner, code, paging route). F59 links only.
10. **An agent contract**: which fathom answers a mitigating agent may rely on, with the answer basis and tiers.
11. **A CI contract**: ADR-063 says selection by blast radius is "not specified", and depgraph has no wire
    format (pe:kits/ci-kit/depgraph/types.go:99-105).
12. **Retention** of deployed commits' index snapshots for rollback gates and incident replay, with a cost cap
    (not as evidence: ADR-069 decision 15).
13. **Q5's threat model**, open since vision 20 (docs/intent/questions.md:16-19).

Not a gap, an existing component to integrate: pe's `pdk/risk` (section 2).

## 5. Build order and capacity

Fathom waves are worker waves of one to three days; language excellence stays the main line, and these ride
as fillers until R3 (`2026-10-11-reviews-and-incidents.md`) clears.

**Language wave correction.** Vision 38 section 9.4 schedules "lf7 (ops formats, Kubernetes facts, observed-by)"
(docs/vision/38-proof-layer-roadmap.md:261). The 2026-10-11 ruling moved ops formats (K8s pod facts, GHA, Helm,
Jenkinsfile, TL-15) to lf8, because the web frameworks needed TypeScript and HTML first, and lf7 launched 10:55Z
with tsx, shell, vuesvelte and angularastro. Until these records, no committed record said lf8, and LB-F451
carries no wave (docs/plans/language-breadth.md:993). Vision 44 section 6 now records the ruling, and vision 38
section 9.4's own clause is the owner-architect's edit with the index-snapshot roadmap entry; wave 2 below rides lf8
and asks that LB-F451 be named in it. At most two waves run at once, and none starts during an RT-M1
window with under 15 GiB of swap free (docs/vision/38-proof-layer-roadmap.md:258-259).

### Wave 1 (next): $0 readings and XS fixes

- Owner: WAE-M0 corpus and stratified labels (40 alerts and 30 workloads by a committed sampling rule, not
  whole repositories); decisions D1 to D19 in section 6.
- fathom: changes 1 to 4; prune F67; park F66 with a re-census date; refresh TL-2 and TL-3 reasons.
- Readings: IXS-M0 (cold, warm and cache-from ingests from a clean checkout, 20 commits); WAE-M1 (hand count on
  WAE-M0).
- Review Kit: index_commit check; anchor fields (`anchors[]`, `blobId`, `digestScope`, `anchorState`).

### Wave 2: facts (rides lf8; starts only after Review Kit commits the bundle's requirements)

- fathom: changes 5 to 8. Exit WAE-M2 (precision >= 0.95, store growth <= 5%, sentinel in no row). Change 9
  (`--cache-from`) and its exit IXS-M0b are index-snapshot phase 1, parked until the owner starts it.
- env-kit locks the fathom release.

### Wave 3: joins and transport

- fathom: changes 10 and 11. Exits: per-join bars WAE-M3 and WAE-M6; EVX-M1 on a fact receipt. Change 12
  (`--read-only` and the answer basis) and IXS-M3 are index-snapshot phase 3, parked.
- Review Kit: `fathom-facts` adapter and a 10-rule reliability bundle (alert quality, probes, resources,
  rollout, PDB, observed-by). Exit: one finding closed on pass with complete coverage.

### Capacity [J]

Estimates in agent worker-days (wd) and owner hours, on the ADR-068 d13 precedent ("about 5.5 worker-days" for
two filler waves, 16.5 in all, ADR.md:4593-4597).

| Wave | Fathom wd | Owner hours | pe-side wd (Review Kit, env-kit) |
|---|---|---|---|
| 1 | 3 | 10 (labels 6, decisions 3, readings 1) | 3 (index_commit, anchor fields) |
| 2 | 9 | 3 (WAE-M2 review, requirements sign-off) | 1 (env.lock pin) |
| 3 | 9 | 6 (WAE-M3/M6 adjudication 4, IXS-M3 and its sidecar session 2, parked) | 5 (adapter, bundle) + owner bundle authoring 4 h |
| Nov (FG4/J3, WAE-M5 pre-registration; the publish job and IXS-M1/M2 parked) | 6 | 4 | 0 |
| Gated rows (FG7 8, rollback 3, `import snapshot` 6, `$HOME` 3, environment-daemon recipe 2, versioned links 6) | up to 28 | 6 | |
| **Total** | **27 ungated, up to 55** | **about 33** | **about 9** |

The index-snapshot cells above were estimated before the design parked them; the design re-estimates them per phase
(`docs/design/index-snapshots.md` section 12). [J] The binding constraint is owner hours and RT-M1 windows, not
worker-days. 33 owner hours over three months beside
language excellence is feasible only with stratified labels and no December policy-kit work.

### Next quarter (to 2027-01-11)

| Month | Fathom | Others | Gate |
|---|---|---|---|
| Late Oct | change 13 (Envoy/Istio, J3) | Review Kit's `index_commit` check and anchor fields | RT-M1's first amended window reads after 2026-10-19 00:00Z (docs/plans/responsible-tenant.md:250) |
| Nov | change 14 if WAE-M1 clears; IM-M0 reading; WAE-M5 pre-registered | Review Kit bundle v2 with a lens mapping (REL05-BP03 first) | R3 precision >= 80% |
| Dec | change 15 if IM-M0 BUILD; SG-2 if RT-M1 PASS | image-kit provenance referrers (bake path) | index-snapshot phases start only when the owner prioritizes them (`2026-10-11-index-snapshots.md` section 11) |
| Early Jan | versioned cross-repo links (pool arithmetic, retry amplification across repos) | | RT-M1 FAIL through 2026-12-31 keeps it single-repo |
| 2027-01-31 | **program kill date** (G37): WAE-M4 unread or CUT drops WAE waves 3 to 4; index snapshots are not part of G37 and get their own goal when started | | |

## 6. Decisions needed from the owner

Owner-architect rulings of 2026-10-11: D4 to D10, D12, D13 (with the retention cap), D15, D16, D17 and D19 are
accepted and recorded in ADR-069 and the records it names; the PR ratchet is elevated to the default delivery. D1,
D2, D14 and D18's F8 half await the owner's answer to U1 (`2026-10-11-index-snapshots.md` section 10.2). D3 and D11
stay with the owner, with the open questions U2 (kits-first force-multiplier or standalone product), U3 (ground
truth labels by the owner, about 6 hours, or agent-drafted with a 20% owner audit, about 1.5 hours) and U4 (keep the
spend hold for this program).

| # | Question | Recommendation |
|---|---|---|
| D1 | Q5: what is the threat model? | **Option (a), qualified:** "Accepted for a loopback daemon on one workstation, while a second user or networked serving remains possible; when it comes, network reach goes through the operator's authenticating proxy in front of a `--read-only` daemon (law 9 as amended, D14)." It does not accept single-user unqualified, so F15 (docs/intent/futures/f11-f22.md:57), F27 (f23-f32.md:64-66) and F30 (f23-f32.md:101) do not drop; they stay as written. **Awaits U1.** Separately, F27's Preserve "the watermark (git HEAD) stays the cache key" (f23-f32.md:59) is amended to name the watermark as the cache key's commit component (landed, ADR-069). |
| D2 | Narrow the hosted non-goal and vision 38's "Hosted anything"? | Yes: keep "operated by fathom, multi-tenant, or writable" refused; admit an operator-run, single-organization, read-only environment daemon serving index snapshots. **Awaits U1.** |
| D3 | Q4: force-multiplier or product? | Force-multiplier for the kit family now; it decides nothing here except F2 and F32, so record it and move on. |
| D4 | Fathom as the findings backend? | No. Evidence store plus edit-time projection; ADR-040 and ADR-0098 stand. |
| D5 | Who is the first consumer (ADR-067 d12's "production manifests", F59's committed alerts, EVX-V1)? | Public calibration subjects for precision; pe's substrate manifests (for example pe:pdk/substrates/modules/mimir/manifests/mimir.yaml) as the internal subject for workload, probe and PDB facts; Review Kit as the named record consumer, and wave 2 starts only once Review Kit commits the bundle's requirements. No internal alert-rule file exists in pe's substrate manifests, so the alert half needs F59's Do-not-build amended explicitly (D15). |
| D6 | End ADR-058's deferral for incident stage 1 now? | No. Run IM-M0 first ($0 deterministic replay); build stage 1 on BUILD. |
| D7 | Lock the fathom release in env.lock for CI and developers? | Yes; without it cache sharing reads near zero. |
| D8 | F27: promote on naming a team, or on a measurement? | Naming a team starts IXS-M1; IXS-M1 KEEP promotes. |
| D9 | Reliability findings: new class or domain? | Domain tag in Review Kit, existing ten classes; revisit if over 20% of rules misfit. |
| D10 | Kubernetes resource and security rules before policy-kit exists? | Upstream Kyverno library as the interim; no rules in fathom (O11). No policy-kit build this quarter. |
| D11 | Lift the budget hold for judged sites (H rows, shared compiled layers)? | Not yet; R0 to R5 are $0 in model spend. Revisit after R3 precision reads. |
| D12 | Prune F67 and decide F66 (E = 0.1451 between bars, docs/plans/telemetry-links.md:55)? | Prune F67; park F66 with a re-census on 2027-01-11. Accepted; not yet applied, because plan row TL-0-f67's lexical oracle reads F67's heading and must move first. |
| D13 | Retain index snapshots of every deployed commit? | Only under a retention cap (deployed commits 90 days, then monthly) or a measured dedup ratio (IXS-M7): uncapped, 50 services at 20 deploys a week grow about 155 GB a week (`2026-10-11-index-snapshots.md` section 12). Retention serves rollback gates and incident replay, never evidence (ADR-069 decision 15). |
| D14 | Amend law 9 (vision 12:103-110, G5 at :196; note on vision 13:172) so auth for an operator-run read-only environment daemon may be the operator's authenticating proxy? | Yes, proven by IXS-M8 (hostile unauthenticated requests against the proxy-plus-daemon pair); P9 marked reinterpreted. The in-fathom bearer path was deleted under SR-17 (docs/plans/gke-module-host.md:94). **Awaits U1.** |
| D15 | Amend F59's Do-not-build "before a named consumer exists" (docs/intent/futures/f54-f61.md:90, P20)? | Yes for the alert-fact half only, on public subjects plus Review Kit's committed requirements; SLO and dashboard halves unchanged. |
| D16 | Amend ADR-067 d12 ("Only the per-container facts and observed-by are built now", ADR.md:4394) to admit the widened fields? | Owner-architect: yes, with env names and `valueFrom` only and arg flag names only, and an ADR-065 d2 sentinel test. |
| D17 | pe-side records? | Owner decides, each in pe: Review Kit `fathom-facts` adapter and `anchors[]` (an ADR beside ADR-0098); pdk/risk accepting fathom concern facts as named reasons (an ADR-0080 amendment); image-kit store-as-referrer on the bake path (an ADR-0115 note); ci-kit stays out until its owner records a scope change. |
| D18 | F8 and F3 (owner decides) | F8: R1's drop trigger ("no external request in writing", rated "FATAL at this stage", docs/vision/07-questions-and-risks.md:26) is **overridden** by the owner's ask, not fired; the new drop-if is "no consumer session recorded with a law 6 stamp file within 60 days". **Awaits U1.** F3: its promote trigger has not fired (docs/intent/futures/f01-f10.md:36-37); `import snapshot` is not federation, and F3 gains the amended query-time federation triggers after the owner's question of 2026-10-11 (landed, ADR-069 decision 9). |
| D19 | A whole-program kill date? | 2027-01-31 in G37: without a WAE-M4 KEEP (Review Kit evaluating a fathom fact on a real repository), WAE waves 3 to 4 are dropped, not deferred. Index snapshots left G37 when the design parked them; G37 keeps only IXS-M0 (owner-architect, the G37 split). |

## 7. Kit feature inspirations (ranked per kit)

| Kit | 1 (highest) | 2 | 3 |
|---|---|---|---|
| Review Kit | reliability and security bundles over fathom facts with `anchors[]` and coverage | lens export (AWS custom lens, https://docs.aws.amazon.com/wellarchitected/latest/userguide/lenses-custom.html) with evidence per question | incident binding: every past incident has a live regression certificate (`#FathomCertifyBinding kind: "incident"`, rk:schema/review.cue:57-60) |
| pdk/risk | concern deltas as named reasons on a change's contribution | compliance-tagged changes carry their code reach | drift attestation over previews and XR diffs |
| CI Kit (only after a pe-side scope record) | index snapshot build as a content-keyed DAG node | test-function selection inside depgraph packages (cmd/fathom/tests_select.go:75) | concern-fact ratchet step on PRs |
| image-kit | digest -> commit provenance as a referrer on the bake path, so `env:prod` resolves with no connector | SBOM per deployed image as the OSV-reach input (ADR-050 d4 reads a user-supplied file, ADR.md:2927) | store as a referrer of the image |
| env-kit | fathom release lock + toolchain digest in `env.lock` | index snapshot pull in devenv and cloud setup | doctor row "index fresh for HEAD" |
| test-kit | incident regression tests certified per commit | attested runs feeding `runtime_pass` | certify gaps at the deploy commit |
| policy-kit (when it exists) | resource and security rules with canonId (probes, PDB, limits, NetworkPolicy, least privilege) | framework crosswalks (CIS, SOC 2) citing fathom receipts | mitigation approval controls for agents |
| docs-kit | runbooks as DocSets; alert -> runbook coverage | owner and classification floors for ADR-065 withholding | DK-64 coverage against the index |
| agentic-kit | incident-responder skill that reads the fact sheet first | evidence-before-claim doctrine for RCA | `evidence-producer` shape alignment |
| app-kit | freshness column pattern | dense evidence table pattern (anchors, coverage, basis) | |

## 8. Opportunities the owner did not mention

1. **The ratchet**: PR-time concern deltas ("this PR removed jitter", "widened an IAM action") beat audits for
   adoption and cost.
2. **Calibration against real incidents**: replay a bundle at each culprit's parent commit over a pre-registered
   corpus before claiming prevention. This turns "well-architected" from opinion into a measured catch rate with a
   stated lower bound.
3. **Rollback safety facts**: migrations, contract keys and flag deltas in `previous..deployed`, extending incident
   maps 6.3's removed-commit list. No vendor ships this deterministically [J].
4. **Retry amplification across hops** computed from config: AWS names it as an anti-pattern; nobody computes it.
5. **Pool arithmetic across repos**: replicas x pool vs `max_connections` in a platform repo's IaC.
6. **Over-privilege from code**: IAM grants minus the API calls the code makes (S1).
7. **Point-in-time compliance receipts**: what ran on date D and which controls held, as ADR-068 receipts
   re-derived at the deployed commit. Retained index snapshots make the re-derivation cheap; they are never the
   evidence themselves (ADR-069 decision 15).
8. **CVE reach per deployed image** through an image SBOM fed to ADR-050's OSV path.
9. **Chaos targeting** from the fact sheet.
10. **Scorecard fact collector**: feed Backstage Soundcheck or Tech Insights
    (https://backstage.spotify.com/docs/plugins/soundcheck/mcp-actions.md) instead of building a portal.
11. **Context source for every AI SRE agent**: fathom's MCP surface registered beside the agent's own
    tools, rather than competing with them [J].
12. **Coverage as a selling point**: an explicit "incomplete" is rare in this market and is what makes
    receipts trustworthy [J].

## 9. Disagreements resolved (master list)

| # | Topic | Decision | Why (one line) |
|---|---|---|---|
| 1 | Hosted non-goal | the narrowing awaits the owner (U1); read-only sidecar first; an environment daemon after U1, IXS-M3, IXS-M8 and a named service with an on-call owner | the old trigger cannot fire from the owner's teams |
| 2 | Refs | `--cache-from` per-ref ingest when phase 1 starts, parity proven; overlay gated on IXS-M5 | measure the cheap path before touching the compatibility surface |
| 3 | Answer basis | exact / differs / unchecked / uncommitted; behind N caller-side | a daemon has no git graph and may not list a registry |
| 4 | Deploy state | per-call parameter, GitOps service, operator-named provenance file (bake path); nothing stored | a stored record is IM-X2's attach renamed |
| 5 | Worktrees | I30 kept | no measured need; INC-3 |
| 6 | Repo identity | root-commit id + subpath | host-free (ADR-011) |
| 7 | Index snapshot transport | the operator's publish job; OCI referrers recommended | fathom stays fetch-free (I56) |
| 8 | Per-extractor epochs | only if IXS-M2 fails (F73) | a locked release may suffice |
| 9 | Merge-ahead | a plain CI bot gated on IXS-M6, outside ci-kit | no stored state for an unmeasured benefit |
| 10 | Across services | `import snapshot` for a named cross-repo consumer; links after RT-M1; no query-time RPC until one of F3's triggers fires | mounted stores federate obligations only |
| 11 | A UI for this | freshness column only, newly wired | incident views are the command-center non-goal |
| 12 | F27 promotion | naming starts IXS-M1; KEEP promotes | demand plus proof |
| 13 | Fact transport | `export receipt --kind fact\|join` at `fathom@<version>` | EVX-25 waits on EVX-M3; git cannot prove a structural fact |
| 14 | Defaults pack | Review Kit, keyed on `pkg@version` plus env facts | judgment knowledge; defaults move with opt-ins |
| 15 | Code client facts | WAE-M1 now, FG7 past its bar | the flagship gets a measured go/no-go |
| 16 | Rollback facts | `rollback` dimension of `review --since`, after IM-M0 | no new tool |
| 17 | Reliability class | domain tag, existing classes | class = defect kind, domain = pillar |
| 18 | `version --json` | exists (cmd/fathom/version.go:80-90) without epochs; add fields | corrects the boundary design's "no CLI path" |
| 19 | Change-risk gate | pdk/risk; fathom feeds reasons | it already ships tiers and floors |
| 20 | Publisher | plain GitHub Actions job; ci-kit only after a scope record | ci-kit needs a deployed plane |
| 21 | Deploy gate failure | fail-open with recorded exceptions | fathom must never stop mitigation |
