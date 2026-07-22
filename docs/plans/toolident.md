# Tool identity — files understood as what they are (ADR-021 §2)

Owner order: docker, helm, cue, timoni, k8s, kustomize "and all these
tools that use yaml/cue/json should be understood as what they are."
Grounded by wf_d0588a59: dispatch is pure path/basename lookup with zero
content sniffing; Helm `templates/*.yaml` fails yaml.v3 on `{{ }}` and
silently emits nothing (a whole-chart blind spot — .tpl partials are the
minority); kustomization payloads are parsed-then-discarded (no
metadata.name); CUE modules/timoni/jsonnet/ipynb have no SourceKind at
all; non-k8s YAML funnels into the k8s extractor with silent zero output;
no generated-file detection; no size caps; symlinks silently dropped
(keep — document).

Design (normative):
- **Classifier in pass 3.** Between hash-gate and extractor dispatch:
  `classify(relPath, content, siblings) → ToolKind`. Extractors register
  by ToolKind; the ext/basename table becomes the candidate pre-filter,
  content decides. A per-ingest **sibling index** built once from the
  Discover list (option (b) from grounding: O(files), threaded into
  extract) answers "ancestor dir contains Chart.yaml / cue.mod /
  kustomization.yaml".
- **Signals, cheapest-first** (fast path = plain YAML stays fast):
  k8s manifest = `apiVersion:` + `kind:` top-level; CRD =
  apiVersion apiextensions.k8s.io; compose = top-level `services:` map
  (any filename, catching docker-compose.override.yml); helm template =
  yaml/tpl under templates/ with ancestor Chart.yaml; kustomization =
  basename kustomization.{yaml,yml}; GHA = path .github/workflows/
  (existing); OpenAPI = top-level `openapi:`/`swagger:`; timoni = CUE
  with timoni.cue / bundle markers; CUE module = ancestor cue.mod/;
  generated = header regex `Code generated .* DO NOT EDIT` (Go
  convention) + protoc/mockgen/stringer markers in first 3 lines.
- **Helm templates parse via placeholder substitution**: replace
  `{{ ... }}` spans with scalar placeholders pre-decode, then extract the
  k8s shape (kind/name may itself be templated → name falls back to the
  file stem, flagged templated:true). Values keys referenced
  (`.Values.x.y`) are captured as reference observations.
- **Symbols that mean something**: k8s → one symbol per document
  (Kind/name, namespace attr, image refs as references); compose →
  services (+image refs); helm → Chart (name/version from Chart.yaml),
  values keys (from values.yaml, nested dotted), per-template emitted
  kinds; kustomization → resources/patches/images entries as references;
  OpenAPI → operations (method+path) and schemas; CUE module/timoni →
  package-qualified defs (builds on lang-truth's LT-4).
- **Generated files**: extracted normally but observations carry a
  `generated` marker; excluded from cloc's code counts (own bucket);
  serving/obligations unaffected (rules can still scope them).
- **Unclassified visibility**: files whose candidate set matched but
  whose content matched no tool emit a per-ingest counter (surfaced in
  ingest output + server_stats) — silent zero-symbol extraction dies.
- **Cross-refs (cheap tier only)**: image ref ↔ Dockerfile (same-repo
  image name match), values key ↔ template usage. Selector↔Service and
  cross-repo image resolution are deferred.
- Size cap: files > 2MB skipped from extraction (counted, surfaced) —
  minified/huge-file class. ExtractorEpoch bumps (yaml-family semantics
  change).

## Tasks

- [ ] Pass-3 classifier + sibling index; extractors keyed by ToolKind; ext table demoted to candidate pre-filter {id: TI-1, symbol: Classify, package: internal/fusion}
- [ ] Helm chart understanding: template placeholder-parse, Chart/values symbols, values-key references; a real minimal chart golden fixture {id: TI-2, symbol: Classify, package: internal/fusion, skip_test: true, skip: "helm extraction fans out through the classifier dispatch; behavior pinned by the golden fixture tests in internal/extractor which call the extractors directly"}
- [ ] Compose/k8s/CRD/OpenAPI/kustomize content-signal routing + meaningful symbols per design; misroute regression fixtures (compose override file, non-k8s yaml, OpenAPI doc) {id: TI-3, symbol: NewYAMLExtractor, package: internal/extractor}
- [ ] CUE module + timoni awareness (ancestor cue.mod/, timoni markers) layered on LT-4 packages {id: TI-4, symbol: NewCUEExtractor, package: internal/extractor}
- [ ] Generated-file detection: marker on observations, cloc generated bucket, header-regex fixtures (protoc/mockgen/stringer) {id: TI-5, symbol: Classify, package: internal/fusion, skip_test: true, skip: "classifier-internal branch pinned by direct classifier tests; dispatch indirection is the documented residue"}
- [ ] Unclassified/oversize visibility: per-ingest counters in ingest output + server_stats; 2MB extraction cap {id: TI-6, symbol: Ingest, package: internal/fusion, skip_test: true, skip: "Ingest is the long-lived pipeline entry reached via the full-pipeline tests; counter behavior pinned by direct assertions there"}
- [ ] Cross-refs cheap tier: image↔Dockerfile, values↔usage as reference observations; queryable via references {id: TI-7, symbol: Classify, package: internal/fusion, skip_test: true, skip: "emitted as observations through the same dispatch; pinned by fixture-driven pipeline tests"}
- [ ] ExtractorEpoch bump + docs (mcp-tools/cloc semantics, use-case doc Maintained lines if verbs change — none expected) {id: TI-8, symbol: FingerprintHash, package: internal/fusion, skip_test: true, skip: "epoch constant + docs; pinned by the existing epoch invalidation test"}

## Deferred

- [-] Selector↔Service, cross-repo image resolution, helm render-time evaluation {id: TI-9, disposition: deferred, reason: "cheap-tier refs prove the loop; render-time helm is a template engine dependency", trigger: "first consumer asking blast-radius questions across the k8s boundary"}
- [-] jsonnet, .ipynb extraction {id: TI-10, disposition: deferred, reason: "no fleet corpus contains them; classification will label them unclassified loudly", trigger: "first indexed repo where their unclassified count is material"}
