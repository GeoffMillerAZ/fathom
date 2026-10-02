# flux — provenance and divergences

LB-Q-flux, design `docs/design/language-breadth.md` §4, §13; the iac wave (EXCELLENCE T3). A doc-engine
dialect (no `tags.scm`, no upstream grammar, no incumbent: a Flux object was a Kubernetes object, read by
the Kubernetes reading as a name and nothing else). The decode is `gopkg.in/yaml.v3` through the doc
engine. Fixtures are written for this dialect (no upstream text is copied); the shapes follow the Flux
documentation's repository layouts (Apache-2.0, fluxcd/flux2-kustomize-helm-example, flux2-multi-tenancy).

## How a file is claimed: every document, never some

The manifest claims no name. `doc.json`'s `detect.content_sniff` is the dialect's one claim: an
`apiVersion` of `kustomize`, `helm`, `source`, `notification` or `image` `.toolkit.fluxcd.io/`.
`fusion.crDispatch` (LB-F149) claims a `.yaml` or `.yml` file only when EVERY non-empty document of it
passes that sniff and the classifier would otherwise call it a plain Kubernetes manifest (a Helm chart's
template, a CRD and a basename claim keep their verdicts), in ingest (`resolveExtractor`) and in
`ReingestFile`. A file with Flux documents beside others (the common `infrastructure/` file that opens
with a Namespace, and `flux-system` files with a Kustomization beside a Secret) is MIXED: it stays
Kubernetes, whole, and `fusion.CRMixed` counts it (the oracle's stderr and the reading's notes carry the
count). The dispatch table (`crDialects`) is generic: another wave's CR dialects append an id.

## What it models

- **The object** (`FluxResource`, canonical `resource`): one entity per document with a `kind` and a
  `metadata.name`, named `Kind/<namespace>/<name>` (`Kind/<name>` with no namespace), the Kubernetes
  dialect's naming, signed by its kind. The source kind is not the Kubernetes dialect's `K8sWorkload`:
  the registry allows one dialect per source kind.
- **References** (`Calls`, L2): the names an object writes: `spec.sourceRef.name`,
  `spec.chart.spec.sourceRef.name`, `spec.chartRef.name`, `spec.dependsOn[].name`,
  `spec.valuesFrom[].name`, as written (no declared-only filter: the target is usually another file).
- **Cross-file references** (L3), strategy `service-name`, the one entity of that name in another file:
  `sourceRef`, a chart's `sourceRef` and `chartRef` (`<kind>/<namespace>/<name>`), `dependsOn` of a
  Kustomization or HelmRelease (the referring kind), and a HelmRelease's `valuesFrom`
  (`<kind>/<own namespace>/<name>`, a ConfigMap or Secret, which resolves only when a dialect reads that
  file). The namespace is the reference's own, else the referring object's; an object that writes none
  (a cluster repository applied with a Kustomization's `targetNamespace`, which is most of them) is named
  `Kind/<name>`, so each entry has a second form without the namespace. `spec.path` of a Kustomization
  and a local chart's path (`chart.spec.chart` of a GitRepository chart) are `repo-root` references to a
  directory.

## Divergences, each disclosed (and where it is a ceiling, a -rest row)

- **Brief: `label-selector` for sourceRef.** The engine's label sets are maps of scalars; an object's
  `metadata` carries `labels` and `annotations` maps, so no `select` reaches a `{kind, name}` map to match.
  `service-name` over the `Kind/<namespace>/<name>` entity name draws the same edge. It excludes the
  referring file (an engine rule), so a source declared in the same file as its Kustomization
  (`gotk-sync.yaml`) is no L3 edge; the oracle scores cross-file edges only.
- **A directory is not a file.** `repo-root` of `./apps/staging` resolves to the one doc file directly in
  that directory, and only when the directory holds exactly one (or one the referring dialect owns). A
  Flux Kustomization's target is a directory of manifests built by Kustomize: the kustomize dialect's file,
  outside what this dialect reads, so lang-oracle drops those pairs as out of scope and the oracle counts
  them on stderr.
- **`dependsOn` and `valuesFrom` are read by index (0 to 5).** The engine's values are relative to the
  selected node and cannot reach the object's namespace from an item, so each index is its own entry.
- **Notification and image kinds are objects only.** An Alert's `eventSources` and `providerRef`, a
  Receiver's `resources`, an ImagePolicy's `imageRepositoryRef` and an ImageUpdateAutomation's `sourceRef`
  are not drawn (the generic `spec.sourceRef` entry does draw an ImageUpdateAutomation's and a HelmChart's).
- **Tier C.** The Flux CLI is not installed, so `flux build kustomization --dry-run` (tier A) did not run
  and no release binary was fetched: every metric caps at good (§1.3).

## Calibration

Subjects in `calibration/corpus.d/flux.yaml`; the oracle is `calibration/oracles/flux` (an independent
`gopkg.in/yaml.v3` walk). fluxcd/flux2's own command testdata is no subject: ingest skips `testdata`
directories, so the harness would read one file of it.
