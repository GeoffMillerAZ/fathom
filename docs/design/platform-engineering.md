# Platform engineering: the fathom side of readiness readings, fleet fact inventories and investigations

Status: proposed 2026-10-25, revised the same day after a critic pass (owner-architect program design for GitHub
issue #14, https://github.com/GeoffMillerAZ/fathom/issues/14). Plan: `docs/plans/platform-engineering.md` (PLT-).
Roadmap: `docs/vision/38-proof-layer-roadmap.md` section 9.15. It extends vision 44, ADR-069, goal G37 and
`docs/plans/well-architected-evidence.md` (WAE-), and its decisions enter the records as a dated ADR-069 amendment
when PLT-0 lands. Read against fathom at `743a91430` and against an adopting platform team's records, which stay
private (ADR-011).

## Decision

1. **Fathom is the evidence side of decisions an adopting platform team has already recorded.** In the consumer shape
   this design serves, readiness is a derived reading and a review kit is the Well-Architected review tool. A
   readiness inventory collects committed facts. The review kit reads fathom through a facts lane, a change-risk
   tierer takes fathom concern deltas as named reasons, and operational catalogs and an observability kit cover
   controls, failure modes and monitoring coverage. Fathom builds only what those consumers read: a fact receipt per
   subject with a commit basis, a concern-delta file, two skills over existing verbs, and the readings that keep or cut
   each one. Facts, joins and the base fact receipt stay WAE-'s. Change review is CX-, cross-repository impact SG-,
   incident context IM-, portable receipts EVX-, index snapshots IXS-.
2. **Fathom ships no question map.** Lens mappings (for example REL05-BP03) and the AWS custom-lens export are the
   review kit's (ADR-055 decision 7). Framework crosswalks are the policy kit's (ADR-069 decision 5). An adopter's
   operational catalogs cite the AWS sources themselves. A Well-Architected question is answered by a review kit
   requirement bound to a fathom fact kind through the facts lane. A question map in fathom's trace mapping is
   deferred until an adopter without a review kit asks in writing (PLT-D9). Its corrected shape is kept in section
   4.6, so the deferral forecloses nothing.
3. **The inventory is a fact receipt per subject, and it is not a readiness reading.**
   `fathom export receipt --kind fact --kinds K1,K2` writes every fact of those kinds for each subject (a workload,
   an alert rule, a module) at one commit, with one coverage line per kind and a basis. It states "declared at commit
   C, not deployed state". Nothing rolls subjects up to a service. The readiness inventory and the facts lane read
   it. Conftest and kyverno-json read it as it is, so a recipe replaces a policy renderer. A service catalog gets
   a shape only through SG-45's `catalog export`, extended once a named consumer runs a catalog (PLT-D2). Ownership
   stays with CODEOWNERS through IM-10 and with the catalog's own descriptor (PLT-X10).
4. **Concern deltas are information, in the file shape a change-risk tierer reads.** For each manifest a change
   touches, head facts are compared with facts re-extracted from the base blob at request time. Each added, removed
   or changed fact is one delta, carrying its address, before and after values, anchors, basis and commit. A delta is
   never a loss, never counted by `--strict`, and has no direction rule. The delta file is CLI-only (`fathom conserve
   --since B --concern-deltas FILE`) and sits beside the report as the reading set does (`Report.Residue`). It is
   never a registered plane, so `conserve.Planes()` and the daemon's `review_diff` path stay unchanged. The
   change-risk tierer is the named consumer. A review kit's ratchet may read the same file.
5. **Investigations and reviews use the existing verbs through two shipped skills.** A feature-labs card measures
   them before any new output is built. A review-packet directory waits until reviewers are seen assembling one by
   hand (PLT-D3).
6. **The fact at the edit rides platform modules and guarded edits.** An org delivers its Standard's requirements to
   coding agents as a bound platform module, with guards on manifest paths (`docs/use-cases/platform-modules.md`).
   This is a recipe and adds no code.
7. **Measurement joins the shared programs.** Subjects are bench-prep manifests tagged `platform`. Bars live in a
   bench-prep prereg. The firing lab is the feature-labs card `lab-platform-skills`, and its rows go to the labs
   ledger. Where an oracle exists, the grader is deterministic: WAE-M2's structural-YAML oracle, WAE-M0's labels and
   `receipt verify`. Owner labels grade only what no oracle can. Each grader must discriminate before its reading
   counts.
8. **The goal is G37, widened.** PLT's readings join G37's threshold and its kill (2027-01-31). No new goal.
9. Plan prefix **PLT-**. It was unused when this was written: `PE-` belongs to progress-and-economics.

## 1. Intent read before designing

### 1.1 What a platform consumer needs from fathom

| Consumer | What it needs | What fathom does with it |
|---|---|---|
| A readiness reading | Readiness is derived, `f(Standard, evidence snapshot, as_of)`, never a stored status. Fathom claims count only when fresh. Absence requirements are coverage-scoped. No composite score gates. Org content (the Standard, packs) is the adopter's configuration | Supplies evidence with coverage and a basis, never a sounding, mark or score. Holds no org content |
| A readiness inventory | Committed facts collected as `present`, `absent` or `unknown`, each with the tool that grounded it. Review catalogs may federate | The fact receipt is one more committed-fact source. A fathom `not-found` is a lower bound, so it maps to `unknown`, never `absent` |
| A review kit's facts lane | Runs `fathom export receipt --kind fact\|join` and re-derives pass or fail itself. A result counts only when the index commit equals the reviewed revision and the basis is `exact`. Lens mappings and the custom-lens export stay in the review kit | PLT-7 carries a basis. Fathom maps no lens |
| A change-risk tierer | Reads a CI-named concern-delta file as named reasons (kind, before, after, basis, commit). Uses only `exact` deltas, fails open, and is never a second gate. Which address joins a delta to a change is the tierer's choice | PLT-5 writes that file and gates nothing |
| Operational catalogs and observability coverage | Controls and failure modes cite AWS Well-Architected sources. Monitoring coverage is assessed against a captured configuration inventory | Alert and monitor facts become one captured-inventory source (PLT-D10) |
| Review criteria with judgment backends | Optional judgment backends on review criteria | "Does this evidence answer the question" is a judgment site in the review kit, not in fathom |

### 1.2 Fathom's records

- **ADR-011.** Fathom ships no framework question text (CX-X3), lens catalog or catalog content. An org's catalog
  choice, review cadence and team names are the operator's configuration (a private pack). They are never committed
  here, including under `docs/bench/readings/`.
- **ADR-069 decision 5.** The review kit judges, the change-risk tierer tiers, and the policy kit owns resource rules
  and framework crosswalks. Fathom has no deploy-gate or readiness verb, and the PR ratchet is the default delivery.
  This design follows it: the receipt carries no verdict, the delta file delivers the ratchet, and fathom adds no
  crosswalk.
- **ADR-055 decision 7.** Lens catalogs, severity, dispositions and the AWS custom-lens export stay in the review kit.
- **ADR-053, ADR-063, ADR-068.** No new MCP tool, no second verifier or result format, and receipts in formats other
  tools already read.
- **Invariants.** I38 and I44: no conclusion the inputs could not earn. I56 and ADR-069 decision 10: no download,
  and every network adapter is the operator's. I57: a link below `typed` never gates. I58: nothing ranks people. I62:
  a judgment is never a fact.
- **Non-goals.** No domain connectors (fathom never fetches ops data), no incident command center, no dispatch loop,
  no hosted backend.
- **Table freeze.** RT-M1 read FAIL on 2026-10-19, and its restarted window (2026-10-20 to 2026-10-27) has not read.
  Nothing here adds a table, column, cache, memo file or background job.
- **Triggers checked.**
  - **TX-4's consumer half is met.** A review kit that reads fathom concern facts through a facts lane is a named
    consumer for a concern pack, and fathom's records never noted it. The corpus and eval-fixture half waits on
    WAE-M0's labels. ADR-069 is already fathom's response: concern facts, with the review kit as the consumer and G37
    as the kill. Pack content is the adopter's configuration (ADR-011), so the firing promotes concern facts through
    WAE-, not a fathom-side pack. F6 carries the note, and PLT-0 records it.
  - **Not fired.** EVX-D1 (OSCAL) waits on a compliance consumer asking in writing. F32's promote waits on WAE-M4.
    SG-45 (`catalog export`) waits on SG-2 and RT-M1 PASS.

## 2. Thesis

Platform teams ask three things of a fleet:

- Review: does this workload meet the framework?
- Inventory: what is true of every service now, and what changed?
- Investigation: what changed near this failure?

Today's tools answer from deployed resources (the AWS Well-Architected Agent, Azure Advisor), from one resource graph
(Checkov, Trivy) or from metadata (catalogs). An adopter's review kit reads committed facts and judges them. None of
these reads an application's call graph.

Fathom answers from code and configuration at one pinned commit, across repositories, with a read set and a receipt.
It supplies evidence to whoever holds the verdict, and says plainly what code cannot show.

| | What only fathom adds | Delivered by |
|---|---|---|
| N1 | Code-to-config joins at one revision across repositories: probe to handler to the clients it reaches, workload to alert to receiver, retry attempts multiplied along a route | WAE-8, WAE-9, WAE-12, WAE-14 as join receipts (WAE-10) |
| N2 | Absence with a read set: "no NetworkPolicy selects this workload, having read these files", a lower bound when a read failed | PLT-4, WAE-14 |
| N3 | Re-derivable receipts: the commit and the fathom version produce the same answer for an auditor or a review board | PLT-7, PLT-15, EVX- |
| N4 | The same fact at the edit: obligations and guards show a gap before merge, not in a quarterly scorecard | PLT-14 |

## 3. What exists and what is reused

### 3.1 Inside fathom (read at `743a91430`)

The 2026-10-25 research inventory was stale in three places, corrected here:

- LB-F451 landed. Kubernetes per-container facts (requests, limits, probes, replicas, init containers) are `K8sFact`
  observations of the manifest's own file. They are stored but not served yet (WAE-5).
- TL-15 landed. `symbol_context` names the monitors that select a workload.
- The OpenTelemetry Collector dialect is at L3.

| Piece | Gives a platform team | State |
|---|---|---|
| `K8sFact` observations, observed-by (LB-F451, TL-15) | Probes, resources and replicas per workload; which monitors select it | stored; observed-by served; facts unserved (WAE-5) |
| Dialects (`docs/languages.md`) | Terraform/OpenTofu, Kubernetes, Helm, Kustomize, Argo, Flux, Compose, Dockerfile, GitHub Actions, CloudFormation, Bicep, Pulumi YAML, Ansible, nginx, HAProxy, Envoy, Prometheus, Alertmanager, OTel Collector, IAM, OpenAPI, Checkov policies | entities and edges at L2 to L3; values mostly not facts yet (WAE-4, WAE-6, WAE-11) |
| `export receipt --kind fact\|join` (WAE-10), anchored receipts with `index_commit`, `receipt verify` (EVX-) | The record a review kit's facts lane reads; a stranger re-checks it with git | fact and join kinds open; the in-toto statement (EVX-16) open |
| `read_source ref`, `version --json` fact-kind versions (WAE-3, WAE-2) | Text at the deployed commit; the cache-key components | shipped |
| `fathom conserve --since`, `review --since` conservation dimension (ADR-044), `Report.Residue` | What a change loses, by plane; the reading set as an informational section no verdict reads | shipped |
| `fathom trace export`, `.fathom/trace/<standard>.cue` (CX-14 to CX-16) | An evidence pack under clauses the operator names. `--commit` reads the mapping at that commit and the facts at the index head | shipped; `trace verify` (CX-17) open |
| Change risk receipt, `review --receipt --format sarif\|check` (CX-1 to CX-8) | Cited reasons, a band (the strongest reason, never a sum), suggested reviewers from CODEOWNERS | shipped; CX-M1 and CX-M5 open |
| Platform modules, guarded edits (`docs/design/platform-modules.md`, `docs/design/guarded-edits.md`) | An org's standards bound once and served at the edit; pre-edit guidance on critical scopes | shipped, experimental, no external consumer |
| `blast_radius`, `platform-callers`, system graph, `catalog export` (SG-, SG-45) | Cross-repository callers in four evidence tiers; a catalog-info shape on request | partly built; SG-2 and SG-45 wait on RT-M1 PASS |
| `localize`, `culprit`, incident maps, `internal/ownership`, `ownersFacet` (IM-, IM-10) | Seed localization, a red commit range ranked, owners from CODEOWNERS with the rule and line | localize and culprit shipped; IM-10 open behind IM-F1 |
| agentic-kit `foundations/investigation` (journal, coverage, hypothesis-ranker), `foundations/review` | Investigation and review protocol shapes | vendored; no fathom verb uses them |

### 3.2 Outside fathom and the adopter: integrate, never rebuild

| Tool | What it is | How fathom fits |
|---|---|---|
| AWS Well-Architected Tool | Custom lens JSON; `UpdateAnswer` writes `SelectedChoices` and Notes up to 2,084 characters (https://docs.aws.amazon.com/wellarchitected/latest/userguide/lenses-format-specification.html, https://docs.aws.amazon.com/wellarchitected/latest/APIReference/API_UpdateAnswer.html) | A review kit's custom-lens export writes the evidence notes, citing fathom receipts. Fathom writes nothing to the tool (PLT-X11) |
| AWS Well-Architected Agent (preview 2026-10-01) | Scans deployed resources and reviews CloudFormation and Terraform, through the AWS MCP Server (https://docs.aws.amazon.com/wellarchitected/latest/userguide/agent.html) | Fathom adds the application-code side the agent cannot see. A user loads both MCP servers. A paired lab arm is deferred (PLT-D11) |
| Azure Well-Architected Framework, Advisor assessments | Five pillars; about 60 questions per workload type; no write API found (https://learn.microsoft.com/en-us/azure/advisor/advisor-assessments) | A report the reviewer reads beside the assessment, never a sync |
| Google Cloud Well-Architected Framework | Six pillars, each with an agent skill of assessment questions (https://github.com/google/skills) | An agent loads the pillar skill and answers through fathom's verbs. PLT-M5's framework-skill arm measures the pairing |
| Production readiness review (Google SRE) | Architecture, instrumentation, emergency response, capacity, change management (https://sre.google/sre-book/evolving-sre-engagement-model/) | Code shows dependencies, instrumentation presence, runbook links and rollout config. Capacity, staffing and drills need a human attestation |
| Service catalogs: Backstage Tech Insights and Soundcheck, Cortex, OpsLevel, Port | Tech Insights fact retrievers on a cadence (https://github.com/backstage/community-plugins/tree/main/workspaces/tech-insights/plugins/tech-insights-backend). Cortex custom data, where the entity descriptor wins unless `force=true` (https://docs.cortex.io/ingesting-data-into-cortex/entities/custom-data). OpsLevel custom events up to 4 MiB (https://docs.opslevel.com/docs/checks/custom-event). Port entity upserts, which overwrite relations (https://docs.port.io/promote-scorecards/) | A shape through SG-45's `catalog export` once a named consumer runs one (PLT-D2). Ownership stays with the catalog's descriptor |
| Conftest and OPA, kyverno-json | Rego over structured config; Kyverno policies over any JSON payload (https://github.com/open-policy-agent/conftest, https://kyverno.io/docs/subprojects/kyverno-json) | They read the receipt JSON as it is. A recipe, no renderer (PLT-12, PLT-X9). The rule stays in the policy tool (WAE-X1) |
| Checkov, Trivy | IaC and secret scanning with SARIF output | A review kit imports their SARIF, and it reaches the edit through the findings loop (ADR-040). Deferred PLT-D8 |
| in-toto v1, SLSA 1.2, OSCAL, OpenSSF Scorecard | Attestation envelope, provenance, assessment results; Scorecard's -1 for an inconclusive check | The inventory receipt is written through EVX-16's statement (PLT-15). OSCAL stays EVX-D1. Scorecard's -1 is the precedent for `not-found` |
| CNCF platform maturity model, DORA | Organizational maturity; delivery metrics from deploy and incident events, with a warning against targets and cross-team comparison (https://dora.dev/guides/dora-metrics/) | Out of scope: neither is a code fact (PLT-X5) |
| incident.io investigations, Datadog Bits, HolmesGPT | Agents that read deploys and code and test hypotheses | Fathom is the code-evidence MCP server they call, never an incident tool (WAE-X4) |

## 4. Architecture

### 4.1 Components

```
code, config and IaC at commit C
  ingest (existing dialects) -> concern facts as observations of their own file (K8sFact today; WAE-4 for the rest)
    request-time joins (WAE-8, WAE-9, WAE-12, WAE-14)
    serving:    symbol_context fact sheet (WAE-5); obligations and guards at the edit (PLT-14)
    inventory:  export receipt --kind fact --kinds ... per subject, with a basis (PLT-7, PLT-4) -> EVX-16 statement (PLT-15)
    deltas:     conserve --since B --concern-deltas FILE, CLI-only (PLT-5)
consumers (adopter): the review kit's facts lane, the readiness inventory, the change-risk tierer,
                     an observability kit later
consumers (other): a policy engine reading the receipt; an investigating agent (MCP, the skills); a review board
```

Data stays in files and request-time computation. Exports go to a path the operator names, and readings go under
`docs/bench/readings/`. Nothing here writes the store beyond the fact observations WAE-4 already plans.

### 4.2 The fact receipt as an inventory (PLT-7, PLT-4, PLT-15)

One receipt per service at one commit carries one entry per subject. A subject is a workload, an alert rule or a
module. Each entry has every fact of the requested kinds, in the fact record of the 2026-10-11 assessment (section
4.4: kind and version, subject, values, anchors, coverage, `rederive: fathom@<version>`). Each subject and kind
pair has one class:

| Class | Rule |
|---|---|
| `facts-present` | the kind has a fact on this subject with complete coverage |
| `facts-incomplete` | facts found, but coverage is incomplete: a tier below `typed`, a blind spot, a withheld value |
| `not-found` | nothing on this subject in the read set. It is a lower bound that lists the read set, never "absent", and never a pass or fail |
| `unmapped` | a requested kind this binary does not know, named |

There is no rollup. One probed Deployment never makes another workload, or the service, read `facts-present`.
The receipt also carries:

- one coverage line per kind (files of that kind read, unreadable and skipped);
- withheld values (ADR-065 decision 2) as state `withheld` with no value;
- the line "declared at commit C, not deployed state";
- a basis.

The basis uses ADR-069 decision 13's states, on these export files only:

- `exact`: the store's indexed commit for the service equals the requested commit, and no read file is dirty.
- `differs`: the commits are not equal, and both are named.
- `uncommitted`: a read file has uncommitted changes.

Consumers that need `exact` (the facts lane, the change-risk tierer) drop the other two. The tool-wide answer
basis stays IXS phase 3. Output is sorted and deterministic.

The receipt carries no owner. CODEOWNERS reaches incidents through IM-10's facet, and a catalog's descriptor already
owns ownership. A second source would disagree with both (PLT-X10).

Runbook presence is one more fact kind, from WAE-6's annotations. When EVX-16 lands, the receipt is written through
its in-toto statement, and `receipt verify` re-checks it (PLT-15).

**Readers.**
- The readiness inventory maps `facts-present` and `facts-incomplete` by its own rule, and maps `not-found` to
  `unknown`.
- The review kit's facts lane re-derives pass or fail itself.
- Conftest or kyverno-json read the JSON as it is.

The fleet aggregator is the consumer. Each repository's CI exports at its merge commit, and the operator's job moves
the file: ship files, never call (ADR-069 decision 9, ADR-078). A daemon that serves several services exports them
one `--service` at a time, CLI-side and under the tenancy budget. No fleet loop runs in the daemon.

### 4.3 Concern deltas (PLT-5)

`fathom conserve --since B --concern-deltas FILE` compares the `K8sFact` observations of every manifest in the range
with facts re-extracted from the base blob by the same extractor. The comparison runs in the CLI process and writes
nothing to the store.

Each delta carries:
- the Kubernetes `<kind>/<name>` that the change-risk tierer's manifest diff keys on, plus namespace and container;
- the fact kind and version;
- `added`, `removed` or `changed`, with before and after values;
- the anchors at both commits;
- the basis (section 4.2) and the head commit.

That is the tierer's reason shape. Which address joins a delta to a change is the consumer's choice, and both keys
are carried so either answer works.

The deltas sit in `Report.ConcernDeltas` beside `Report.Residue`. Neither has a verdict, and `--strict` reads
neither. "Replicas 3 to 1" is reported, and the change-risk tierer, the review kit or the reader judges it.

This is not a plane. A plane reports losses, and calling a removal a loss is a direction rule. A registered plane
would also run inside the daemon's `review_diff` (`internal/mcp/review_tool.go` calls `conserve.Planes()`), where
base re-extraction would cost daemon time. The file starts with `K8sFact` and widens as WAE-6, WAE-7 and WAE-11
land.

### 4.4 Investigations and reviews (PLT-10, PLT-11)

Two skills ship in `dist/agents/skills/`. Each composes the existing verbs and adds none.

**`fathom-investigate`** covers an incident, a regression or a failed deploy:
1. Run `localize` on the alert or frame.
2. Read the workload's fact sheet with `symbol_context`, and its text with `read_source ref=<deployed>`.
3. Run `blast_radius`, then `review --since <previous>` over previous..deployed, then `culprit` for a red range.
4. Cite a receipt for every fact.
5. Keep a journal in agentic-kit's investigation-journal shape. Every hypothesis carries its evidence and the label
   "not cause" until a human decides.
6. Name the blind spots.

The skill never mitigates and never names a cause as fact (IM-X9).

**`fathom-platform-review`** covers Well-Architected, production-readiness and inventory questions:
- Read the inventory receipt or join receipts per subject, or `symbol_context` per workload.
- Report each class and basis as computed.
- Name the review kit requirement when the operator's review kit binds one.
- Say that an attestation-only item (PLT-M0) needs a human.
- Never choose an answer.

The use-case guide (PLT-1) carries two recipes:
- The architecture-review packet: the ADR, `review --since --receipt`, `conserve`, `certify gaps` and the receipts.
- The investigation packet: the journal plus receipts.

A packet directory written by one command waits on PLT-D3.

### 4.5 The fact at the edit (PLT-14)

An org's Standard is the adopter's content, supplied as configuration (ADR-011). Its code-facing requirements reach
coding agents in three steps:
1. They are compiled once as a platform module and bound in each repository's `.fathom/modules.cue`. Obligations then
   name the requirement at the edit.
2. A guard in `.fathom/guard.cue` puts the requirement before the edit on manifest and IaC paths.
3. `fathom research report` says whether either was used.

The guide states that both are experimental with no external consumer. This is N4, and it needs no code.

### 4.6 Deferred: the question map's shape, if an adopter without a review kit asks (PLT-D9)

This design is not built now. It is recorded so a later build starts from the corrected version:

- A clause of `.fathom/trace/<standard>.cue` gains `evidence` (fact kinds by versioned id, join names),
  `answered_by` (`evidence` or `attestation`, default `attestation`) and `attestation` (the operator's words).
- The attestation clears only where the operator wrote `answered_by: evidence`. Code alone never clears it.
- `trace export` answers per subject in section 4.2's classes, with no rollup. It reads mapping and facts at one
  commit, with the basis, and refuses or reads `differs` on skew. Answers say "declared at commit C".
- Building it needs an ADR-069 decision 5 amendment, because a question map is a crosswalk.

### 4.7 The platform lab

**Subjects (PLT-L0).** These are bench-prep manifests, `calibration/bench.d/<subject>.yaml`, tagged `platform`:
- the WAE-M0 four at pinned SHAs (opentelemetry-demo, microservices-demo, kube-prometheus, the istio samples);
- one Terraform or OpenTofu subject;
- one GitOps subject.

BX-1's `#Subject` gains an optional `tags` field for this, additively. Bench prep's loader vets them for C1
(licence) and C7 (independence). C2 to C6 are waived because no agent task is derived from them.
The subjects are stratified for the T5 traps: retries set only in a mesh, Helm-templated values and Kustomize
overlays. The sampling rule, the seed and every bar are fixed in `docs/bench/prereg/platform.md`, in bench prep's
template (BX-24), before any run (K19). WAE-M0 samples from the same manifests, so the owner's labels serve both
programs.

**Graders.**
- WAE-M2's structural-YAML oracle computes the true facts and the true delta at every commit.
- WAE-M0's labels and `receipt verify` grade anchored answers.
- Owner labels grade only the `facts-incomplete` residue and the census audit.
- Before a reading counts, its grader must fail two stubs: an always-`facts-present` (or every-field) stub and an
  always-`not-found` (or empty) stub. This is bench prep's section 9 pattern.
- A false clean has a bar of zero in every reading. A false clean is a `facts-present` the oracle says lacks the
  fact, or a `not-found` stated as absence.

| Id | Question | Grader | Kills |
|---|---|---|---|
| PLT-M0 | Census: the share of framework items (AWS best practices at the REL05-BP03 granularity, Azure checklist recommendations, Google pillar recommendations, PRR areas) fully answerable from code and config, through the decision port | a 20% owner audit as labels; void under Cohen's kappa 0.6 | under 10% fully code-answerable removes `fathom-platform-review` (PLT-11) and refuses PLT-D9 |
| PLT-M1 | Baseline: what v0.20's verbs answer, per code-answerable item, beside grep | WAE-M0 labels and `receipt verify` | none: the reading later ones are compared with |
| PLT-M2 | Delta replay over at least 60 manifest-touching commits | the structural-YAML oracle's true delta per commit; precision and recall over all deltas | a Wilson 95% lower bound under 0.90 on either, or more than 5 deltas per commit at the median, removes PLT-5 |
| PLT-M3 | Receipt class precision on 3 subjects, every subject and kind pair | the oracle for `facts-present` and `not-found`; owner labels for `facts-incomplete` | a Wilson lower bound under 0.90, or any false clean, removes PLT-7's multi-kind mode |
| PLT-M4 | Consumer: an adopting platform team's consumer reads a PLT output on a real repository | the consumer's reading, cited by commit | unread or CUT on 2027-01-31 removes PLT-7's multi-kind mode and PLT-5 |
| PLT-M5 | The `lab-platform-skills` card: prompts drawn from PLT-M0's item ids, near-miss negatives, four arms (no skill, skill, skill plus a framework skill, grep only) | correctness against WAE-M0 labels and `receipt verify`; attestation honesty by a lexical check of the answer's attestation statement | anything short of correctness non-inferior to both the no-skill and grep-only arms, with zero false cleans, cuts the skill |

Firing is reported and never counts as helping (LAB K4). PLT-M5's prompts are drawn by seed from the census ids,
never written by the designer. One revision is allowed: it is made on the train split and read on a fresh test
draw. Results are appended to `docs/bench/labs-ledger.jsonl`.

Reused: WAE-M2, WAE-M3 and WAE-M6 (fact and join precision), WAE-M4 (the review-kit consumer), WAE-M5 (postmortem
replay), IM-M0 (revert replay), CX-M1 and CX-M5 (receipts in CI, band calibration), EVX-M1 (the stranger test).

**How results feed tuning.**
1. A CUT deletes what it killed in the same change, under a `gone:` row.
2. The census is handed to the review kit as data: which item ids a fact kind can serve, ids and URLs only. It sets the
   kind priority for WAE- and PLT-D7.
3. WAE-M2's precision per fact kind sets the tier a receipt shows for that kind.
4. The firing results tune the skill descriptions through the labs program's text knobs, read on held-out cases.

## 5. Phased roadmap

| Phase | Builds | Starts | Exit |
|---|---|---|---|
| P0 | The ADR-069 amendment and G37's widening; the use-case guide with the policy, packet and edit recipes; the platform prereg with its subjects; the census; the baseline; the concern-delta file and its replay | now; no store, hook or daemon-path change; the census's decision-port calls within the owner's decision-experiment allowance | PLT-L0 signed; PLT-M0 read; PLT-5 landed |
| P1 | The inventory receipt per subject with its basis, absence honesty, the sentinel over both exports, the EVX-16 statement, the operator recipe, the two skills, the lab card, the consumer-side asks | rides lf8 beside WAE wave 2 (WAE-4, WAE-5) and needs WAE-10; PLT-11 needs PLT-M0 KEEP | PLT-M2 and PLT-M3 read |
| P2 | The consumer and firing readings | PLT-M4 when an adopting platform team's consumer runs; PLT-M5 on the owner's cap | PLT-M4 and PLT-M5 read |
| P3 | Deferred rows, each on its trigger: a service catalog shape, the review-packet directory, deployed-commit answers, the decision site, golden-path conformance, encryption facts, scanner SARIF without a review kit, the question map, observability coverage, the Well-Architected Agent arm | per row | per row |

**Kill.** This is G37's, widened. On 2027-01-31, a WAE-M4 that is unread or CUT drops WAE waves 3 to 4. A PLT-M4
that is unread or CUT removes PLT-7's multi-kind mode and PLT-5's delta file.

**Capacity, estimated.**
- P0: about 2 agent-days and 3 owner hours (prereg sign-off, census audit).
- P1: about 6 worker-days.
- P2: owner hours for PLT-M3's residue labels and the PLT-M5 run.

Model spend arises only in PLT-M0 (decision-port calls on public framework items) and PLT-M5 (on an owner-named cap).

**Sequence.** P0's documents and readings are fillers that touch no daemon path. PLT-5 is the one P0 code row. It is
a CLI-only filler within the two-wave limit behind the current main line (vision 38 sections 9.6 to 9.12). P1 rides
lf8, as WAE wave 2 does. PLT-L0, PLT-M0's ledger row, PLT-16 and PLT-M5 name bench-prep and feature-labs rows, so
those two plans merge first.

## 6. Risks and honesty traps

| Risk | Mitigation |
|---|---|
| T1 Claiming compliance from code: code shows intent at a commit, not deployed state, staffing, drills or capacity | fathom answers no question; every receipt says "declared at commit C, not deployed state"; in the deferred question map, only the operator clears an attestation |
| T2 Picking choices or grading, which invites gaming the measure (DORA's warning) | no choice, risk level, score or pass anywhere (PLT-X1); the review kit and a human choose |
| T3 Building a catalog, a scorecard UI or a drift store | files only; history is the consumer's; drift is the delta file (PLT-X2) |
| T4 Rebuilding scanners or an IaC reviewer | their SARIF reaches the edit through a review kit and the findings loop; policy stays in the policy tool (PLT-X4) |
| T5 Reading absence as a pass: retries can live in a mesh, a library default or a withheld env value | `not-found` is a per-subject lower bound with its read set (PLT-4); the lab corpus is stratified for these traps; WAE-D1 owns code clients behind WAE-M1 |
| A false clean by rollup: one probed workload makes the repository read clean | classes are per subject and kind, with no rollup (PLT-4's two-workload fixture) |
| Mixed commits: mapping or facts from one commit, the index from another | every export carries a basis; skew reads `differs` naming both commits, and the consumers drop it (PLT-7's skew test) |
| A second gate through the ratchet | deltas are information, never losses and never counted by `--strict` (PLT-X12); the change-risk tierer's tier decides |
| Labs that favor fathom | the census counts fully code-answerable items only, with a kappa floor; prompts come from census ids, with near-miss negatives and a grep-only arm; keep is correctness, never firing; graders must fail both stubs first |
| Rebuilding the adopter's or fathom's own work | no question map (the review kit, the policy kit), no renderer (Conftest reads JSON), no ownership (IM-10, the catalog), no corpus manifest (bench prep), no lab ledger (feature labs) |
| The consumer side is unready (a review kit's reliability catalog may be nearly empty, and its facts lane and the delta reader may be unbuilt) | PLT-M4 is the bet with a date; the receipt and the delta file are useful to Conftest and to a reader meanwhile; G37's kill applies |
| A catalog changes its ingestion shape | no renderer here; SG-45 owns a catalog shape with golden tests when it lands (PLT-D2) |
| Secrets leak through an export | ADR-065 decision 2; WAE-13's sentinel over the receipt and the delta file (PLT-6) |
| Fleet-scale export loads the daemon | per-service and CLI-side under the tenancy budget; the delta file never runs in the daemon |
| Consumers rank teams or people with fathom facts | exports are keyed by subject and service; the guide says so; fathom ships no rule (PLT-X7, I58) |
| Org detail leaks into a public repository | the consumer's catalog, cadence and team names live in the operator's private pack (ADR-011); readings name public subjects only |
| Scope sprawl against language excellence | P0 is documents, readings and one CLI-only file; P1 rides lf8; every piece has a kill |

## Decision models

Two sites. One runs in a reading now, and one stays unbuilt.

**Site 1: the census classification (PLT-M0), in a lab and not in the product.**
- **Typed question.** Kind `choice` through the hexagonal port, one call per public framework item: is this item
  `code-answerable`, `partly` or `attestation-only` from code and configuration at a commit, and which fact kinds
  does it need? The item's public text goes to the provider. Only its id and URL are committed.
- **Provider.** OpenAI Decisions or Jev, named in the reading, within the owner's decision-experiment allowance.
- **Fallback with provider `none`.** No classification: the census reads only the owner-audited sample, and it
  states that it does.
- **Opt-out.** Run with `[decide] provider = none`. The user kill of ADR-073 beats every layer.
- **Measurement.** Cohen's kappa against the 20% owner audit, which serves as the labels. Under 0.6 the census is
  void. One row goes to the labs decision ledger through `DecisionLabReading` (feature-labs LAB-34), beside the
  provider `none` row.

**Site 2: does this evidence answer clause Q (PLT-D5), unbuilt.** It has a fathom caller only if the question map
(PLT-D9) revives. While a review kit answers, the judgment is the review kit's, through its own judgment backends.
- **Typed question.** Kind `choice`, with options `answers`, `partly`, `does-not-answer` and `needs-human`.
- **Band policy (P24).** The site serves a judgment only inside the band where its Wilson lower bound against
  adjudicated labels is at least 0.90. Outside the band it answers "not judged".
- **Never a fact.** The judgment never changes a class, an attestation, a receipt or a delta (I62).
- **Fallback.** Provider `none` is the default, and nothing degrades without it.
- **What blocks it.** Labels only.

None applies elsewhere. The receipt, its classes and basis, and the delta file are deterministic transforms. Runbook
staleness belongs to incident maps (IM-14), and ADR rule proposals to CX-23, each with its own record.

## Two horizons

**Build for now.**
- P0's documents, readings and the CLI-only delta file over facts that are stored today.
- Then the per-subject receipt for the adopter's consumers that are already designed to read it.

**Foreclose nothing documented.**
- Subject ids and fact kinds are opaque, so review kit requirement ids, lens ids and OSCAL control ids all bind to
  them from outside.
- The receipt's fact record is the predicate body EVX-16 wraps.
- Each receipt carries its commit, basis, fathom version and cache-key components, so an index snapshot can answer at
  a deployed commit (IXS, F74) with no format change.
- The delta file keeps both address keys open while the consumer chooses its address.
- The question map's corrected shape is written down (section 4.6).
- The decision site holds at provider `none`.
- No catalog id, framework id or threshold is written into code.

**Never pre-build.** No sync to a review tool, no catalog renderer, no policy renderer, no drift store and no fleet
daemon loop.

## Records

- GitHub issue #14.
- Plan `docs/plans/platform-engineering.md` (PLT-).
- It extends `docs/plans/well-architected-evidence.md` (WAE-) and vision 44.
- It joins `docs/plans/bench-prep.md` (BX-) and `docs/plans/feature-labs.md` (LAB-) for measurement.
- It cross-links to CX-, SG-, IM-, EVX- and IXS-.
- The adopting platform team's own records are cited only in the operator's private pack (ADR-011).
- The ADR-069 amendment is PLT-0, and G37's widening is PLT-G.
- This document records the 2026-10-25 research: the landscape in section 3.2 with its sources, and the corrections
  in section 3.1.

Owner questions:
- **Q1.** Is a per-subject fact receipt with coverage and no verdict outside ADR-069 decision 5's "readiness verb"?
- **Q2.** Does any team you serve run a service catalog (Backstage, Cortex, OpsLevel, Port) that fathom should target?
  Record the answer in the operator's private pack, never here. It is PLT-D2's trigger.
- **Q3.** G37 is widened rather than a new goal opened (PLT-G). Confirm?
- **Q4.** WAE-M0's labeller (U3) still blocks WAE wave 2 and PLT-M3's residue labels. Does PLT-L0's prereg unblock
  it?
- **Q5.** Which provider runs PLT-M0's census, and what is PLT-M5's spend cap?
- **Q6.** PLT-M4 needs the consumer side to read a PLT output: the facts lane's multi-kind read or the change-risk
  tierer's delta adapter. Which comes first, and which address should join a delta to a change? Record the answer in
  the operator's private pack, never here.
