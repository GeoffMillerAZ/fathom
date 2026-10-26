# Platform pre-registration (PLT-L0): draft for the owner's sign-off

Status: **draft, unsigned.** This file fixes the subjects, strata, sampling rule, seed, stubs and bars for the
platform readings PLT-M0 to PLT-M5 (`docs/plans/platform-engineering.md`) before any run (K19). The owner signs it
off. Until then, no reading may cite it as fixed.

Bench prep's prereg template (BX-24) is unbuilt. This draft therefore follows the template's required contents as
`docs/design/bench-prep.md` section 10 lists them, and the platform design's subjects-and-graders section
(`docs/design/platform-engineering.md` section 4.7). Contents not applicable here (arms, Holm adjustment, turn cap)
are named as such, not dropped silently. When BX-24 lands, this draft is re-cast into the template with no change
to any bar.

PLT-L0 subjects (draft): 6 candidates, 0 manifests

Manifests are not written here. A `calibration/bench.d/<subject>.yaml` entry must satisfy `#Subject`
(`schema/bench/schema.cue`), whose `bench` block (toolchain, lockfile sha, test command, `flake_runs` of at least 3,
`l0_reading`) is mandatory, and `#Subject` has no `tags` field yet. Writing the manifests, the optional `tags`
field and that schema relaxation is a codex row (owner ruling, 2026-10-26).

## Hypotheses

One per reading. The kill each one carries is in the bars section.

- **H-M0 (census).** At least 10% of the three frameworks' items are fully `code-answerable` at a commit: every
  part is answerable by a fact kind fathom stores or a WAE- row plans.
- **H-M1 (baseline).** No direction is predicted. The reading measures what v0.20's verbs answer on each
  `code-answerable` item at the pins, beside grep. Later readings are compared with it.
- **H-M2 (delta replay).** A per-commit concern delta over Kubernetes facts matches the structural-YAML oracle's
  true delta with precision and recall each at least 0.90 (Wilson 95% lower bound). The deltas are small enough to
  read (median at most 5 per commit).
- **H-M3 (receipt classes).** The multi-kind receipt's per-subject class (`facts-present`, `not-found`) is precise
  (Wilson 95% lower bound at least 0.90) with zero false cleans, including on the T5 strata.
- **H-M4 (consumer).** An adopting platform team's consumer reads a PLT output on a real repository by 2027-01-31.
- **H-M5 (skills).** Each skill arm's answer correctness is non-inferior to both the no-skill arm and the grep-only
  arm, with zero false cleans.

## Subjects: the candidate table

The pins are origin `HEAD` on the retrieval date, read with `git ls-remote`. Each licence was read from `LICENSE` at
that sha with `git show <sha>:LICENSE`. Every one is the Apache License, Version 2.0, January 2004.

| # | Candidate | URL | Pinned sha | Licence (LICENSE at sha) | Retrieved | Role |
|---|---|---|---|---|---|---|
| 1 | opentelemetry-demo | https://github.com/open-telemetry/opentelemetry-demo | `5c8d404a85380226ff45a39dcb4159eda9552c56` | Apache-2.0 | 2026-10-26 | WAE-M0 four |
| 2 | microservices-demo | https://github.com/GoogleCloudPlatform/microservices-demo | `38e7348eb289eb5b87c0c6e8cb19ced0449dc389` | Apache-2.0 | 2026-10-26 | WAE-M0 four |
| 3 | kube-prometheus | https://github.com/prometheus-operator/kube-prometheus | `f71fb6c77d108ad2b4cbf4d9771b56cbe92611fb` | Apache-2.0 | 2026-10-26 | WAE-M0 four |
| 4 | istio samples (`samples/` of the istio repository) | https://github.com/istio/istio | `5a51fa09a7ea13dc4f5addfaf4b51ce2d067b169` | Apache-2.0 | 2026-10-26 | WAE-M0 four |
| 5 | terraform-aws-eks | https://github.com/terraform-aws-modules/terraform-aws-eks | `91b8a8d944e3cf169483108acd12d5b1e47856f7` | Apache-2.0 | 2026-10-26 | Terraform subject |
| 6 | flux2-kustomize-helm-example | https://github.com/fluxcd/flux2-kustomize-helm-example | `4f1a5b1051b469108d1bdf0ebec6c814427e3834` | Apache-2.0 | 2026-10-26 | GitOps subject |
| alt | opentelemetry-helm-charts (`charts/opentelemetry-demo/`) | https://github.com/open-telemetry/opentelemetry-helm-charts | `9c35a4dcae9badd3c9601c5530924dd1a4d986de` | Apache-2.0 | 2026-10-26 | alternate for #1 (see deviation 5) |

Rejected: argoproj/argocd-example-apps at `8088f4c0d970abb09e250248cc97e35623447cb5` has no `LICENSE` file at that
sha, so it fails C1.

**C1, licence.** All six candidates and the alternate pass, on the allowlist MIT, Apache-2.0, BSD or CC0. Bench
prep's loader re-reads each licence at the pin when the manifests exist.

**C7, independence.** Count per organization: open-telemetry 1 (2 with the alternate), GoogleCloudPlatform 1,
prometheus-operator 1, istio 1, terraform-aws-modules 1, fluxcd 1. None is fathom. Whether any is a repository that
fathom's own fleet indexes is not checkable from this repository alone. The owner confirms it at sign-off.

**C2 to C6, waived.** No agent task is derived from these subjects, so size bands, suite, index precision, fix-commit
yield and fan-in do not apply (`docs/design/platform-engineering.md` section 4.7).

## T5 strata each subject covers

The traps are: retries set only in a mesh, Helm-templated values, and Kustomize overlays. Coverage below was read
from each tree at its pin by path (`Chart.yaml`, `templates/`, `kustomization.yaml` under overlay or component
directories, mesh `VirtualService` resources).

| Subject | Mesh-only retries | Helm-templated values | Kustomize overlays | Notes |
|---|---|---|---|---|
| opentelemetry-demo | no | no | no | At the pin the repository carries Compose files and no Kubernetes manifests (deviation 5) |
| microservices-demo | partial: `kustomize/components/service-mesh-istio/` routes through a `VirtualService` with no explicit retry stanza | yes: `helm-chart/` | yes: `kustomize/components/` (18 component kustomizations) | Covers all three strata |
| kube-prometheus | no | no | no: one root `kustomization.yaml` over jsonnet-generated `manifests/` | Alert-rule subject for WAE-M0 |
| istio samples | partial: `VirtualService` routing, no explicit retry stanza found at the pin | yes: `samples/ambient-argo/tag-chart/` | no | Mesh stratum |
| terraform-aws-eks | no | no | no | IaC subject; Terraform facts only |
| flux2-kustomize-helm-example | no | yes: a `HelmRelease` (`apps/base/podinfo/release.yaml`) | yes: `apps/staging`, `apps/production`, `clusters/*` | GitOps subject |
| alt: opentelemetry-helm-charts | no | yes: `charts/opentelemetry-demo/` with values files | no | Restores #1's Kubernetes workloads |

Mesh-only retries are thin. "Partial" means mesh routing exists and any retry policy on it comes from the mesh's
default, not from a stanza in the tree. Whether a mesh default counts for this stratum is an owner question
(deviation 6).

## Sampling rule and seed

Seed: `20261026`

One ordering applies wherever a draw is made. Candidates are ranked by the lowercase hex SHA-256 of
`<seed>:<key>`, ascending, and taken from the top. The key is a commit sha, a subject-and-kind pair, or a census
`item_id`. Ties cannot occur, because keys are distinct.

- **PLT-M2 (delta replay).** The population is every first-parent commit reachable from each subject's pin that
  touches a manifest path: `*.yaml` or `*.yml` outside test and doc directories, Helm `templates/` and values files,
  `kustomization.yaml`, and `*.tf` for the Terraform subject. Each commit gets every stratum its diff touches:
  mesh routing, Helm-templated, Kustomize overlay, or plain. Draw in seed order, filling each of the three T5
  strata to an equal quota of 15 commits where its population allows, then fill from plain commits until the total
  is at least 60. A stratum with a population under its quota contributes all it has, and the shortfall is recorded.
  No commit is drawn twice.
- **PLT-M3 (receipt classes).** The 3 subjects are microservices-demo, flux2-kustomize-helm-example and the istio
  samples, which together cover all three strata. Every subject-and-kind pair of PLT-7 on those three is graded.
  Nothing is sampled.
- **PLT-M5 (skills lab).** Prompts are drawn in seed order over the census `item_id`s that PLT-M0 classed
  `code-answerable` or `partly`. Draws alternate train, test, train, test. The fresh test draw for the one allowed
  revision continues the same order past the last id used. Near-miss negatives are drawn the same way from
  `attestation-only` ids.
- **WAE-M0.** It samples its 40 alerts and 30 workloads from the same manifests at the same pins, by its own
  committed rule. That rule is the owner's (WAE-M0, U3) and is not restated here.

## Graders and the two stubs each must fail

Before a reading counts, its grader is run against both stubs and must score each below the bar. This is bench
prep's section 9 pattern.

| Reading | Grader | Stub 1 (always-facts-present or every-field) | Stub 2 (always-not-found or empty) |
|---|---|---|---|
| PLT-M0 | the 20% owner audit as labels; Cohen's kappa | every item `code-answerable` | every item `attestation-only` |
| PLT-M1 | WAE-M0 labels and `receipt verify` | every item answered with every candidate anchor | every item answered "not found" |
| PLT-M2 | the structural-YAML oracle's true delta per commit (WAE-M2) | every field of every touched manifest reported as a delta | an empty delta for every commit |
| PLT-M3 | the oracle for `facts-present` and `not-found`; owner labels for `facts-incomplete` | every subject-and-kind pair `facts-present` | every subject-and-kind pair `not-found` |
| PLT-M4 | the consumer's reading, cited by commit | not applicable: a single external event, not a scored set | not applicable |
| PLT-M5 | correctness against WAE-M0 labels and `receipt verify`; a lexical check of the attestation statement | an answer claiming the fact present for every prompt | an answer claiming absence for every prompt |

A false clean has a bar of zero in every reading. A false clean is a `facts-present` that the oracle says lacks the
fact, or a `not-found` stated as absence.

## Bars, copied from the plan rows

Each bar below is copied verbatim from `docs/plans/platform-engineering.md`.

- **PLT-M0:** "classified through the decision port (kind `choice`, provider named) with a 20% owner audit as labels
  and one row in the labs decision ledger beside the provider `none` row; void when Cohen's kappa on the audit is
  under 0.6. KEEP when at least 10% of the three frameworks' items read fully `code-answerable` (`partly` is
  reported, never counted); CUT removes PLT-11 under a `gone:` row and refuses PLT-D9; either way the item-to-kind
  table is handed to the review kit as data"
- **PLT-M1:** "for each `code-answerable` item of PLT-M0, whether v0.20's verbs (`symbol_context`, `search`,
  `obligations`, `review --since`) return an answer that agrees with WAE-M0's labels and whose anchors pass `receipt
  verify`, beside grep's; builds nothing; the reading later ones are compared with"
- **PLT-M2:** "Over at least 60 manifest-touching commits of PLT-L0's subjects, stratified for the T5 strata,
  WAE-M2's structural-YAML oracle computes the true delta of every commit, and precision and recall are scored over
  all deltas with Wilson 95% lower bounds, after an empty-delta stub and an every-field stub each fail the bar. KEEP
  when both lower bounds are at least 0.90 and the median commit carries at most 5 deltas; CUT removes PLT-5 under a
  `gone:` row"
- **PLT-M3:** "On 3 PLT-L0 subjects, stratified for the T5 strata, every subject-and-kind class of PLT-7 is graded by
  the structural-YAML oracle for `facts-present` and `not-found`, with owner labels only for `facts-incomplete`, after
  an always-`facts-present` stub and an always-`not-found` stub each fail the bar. KEEP when the Wilson 95% lower
  bound of precision is at least 0.90 and false cleans are zero; CUT removes PLT-7's multi-kind mode under a `gone:`
  row and leaves WAE-10's single-kind receipt"
- **PLT-M4:** "KEEP when one does by 2027-01-31; CUT, or unread on that date, removes PLT-7's multi-kind mode and
  PLT-5's delta file under `gone:` rows (G37's kill)"
- **PLT-M5:** "KEEP only when each skill arm's answer correctness is non-inferior to both the no-skill arm and the
  grep-only arm (the prereg's margin) with zero false cleans; firing is reported and never counts as helping (LAB K4);
  one revision on the train split, read on a fresh test draw; then CUT removes the skill"

The PLT-M5 bar defers to "the prereg's margin". Proposed for sign-off: a non-inferiority margin of 5 percentage
points of answer correctness, read on the one-sided 95% interval of each skill arm minus its comparator.

Not applicable to these readings, from bench prep section 10's list: arms beyond PLT-M5's four, Holm across
primaries (each PLT reading has its own single bar), the turn cap, and the power table. PLT-M5's cost cap is the
owner's named cap. Exclusions: a subject whose pin cannot be fetched is replaced by its alternate, or dropped, and
the drop is logged here. Nothing is excluded after a reading starts.

## Decision models

PLT-M0 is the one reading that asks a typed question through the hexagonal port: kind `choice`, over
`code-answerable`, `partly` and `attestation-only`, per census item, with a named provider. The deterministic
fallback is provider `none`, which answers `unclassified` for every item. Its ledger row sits beside the provider's
row. There is no opt-out beyond not running PLT-M0. The measurement is the 20% owner audit: the reading is void under
kappa 0.6. No other reading here asks the port.

## Deviation log

1. **The BX-24 template is unbuilt.** This draft follows bench prep section 10 and platform design section 4.7
   directly. It will be re-cast into the template when BX-24 lands, with no change to any bar.
2. **No manifests and no `tags` field.** The plan row's pattern expects `calibration/bench.d/` entries tagged
   `platform`. `#Subject`'s mandatory `bench` block (`schema/bench/schema.cue`) and the missing `tags` field make
   that a codex row (owner ruling). The subject line above therefore reads "6 candidates, 0 manifests" and does not
   match the plan pattern. PLT-L0 stays open.
3. **No `platform.json`.** `#Prereg` (`schema/bench/schema.cue`) models arm comparisons (arms, a baseline arm,
   strata, a per-hypothesis MEI and Holm's family at alpha 0.05). It cannot carry Wilson lower bounds, kappa or a zero false-clean bar, so this prereg is prose only.
4. **Census sources.** The framework hosts did not resolve from the session that drafted this file. PLT-M0's
   inventory (`docs/bench/readings/platform/census.md`) was read from pinned git mirrors and web-search listings,
   and its count discrepancies are recorded there.
5. **opentelemetry-demo has no Kubernetes manifests at its pin.** The tree carries Compose files only. The demo's
   Kubernetes workloads live in the opentelemetry-helm-charts chart (the alternate). The owner chooses at sign-off:
   keep #1 for Compose-only probes and add the alternate (2 subjects for open-telemetry, within C7), or replace #1
   with the alternate.
6. **Mesh-only retries are thin.** No explicit `retries` stanza was found in the istio samples or microservices-demo
   at their pins. Mesh routing exists, so any retry is the mesh default. The owner rules whether a mesh default
   counts for the stratum, or names a subject with an explicit mesh retry policy. Until then, PLT-M2's mesh quota may
   fall short and the shortfall is recorded.
7. **C7 fleet check unverified.** Whether fathom's own fleet indexes any candidate is confirmed by the owner, not
   from this repository.
