# helm — provenance and divergences

LB-Q-helm, design `docs/design/language-breadth.md` §2, §3, §4, §13. A `tags+doc`
dialect: `Chart.yaml` and `values.yaml` are read by the document engine (`doc.json`), every
other file the manifest owns — `*.tpl`, `templates/**/*.yaml|yml`, `templates/NOTES.txt` — by
the tags engine over the go-template grammar (`tags.scm`). `extractor.TagsDocExtractor`
routes by the doc spec's `detect.basenames`.

## Where each piece comes from

- **Grammar.** `ngalaiko/tree-sitter-go-template` at commit
  `aa71f63de226c5592dfbfc1f29949522d7c95fac` (MIT; the repository publishes no release tag, so
  the lock's `tag` carries the commit and `build.sh` fetches that archive), the `helm` dialect
  (`dialects/helm/src/parser.c`, entry point `tree_sitter_helm`). No external scanner; the
  blob is 189,463 bytes, 68,230 gzip (`gotemplate` in `grammars.lock.json`).
- **`tags.scm`.** The grammar's own `queries/highlights.scm` has no `@definition` or
  `@reference` captures to borrow, and neither aider nor nvim-treesitter carry a Go-template
  tags query; it is written from scratch against `indexbench lang-probe` output over `testdata/`.
- **`doc.json`.** Written from Helm's chart layout; the incumbents (`helmextract.go`) fix the
  kinds and the dotted names.
- **Fixtures.** Written for this dialect in the `helm create` house style; no upstream file
  is copied.

## Port of `helmextract.go` and `helmtemplateyaml.go` — and why dispatch stays

The incumbents' `SourceKind`s are kept verbatim: `HelmChart`, `HelmDependency`, `HelmValue`
(dotted names to depth 8, keys written with no value included), `HelmTemplate`,
`HelmValueRef`. `HelmTemplateRef` is new. `TestHelmDialectMatchesIncumbents` runs the
incumbents' own fixtures through the dialect and requires every (kind, name) the incumbent
emits.

`lang-parity` against fusion's dispatch reads MERGE (every gated L1 metric is at least the
incumbent's, the target is met), with two kinds lost: `K8sWorkload` and `K8sContainer`, which
`helmtemplateyaml.go` recovers from templated manifests by substituting a placeholder for each
`{{ }}` span and decoding what is left. The go-template grammar leaves a manifest's YAML as
opaque `text`, so its `kind:`, `metadata.name` and `image:` are not reachable by a query.
Switching dispatch therefore needs a decision — a hybrid that keeps `helmtemplateyaml.go` for
those two kinds beside the dialect, or accepting the loss — recorded as `LB-Q-helm-rest`.
Until then `fusion.New` registers nothing for this dialect (it registers only `tags`
manifests), `TestHelmDialectIngestsEndToEnd` proves the registration works when made, and the
only production effect is classification: `*.tpl` and `templates/NOTES.txt` are now
dialect-owned, so the generated/minified gate applies to them.

Differences from the incumbents, each deliberate:

- **Dependency signature** is the version; the incumbent appended the repository (`sig`
  has no string operations, §4).
- **A template's line** is its real line (the incumbent recorded 1), its signature is its
  header (`{{- define "chart.name" -}}`, wrapped headers whole), and its range is the whole
  `define … end`.
- **A `.tpl` Go's parser rejects** keeps its verified tier and every definition tree-sitter
  can recover; the incumbent fell back to a regex at `TierPreScan`. An unterminated `define`
  is an `ERROR` node opening with the `define` keyword and is matched as one
  (`(ERROR "define" . (interpreted_string_literal))`), the regex fallback's case.
- **`.Values` accessors** are structural, not a regex: `.Values.a.b` and `$.Values.a.b`
  outside a comment or a string, one `HelmValueRef` per occurrence (the incumbent: one per
  unique path per file), in every file the dialect reads (the incumbent: `templates/*.yaml`
  only). A `$root.Values.x` through a variable, and `index .Values "x"`, are not read.
- **A dependency's edge** is a cross-file reference by name (`module-source`, the
  package.json convention): a chart depends on the one chart of that name in another
  `Chart.yaml`. Two charts of a name leave it unresolved.

## What the reading does and does not say

- **Tier C throughout** (`docs/bench/readings/lang/helm.json`, `calibration/oracles/helm`):
  the oracle reads templates with Go's own `text/template/parse` — independent of the grammar —
  but Chart.yaml and values.yaml with `gopkg.in/yaml.v3`, the doc engine's own library, and the
  L3 join re-implements the dialect's rule. `helm show chart` (v3.19.0) loaded every chart and
  agreed on name and dependencies. `helm template` (tier A) needs each chart's dependencies
  fetched from a registry and was not run.
- **L3 is thin evidence.** 180 of the 181 edges are `oci://` registry addresses and one is a
  `file://` path; nothing but the shared name says the in-repo chart is the one the registry
  serves (it is, for bitnami/charts and prometheus-community/helm-charts, which publish from
  those directories).
- **L2 covers the minority of include calls.** Of 48,363 `include`/`template` calls with a
  literal name in the two subjects, 92.7% sit in manifest bodies outside any `define`, where
  there is no enclosing symbol to attach a call edge to (`attachTSCalls` needs a callable
  containing the site). They are `HelmTemplateRef` symbols, found by name, and scored at L1.
  The 3,511 calls inside a `define` are the call edges `ref_recall`/`ref_precision` score.
- **One line of the subjects is lost to the grammar**: `("https" "http" .Values.x)` — a
  command that starts with a string, which Go's parser accepts — breaks the tree-sitter
  parse of the rest of its action.
- **`SymbolKey` carries no kind.** A `HelmTemplateRef` and the `HelmTemplate` it names
  share a key when they are in one file (30% of bitnami/charts' defined templates); both
  observations are stored and `find_symbol` returns both, `find_callers` still answers with
  the calling template (`TestHelmDialectIngestsEndToEnd`).
- **Not modelled.** `Chart.lock`, `requirements.yaml`, `values.schema.json`,
  `values-*.yaml`, `crds/`, `.helmignore`; `tpl` strings and `lookup`; a `.Values` chain
  read through a variable; YAML merge keys in `values.yaml` (the doc decoder emits the `<<`
  key and follows the alias, where Helm merges — none occur in the subjects).
