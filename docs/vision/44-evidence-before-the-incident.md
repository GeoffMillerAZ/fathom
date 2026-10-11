# 44 - Evidence before the incident: concern facts for review, context for incidents, and index snapshots

Status: accepted 2026-10-11 (ADR-069) for the evidence plane, sections 2 to 4; the connection rule and the
ports-and-adapters boundary, section 5, are accepted with it. Index snapshots are decided in shape and parked
(`docs/design/index-snapshots.md`), and their environment daemon waits on the owner's answer to U1. The owner's note
of 2026-10-11 asked for three things: fathom as the investigation and evidence engine behind Review Kit's
well-architected, compliance and incident-prevention catalogs; fathom in real-time incident investigation, RCA and
post-incident review, ultimately under an AI agent that mitigates; and indexes that CI/CD keeps fresh, that teams
share instead of re-indexing, and that know about branches and deployments. A later question the same day asked
whether live per-service instances networked to their dependencies' deployed versions would serve the CD gate and
incident investigation. This document answers all of it with one idea and one new subsystem, code reads over an
imported index snapshot (`import snapshot`), gated on a measurement and a named consumer. Assessments:
`docs/assessments/2026-10-11-reviews-and-incidents.md`, `2026-10-11-index-snapshots.md`,
`2026-10-11-making-the-vision-happen.md`.

## 1. The idea

Prevention and incident response are the same question at different commits: what does the code and config at
commit C do on concern K? It is asked at PR time (the head), at deploy time (the deploying commit) and at page time
(the deployed commit). Fathom answers it with facts and joins, anchored to git, with a coverage statement. Review
Kit judges; pe's pdk/risk tiers the change; policy-kit owns resource and security rules. CI keeps the answer fresh
by exporting an index snapshot per commit. An external agent acts.

## 2. What fathom does

- Stores concern facts as observations of their own file, under the table freeze: probes, resources, shutdown,
  rollout strategy, env names and `valueFrom` references, arg flag names, alert `for` and annotations, and proxy
  retry, timeout and outlier policy. Env values and arg values are withheld (ADR-065 decision 2), except two
  allow-lists of non-secret keys.
- Computes cross-file joins at request time and never stores them: probe to handler to network callee; workload to
  monitor to alert to receiver; retry and timeout along a route chain; IAM grant to service account to workload;
  public route to handler; workload to the NetworkPolicy and policies that select it; later, replicas x pool
  against the database's `max_connections`.
- Exports both as receipts (`export receipt --kind fact|join`, re-derivable at `fathom@<version>`), each with
  anchors and a coverage statement. An absence fact carries its read set and is a lower bound when a read is
  missing.
- Serves the per-service fact sheet through existing tools: `symbol_context` on a workload, `localize` once its
  incident section is built, and receipts. The PR ratchet diffs concern facts at the head against the merge-base
  and is the default delivery: "this PR removed jitter", "widened an IAM action to `*`".
- Reads the deployed commit from a per-call parameter, a GitOps repository indexed as a service, an operator-named
  provenance file, or (parked) a lineup an environment daemon reads once at start. It stores no deploy state.

## 3. What fathom does not do

Grade, set severity, admit, tier a change, or hold the findings record (Review Kit, pdk/risk). Evaluate Kubernetes
resource or security rules, or map frameworks (policy-kit, Kyverno until it ships). Fetch telemetry, cluster state,
deploy events or index snapshots. Execute, propose or record a mitigation. Operate a daemon for anyone, bind a
non-loopback address, authenticate users, hold a key or verify a signature. Block a deploy when it is unavailable
(the gate is fail-open). Build a second package graph, verifier or result format (ADR-063).

## 4. The loop that joins the three asks

An incident's post-incident review produces a Review Kit requirement bound to a regression certificate
(`#FathomCertifyBinding kind: "incident"`) and, when the cause was invisible, a new fact kind. The next PR that would
reintroduce the cause fails the ratchet. Prevention learns from incidents; incidents start from prevention's fact
sheet.

## 5. How instances connect, and where fathom stops

**Ship files, never call.** Fathom instances connect by shipping immutable, commit-pinned index snapshots:
exported on merge, named by a lineup on deploy, and served by a daemon that a lineup change replaces (a rolling
update). They do not connect by query-time RPC between live daemons. An incident is when a dependency or its host
is degraded, so a daemon that must call another daemon fails exactly then; a snapshot at a commit never changes
under a reader; cross-service joins run in one process; and every tool works unchanged over imported snapshots.
Query-time federation is reconsidered only when one of F3's amended triggers fires: an imported dependency closure
exceeds one reader's measured budget, a dependency owner refuses in writing to let its snapshot leave its
boundary, or a reading shows an answer class needs on-demand compute the precomputed fact sheet misses.

**Ports and adapters.** Fathom's core reads paths the operator names in that invocation and listens on loopback.
Every network, credential or signature adapter (the registry pull, the authenticating proxy, the signer and its
verification, the deploy job that writes a lineup) belongs to the operator's recipe. This extends I56 (fathom never
downloads on its own authority) and ADR-068 decision 7 (the operator's signer signs). It keeps the core small enough
for one maintainer, keeps every tenancy and data-governance question on the operator's side, and leaves a hosted
form possible without building one.

The set of environment daemons has no name and no control plane: nothing acts on it.

## 6. Sequence

Vision 44 adds no pillar to vision 38. Language excellence stays the main line, and these waves ride as fillers
under the two-wave limit.

- **The language-wave ruling of 2026-10-11.** Ops formats (Kubernetes pod facts including LB-F451, GitHub Actions,
  Helm, Jenkinsfile, observed-by TL-15) moved from lf7 to lf8, because the web frameworks needed TypeScript and
  HTML first; lf7 launched with TypeScript TSX, shell, Vue, Svelte, Angular and Astro. Vision 38 section 9.4
  (`docs/vision/38-proof-layer-roadmap.md`) still names lf7 for ops formats; its correction is the
  owner-architect's edit with the index-snapshot roadmap entry. This section is the committed record of the ruling
  until then.
- **WAE wave 1** (war1): the $0 readings WAE-M0 and WAE-M1, and three defects: the findings importer hashes at the
  claimed revision (WAE-1), `version --json` carries the cache-key components (WAE-2), `read_source` reads at a
  commit (WAE-3).
- **WAE wave 2** rides lf8 and starts only after Review Kit commits the bundle's requirements: concern facts
  stored and served, alert facts, the widened Kubernetes container facts with the planted-sentinel test.
- **WAE waves 3 and 4** follow WAE-M2 KEEP: joins, fact receipts, proxy facts, and Review Kit's adapter as the
  consumer (WAE-M4).
- **Index snapshots** are parked. Only the IXS-M0 timing reading and IXS-0 (the machine-local store fields, a
  defect fix) ride war1. The owner prioritizes the rest when ready, no earlier than war1 landing and RT-M1 reading;
  each later phase has its own start trigger and kill (`docs/design/index-snapshots.md` section 12).
- G37's kill date, 2027-01-31, covers the WAE waves only. Index snapshots get their own goal, provisionally G38,
  when the owner starts phase 1.

## 7. Measurements (bars fixed before any run)

WAE-M0 labels (stratified), WAE-M1 (code clients), WAE-M2 (fact precision and the secret sentinel), WAE-M3
(reliability joins), WAE-M4 (consumer precision), WAE-M5 (a pre-registered postmortem replay, n at least 20, Wilson
lower bound), WAE-M6 (security joins), WAE-M7 (gate latency and fail-open share); IXS-M0 (the cache-from cost
baseline), IXS-M0b (cache-from equals cold), IXS-M1 (cache-from cost against warm), IXS-M2 (hit rate), IXS-M3 (the
read-only sweep), IXS-M4 (a second team), IXS-M5 (branch divergence), IXS-M6 (merge-ahead), IXS-M7 (cost), IXS-M8
(the proxy hostile test), IXS-M9 (an environment daemon used within 60 days). Every reading is $0 in model spend
while the hold stands; owner hours are counted in the plans.

## 8. Jev, weighed

Typed questions it could answer, all opt-in judged sites under ADR-056 that propose and never decide: is this
hand-rolled loop a retry with backoff; does this health check use a pool separate from request traffic; is this
alert's runbook investigatively useful; does this middleware authenticate this route. Band policy (P24): a site
serves a judgment only inside the calibrated band where its Wilson lower bound against adjudicated labels is at
least 0.90; outside the band it answers "not judged" and the fact's coverage reads `incomplete`. A judgment never
enters a fact, a join, a receipt or an index snapshot's cache key (I62), never closes a finding and never decides
which commit a lineup names. With provider none, the default, everything here holds: facts, joins, receipts, cache
keys, the answer basis, parity and gates are computed, and the four judged questions read `incomplete`. An
environment daemon holds the decision port at provider none. Measurement: the Wilson lower bound on WAE-M0's labels
and cost per true positive under the owner's bar; below either, or while the spend hold stands, the site stays
unbuilt.

## 9. Records

ADR-069 (fifteen decisions: 1 to 10 accepted, 11 to 15 decided in shape and parked). Plans
`docs/plans/well-architected-evidence.md` (WAE-) and `docs/plans/index-snapshots.md` (IXS-). Design
`docs/design/index-snapshots.md`. Goal G37; G38 provisional. Futures F70 to F74, and F3, F4, F27, F32 and F59
amended. Amendments: ADR-067 decision 12 (widened container facts), ADR-068 decision 4 (the `fact` and `join`
kinds). Readings: `docs/bench/readings/self-use/2026-10-11-hook-path-abandonment.md` and
`2026-10-11-compiled-layer-staleness.md`.

Awaiting the owner (U1, a self-hosted, single-organization, read-only environment daemon): the hosted non-goal's
narrowing, vision 38 section 7, the law 9 amendment with its vision 12 and vision 13 notes, P9's reinterpretation,
F8's override and Q5's qualified answer. Their proposed texts are in `docs/assessments/2026-10-11-index-snapshots.md`
section 10.2. Until U1 is answered, phase 5 (the environment daemon, F74) cannot start; phases 0 to 4 do not need it.
