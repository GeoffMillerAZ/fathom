# IaC facts: what a provider's contract says, joined to the repository at its locked version

Status: accepted 2026-11-02 (ADR-091, goal G49). Plan: `docs/plans/iac-facts.md` (IF-). Replaces the typed-layer half of
`docs/design/iac-provider-schemas.md`, which keeps its Terraform L3, Kustomize, Crossplane and triage content. Evidence:
`docs/assessments/2026-11-02-iac-facts-probe.md`, `docs/assessments/2026-10-26-iac-provider-schema-feasibility.md`.

Owner request (2026-11-02): "understanding Terraform and Pulumi is one thing, but understand the major providers, like the
AWS provider and the GCP provider and the Azure provider and the Kubernetes provider ... would add far better capability."

The word "provider" is taken twice in fathom: `fathom providers` lists decision-model providers, and a "provider pack" is
a kit's pack (F5, G35). This design says **IaC provider** where it must, names the feature **IaC facts**, its data file
a **catalog**, and its verb `fathom iac`.

## 1. Intent that applies, named before designing

- **Trigger fired:** IAC-7, IAC-8, IAC-10, IAC-12 and IAC-16 wait for G18 to accept a representation "then the owner
  explicitly reprioritizes the typed layer". The owner has. The G18 half cannot fire usefully, for the reason in section 2.
- **I56, I14:** the product fetches nothing and opens no new off-host path: an operator downloads a catalog with their own
  tools and hands fathom a local file. The product never runs `init`, a schema command, a plan, a provider executable, a
  registry client or a cloud call. I56's second clause is amended to name catalogs.
- **I48, ADR-050 decision 8:** one fact is a point read under the point ceiling. Binding and the cascade read other files, so
  they are budgeted tree-class work that reports a lower bound. ADR-091 amends decision 8 to say so. Nothing is materialized.
- **ADR-050 decision 4** reads dependency facts from what the toolchain left on disk. An IaC provider on disk is a compiled
  binary: its schema comes only from executing it, which I56 forbids. So the dependency plane has nothing to read for this
  ecosystem. ADR-091 takes IaC providers out of the `pkg:` kind and leaves G18 otherwise untouched.
- **ADR-049 decision 2, the table freeze (RT-M1 reads FAIL):** no table, column, cache, memo file or background job. This
  design needs none: catalogs are read-only inputs read at query time.
- **ADR-048:** tool formats grade L1 to L3. Terraform stays L3 on the board. IaC facts get their own readings, so the
  L4 policy amendment IAC-16 asked for is no longer needed.
- **Law 4 and the surface reckoning:** no new MCP tool. Facts arrive on `symbol_context`, `review_diff` and `blast_radius`.
- **I1, P2, I47 to I53, P23:** deterministic output, bounded work, whole-input refusal at limits.
- **I3:** repository facts keep repository anchors. A catalog fact cites the catalog, never a repository line.
- **I62, ADR-073:** no model resolves a field or upgrades a tier. **ADR-011:** vendor names in data are not org names.
- **F25** (infrastructure migration conservation) stays deferred.

## 2. Why the earlier design stalled

`docs/design/iac-provider-schemas.md` is careful and correct about identity, and most of its binding rules carry over
(section 5). Its delivery could not ship, for three reasons:

1. **Friction.** It requires an operator to run the export and author a JSON manifest with hashes per root. No adopter
   will, so the feature would never fire.
2. **Storage.** It persists a declaration closure in a dependency plane that does not exist (SG-36, SG-37) behind a table
   freeze that holds.
3. **Target.** It aims at an L4 grade for attribute resolution, which `tofu validate` already provides. The most valuable
   fact is not in the export at all (section 3).

## 3. What an agent gets

| # | Fact | Question it answers | Source | Native oracle |
|---|---|---|---|---|
| 1 | Attribute exists, with its type | Is `ssl_policy` an argument of `aws_lb_listener` at my version? | export | `tofu validate -json` |
| 2 | Required arguments | What must this block set? | export | `tofu validate -json` |
| 3 | Sensitive | Does this output expose a secret? | export | `tofu validate -json` |
| 4 | Deprecated | Am I writing something on its way out? | export | `tofu validate -json` warnings |
| 5 | **Forces replacement** | Does changing this destroy the resource? | provider source (SDKv2 `ForceNew`, plugin framework `RequiresReplace`), never the export | `tofu show -json` `replace_paths` |
| 6 | **Replacement cascade** | What else is destroyed with it? | fact 5 joined to the repository's reference edges | `resource_changes` in a plan |
| 7 | **Upgrade impact** | What in this repository breaks when `aws` moves from 5.x to 6.x? | two catalogs' difference joined to usage | `tofu validate` before and after; mined bump commits |

Facts 1 to 4 keep an agent from writing something invalid without an `init`. Facts 5 to 7 are the join only fathom can do:
**a static answer, with no credentials and no state, to what a change does**, in the agent's loop, at the repository's locked
version.

**What this is not** (the re-inventing check):

- Not a docs lookup. The registries' own MCP servers serve provider docs. A catalog holds no prose.
- Not a replacement for `plan` or `validate`. An answer never says a change is safe. It says what the contract says, and
  an agent with credentials should still plan. Fact 5 is a static floor: a provider's `CustomizeDiff` can force a
  replacement a static read cannot see, and each answer discloses when the resource has one.
- Not a linter's ruleset. `tflint` rulesets check values against cloud enums. Row IF-P5 reads where they overlap.
- Not cost, policy or runtime values.

## 4. The catalog

A catalog is a deterministic zip (stdlib `archive/zip`, sorted entries, zero timestamps) holding:

- `manifest.json`: provider source, versions, tool and schema format, build inputs' hashes, licence, catalog format version.
- `versions/<version>.json`: resource and data-source type to blob hash.
- `blobs/<sha256>`: one pruned resource schema. Descriptions are dropped. Only names, types, flags and replacement facts
  stay, with the provenance of each replacement fact and whether the resource has a `CustomizeDiff`.

Blobs are content-addressed so that one catalog can carry many versions: most resources do not change between releases.
One catalog holds every stable release of one provider major line (IF-M0: a recent-N catalog binds 3% to 60% of public lockfile
selections, IF-P6). Rows IF-P1 and IF-P8 measure the size. Reading one fact opens the zip, reads one version index and one blob, and closes: no
residency, no memo.

**Where they come from, and I56.** The product has one directory (`$FATHOM_HOME/iac/`) and fills it only from a local file on
an operator's command. It has no download path at all:

- `fathom iac add <path>`, after the operator fetches a release asset with their own tools (`gh release download`, `curl`).
  The digest is printed, and `gh attestation verify` is the documented check. `fathom init` may print those commands. It
  never runs them.
- `fathom iac import <export.json>` turns the operator's own `tofu providers schema -json` for one provider into a catalog,
  for private providers. It carries facts 1 to 4 and 7, and no replacement facts. A multi-provider export is refused, so
  the help says to make one in a scratch directory that requires that provider, and that `--version` is the operator's
  assertion, not checked. A catalog of the same line that is installed is merged: every release stays, the result says
  what was added, kept, unchanged and replaced, and a release with different content is refused unless `--replace`.
- `fathom iac list`, `fathom iac remove <name>`.

**Who builds them.** `cmd/iaccatalog`, a maintainer tool outside the product binary, reads a pin list
(`calibration/iac/pins.yaml`), runs the export under OpenTofu, scans the provider's tagged Go source for replacement facts,
and writes a catalog with a receipt. A CI workflow publishes catalogs as release assets with attestations. The Go source
scan uses `go/ast`, with no type-check of the provider.

**Licence.** Catalogs carry field names, types and flags, without description prose. The manifest records each provider's
licence at its pin; row IF-11 checks it before the first publish.

## 5. Binding: which contract applies to this block

Carried over from `docs/design/iac-provider-schemas.md`, without the authored manifest:

- The nearest `.terraform.lock.hcl` of each root gives source and exact version: status `locked`.
- No lockfile (a reusable module, the common case): the highest catalog version satisfying `required_providers`, labelled
  `constraint-matched`, never presented as locked.
- A local name resolves through `required_providers`, an explicit `provider =` selection and module `providers` mappings,
  falling back to Terraform's implied provider only where Terraform itself does.
- A shared child module reached by several roots with different versions reads `ambiguous` and is counted. No first-root
  choice. The reusable module plus `examples/*` layout is a required fixture.
- No catalog for the exact version: `unbound`, no facts, one disclosure naming the major line to add. Never a nearest version, labelled or not (IF-M0).
- OpenTofu and HashiCorp registry addresses stay distinct in identity. Row IF-P2 measures whether builds at one version
  are equal, and a catalog built from one address serves the other only with the difference disclosed.
- Addresses compare lower case. A source with no host matches a lock key on either default registry (OpenTofu writes
  `registry.opentofu.org` for the shorthand) and the block then resolves to that key's host, and an explicit host matches
  exactly. A catalog serves a block only on the
  same host or between those two default registries. Any other host reads `unbound` unless a catalog for exactly that host
  is installed.
- Provider constraints follow OpenTofu's Ruby-style parser: `~> 5` and `~> 5.0` are `>= 5.0, < 6`. An unreadable
  constraint reads `unbound` with the reason. Binding reads `.tofu` files, override files and the JSON syntax with the reader's shadowing, follows a symbolic link to a
  file, and skips dot files and editor backups, as OpenTofu does.
- A bound that hides a root (more than 16 lockfiles, a stopped lockfile search, a module walk that stopped before it saw
  every module) refuses that answer with a reason. It never answers from the roots it did read. So does an input it did
  not read: a file over 1 MiB or past the 256th file of a directory, a file past the nesting bound, a lockfile that does
  not parse, and a module call whose source is not a literal string, which OpenTofu resolves and the walk cannot.

## 6. Surfaces

| Surface | Addition |
|---|---|
| `symbol_context` on a Terraform resource or data source | an `iac` block: binding status, the version, required arguments unset, written attributes with their flags |
| `review_diff` | findings for an unknown attribute, a changed attribute that forces replacement, a deprecated attribute, a sensitive value in an unmarked output; when `.terraform.lock.hcl` changed, an upgrade impact section |
| `blast_radius` on an IaC change | the replacement cascade, tagged as prediction, under the existing depth and symbol caps |
| `fathom iac` CLI | catalog management only |

An edit-time hook line is deliberately out until a labs card reads it (`docs/design/feature-labs.md`): injected text costs
every session, and "forces replacement" has to earn its place. Every fact carries `catalog`, `version`, `digest` and the
JSON path it came from, so a reader can check it. With no catalog, every surface answers as it does today.

The cascade is computed at query time from stored file edges, re-parsing the bounded set of referring files for attribute
paths, as budgeted tree-class work that reports a lower bound (I48). A reference to `aws_vpc.main.id` from an attribute that itself forces replacement propagates. Anything capped is
disclosed. The answer is a prediction from contract facts, with its limits stated: it knows nothing of state.

## 7. Kubernetes

Two different things share the name, and both are in scope:

1. **The `hashicorp/kubernetes` and `hashicorp/helm` Terraform providers** are ordinary catalogs. `kubernetes_manifest` is
   opaque by nature and says so.
2. **The Kubernetes API itself, for manifests:** `apps/v1 Deployment` field validity and API lifecycle per minor version,
   from upstream `api/openapi-spec`, keyed by group, version and kind. CRDs arrive through `fathom iac import` of the
   operator's own CRD files. The version comes from `kubeVersion` in `Chart.yaml` or a new `[iac] kubernetes` config key.
   Unbound, only lifecycle facts are served ("removed in 1.22"). Oracle: `kubectl apply --dry-run=server --validate=strict`
   against `kind` at that minor (tier A), with kubeconform as a cross-check. This can also lift the Kubernetes dialect above
   its tier-C cap, which is the same reference oracle #55's lane lacks.

## 8. Pulumi

Pulumi programs get the same facts through `pulumi package get-schema`, keyed by token (`aws:ec2/instance:Instance`). The
probe showed one bridged schema (`random`) carries no replacement fact. If IF-P4 finds the same for AWS and Google, their
tokens map to the Terraform catalog (IF-P4 also measures the mapping's source). If they carry it, Pulumi's own schema serves.
Azure Native reads its own schema either way. Order: Pulumi YAML, then TypeScript
and Python constructor arguments read syntactically from an import of the SDK. Go last. The SDK's own compiler is the oracle.
No program runs, no preview. Pulumi's constructor calls resolve in ordinary language support with the SDK installed, per
the 2026-10-26 triage, so this needs no SDK store.

## 9. Bounds

Catalog file 256 MiB, 500,000 zip entries, manifest 1 MiB (amended from 4 MiB: a lookup decodes it whole, and a real one is tens of KB), version index 4 MiB, blob 4 MiB (IF-M0: the largest AWS type is
2.43 MiB), JSON depth 64. A lookup decodes only the requested path with a streaming decoder, and one fact stays under I48's
8 MiB point ceiling, or that one answer is refused with a reason. Per answer: binding reads at most 16 lockfiles, `review_diff` reads at most 256 blobs, and the
cascade re-parses at most 64 referring files to depth 8, each disclosed when hit. Cap + 1 refuses the whole catalog at `add`
and the single answer at query time, with a reason. Lockfiles are read at query time: ingest does not change. IF-M0 confirmed them
except the blob bound. No read happens on a hook path until IF-M1 reads its latency.

## 10. Measurement and kill

Graded against native tools on public subjects chosen before reading. Readings live in `docs/bench/readings/iac-facts/`.

| Row | Reads | Bar | If it misses |
|---|---|---|---|
| IF-P1 | catalog bytes and lookup cost | AWS single version at most 16 MiB; 12 consecutive versions under 2x that; lookup p95 at most 20 ms and at most 8 MiB allocated | Redesign the format before building on it |
| IF-P6 | what share of public lockfile selections a catalog of N versions would bind | a catalog within IF-P1's size budget binds at least 80% of selections for the first five providers | Exact-version binding would rarely fire: widen coverage or rethink before building |
| IF-P8 | every stable AWS 5.x and 6.x release, deduplicated, with older lines projected | each major line's catalog at most 32 MiB and 500,000 entries | No catalog is published for that provider, the IF-P6 bar is reported unmet, and operators use `fathom iac import` |
| IF-M1 | facts 1 to 4 against `tofu validate` with seeded and real errors | precision at least 0.99, recall at least 0.95; lookup p95 as IF-P1 | Withhold from `review_diff`; a false alarm costs more than a miss here |
| IF-M2 | facts 5 and 6 against `replace_paths`, at least 200 sampled pairs per provider, stratified by SDKv2 and framework | fact 5 precision at least 0.95, recall at least 0.90; cascade recall reported | Serve only for providers that pass; report the rest |
| IF-M3 | fact 7 on mined public provider-bump commits | precision at least 0.9 (G18's upgrade-impact bar), recall reported | Withhold the section |
| IF-M4 | Kubernetes against server-side dry run | precision at least 0.99, recall at least 0.95 | Lifecycle facts only |
| IF-M5 | Pulumi arguments against the SDK compiler | precision at least 0.99 | YAML only |
| IF-M6 | **agents**: IaC edit tasks with hidden `validate` and plan graders, arms A1 and A1 plus IaC facts | preregistered; fewer failed validates and fewer unintended replacements | A powered null cuts the agent surfaces and keeps the CLI and `review_diff` |

Prediction, recorded before any reading: facts 1 to 4 pass easily and matter little to an agent that can already run
`validate`. Fact 5 passes for SDKv2 resources and reads weaker for framework resources with custom plan modifiers. Facts 5
to 7 are where any lift comes from. IF-M6 runs through the bench harness once its dry runs pass, with an approval line.

## 11. Phases

| Phase | Content | Spend | Exit |
|---|---|---|---|
| P0 | size, registry equality, replacement sources and offline oracle, Pulumi schemas, baselines, lockfile coverage | $0 | IF-M0 go or no-go |
| P1 | catalog, reader, CLI, builder and pins, binding, facts 1 to 4 on two surfaces | $0 | IF-M1 |
| P2 | replacement facts, cascade, upgrade impact | $0 | IF-M2, IF-M3 |
| P3 | Kubernetes API and CRDs | $0 | IF-M4 |
| P4 | Pulumi | $0 | IF-M5 |
| P5 | the agent experiment | approval line | IF-M6 |

First providers, chosen for reach: `hashicorp/aws`, `hashicorp/azurerm`, `hashicorp/google`, `hashicorp/kubernetes`,
`hashicorp/helm`. A later provider earns its place by public usage counts, one pin-list line each.

## 12. Two horizons

Built now: one format, one reader, Terraform first. Kept open for free: the catalog is keyed by a provider source string and
a type name, so Kubernetes and Pulumi reuse it with no format change, and a later cross-tool resource identity (F97) can add
an alias map without a migration. Not built: anything for CloudFormation, Crossplane, CDK or Bicep, IAM action catalogs, or
GitHub Action inputs. F97 holds them. G18 stays as it is for package ecosystems whose sources are readable.

## 13. CloudFormation (ADR-091 amendment, 2026-11-04)

The owner moved CloudFormation forward, ahead of F97's trigger. Registry schemas state replacement in the contract: `createOnlyProperties` in 1,643 of 1,876 types (`docs/assessments/2026-11-04-iac-equivalence.md`). A catalog is keyed by `aws/cloudformation`, region and a digest over entry names and bodies, because the zip embeds its build time. AWS's schema files carry no licence allowing redistribution (IF-P7), so an operator builds it: `fathom iac import` reads a schema zip they fetched, and no CloudFormation catalog is published or committed unless the owner decides otherwise. Regions differ by 2% to 50% of shared types, so an answer names its region. A template names no version, so binding reads `unversioned` with the catalog's date. Transform types such as `AWS::Serverless::*` read `unbound`. cfn-lint grades validity. No independent offline oracle exists for CloudFormation replacement: the resource specification's `UpdateType` agrees 99.93% across 1,872 types because the two share a source, so it only checks the reader (IF-CM2), and a change set needs credentials. An answer says the contract states it, which is true by reading, and IF-CM3's comparison with Terraform is the only independent evidence. A consumer of `pkg/lang/iac` has no `fathom` binary, so `iac.ImportCatalog` builds both kinds of catalog from local files, reached through `fathom-lang catalog import`. Rows: IF-P7, IF-C1 to IF-C8, IF-CM1 to IF-CM3.

Attribute reads for both tools come from `docs/design/lang-library.md` section 5, so the product's checks and the public door share one reader.

## Decision models

None applies. Whether an attribute exists, forces replacement or changed between versions is a fact read from a contract, or
a computation over it, so a model could only guess what the data records (I62). Provider `none` is the whole feature, and
opting out means adding no catalog. Natural-language explanation of a finding is an agent's job, not this feature's.
