# Fathom for well-architected reviews, breach prevention and incident response

Owner-architect assessment, 2026-10-11, revised after critique; summarized into the repository 2026-10-11. Read at
fathom `c39875f3` and platform-engineering `84f9d41`; adapted to the records at `65e5acfa`. Inputs: five readers
(intent, state, kits, landscape, concerns) and two architect designs (platform, boundary), reconciled here. Where
they disagreed this document decides; section 11 lists each call. Companions:
`2026-10-11-index-snapshots.md` and `2026-10-11-making-the-vision-happen.md`.
Decisions it drove: ADR-069 decisions 1 to 8, the ADR-067 decision 12 and ADR-068 decision 4 amendments, vision 44,
G37, F71, F72, the F4, F32 and F59 edits, plan `docs/plans/well-architected-evidence.md` (WAE-).

Conventions. Plain paths are fathom repo paths. `pe:` = platform-engineering. `rk:` = `pe:kits/review-kit/`.
[J] marks a judgment, a *hypothesis* in this directory's grading; a sourced row without [J] is *measured* (read
from the cited file at the read commit) or *claimed by a vendor* (an external page, cited by URL). Plan rows are
cited by id; measurement ids are the plans' `WAE-` and `IXS-` ids only. Fact gaps are `FG0` to `FG11` (not goal
ids). Phases R0 to R7 are this document's labels for plan waves. A finding first read from the gitignored operator
notes is cited to the reading that committed it under `docs/bench/readings/`.

## 1. Verdict

**Partly yes.** Fathom is the right engine for one thing nobody ships: deterministic, source-linked joins
across code, manifests, proxy config, IaC, IAM and alert rules, at a named commit, with receipts a stranger can
check (ADR-068, ADR.md:4538-4600). That is the evidence behind incidents and breaches that cross layers: retry
storms through proxy hops, probes that reach a database, pools that outgrow the database, alerts that route
nowhere useful, a public route to a handler with no auth, a role granting more than the code calls.

Fathom must not become the grader, the findings record, a change-risk scorer, a live-ops connector, or a
mitigating agent. Review Kit judges; policy-kit (Kyverno until it ships) owns resource and security rules and
framework crosswalks; pe's `pdk/risk` already scores Pulumi previews and manifest diffs into approval tiers
(pe ADR-0080, pe:pdk/risk/model.go:1-4); CI keeps the evidence fresh; an external AI SRE agent acts.

Order of value [J]:

1. Alert quality and routing. The routing joins ship (internal/fusion/alertroute.go:47-51); the gap is FG0
   (store and serve facts, a prerequisite for every row) plus FG2's five keys. There is no internal alert-rule
   subject today (section 3.5).
2. Kubernetes workload facts and the probe-to-handler join (LB-F451, widened under an ADR-067 d12 amendment).
3. Security and compliance joins over facts fathom already extracts (IAM, Ingress, policy membership; section 3.4).
4. Proxy retry, timeout and outlier facts (Envoy, Istio, nginx, HAProxy, Kong).
5. Code-level client construction (the owner's flagship: backoff with jitter). This is the costliest gap.
   It gets a $0 measurement (WAE-M1) first and a build only if the measurement clears.

Two preconditions sit outside fathom: Review Kit's reliability catalog is three findings, all off, with no
checks (rk:internal/catalog/assets/data-reliability/pack.cue:6-8), and its fathom adapter answers only
`verify-plan` (`claim-status` is reserved, rk:internal/evidence/adapters.go:444-453).

The largest risk is a false "clean": most concerns are absence claims ("no jitter", "no PDB selects W",
"no NetworkPolicy selects W"), and an absence without a coverage statement is worse than no review.

## 2. Why the gap is real (market, short)

| Observation | Source |
|---|---|
| AWS rates missing backoff, jitter and retry caps High risk, and names "retrying at multiple layers" a retry-storm anti-pattern: a cross-layer property by definition | https://docs.aws.amazon.com/wellarchitected/latest/reliability-pillar/rel_mitigate_interaction_failure_limit_retries.html |
| The AWS Well-Architected Agent (preview 2026-10-01) reads resource config, metrics, topology and uploaded IaC; the announcement does not mention application source, and warns output "may contain errors" | https://aws.amazon.com/blogs/aws/announcing-aws-well-architected-agent-an-ai-powered-intelligence-to-optimize-your-cloud-environment-preview/ |
| Lens reviews are questions, choices and improvement plans answered by people | https://docs.aws.amazon.com/wellarchitected/latest/userguide/lenses-custom.html |
| IaC scanners graph IaC resources only (Checkov 2.x) | https://www.paloaltonetworks.com/blog/cloud-security/checkov-2-deepening-open-source-iac-security/ |
| Scorecards check existence ("any SLO defined?"), not behaviour | https://docs.datadoghq.com/internal_developer_portal/scorecards |
| AI SRE agents score 11.34% best on OpenRCA; a 2026 replication reads 3.9 to 12.5% | https://proceedings.iclr.cc/paper_files/paper/2025/file/d29b8d53678015079e1d245c023e49d2-Paper-Conference.pdf ; https://arxiv.org/pdf/2602.09937v1 |
| Datree, a standalone Kubernetes policy vendor, closed; repo archived 2024-06-06 | https://github.com/datreeio/datree |

[J] Single-file manifest linting is a thin, crowded business. Cross-artifact joins with receipts are open.

## 3. The concern catalog

Kinds: **D** a rule over one entity's facts (runs in Review Kit, policy-kit or pdk/risk); **J** a cross-file join
(fathom's own value); **H** needs judgment (opt-in judged site, spend hold applies).

### 3.1 The owner's four examples, mapped

| Owner example | Concerns | Config side today | Code side today | Verdict |
|---|---|---|---|---|
| Exponential backoff with jitter on things that connect, especially proxy fleets | #1, #2 | Envoy routes and clusters carry no retry facts (internal/extractor/dialects/envoy/doc.json:14-18); Istio is route_ref only | Third-party call sites are not stored; Go skips composite-literal keys (internal/extractor/goextract.go:591-598); evidence is grep plus enclosing symbol | Config side cheap (FG4, FG5). Code side is FG7, the costliest gap. Library defaults decide behaviour, and they move: AWS's documented standard-mode full jitter "requires opting in until it becomes the default" (`AWS_NEW_RETRIES_2026=true`), mode and attempts come from `AWS_RETRY_MODE`/`AWS_MAX_ATTEMPTS`, the shared config file or code, and legacy mode remains for backward compatibility (https://docs.aws.amazon.com/sdkref/latest/guide/feature-retry-behavior.html). So the defaults pack keys on `pkg@version` plus those env facts read from manifests (names and literal values of these three non-secret keys only, section 3.3), and belongs to Review Kit |
| Connection pools that reserve connections for health checks | #3, #6 | PgBouncer unsupported (docs/languages.md:222); replicas not a fact (LB-F451 open) | Pool sizes are literals in code or app config; Spring `application.yml` has no dialect and reads only as a generic YAML document (docs/languages.md:369) | Pool arithmetic (replicas x pool vs DB max) is a J row across repos. Health-pool separation is H. Late |
| Observability and alerts configured to be noticed and to maximise investigative value | #17 to #21 | Prometheus facts are `labels` only (internal/extractor/dialects/prometheus/doc.json:23-26); Alertmanager facts are `receiver` and `continue` (internal/extractor/dialects/alertmanager/doc.json:39-42); routes_to, may_route_to and inhibit joins ship (internal/fusion/alertroute.go:47-51) | Log templates for Python, Java, JS/TS, PHP (internal/fusion/logtemplate_pass.go:3-7), not Go | First. "Investigative value" = alert has `for`, severity, `runbook_url`, routes to a paging receiver, and the workload it watches resolves to code and an owner |
| Kubernetes best practices | #6 to #11, #24 | K8s L3; per-container facts are LB-F451, open (docs/plans/language-breadth.md:993) | n/a | Resource rules belong to policy-kit, Kyverno in the interim (ADR-063 O11, ADR.md:3998-4004). Fathom supplies facts and the joins a manifest linter cannot do |

### 3.2 Top 12 reliability concerns, ranked by incident frequency x fathom feasibility

| Rank | # | Concern | Failure prevented | Fathom today | Fact gap | Kind | Evidence shape |
|---|---|---|---|---|---|---|---|
| 1 | 18 | Alert quality: `for`, severity, `runbook_url`, routing, inhibition | Ignored or missed pages, https://prometheus.io/docs/practices/alerting/ | labels facts; routing joins shipped (alertroute.go:47-51) | FG0, FG2 | D; J critical alert routes to a non-paging receiver | alert:line facts + routes_to receiver:line |
| 2 | 6 | Probes test the right thing | Liveness probing a dependency restarts the fleet, https://srcco.de/posts/kubernetes-liveness-probes-are-dangerous.html | probes not facts (LB-F451) | FG1 + probe path to handler | J liveness handler reaches a DB or network client | Deployment:line `/healthz` -> handler:line -> `db.Ping`:line |
| 3 | 8 | Requests, limits, heap headroom | OOMKill and throttling, https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/ | LB-F451 open; Helm values keep key paths, no scalars (internal/extractor/helmextract.go:233-241) | FG1, FG6 | D; J heap flag vs limit | limit 512Mi:line + `-Xmx512m` flag name and value:line = 0 headroom (an allow-listed non-secret flag, section 3.3) |
| 4 | 1 | Retries: backoff, jitter, budgets, no amplification across hops | AWS 2021-12-07, https://aws.amazon.com/message/12721/ | see 3.1 | FG4, FG5, FG7 | D per hop; J amplification along route_ref; H hand-rolled loops | client:line (3) -> Envoy route:line (3) -> VirtualService:line (3) = 27 attempts |
| 5 | 2 | Timeouts at every hop, outer >= inner x (retries+1) | Tail amplification, https://research.google/pubs/the-tail-at-scale/ | as #1 | FG4, FG5, FG7 | J inequality per chain | ordered hops, value + file:line each, violated inequality named |
| 6 | 24 | Rollout safety: maxUnavailable, minReadySeconds, progressDeadlineSeconds, analysis | Changes cause most outages, https://sre.google/sre-book/introduction/ ; https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019/ | Argo L3 (docs/languages.md:302), no strategy facts | FG1 widened | D; J AnalysisTemplate metric exists | Rollout -> AnalysisTemplate -> rule |
| 7 | 17 | An alert exists per workload or SLO | Customers detect first, https://sre.google/sre-book/monitoring-distributed-systems/ | TL-15 observed-by open (docs/plans/telemetry-links.md:35) | FG3 | J | workload:line -> monitor:line -> alerts or none |
| 8 | 9 | PDB, spread, replica minimum | Drain or zone loss takes every replica, https://github.blog/news-insights/company-news/oct21-post-incident-analysis/ | PDB is an entity; selector join is Service-only (internal/extractor/dialects/kubernetes/doc.json:54-55) | FG3 | J replicas >= 2 and a PDB selects it | workload labels:line + matching PDBs (none) |
| 9 | 7 | Graceful shutdown | 5xx on every deploy, https://learnk8s.io/graceful-shutdown | not in LB-F451's field list | FG1 widened | J grace >= preStop + drain | value chain + inequality |
| 10 | 4 | Circuit breaking, outlier detection | Traffic keeps hitting a failing upstream, https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier | Envoy cluster entity only | FG4 | J every route target covered | route->cluster edge + cluster:line lacking outlier_detection |
| 11 | 3 | Pool sizing and health reservation | Exhausted pool fails health checks, https://aws.amazon.com/builders-library/implementing-health-checks/ | see 3.1 | FG7, FG8, FG9 | J replicas x pool vs DB max; H health pool | replicas 12:line x pool 50:line = 600 > max_connections 500 (tf:line) |
| 12 | 22 | IaC and GitOps drift | Manual change diverges, https://developer.hashicorp.com/terraform/tutorials/state/resource-drift | out of scope: fathom never fetches ops data (docs/intent/scope/non-goals.md:7-13) | FG11 (an attested `pulumi preview --json`, Crossplane XR diff or flux status artifact) | J declared vs attested | resource:line + preview or diff entry; pe's `pdk/risk` already parses both preview and manifest diff (pe:pdk/risk/pulumi.go:43, pe:pdk/risk/manifest.go:31) |

pe's IaC is Pulumi plus Crossplane (pe:pdk/risk/model.go:1-4) and its GitOps is flux (pe:docs/adr/0005-cluster-topology.md:18,28);
fathom has a flux dialect at L3 (docs/languages.md:194). OpenTofu and Argo stay supported for other adopters
(Argo L3, docs/languages.md:302).

The remaining 13 reliability concerns (load shedding, init order, secret rotation, cert expiry, DNS TTL,
thundering herd, fan-out, SLO burn rate, dashboards, trace propagation, backups, HPA bounds, kill switches) are in
the concerns reader. They reuse the same fact families.

### 3.3 Fact gaps, ranked (cost x rows served)

| Gap | What | Serves | Cost [J] | Home row |
|---|---|---|---|---|
| FG0 | Store and serve facts. DocFacts exist but feed joins only inside a pass (internal/fusion/alertroute.go:397,515). Store them as observations of the entity's own file, the LogTemplate precedent (ADR-064, internal/fusion/logtemplate_pass.go:7-10); no table under the freeze (internal/fusion/routeref.go:14-21) | every row | S | new (WAE-4, WAE-5) |
| FG2 | Prometheus `for`, `keep_firing_for`, annotations; Alertmanager `group_wait`, `repeat_interval` | #18, #19 | XS (dialect data) | new (WAE-6) |
| FG1 | Widen LB-F451: preStop, terminationGracePeriodSeconds, strategy, minReadySeconds, progressDeadlineSeconds, affinity, spread; env as **names and `valueFrom` references only, never a literal value** (except an allow-list of non-secret retry keys: `AWS_RETRY_MODE`, `AWS_MAX_ATTEMPTS`, `AWS_NEW_RETRIES_2026`); args as **flag names only**, values withheld (except an allow-list of heap flags such as `-Xmx`). Needs an ADR-067 d12 amendment, since d12 says "Only the per-container facts and observed-by are built now" (ADR.md:4394), and must meet ADR-065 d2, "a secret-bearing field is never selected" (ADR.md:4137-4141), proven by a planted-sentinel test as ADR-065 does | #6 to #11, #24 | M | LB-F451 + new (WAE-7, WAE-13) |
| FG3 | Selector joins: PDB, HPA scaleTargetRef, NetworkPolicy podSelector, ServiceMonitor observed-by | #9, #10, #17, S3 | M | TL-15; new (LB-F456 only grades the existing K8s edges and adds no PDB or HPA join, docs/plans/language-breadth.md:998) |
| FG4 | Envoy retry_policy, timeouts, circuit_breakers, retry_budget, outlier_detection; Istio DestinationRule trafficPolicy, VirtualService retries/timeout | #1, #2, #4 | M | new (WAE-11) |
| FG5 | nginx, HAProxy, Kong, Traefik retry, timeout, health | #1, #2, #4, #14 | M | new |
| FG6 | Rendered values: Helm scalars, Kustomize overlays | resolves #6 to #8 when templated | L | new (LB-F458 covers Helm L3 only) |
| FG7 | Code client construction: third-party call site + literal options + `pkg@version` | #1 to #5, #15, #21, #25, S1 code side | XL | new; gated on WAE-M1 |
| FG8 | PgBouncer, Spring/app config keys | #3 | M | LB-Q-pgbouncer |
| FG9 | TF/CFN/Pulumi attributes (health_check, deregistration_delay, backup_retention_period, max_connections) | #3, #6, #7, #23 | M | LB-Q-terraform |
| FG10 | Go log templates | #21 | M | TL-4 follow-on |
| FG11 | Drift as an attested CI artifact; fathom resolves a resource address to file:line | #22 | XS in fathom | pdk/risk and the publisher job (`2026-10-11-making-the-vision-happen.md` section 2) |

### 3.4 Security, compliance and breach prevention

The owner asked for governance and breach prevention, not only reliability. Fathom already extracts the inputs:
IAM statements with `action` and `resource` edges (internal/extractor/dialects/aws-iam-policy/doc.json:10-16),
`serviceAccountName` edges (internal/extractor/dialects/kubernetes/doc.json:24-25), Ingress backend edges
(kubernetes/doc.json:57-61), Kyverno, Gatekeeper, Rego and Azure Policy entities (internal/extractor/dialects/kyverno/doc.json:6-9),
and policy membership dispatch that already names NetworkPolicy kinds (internal/fusion/policydispatch.go:571,588).
pe already has the control and framework side: policy-kit's design carries `controls/security` and `frameworks/`
clause catalogs with crosswalks (pe:docs/architecture/policy-kit.md:14-22), and `pdk/risk` refuses any
compliance-regime change below approval tier T1 (pe:pdk/risk/gates/compliance-below-floor-refused.cue:4-6).

| # | Join | Breach or control it serves | Kind | Fathom gap | Placement |
|---|---|---|---|---|---|
| S1 | IAM statement with wildcard action or resource -> role -> ServiceAccount (role-binding annotation) -> workloads using it -> AWS API calls in their code | over-privilege: granted minus used | J | role annotation fact (no `annotations` select in internal/extractor/dialects/kubernetes/doc.json); code side is FG7 | fathom join; least-privilege rule in policy-kit `controls/security` |
| S2 | Public Ingress or Gateway route -> Service -> workload -> handler with no auth middleware on the route | unauthenticated exposure | J; middleware detection is H until a typed rule exists | HTTP route extraction exists for Java, Node, PHP and Python (internal/fusion/routes_java_ingest_test.go, routes_node_ingest_test.go, routes_phppy_ingest_test.go); middleware detection is new | fathom join; rule in Review Kit |
| S3 | Workload with no NetworkPolicy whose podSelector selects it | lateral movement | J | FG3 NetworkPolicy podSelector | fathom join; rule in policy-kit |
| S4 | Secret-shaped literal in a manifest env value or ConfigMap | credential leak | D | presence recorded as a `withheld` fact, never the value (ADR-065 d2, ADR-068 d3 `withheld`) | detection by Gitleaks or Trivy, imported as SARIF; fathom records only where |
| S5 | Image referenced by tag not digest, or from a registry outside the signed set | supply chain | D | image ref fact | admission in policy-kit or Kyverno `verifyImages`; signing is image-kit (pe ADR-0115) |
| S6 | Workload selected by no enforcing policy for a named control | control coverage gap | J | policy membership exists (policydispatch.go); per-control mapping is policy-kit's | fathom membership facts; control mapping in policy-kit |
| S7 | Change to a compliance-tagged resource that reaches code paths handling regulated data | change risk | J feeding D | concern facts as named reasons | pdk/risk adds them to `#ChangeContribution.reasons` (pe:pdk/risk/model.cue:43-51), a pe-side additive change |
| S8 | Framework mapping (CIS Kubernetes Benchmark, SOC 2 CC6/CC7) | audit evidence | none in fathom | n/a | policy-kit `frameworks/` crosswalks; Review Kit lens export; fathom receipts are the evidence a mapped control cites |

Measurement: WAE-M6, the WAE-M3 bar applied to S1 to S3 and S6 (at least one true instance per two repositories
at precision at least 0.90 against committed labels). Fathom builds no security grader and no framework catalog.

### 3.5 Subjects: public and internal

Public calibration subjects carry the precision readings (opentelemetry-demo, microservices-demo, kube-prometheus,
istio samples). The internal subject for workload, probe and PDB facts is pe's substrate manifests, for example
`pe:pdk/substrates/modules/mimir/manifests/mimir.yaml` (8 Deployments, 11 StatefulSets, 14 PodDisruptionBudgets).
No internal alert-rule file exists: no `PrometheusRule` kind or `alert:` key appears under
`pe:pdk/substrates/modules/*/manifests`. So the #1-ranked value has no internal subject, which is what F59's
Do-not-build guards (docs/intent/futures/f54-f61.md:90, P20 at docs/intent/principles.md:72-73). F59 is amended
for the alert half only (ADR-069, owner-architect ruling D15), and WAE wave 2 waits on Review Kit committing the
bundle's requirements.

## 4. The Review Kit and fathom contract (under ADR-063)

### 4.1 Who owns what

| Concern | Owner | Record |
|---|---|---|
| Catalog: requirement, why, remedy, severity (ships `off`), lens mapping (for example REL05-BP03), AWS custom-lens export | Review Kit | ADR-055 d7 (ADR.md:3222-3229); rk:schema/review.cue:97-114 |
| Library-defaults pack (`pkg@version` plus env facts -> default retry, jitter, timeout) | Review Kit, as bundle data | [J] it is judgment knowledge, decided in section 11 |
| Resource-shaped and security rules (probe set, PDB present, NetworkPolicy present, least privilege), framework crosswalks | policy-kit canonId; upstream Kyverno until policy-kit exists (it is design only, pe:docs/architecture/policy-kit.md:14-22) | ADR-063 O11 (ADR.md:3998-4004) |
| Change-risk tiers over Pulumi previews and manifest diffs | pe `pdk/risk` | pe ADR-0080 |
| Versioned facts with anchors and coverage; cross-file joins | fathom | ADR-067 d12 (ADR.md:4385-4394), amended for the widened fields |
| Pass/fail, severity, disposition, admission | Review Kit (`admit`) | ADR-040 d1 (ADR.md:2175), ADR-063 O2 |
| Findings record | Review Kit bundle (and DATUM, the portal); fathom holds a tier-capped projection served at the edit (FL-10) | ADR-040, pe ADR-0098 |
| Drift judgment | Review Kit over an attested artifact | non-goals.md:7-13 |
| Evidence envelope | ADR-068 in-toto predicate (wave 3), SARIF only as projection | ADR-068 d2, d8 |

### 4.2 Evaluation flow

1. A bundle declares `requires: [factKind@v]` and its J joins.
2. CI computes the facts at the commit (deterministic, $0).
3. Review Kit's new `fathom-facts` adapter reads them as receipts and re-derives pass/fail itself, as it
   already does for certify (rk:README.md:103-108).
4. A finding carries the anchors; it closes only on pass with complete coverage
   (rk:schema/findings.cue:106-110).

### 4.3 Transport: a receipt kind, not a new verb

`fathom export receipt --kind fact|join` beside the four Go kinds of EVX-9
(docs/plans/portable-receipts.md:40). ADR-068 d4 keeps a closed table of kinds; this decision grows it by two.
Re-derive levels: both `fact` and `join` re-derive at `fathom@<version>`. ADR-068 d4 says "Git proves the text is
there, never that it is a definition" (ADR.md:4557-4561); a structural YAML fact (this probe, on this container,
of this workload) needs a parser, so it is not `git`. A `toolchain:<yaml decoder>` rung is possible only if the
owner-architect adds it to d4's closed table. An absence fact re-derives at `fathom@<version>` with its read set,
and reads as a lower bound when a read is missing (I38).

Not EVX-25's `export receipt <tool> --args` (docs/plans/portable-receipts.md:65): that row waits on EVX-22
and wave 4, which waits on EVX-M3 KEEP. A reliability bundle should not inherit that gate.

### 4.4 Fact record

```
{kind@v, subject{repo, commit, path, entity|symbol}, values,
 anchors[]{path, commit, blobId, start, end, excerptSha256, digestScope, state},
 coverage{status: complete|incomplete, extractor, tier, readSet, blindSpots[]},
 rederive: fathom@<version>, joinPath[], cacheKey}
```

Anchor `state` is ADR-068 d3's `committed|uncommitted|filtered|withheld`. A withheld field (an env value, an
arg value) carries the anchor with no excerpt and no digest.

### 4.5 Changes Review Kit needs (additive)

| Field | Today | Why |
|---|---|---|
| `anchors[]` | one optional `anchor` per event (rk:schema/findings.cue:94) | a reliability finding spans dial site, pool config and alert |
| `blobId`, `digestScope`, `anchorState` | anchor is `{path, digest, start?, end?, symbol?}` (rk:schema/findings.cue:37-43) | ADR-068 d3 alignment; Review Kit's own brief demands a digest scope (rk:docs/design/fathom-uplifts.md:131-134) |
| `index_commit` == `sourceRevision` check | passed through, not compared (rk:internal/evidence/certify.go:252) | revision skew |
| reliability domain on rules | class axis is fathom's closed ten (rk:schema/review.cue:134-138) | see section 11, row 9 |

These are pe-side records (owner decides): `2026-10-11-making-the-vision-happen.md` section 6 lists them.

### 4.6 Fix in fathom first

The findings importer stamps `CompiledAtWatermark: ev.SourceRevision` (internal/findings/external/importer.go:256)
but hashes cited lines through its `FileContent` hook (internal/findings/external/importer.go:36-41), which the
product wires to `os.ReadFile` under the service root, the working tree (pkg/client/findings.go:56-66). Anchor and
claimed revision can differ. Fix: hash at `SourceRevision` via `git show`, or refuse the cites link naming the
skew when that revision is unreadable (WAE-1).

### 4.7 "Fathom as the findings backend"

Answer: **evidence store plus edit-time projection, not the record.** ADR-040 d1 makes fathom "a projection
adapter, never the organizational record" (ADR.md:2155-2175). Review Kit's ADR-0098 sets the bar to revisit:
export, isolation, retention, lifecycle, availability (pe:docs/adr/0098-review-kit-is-portable-organizational-assurance.md:55-65).
Superseding needs both records changed together and vision 30 section 5's durability list
(docs/vision/30-findings-loop.md:300-303). Owner decides; recommendation is keep.

## 5. Pre-investigation and its freshness gate

**The fact sheet.** Per service, at a commit: every fact kind any enabled bundle requires, stored as observations
at ingest, plus the J joins, computed at request time and never stored (ADR-058 decision 2). Deterministic, no
model, $0. Its facts travel inside the service's index snapshot (`2026-10-11-index-snapshots.md`) and are served
by existing tools: `symbol_context` on a workload entity, `localize` on an
alert name once its incident section is built, `export receipt --kind fact`.

**The ratchet** [J, the platform design's gap e, adopted]. Whole-repo audits drown teams. A PR job diffs
concern facts at head against the merge-base index snapshot: "this PR removed jitter", "added a client with no
timeout", "dropped a PDB", "widened an IAM action to `*`". This is the conservation shape (ADR-044) applied to
concern facts, and it reuses `review --since` (cmd/fathom/review.go:47). Where pe's `pdk/risk` already gates
the change, the delta enters as named reasons on the change's contribution (pe:pdk/risk/model.cue:43-51), not as
a second gate.

**The freshness gate in CD** (Review Kit `admit` decides, never a fathom verb):

| # | Check | Mechanism | Today |
|---|---|---|---|
| 1 | Store watermark equals the deploying SHA | `fathom review --expect-index-watermark <sha>` sets `stale_index` and refuses persist (cmd/fathom/review.go:64,225-229) | shipped |
| 2 | Review Kit compares `index_commit` with `sourceRevision` | rk:internal/evidence/certify.go:252 | missing |
| 3 | Required fact-kind versions are present in the store | `fathom version --json` + a fact-kind manifest | missing (version --json exists without epochs or kinds, cmd/fathom/version.go:80-90) |
| 4 | An evaluation is recorded for that commit | Review Kit | Review Kit side |

**Fathom never stops mitigation.** The gate is fail-open on any fathom operational failure, the repo's own
convention (verify exit 2 = operational, fail-open; root `AGENTS.md`), and records the exception:

- Daemon down, store missing or stale beyond its budget: admit, record `evidence: unavailable`, open a
  follow-up evaluation. Never wait 76 to 299 s for an ingest on the deploy path
  (docs/bench/readings/efficiency-wave/typed-cold-cost.md:149,152).
- Rollback to a commit that already has a recorded evaluation: admit on that evaluation.
- A break-glass deploy (pdk/risk tier T3, "written rollback + WORM record", pe:pdk/risk/model.go:14): admit;
  the evaluation runs after.

Reading: WAE-M7, the latency the gate adds per deploy (median and p95 over 30 deploys), with a bar fixed before it
runs.

Every answer carries an answer basis: `exact`, `differs`, `unchecked` or `uncommitted`, naming each
`service@commit` it read (`2026-10-11-index-snapshots.md` section 3.2). The gate accepts `exact`, or `differs`
when its own per-service comparison passes; `unchecked` takes the fail-open path above.

## 6. Incidents: investigation, RCA, PIR and agent-led mitigation

### 6.1 What fathom answers, minute by minute

All calls are existing tools. `localize`'s incident section is decided (ADR-058 d7, ADR.md:3581-3584), not built.

| t | Question | Fathom call | Answer | State |
|---|---|---|---|---|
| 0 | Which code and owner does this alert or pasted frame point at? | `localize` with alert name or frame | rule:line, routes_to receiver, emitting call (log template), owners | Stage A shipped (TL-1, TL-4, ADR-062); owners need IM-F1 |
| 0-1 min | What did we already know was risky here? | `symbol_context` on the workload | the fact sheet at the deployed commit | needs FG0 to FG3 (`2026-10-11-making-the-vision-happen.md` waves 2 and 3); TL-15 open |
| 1-5 min | What changed? | `localize --deployed A --previous B` | git-visible suspect list titled "not cause", blind spots named | IM-1..3 open, gated on IM-M0 (docs/plans/incident-maps.md:21,32-34) |
| 5-15 min | What does each suspect reach? | `blast_radius`, `review_diff`, `obligations`, `read_source ref=<sha>` | reach, rules, certification gaps, text at the deployed commit | `read_source` reads disk today (internal/mcp/readsource.go:170-188); ref-pinned read is WAE-3 (`2026-10-11-making-the-vision-happen.md` wave 1) |
| mitigate | Is rollback safe? | `review --since previous --dimension rollback` over previous..deployed | extends incident-maps 6.3, which already lists the commits a rollback removes when deployed is an ancestor of previous (docs/design/incident-maps.md:214-215); the dimension adds what that list lacks: migrations a rollback cannot undo, contract-key changes and their consumers, config and flag deltas | new, gated on IM-M0 BUILD (decided in section 11, row 4) |
| RCA | Can a stranger check the narrative? | `fathom export receipt` per cited fact; `receipt verify` | checkable receipts | EVX-6 shipped; EVX-9..13 open |
| PIR | How does this never recur? | none new | (a) a committed incident-map declaration (ADR-058 d1); (b) a Review Kit requirement with `#FathomCertifyBinding kind: "incident"` (rk:schema/review.cue:57-60) and a test-kit certifying test; (c) a new fact kind if the cause was invisible | **this loop is how incidents feed prevention** |

### 6.2 Agent-led mitigation

Fathom is a **context source** for the agent, not the agent. The agent (HolmesGPT, CNCF sandbox,
https://www.cncf.io/blog/2026/01/07/holmesgpt-agentic-troubleshooting-built-for-the-cloud-native-era/; a
vendor AI SRE; or the team's own Claude agent) reads fathom over MCP from a read-only sidecar, and acts
through its own ops tools under approval controls that policy-kit expresses.

Fathom must not:

- execute a probe, rollback or flag flip (IM-X3);
- propose or rank a mitigation through its own model, name a cause, or set severity (IM-X9, ADR.md:3571-3580);
- store incident state, a timeline, or drills (IM-X4);
- fetch telemetry, deploy events or cluster state (IM-X1, IM-X2; non-goals.md:7-13).

What fathom adds to an agent [J]: vendor agents already connect to source repositories, run scheduled sweeps and
retain context from prior investigations (Azure SRE Agent: "connects to your Azure resources, observability tools,
incident platforms, and source code repositories", "Schedule proactive health checks, compliance sweeps", "retains
context from prior investigations", https://learn.microsoft.com/en-us/azure/sre-agent/overview). So the
differentiator is not timing. It is that fathom's fact sheet is **deterministic, commit-pinned and receipted**: the same
answer at the same commit, anchored to the deployed bytes, with coverage stated, checkable by a stranger. Benchmarks
put agent root-cause accuracy near 4 to 12 percent (section 2); a sheet the agent cannot hallucinate removes one
error surface.

### 6.3 What must read first

- **Latency.** Under load the test-run hook's `localize` was abandoned 200 of 223 times between 2026-10-09 and
  2026-10-11 (p50 2,003 ms against a deadline of about 2 s), and the edit hook's `obligations` 58 of 259
  (`docs/bench/readings/self-use/2026-10-11-hook-path-abandonment.md`, hygiene note G11). IM-M5 (cold p95 < 3 s,
  docs/plans/incident-maps.md:54) and that fix must read before "real-time" is claimed.
- **Suspect value.** IM-M0 (revert-corpus replay, docs/plans/incident-maps.md:21) decides whether the suspect
  list is built at all.

## 7. Placement

| Piece | fathom | Review Kit | Other kit | New | External |
|---|---|---|---|---|---|
| Facts from indexed files (manifests, proxies, alerts, IaC, IAM, code) | yes | | | | |
| Cross-file joins (probe->handler->callee, retry chain, workload->monitor->alert->receiver, replicas x pool, IAM->SA->code, Ingress->handler, NetworkPolicy coverage) | yes | | | | |
| Coverage statements on absence facts | yes | consumes | | | |
| Catalog, bundles, lens mapping, severity | | yes | | | |
| Library-defaults pack | | yes | | | |
| Resource and security rules, framework crosswalks (CIS, SOC 2) | | | policy-kit | | Kyverno interim |
| Change-risk tiers over Pulumi preview and manifest diffs; compliance floors | concern facts as named reasons | | pe `pdk/risk` | | |
| Single-file IaC and secret scanning | | imports SARIF as external findings | | | Checkov, Trivy, KICS, Gitleaks |
| Pass/fail, findings record, admission | | yes | | | |
| Drift attestation (`pulumi preview --json`, Crossplane XR diff, flux status) | joins by address | judges | pdk/risk parses; the publisher job attests | | Pulumi, Crossplane, flux (OpenTofu, Argo for other adopters) |
| Test that certifies an incident fix | certify gaps | binding | test-kit | | |
| Runbook docs and alert->runbook coverage | link facts | requirement | docs-kit | | |
| Mitigation approval controls | | | policy-kit | | |
| Investigating or mitigating agent | context only | | agentic-kit skills | | HolmesGPT, vendor agents |
| Postmortem calibration corpus | replay harness (IM-M0 shape) | bundle calibration | | yes: a pre-registered public corpus | |
| Scorecard display | fact collector only | | | | Backstage Soundcheck, Tech Insights |

## 8. Ranked roadmap

Bars are fixed before each reading runs (K19, ADR.md:4412). Spend: R0 to R5 are $0 in model spend; owner hours
are counted in `2026-10-11-making-the-vision-happen.md` section 5.

| Phase | Build | Exit measurement | Kill |
|---|---|---|---|
| R0 | Readings only. **WAE-M0**: owner commits a 6-concern draft bundle (#18, #6, #8, #1/#2, #9, #24) and stratified hand labels before any fathom run: 40 alerts and 30 workloads sampled by a committed rule across opentelemetry-demo, microservices-demo, kube-prometheus and istio samples, plus pe's mimir manifests for workload facts. **WAE-M1**: on the same corpus, hand-count client construction sites with literal retry or timeout options vs sites relying on library defaults. Importer skew fix (WAE-1) | Labels committed; WAE-M1 share recorded | A J concern with no true instance in the corpus is dropped |
| R1 | FG0 fact storage; FG2 alert facts; FG1 (LB-F451 widened, env names and args flag names only, sentinel test); `version --json` adds epochs and fact-kind versions | **WAE-M2**: precision >= 0.95 against a structural-YAML oracle on calibration manifests; store growth <= 5% (K26); the sentinel appears in no store row | precision < 0.90 or growth > 10%: revert that family; a sentinel in any row blocks the family |
| R2 | Joins: J1 probe->handler->callee; J2 workload->monitor->alert->receiver (TL-15, TL-16); J3 per-hop retry x timeout (FG4); S1 to S3, S6 | **WAE-M3** per reliability join and **WAE-M6** per security join on WAE-M0: >= 1 true instance per 2 repos at precision >= 0.90 | below either bar: removed, not re-tuned on the same holdout |
| R3 | `export receipt --kind fact\|join`; Review Kit `fathom-facts` adapter and a 10-rule reliability bundle (Review Kit side) | EVX-M1 stranger test passes on a fact receipt; **WAE-M4**: Review Kit closes >= 1 finding on pass + complete coverage; owner-judged precision >= 80% on 30 sampled findings over 3 repos | precision < 50%: cut to alert quality only |
| R4 | Calibration (**WAE-M5**): before the bundle is authored, pre-register at least 20 public postmortems selected by a written rule (each names a culprit commit in a public repository and a cause in a concern family the bundle covers); replay the bundle at each culprit's parent | recall with its Wilson 95% lower bound; a "prevention" claim only if the lower bound is >= 0.20, fixed now (at n = 20 that needs 8 caught) | lower bound < 0.20: no "prevention" claim in any doc |
| R5 | FG7 code client facts for Go (typed pass), then SCIP lanes, only if WAE-M1 shows >= 30% of sites carry literal options or a named library | J3 extended to code: precision >= 0.90 on WAE-M0 code labels | below bar: code side stays grep-level, documented as `incomplete` coverage |
| R6 | Incident stage 1 (IM plan) and the `rollback` review dimension | IM-M0 BUILD, IM-M5, IM-M2 KEEP | IM-M0 KILL drops F56 (ADR.md:3632) and the rollback dimension |
| R7 | Judged sites for H rows (hand-rolled retry loops, health-pool separation, auth middleware) | Wilson lower bound >= 0.90 (ADR-056 R3); cost per true positive under the owner's bar | spend hold not lifted: stays unbuilt |

Why R4 changed: 3 of 5 caught has a Wilson 95% lower bound of about 0.23, and fixtures chosen after the bundle is
written are biased toward what it catches.

## 9. Risks

| Risk | Mitigation |
|---|---|
| False "clean" from incomplete coverage (absence claims) | coverage is mandatory on every fact; `incomplete` never closes a finding (rk:schema/findings.cue:106-110) |
| Secrets in env values or args reach a store or a receipt | env names and `valueFrom` only, arg flag names only, two allow-lists of non-secret keys; ADR-065 d2 sentinel test (WAE-13) |
| Declared is not live | every fact is declared state; Review Kit marks coverage `incomplete` when drift is unattested |
| Noisy facts make teams mute the catalog | WAE-M2 and per-join K19 bars |
| Fathom blocks a hotfix or rollback | the gate is fail-open with recorded exceptions (section 5) |
| AWS or Datadog add source reading | moat is determinism, receipts, multi-cloud, self-hosted, cross-layer joins; not breadth |
| Review Kit maturity (v0.1.0, rk:CHANGELOG.md:12; RP-9 pending, rk:docs/design/fathom-uplifts.md:189-194) | R3 is a joint milestone; fathom does not build Review Kit's side |
| Duplicating pdk/risk | concern facts feed its reasons; no fathom gate (section 5) |
| Hook latency undermines real-time claims | IM-M5 reads first |
| Scope sprawl vs the language-excellence program | each phase has a kill; R1 rides language wave lf8 (`2026-10-11-making-the-vision-happen.md` section 5); the whole program has a kill date (G37) |
| RT-M1 freeze persists: the 2026-10-04..10-11 window FAILed and the amended first window is 2026-10-12 00:00Z..2026-10-19 00:00Z (docs/plans/responsible-tenant.md:250) | facts as observations need no table; cross-repo pool arithmetic waits |

## 10. What this answers of the owner's ask

| Ask | Answer |
|---|---|
| Fathom as the investigation and evidence engine behind Review Kit's well-architected catalogs | yes for facts, joins and receipts (sections 3, 4); Review Kit judges |
| Compliance, governance, breach prevention | joins S1 to S8 (section 3.4); policy-kit and pdk/risk own the rules and tiers |
| Real-time investigation, RCA, PIR | existing tools plus decided-not-built pieces (section 6.1); "real-time" waits on IM-M5 |
| An AI agent that mitigates | fathom is its read-only, receipted context source; it never acts (section 6.2) |
| Fathom as the findings backend | no (section 4.7) |

## 11. Disagreements resolved here (owner-architect)

| # | Question | Platform | Boundary | Decision | Why (one line) |
|---|---|---|---|---|---|
| 1 | Fact transport | new adapter, transport unstated | EVX-25 `export receipt <tool> --args` | `export receipt --kind fact\|join` (EVX-9's kind table), both at `fathom@<version>` | EVX-25 waits on EVX-M3; git cannot prove a structural fact (ADR-068 d4) |
| 2 | Library-defaults pack | a fathom future | not addressed | Review Kit bundle data keyed on `pkg@version` plus env facts; fathom supplies both | defaults are judgment knowledge (ADR-055 d7) and depend on env opt-ins; fathom never grades |
| 3 | Code client facts timing | not scheduled | not scheduled | $0 WAE-M1 in R0, build in R5 only past its bar | the owner's flagship needs a measured go/no-go, not silence |
| 4 | Rollback facts | new `localize` section | `review_diff` over revert range | a `rollback` dimension of `review --since`, extending incident-maps 6.3's removed-commit list, gated on IM-M0 | review already owns range analysis; no new tool (ADR-053) |
| 5 | Ratchet vs audit | ratchet first | not addressed | ratchet in PRs is the default delivery; deltas feed pdk/risk reasons where it gates | durable and cheap; audits drown teams |
| 6 | Calibration on real incidents | R4 replay | WAE-M0 labels | both, in order: labels first, a pre-registered n >= 20 replay before any "prevention" claim | precision and recall need different corpora; n = 5 proves nothing |
| 7 | Findings backend | no | no | no; evidence store plus projection | ADR-040 and ADR-0098 bars are unmet |
| 8 | Fact storage | observations | observations | observations under the freeze | agreed; LogTemplate precedent |
| 9 | Reliability finding class | not addressed | Review Kit decouples from fathom's classes | keep fathom's ten classes; tag rules with Review Kit's reliability domain (rk:docs/design/review-domains.md:20); map missing jitter to `conformance`, pool exhaustion to `resource-leak`, drift to `config-drift` | the class says what kind of defect, the domain says which pillar; revisit if over 20% of reliability rules misfit |
| 10 | Security scope | not addressed | not addressed | joins S1 to S3, S6 in fathom; rules in policy-kit; tiers in pdk/risk | the owner asked for breach prevention; the dialects already exist |
