# 10 — The rearchitecture program (2026-07-22)

Owner mandate: uplift fathom for its use cases even at the cost of major
rearchitecture; make config-ecosystem tools (docker, helm, cue, timoni,
k8s, kustomize, "everything that uses yaml/cue/json") understood as what
they are; anticipate the code cases we are not prepared for. Full
delegation ("you are the owner... implement without conferring further").

Doctrine unchanged: every change here REMOVES operational machinery or
restores truth. No microservices, no hosted control plane, no plugin
runtime, no event bus, no embeddings pivot. The never-until list stands.

## The four ceilings (grounded, wf_d0588a59)

1. **Store**: DuckDB's cross-process single-writer made the daemon a
   single point of failure for reads (two wedges in one week took every
   session down), forced daemon-stop rituals for claim/report/compile
   verbs, and its CGO static libraries (538MB vendored weight; 5 platform
   binding modules) make cross-platform release builds hard. Audit: 26
   Store methods, 9 tables, zero non-portable SQL (the five ON CONFLICT
   upserts port verbatim; only DDL's `ADD COLUMN IF NOT EXISTS` differs),
   26MB fleet store, JSON-as-TEXT throughout. → ADR-020: pure-Go SQLite
   WAL backend.

2. **Language truth**: `pkg/client.langFromPath` is a stale 10-language
   subset of the extractor registry (25 language tags across 45 registered keys) — cue, cs, c/cpp, templ, docker,
   proto, graphql, toml, sql, json, scala, php, sh, lua are all invisible
   ("other") to cloc and every lang-labeled surface. The reported 423K
   "other" lines were a stray 103MB build binary at the repo root:
   `.gitignore`d but not `.fathomignore`d, no binary detection anywhere,
   line-counted byte blob. CUE extraction is real but shallow: top-level
   decls only, no nested fields, no imports, and package attribution uses
   a FILENAME heuristic instead of the `package` clause. → lang-truth
   spec (immediate), then ADR-021.

3. **Call edges**: emission is syntax-only per file; resolution refuses
   any method name defined by >1 receiver ("ambiguity resolves to
   nothing" — correct honesty, wrong ceiling). Every O9-residue cited
   skip this month is this limit. → typed enrichment pass
   (go/packages + go/types), edge kinds direct / iface-dispatch(set) /
   unresolved, additive beside the existing Calls contract; verify learns
   to count dispatch-sets honestly.

4. **Tool identity**: extractor dispatch is pure path/basename lookup —
   no content sniffing exists at all. Helm `templates/*.yaml` (the
   majority of every chart) fails YAML parse on `{{ }}` and silently
   yields zero symbols; kustomization payloads are discarded; CUE
   modules/timoni have no SourceKind; arbitrary YAML funnels into the k8s
   extractor with silent zero output. No generated-file detection, no
   size caps, `.gitignore` never honored. → classification layer in
   pass 3 (content + path + sibling index), extractors keyed by ToolKind,
   meaningful symbols per tool, cross-refs (image→Dockerfile,
   values→usage), unprepared-cases hardening.

## Sequencing and gates

Wave 1 (parallel): `store-sqlite` + `lang-truth` (small, restores truth
immediately). Wave 2 (after lang-truth merges): `toolident` +
`callgraph-types` (worktrees; fusion/pipeline.go is the known collision).
Wave 3: `extraction-treesitter` (wazero/WASM — never CGO; language by
language against the June calibration corpus). Every slice: gated plan,
TDD, adversarial critic, store-verified boxes, push through the gate.

Kill criteria: store swap reverts if the contract suite or a fleet
re-ingest shows semantic divergence DuckDB didn't have; typed callgraph
ships only if full-repo typed load stays under ~2× current ingest time
(cached thereafter); tree-sitter languages ship only on calibration-gate
pass (same §4.6 discipline as layers).
