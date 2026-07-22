# Language truth — one registry, no silent buckets, honest CUE (ADR-021 §1)

Grounded by wf_d0588a59, all three defects reproduced:
- `pkg/client.langFromPath` (client.go:36-75) is a hand-maintained
  10-language subset of the 24-extractor registry — cue, cs, c/cpp,
  templ, dockerfile, proto, graphql, toml, sql, json, scala, php,
  sh/bash, lua all serve as `Lang:""` → cloc "other".
- The owner-visible 423K-line "other" was the untracked 103MB `fathom`
  build binary at repo root: excluded by .gitignore but connectors read
  only .fathomignore; no binary detection exists; cloc line-counts the
  Mach-O blob.
- CUE extraction (cueextract.go) walks only top-level decls (no nested
  fields — #Layer's budgets/serving/corpus fields are invisible), never
  captures imports, and stamps Package from the FILENAME
  (pkgFromPath) instead of the file's `package` clause — config_schema.cue
  and schema/config/schema.cue (same `package config`) land as two
  different wrong packages.

Design constraints (normative):
- **One registry.** The fusion extractor registry (or a table it and the
  connector both consume) becomes the single ext→language mapping;
  `langFromPath` is deleted and `Client.Sources()` consumes the shared
  mapping. Every registered extractor's files carry its language; cloc
  gains all 24. Files with NO registered extractor keep an explicit
  label ("unclassified"), and cloc reports them under it with a count —
  never silently "other".
- **Binary detection.** A cheap sniff (NUL byte in the first 8000 bytes —
  the classic text-tool heuristic, precedent: constitution-lint's sweep)
  runs at fetch/classification; binaries are excluded from extraction AND
  line stats, counted in a `binary` bucket by file count + bytes (never
  lines). Also: the repo's own .fathomignore gains `/fathom` and the
  scratch-binary class is documented in docs (build artifacts belong in
  bin/ or workspace/).
- **CUE depth.** cueextract recurses struct literals to emit nested
  fields (dotted names: `#Layer.budgets.max_usd_per_run`), captures
  imports as reference observations, and reads the actual `package`
  clause for Package (filename fallback only when absent). Golden tests
  against schema/semantic/schema.cue asserting the #Layer field tree.
  Depth cap (e.g. 6) documented and tested.
- Determinism: sorted outputs; golden fixtures per repo convention.
- Serving effect is immediate (cloc/lang stats are live-computed), no
  recompile, no epoch bump needed UNLESS extractor OUTPUT for already-
  indexed files changes — the CUE depth change DOES change extraction
  semantics → bump fusion.ExtractorEpoch (the O9 mechanism exists for
  exactly this) so the next ingest re-extracts CUE files.

## Tasks

- [ ] Shared ext→language mapping derived from the extractor registry; langFromPath deleted; Sources() consumes it; cloc shows all 24 languages + explicit unclassified bucket {id: LT-1, symbol: Sources, package: pkg/client}
- [ ] Binary sniff at classification: excluded from extraction and line counts, reported as file-count/bytes bucket {id: LT-2, symbol: IsBinary, package: internal/connector}
- [ ] Repo hygiene: /fathom in .fathomignore; regression test that a NUL-bearing root file contributes zero code lines {id: LT-3, symbol: IsBinary, package: internal/connector, skip_test: true, skip: "ignore-file line + the LT-2 tests are the coverage; no new symbol"}
- [ ] CUE extractor v2: nested-field recursion with dotted names, package-clause attribution with filename fallback, import capture; golden test on the real #Layer manifest {id: LT-4, symbol: NewCUEExtractor, package: internal/extractor}
- [ ] ExtractorEpoch bump (CUE semantics changed) with the epoch test extended {id: LT-5, symbol: FingerprintHash, package: internal/fusion}

## Deferred

- [-] Per-symbol language on SymbolNode (today only SourceFile carries Lang) {id: LT-6, disposition: deferred, reason: "no consumer queries symbols by language; SourceFile-level truth fixes every reported symptom", trigger: "first query surface needing per-symbol language filtering"}
