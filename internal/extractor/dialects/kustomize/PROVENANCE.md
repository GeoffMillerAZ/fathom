# kustomize — provenance and divergences

LB-Q-kustomize, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar) that ports `internal/extractor/kustomizeextract.go`: this
file records the port decisions, what the dialect deliberately does not do and why, and where
the calibration oracle comes from. The decode is `gopkg.in/yaml.v3` through the doc engine.
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
  at the file's first key, signed by its `kind` when it writes one (`Component`). Most
  kustomizations carry no `apiVersion` or `kind` at all (both argocd-example-apps subjects and
  most of the tool's examples), so the root is selected by shape, not by a key. It is the entity
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
  file the doc engine reads resolves to its first root entity (a Kubernetes manifest is not one
  today, below).

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
  `generators`, `validators`, `configurations`, `crds`, `helmCharts`, `openapi`** are not
  entities and not references: none is a file the dialect names an edge to (or, for a plugin
  file, none of the subjects writes one), and a name that selects a resource by kind and name
  in another file (`patches[].target`, `replicas[].name`) needs a join the engine has for
  Services only.
- **A directory resolves to the one doc file in it** (design §4), and this is the format's
  main edge (`resources: - ../base`). Today an ingest dispatches a Kubernetes manifest to the
  incumbent `K8sExtractor`, which is not a doc-engine extractor, so a base holds one doc file, its
  kustomization, and resolves (12 edges on `kubernetes-sigs/kustomize` examples and 1 on
  argocd-example-apps, the 13 the reading scores). **The day the kubernetes dialect is
  dispatched**, every base that holds a manifest holds two doc files and no longer resolves,
  unless `fusion.ResolveDocXrefs` learns to prefer the file the referring dialect owns.
  `TestKustomizeBaseDirectoryNeedsTheOneFileRule` pins today's rule and fails when it changes;
  that is `LB-Q-kustomize-rest`'s trigger, and `internal/fusion/docxref.go` was not this item's
  to edit.
- **A resource that names a manifest** (`resources: - deployment.yaml`) is a cross-file
  reference the dialect records, but the harness scores only edges between the files the
  dialect owns (`langbench` scope, §1.4): the oracle's 53 edges to manifests are counted as out
  of scope in the reading's notes, and an ingest resolves none of them yet because no manifest
  has a root entity in the store until the kubernetes dialect is dispatched.
- **YAML merge keys and multi-document files.** A kustomization is one document; a second
  document is not read (`multi_document` is off).

## The calibration oracle

Tier A, and the tool the queue entry names: Kustomize itself, through `kubectl kustomize`
(kubectl v1.36.1 embeds Kustomize v5.8.1, the version the subject's tag is; recorded as
`kustomize (kubectl-embedded) v5.8.1`). `calibration/oracles/kustomize/` builds every
`kustomization.yaml` and `kustomization.yml` of a scratch copy of the subject, each as its own
root, with a dead proxy so that no remote is fetched.

Kustomize lists nothing of what a file declares, and its `originAnnotations` name the file a
resource came from but not the kustomization that named it (an overlay of an overlay of a base
looks like the base's resources), so the oracle asks the tool one question of each entry: **does
the build change when this entry is removed?** Entries are proposed by an independent decode
(`yq`, which is its own YAML reader with line numbers; not the doc engine's walk) and disposed by
the tool: an entry is in the truth when the build fails without it or its output differs
(`buildMetadata: [originAnnotations]` is set on the copy). Every entry of both subjects was
evidenced (70 of 70), so no truth entry was withheld and the dialect was not helped by the
filter; an entry the tool does not evidence would be counted on stderr and would cost the
dialect precision. Definitions, references and cross-references all come from those entries.

A kustomization the tool cannot build contributes nothing and is left out of the measured
files (`-only`), counted, with its reason: `examples/loadHttp` (remote bases and a remote
patch), `examples/transformerconfigs/images` (`kustomizeconfig/mykind.yaml`, written by the
tool's documentation), `plugins/kustomized-helm` (`all.yaml`, written by helm at render time).
A Component is not a root the tool builds (none in the subjects); it would be wrapped. The
reading's `lang-parity` runs on copies of the subjects without those three files, because
`lang-parity` has no `-only`.

## Subjects

`kubernetes-sigs/kustomize` at `kustomize/v5.8.1`, `examples` only (Apache-2.0; 18
kustomization files, 16 built by the tool) and `argoproj/argocd-example-apps` at `master`
8088f4c (Apache-2.0; 4 files, 3 built). Between them 19 files, 70 entries and 13 base or
overlay edges: a small measure, and the reading says so.
