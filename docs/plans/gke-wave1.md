# GKE Wave 1 — the store-format spec + research flywheel (retrospective)

**Retrospective plan.** Every `[x]` row below documents work already merged
to `main` before this plan was written (`docs/spec/`, `docs/design/
research-flywheel.md`, and their code) — see
`docs/vision/12-governed-knowledge-engine.md` §1/§4 Phase F for the wave
this plan proves. The one `[ ]` row is genuinely open: an operational event
(the live fleet store picking up the format markers) that has not yet been
directly confirmed against the fleet's own file.

Design (normative, restated from the merged docs — not new decisions):

- **Store Format v0** (`docs/spec/store-format-v0.md`) is normative on the
  on-disk SQLite format a fathom store MUST satisfy: single-file SQLite
  container, WAL-mode writer, the two file-identity pragmas
  (`application_id = 0x46415448`, `user_version = 1` for v0), required
  tables per conformance level (L0/L1/Writer/Server), repo-relative anchor
  rules with anchor-text healing, watermark formats, the closed
  five-value evidence-tier vocabulary, and append-only claim-lineage rules
  including refutation durability. Every MUST in the doc is either a named
  Go test or an explicit **UNENFORCED** pointer — this plan's rows anchor
  to those same named tests so the plan and the spec cannot silently drift
  apart from each other, the same discipline `TestStoreFormatV0_
  DocMatchesGoSource` already enforces between the spec doc and the Go DDL.
- **Expert Module v0** (`docs/spec/expert-module-v0.md`) defines a module as
  one conforming store plus one manifest — the manifest's field contract is
  `schema/module/schema.cue`'s `#Module`, CUE-shape-validated by
  `internal/modspec/schema_test.go`. The portability contract (compile on
  machine A, serve on machine B, zero recompile) and the non-git-corpus
  end-to-end demo are both proven directly against `pkg/client`'s public
  API, not asserted in prose.
- **Research flywheel** (`docs/design/research-flywheel.md`) is `fathom
  research report` (local-file-only aggregation of five pre-existing
  instrumentation streams into one honest, never-fabricates-a-zero
  snapshot) and `fathom research bundle` (the sanitized, opt-in, salted-hash
  twin, allowlist-enforced twice — Go-side typed fields with no free-text
  field, CUE-side closed `#Bundle` struct) — no new MCP tool, per doctrine 4.
  The critic pass (2026-07-28) closed six findings; the rows below anchor to
  the fixed code, not the pre-fix state.

## Tasks

- [x] Store Format v0: file container + the two format-identity pragmas (`application_id`/`user_version`) documented and enforced end-to-end, including the never-regresses guarantee for a future higher `user_version` {id: GKE-W1-1, symbol: TestStoreFormatV0_RequiredSchema, package: internal/store, skip_test: true, skip: "the row's own contract IS this named test (docs/spec/README.md's falsifiability principle) — it is the enforcement instrument, not a symbol requiring a separate Test to reach it"}
- [x] Format-version marker write policy proven: stamped on first open of an unversioned store, never regressed by an older/equal-versioned writer on a store already at or above its own version {id: GKE-W1-2, symbol: TestStoreFormatV0_FormatVersionNeverRegresses, package: internal/store, skip_test: true, skip: "enforcement-instrument row, same class as GKE-W1-1; companion TestStoreFormatV0_StampsUnversionedStore covers the first-stamp half"}
- [x] Spec doc and Go DDL kept in lockstep: the L0/Writer required-schema tables in store-format-v0.md are scraped out of the doc and diffed against the live `createSchema` DDL, so a drift on either side is a failing test, never a silently stale doc {id: GKE-W1-3, symbol: TestStoreFormatV0_DocMatchesGoSource, package: internal/store, skip_test: true, skip: "enforcement-instrument row, same class as GKE-W1-1"}
- [x] Store portability contract proven: repo-relative anchors, root remap via `fathom remap`, and watermark equality across a corpus move, with zero re-extraction — the module-level acceptance test store-format-v0.md §5 and expert-module-v0.md §5 both cite {id: GKE-W1-4, symbol: TestStorePortability_RoundTrip, package: pkg/client, skip_test: true, skip: "enforcement-instrument row (docs/spec/README.md falsifiability principle #1)"}
- [x] Expert Module v0 manifest shape (`#Module`: identity, corpus selectors, watermark-required freshness doctrine, tier profile, eval-evidence pointer, recompile rights, license/provenance) CUE-validated, positive and negative fixtures {id: GKE-W1-5, symbol: TestModuleSchema_ValidInstancesConcrete, package: internal/modspec, skip_test: true, skip: "enforcement-instrument row; TestModuleSchema_Negatives (same package) covers the invalid-manifest half (missing watermark, empty corpus.include, evaled:true with no eval_evidence.path)"}
- [x] A real non-git module compiled, served standalone, edited-and-restaled, then moved+remapped end-to-end — the demo that a module is configuration, not a new code path (expert-module-v0.md §7) {id: GKE-W1-6, symbol: TestExpertModule_NonGitCorpus_EndToEnd, package: pkg/client, skip_test: true, skip: "enforcement-instrument row"}
- [x] `fathom research report`: local-file-only aggregation (call log, hooks beat log, `.fathom/observations.md`, bench artifacts) into one honest snapshot; every metric that cannot be computed prints a named reason, never a fabricated zero {id: GKE-W1-7, symbol: cmdResearchReport, package: cmd/fathom}
- [x] `fathom research bundle`: sanitized, opt-in, review-first twin; `--dry-run` previews with zero writes and zero network I/O {id: GKE-W1-8, symbol: cmdResearchBundle, package: cmd/fathom}
- [x] Bundle allowlist enforced Go-side: `Bundle`/`Input` carry only a fixed, small set of typed fields with no free-text field a secret or hostile string could reach by construction {id: GKE-W1-9, symbol: Build, package: internal/research/bundle}
- [x] Critic Finding 1/6 closed: Tool/Surface/bench-Arm values sanitized to a fixed label shape before reaching `Input`, closing the client-controlled-string leak the critic proved end-to-end (a hostile path/newline string no longer survives into a written bundle or breaks a rendered Markdown table cell) {id: GKE-W1-10, symbol: sanitizeLabel, package: cmd/fathom}
- [x] Critic Finding 2 closed: the hash salt relocated to `workspace/state/research.salt`, outside the shareable `workspace/artifacts/research/` tree, with automatic migration of a pre-existing repo's legacy salt (value preserved, old file removed) {id: GKE-W1-11, symbol: LoadOrCreateSalt, package: internal/research/bundle}
- [x] Bundle CUE schema (`#Bundle`, closed struct, bundle schema version v2 for critic Finding 3's nil-vs-zero groups) ships with the binary and the repo, kept in lockstep with its embedded copy, and is enforced before any bundle write {id: GKE-W1-12, symbol: Validate, package: internal/research/bundle}
- [x] Bundle schema source-of-truth drift test: the authoritative `schema/research-bundle/schema.cue` and its embedded byte-identical copy under `internal/research/bundle/` cannot silently diverge {id: GKE-W1-13, symbol: TestResearchBundleSchemaMatchesSource, package: internal/research/bundle, skip_test: true, skip: "enforcement-instrument row"}
- [ ] (op) Live-store format-marker rollout: the fleet's `workspace/db/fleet.sqlite` gets `stampFormatMarkers` on its next writer open (the daemon's normal open path, not a special migration step) — confirm directly against the live file (`PRAGMA user_version` reads `1`, `PRAGMA application_id` reads `0x46415448`) rather than inferring it from the write-path code, which `TestStoreFormatV0_StampsUnversionedStore` already covers against a synthetic store {id: GKE-W1-14, symbol: stampFormatMarkers, package: internal/store, skip_test: true, skip: "operational row; evidence is a direct pragma read against the live fleet store, not a code-review claim — left open until that read is recorded here"}
