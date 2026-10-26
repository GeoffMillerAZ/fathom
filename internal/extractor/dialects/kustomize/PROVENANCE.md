# kustomize — provenance and divergences

LB-Q-kustomize, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar) that ports `internal/extractor/kustomizeextract.go`: this
file records the port decisions, what the dialect deliberately does not do and why, and where
the calibration oracle comes from. The product uses the doc engine's YAML reader.
The manifest claims a file by basename, `kustomization.yaml` or `kustomization.yml`, at any
depth. A bare `Kustomization` file (kustomize reads it, and it has no extension) and every
other YAML file stay with the Kubernetes dialect's extension claim or unclassified, as before.

## Port of `internal/extractor/kustomizeextract.go`

The incumbent's three `SourceKind`s are kept verbatim (§2.2, §13.2 step 15): `KustomizeResource`
(an entry of `resources:` or the legacy `bases:`), `KustomizePatch` (`patchesStrategicMerge:`,
`patches:` and `patchesJson6902:`, named by the patch's path) and `KustomizeImage` (`images:`,
named by the original image). Names, lines, tier and count are the incumbent's on its own
fixture (`TestKustomizeDialectKeepsIncumbentContracts`), and on every dialect fixture the
incumbent also read (a parity test ran both extractors side by side before the retirement commit
and went with the incumbent); `lang-parity` against `kustomizeextract` on the calibration subjects was
MERGE: `sym_recall` .7416 to 1, `sym_precision` 1 to 1, kinds gained `Kustomization` and
`KustomizeConfigMap`, none lost. Malformed YAML yields no symbols, as before, and an empty file
yields none. `kustomizeextract.go` is retired: dispatch is the dialect's: `fusion.New` maps both basenames to it, the connector
classifies them language `kustomize` (they were `yaml`) and the ToolKind tag stays `kustomize`.

What the dialect adds to what the incumbent read:

- **The kustomization itself** (`Kustomization`, canonical `package`): one entity per file,
  named by the file (`$file`, so `kustomization.yaml` and `kustomization.yml` read as written),
  at the file's first key, signed by its `kind` when it writes one (`Component`). Some
  kustomizations in the historical subjects omit `apiVersion` or `kind`, so the
  root is selected by shape rather than requiring either key. It is the entity
  the entries hang from and the one a base or overlay reference resolves to: without a root
  entity `fusion.ResolveDocXrefs` has nothing to join a file-level reference to
  (`docker-compose` is deferred for that).
- **`components:`** entries, read as `KustomizeResource` (a component is a reference to another
  kustomization directory, as a base is), and **`configMapGenerator:` / `secretGenerator:`**
  entries (`KustomizeConfigMap`, `KustomizeSecret`, canonical `resource`), named by the
  generator's `name`.
- **References** (`Calls` on the kustomization): the name of every entry the file declares,
  the same string the entry has as an entity, so a kustomization refers to each resource, base,
  component, patch, image and generator it names (intra-file, L2).
- **Cross-file edges** (`fusion.ResolveDocXrefs`, file-tier, entity to entity): a `resources:`,
  `bases:`, `components:` or patch entry that names another kustomization or the directory that
  holds one resolves by `repo-path` to that file's kustomization entity; an entry that names a
  file the doc engine reads resolves to its first root entity (Kubernetes manifest destinations are outside this dialect reading's
  owned-file scope).

## Divergences from the incumbent, all forced by §4's selector subset

- **Signature.** The incumbent composed `Kind/name` for a patch target (`Deployment/my-app`)
  and `newName:newTag` for an image (`registry.example.com/myapp:1.2.3`); the doc engine has no
  concatenation and no literal, so the dialect signs a patch by its target's `name` (else its
  `kind`) and an image by its `newName`, else its `newTag`, else its `digest`. The incumbent's
  spelling always contains the dialect's, which the parity test asserts. Nothing else read a
  signature here.
- **An inline patch is not an entity.** `patches: - patch: |- ...` has no `path`, so it has no
  name; the incumbent skipped it too. An inline `patchesStrategicMerge:` string, which the
  incumbent also named by its whole text, is named the same way (a `where` on the text is
  outside the subset); it is rare and the name is what the incumbent produced.
- **A scalar where a list belongs** (`patches: patch.yaml`) reads as a one-entry list: the
  engine's `[*]` takes a scalar for one item (`odd.yaml`). Kustomize rejects the file, the
  incumbent read nothing from it, and the dialect reads an entry.
- **Entities of one name share a symbol key.** `core.SymbolKey` carries no kind, so a resource
  and a patch both named `deployment.yaml` in one file are one key (the incumbent had the same
  property).

## What is not modelled, and why

- **A remote reference** (`github.com/x/y//dir?ref=v1`, `https://...`, `git::...`) is an entity
  and a cross-file reference like any other, and resolves to nothing: it names no path of the
  repository, and crossing repositories is the `service_ref` layer's job (§1.1). The
  reference's `Target` is the URL taken as a relative path, which no repository file has.
- **`vars`, `replacements`, `replicas`, `commonLabels`, `namePrefix`, `transformers`,
  `generators`, `validators`, `configurations`, `crds`, `helmCharts`, `openapi`**
  are not mapped to entities or references by this dialect. The expanded native
  population includes one `configurations` entry; the assessment inventories
  the other populated families. File-valued families can express additional
  paths, and selectors such as `replicas[].name` need format-specific joins.
  This reading does not certify those paths or selector bindings.
- **Directory references prefer the referring dialect's file.** The Kubernetes
  dialect is dispatched, so a base can contain several document files.
  `fusion.ResolveDocXrefs`' `oneDocFile` first selects a unique file owned by the
  referring dialect, then uses the unique-document fallback. This preserves a
  Kustomize base's edge when that directory also contains Kubernetes manifests.
  `TestKustomizeBaseDirectoryNeedsTheOneFileRule` exercises this fired trigger.
  No resolver change is needed for this evidence lane.
- **References to Kubernetes manifests** are emitted and can resolve at ingest.
  This dialect reading scores only destinations in the files it owns
  (`langbench` scope, §1.4). Native manifest-target edges outside that scope
  remain counted separately; this reading does not certify their resolution.
- **YAML merge keys and multi-document files.** A kustomization is one document; a second
  document is not read (`multi_document` is off).

## The calibration oracle

The current calibration uses the pinned official Kustomize v5.8.1 binary and its
matching API/kyaml v0.21.1, in `calibration/oracles/kustomize/`. Native typed
configuration loading enumerates candidates and fields; native kyaml supplies
source positions. Reconciliation errors stop the whole subject. Native-invalid
configurations and every other excluded candidate remain counted in the receipt.

The CLI grades an entry by removing it from a scratch copy and comparing the
native build. Entries that change the output or cause native rejection are
truth; no-ops stay outside truth and can cost the dialect precision. Every real
non-Component parent closure that reaches a Component supplies its context.
No synthetic wrapper enters canonical evidence. Build, mutation, restore,
timeout and population failures have explicit whole-unit refusal rules.

The environment is scrubbed, plugins and Helm are disabled, and reachable remote
or absolute path inputs are excluded before builds. Source copies, native-call
fan-out, output and whole-run duration are bounded. Private receipts reconcile
all candidates, typed fields, contexts, exclusions and selected files. Public
records contain counts and hashes, not source or file lists.

## Subjects and measurement

The original `kubernetes-sigs/kustomize` examples and
`argoproj/argocd-example-apps` root scopes remain pinned. I-R4 adds the pinned
Flux Helm and multi-tenancy repositories and GCP microservices-demo, all in the
single `calibration/corpus.d/kustomize.yaml` shard. Membership was frozen before
canonical measurement; earlier exploratory selection is disclosed and is not a
blind holdout. Exact pins and criteria are in the oracle's `pins.json`.

The historical 13/13 cross-file result is thin evidence under D19. Each L3
metric requires at least 20 references in its own denominator. The dated
[assessment](../../../../docs/assessments/2026-10-26-kustomize-d19-evidence.md)
records the canonical result, per-subject populations, exclusions, controls and
remaining scope. The committed reading is the computed support result.
