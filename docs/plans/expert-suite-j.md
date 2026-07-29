# Expert Suite Phase J — module verbs + doctor

Design (normative — see `docs/vision/13-expert-suite.md` §4 ("The Builder
(studio): a guided path to a provable state") and §9's row **J** for the
owner-commissioned scope this plan implements; `docs/spec/expert-module-v0.md`
§3/§7 for the manifest contract and the honest loader gap this plan closes;
`docs/spec/store-format-v0.md` for the store this plan packages, never
forks):

- **Scope.** The `fathom module` verb family (`init`, `sources`, `map`,
  `compile`, `eval`, `doctor`, `pack`), the `#Module` manifest loader
  (`internal/modspec`), and the module doctor's readiness ladder. Every verb
  is a plain, scriptable CLI verb emitting `--json` for machine consumption
  (law 10, `docs/vision/13-expert-suite.md` §8) — no TUI, no web, no
  agentic advisor anywhere in this phase (law 11: `module map`'s gaps are
  100% deterministic; there is no gap ADVISOR to gate in Phase J at all,
  only the deterministic half of the studio's eventual Map panel).
- **Non-goals (deferred to K/L/M per `docs/vision/13-expert-suite.md` §9).**
  `fathom studio` (Bubble Tea TUI, phase K); the agentic gap advisor and
  flywheel intake (phase L); the web studio/chat (phase M); hosted/published
  serving hardening (phase N). None of this phase's code imports a Charm
  package, a web framework, or an LLM-as-advisor prompt — every LLM call
  this phase makes is the EXISTING obligations-style claim compiler
  (`internal/semantic.Compiler`/`RunEval`), never a "propose a gap" call.
- **A module's compile identity (the design decision this plan pins).**
  `schema/module/schema.cue`'s `#Module` carries a corpus, a watermark, an
  eval-evidence pointer, and provenance — but deliberately no compile
  prompt (`docs/spec/expert-module-v0.md` §1: "a module is fathom's
  ordinary store format, packaged with a manifest"). `module compile`/
  `module eval` therefore resolve a module's compile identity as **a
  project-local semantic-layer pack of the SAME NAME as the module**
  (`.fathom/layers/<module.name>/{layer.cue,prompt.md}`, LC-3's
  `semantic.LoadLayerIn`) — `module compile` is a thin wrapper over the
  EXISTING `compileViaStore`/`pkg/client.CompileLayerWithEscape` machinery
  (`cmd/fathom/compile.go`, unmodified) with `--layer` resolved from the
  manifest's own name instead of an ad-hoc flag; every A2-1/D3-1 guard,
  every budget check, every digest gate is reused verbatim. A mismatch
  between the layer pack's own `corpus.include` and `module.cue`'s is an
  advisory warning at compile time and a `module doctor` signal, never a
  hard refusal — an author may be mid-iteration on either file.
- **The mock/dev compile+eval lane (`--mock --mock-entries-file`).** Real
  module authoring spends real tokens through the real guarded path above.
  A second, clearly-labeled lane exists ONLY for scripted/CI use
  (`scripts/module-cold-run.sh`, and any test that must never touch a real
  key): `moduleCompileMock`/`module eval --mock` call the SAME production
  entry points (`pkg/client.CompileLayerWithEscape`,
  `internal/semantic.RunEval`) with `llmclient.NewMockClient` instead of a
  real provider — same engine, same store writes, same claim verification,
  only the transport differs. The mock compile lane deliberately does NOT
  run `compileViaStore`'s D3-1/A2-1 CLI guards (they reason about a real
  shipping decision that has no meaning for a throwaway scripted compile);
  both `--mock` flags carry an explicit doc comment saying so.
- **Deterministic gaps only (`module map`), the exact set this phase
  computes.** Zero-match corpus selectors; watermark drift vs. the current
  corpus; a compiled layer's zero claim-density per declared selector;
  broken anchors (legacy-empty anchor text, or a citation that no longer
  hash-matches the corpus). Every gap carries a stable machine `code` and a
  human `line`; a store-dependent gap that cannot be computed because no
  `--store` was given, or because the layer has no claims compiled yet, is
  reported as an explicit note (`store_not_provided` / `layer_not_compiled`)
  — never a fake zero. **Explicitly deferred** (the vision doc's own
  STUDIO-scope items, `docs/vision/13-expert-suite.md` §4 step 3's second,
  agentic-gated bullet list): "spec IDs cited in corpus but never defined"
  and a contradiction scan — both need either a spec-ID cross-reference
  index this repo does not yet expose to a module's arbitrary corpus, or
  the eval-gated agentic advisor phase L commissions. Recorded here as an
  honest scope boundary, not silently assumed away.
- **The corpus-scoped watermark (a correctness fix found by this plan's own
  cold run, not a pre-existing bug).** `moduleWatermark` (`cmd/fathom/
  module.go`) — never the raw `internal/connector.Watermark` — computes a
  non-git module's freshness token over exactly the manifest's SELECTED
  files (via `resolveSourceInventory`), not the whole directory. A module's
  own `module.cue`, eval fixtures, and pack output typically live inside
  the same root the corpus is discovered from; hashing the whole directory
  would make a module's watermark self-invalidate the instant `module
  init` writes `module.cue` next to the corpus it just measured —
  `scripts/module-cold-run.sh`'s first real end-to-end run caught exactly
  this (a real, non-git module directory drifted on every subsequent
  `module map`/`doctor` call). `module init` and `module map`'s
  watermark-drift gap both go through `moduleWatermark` now, consistently.
  Known limitation, inherited unchanged from `internal/connector`'s own
  documented gap: a corpus nested inside an ENCLOSING git repository
  (exactly `scripts/module-cold-run.sh`'s own scratch dir under
  `workspace/tmp/`, itself inside the fathom checkout) resolves to the
  enclosing repo's HEAD rather than a true content hash — the cold run's
  own watermark check therefore exercises the trivial git-HEAD-match path,
  not the content-hash path (that path IS exercised by
  `TestModuleInit_ScaffoldsLoadableManifest` and friends, which use
  `t.TempDir()`, always outside any git work tree).
- **The readiness ladder's criteria (designed precisely here, per the
  mission's own instruction).** `draft → eval-passed → servable →
  published`, each additive over the last; `module doctor` computes and
  reports EVERY criterion at every level regardless of an earlier failure
  (the honest-scorecard doctrine) and the verdict is the highest level all
  of whose own AND every lower level's criteria pass:
  - **draft**: manifest loads (`modspec.LoadManifest` succeeds) + at least
    one declared corpus selector resolves ≥1 file.
  - **eval-passed**: draft, plus `eval_evidence.evaled == true` AND the
    declared `eval_evidence.path` exists on disk (repo-relative to the
    manifest's own directory) — the D3 trust seam `docs/spec/expert-
    module-v0.md` §3.5 already mandates at the schema level, checked here
    at doctor time instead of only at authoring time.
  - **servable**: eval-passed, plus (a) the store conforms —
    `store.MountReadOnly(path, LevelL0)` succeeds (reusing the EXACT
    conformance check `docs/plans/gke-module-host.md`'s module host
    already built, per this phase's own instruction to reuse it); (b) zero
    broken anchors, per `module map`'s anchor-health gaps, AND the layer
    must actually have compiled claims (a `layer_not_compiled`/
    `store_not_provided` note from `module map` counts as NOT zero-broken,
    not as vacuously zero — an uncompiled module is not servable); (c) the
    manifest's watermark matches the corpus's current watermark
    (`moduleWatermark`, no drift).
  - **published**: servable, plus (a) `license` is a real, non-default
    value (schema's own honest default is `"unspecified"` — MUST-present
    is satisfied by the default alone, but "published" demands more than
    the honest-default floor); (b) `provenance.compiled_by`/`compiled_at`
    are both present (already schema-required, re-asserted here as a named
    criterion so the doctor output is self-contained); (c) a pack artifact
    exists at a caller-supplied `--pack-dir` (`module.sqlite` +
    `module.cue` both present) — omitted `--pack-dir` is an honest "no
    pack artifact given" failure, never assumed passing.
  - Doctor NEVER auto-promotes past a failing lower rung (`highestSupportedLevel`,
    `cmd/fathom/module_doctor.go`) — this is the advisor-free honest gate
    law 11 names; there is no agentic judgment anywhere in this
    computation.
- **`fathom module pack`'s safe-copy discipline.** `docs/spec/store-
  format-v0.md` §1's 2a WAL caveat, applied literally:
  `checkpointWAL` (`cmd/fathom/module_pack.go`) runs `PRAGMA
  wal_checkpoint(TRUNCATE)` against the SOURCE store file through a
  short-lived raw connection (never through `internal/store`'s
  write-schema path — `store.MountReadOnly`'s genuinely-read-only contract
  is untouched) before the file copy, then verifies the COPY's conformance
  markers post-copy via `store.MountReadOnly` — a pack whose copy fails its
  own conformance check is refused loudly, never shipped silently broken.
- **`internal/modspec`'s two validation strengths (the loader's own
  design).** `ValidateStrict` unifies a `module.cue` instance against the
  embedded, CLOSED `#Module` definition (mirrors `internal/semantic`'s
  `decodePack` for `#Layer`) — the AUTHORING gate `module init` runs
  against what it just wrote; an unrecognized field fails here,
  deliberately. `LoadManifest` is the READER: it decodes only the fields
  this package knows via presence-checked `cue.Value.LookupPath` calls
  (never a schema unification), so a manifest carrying a field from a
  future `#Module` revision still loads — the exact "a conforming manifest
  READER MUST ignore unknown fields" contract `docs/spec/expert-module-
  v0.md` §3 states. `TestLoadManifest_UnknownFieldIgnored` proves the two
  functions really do disagree on the identical input.

## Tasks

- [x] `fathom module init`: scaffold a `module.cue` in a target dir (name/description/scope-statement/corpus-glob flags), refuses to overwrite without `--force`, the emitted file passes both the strict authoring gate and the lenient reader {id: J-1, symbol: cmdModuleInit, package: cmd/fathom}
  - Watermark is computed BEFORE the file is written (`moduleWatermark` over the target corpus root), and the rendered CUE is validated in a temp file via `modspec.ValidateStrict` before ever touching the real target path — a scaffold that would fail its own authoring gate is never left on disk. Flags: `--dir`, `--root`, `--name`, `--description`, `--include`, `--exclude`, `--tier-profile`, `--license`, `--force`, `--json`.
  - Tests: `TestModuleInit_ScaffoldsLoadableManifest` (round-trips through both `ValidateStrict` and `LoadManifest`), `TestModuleInit_RefusesOverwriteWithoutForce` (both the refusal and the `--force` override, asserting the overwritten content). `internal/modspec`'s own `TestTemplate_RoundTripsThroughLoadAndValidateStrict`/`TestTemplate_WithExcludeAndOverrides` (`internal/modspec/manifest_test.go`) prove the underlying `modspec.Template` renderer directly.

- [x] The `#Module` manifest loader (`docs/spec/expert-module-v0.md` §7's promised consumer): `LoadManifest(path)` parses + validates a `module.cue` into a typed `Manifest`, with typed errors naming file+field; the reader ignores unknown fields per the spec's compat rule {id: J-2, symbol: LoadManifest, package: internal/modspec}
  - `FieldError{Path, Field, Err}` / `LoadError{Path, Fields}` name exactly which field failed and why — mirrors `internal/config`'s `LoadError` precedent (`internal/config/config.go`). `ValidateStrict(path)` is the separate authoring-gate function (unifies against the embedded closed `#Module`, `cue.Concrete(true)`) — used by `module init`'s self-check, never by the reader path.
  - `docs/spec/expert-module-v0.md` §7's UNENFORCED note updated to point at this loader (see the "Docs" row below).
  - Tests: `internal/modspec/manifest_test.go` — `TestManifestSchemaMatchesSource` (embed drift gate), `TestLoadManifest_Valid`, `TestLoadManifest_DefaultsAppliedWhenFieldAbsent` (tier_profile/license schema defaults applied via presence-checked lookups, not schema unification), `TestLoadManifest_UnknownFieldIgnored` (the exact reader-vs-authoring-gate distinction, proven on ONE input against BOTH functions), `TestLoadManifest_Negatives` (seven cases mirroring `schema/module/testdata/invalid.cue` plus a bad-name-shape case), `TestLoadManifest_MissingFile`, `TestValidateStrict_ValidPasses`, `TestValidateStrict_MissingWatermarkFails`.

- [x] `fathom module sources`: resolve the manifest's corpus selectors through the connector discovery path and print the inventory — files by class/language, total sizes, zero-match selectors flagged loudly {id: J-3, symbol: resolveSourceInventory, package: cmd/fathom}
  - Discovers via `connector.NewLocalConnector().Discover` (respects `.fathomignore`, the exact ingest-time discovery path — `docs/design`'s "no second engine" doctrine extended to a module's inventory) and buckets matched files by `core.SourceKind` ("class") and `connector.LanguageForPath` ("language"). Tool-kind bucketing (`fusion.Classify`, yaml/cue-family only) was evaluated and explicitly SCOPED OUT of this v0 pass: `fusion.Classify` accepts a nil `*siblingIndex` safely (documented nil-receiver contract), but wiring a real sibling index would require exporting `fusion.buildSiblingIndex` for a bucket that matters only for Helm/CUE-module corpora — deferred rather than half-built; `by_class`/`by_language` are the two groupings this phase actually ships. `sources`' exit code is 1 when any selector zero-matches (loud, scriptable failure) — `--json` still emits the full inventory either way.
  - Tests: `TestModuleSources_FlagsZeroMatchSelector` (asserts the exact zero-match selector list and a real selector's match count via `--json`), `TestModuleSources_NoZeroMatch`.

- [x] `fathom module map`: DETERMINISTIC gaps only — zero-match selectors, watermark drift, zero claim density per declared selector (store-dependent, "not compiled yet" when absent, never a fake zero), broken/legacy-empty anchors; every gap has a stable code + human line {id: J-4, symbol: computeMap, package: cmd/fathom}
  - Reuses `resolveSourceInventory` for selector matching (J-3) so `sources` and `map` can never disagree about what "matches"; reuses `moduleWatermark` (see design section) for drift; anchor-health re-fetches each cited document via `connector.NewLocalConnector()` with an explicit `Discover` call FIRST (a real bug this phase's own cold run caught: `LocalConnector.Fetch` requires `Discover` to have run on the SAME connector instance to resolve a repo-relative ref's root — a fresh `Discover`-less connector made every real citation look like `anchor_drifted` "no longer exists", a false positive fixed before this phase's evidence was recorded and covered by a regression test).
  - Gap codes: `zero_match_selector`, `watermark_drift`, `store_not_provided`, `layer_not_compiled`, `zero_claim_density`, `legacy_empty_anchor`, `anchor_drifted`.
  - Tests: `TestModuleMap_NoStoreGivenIsHonestNote`, `TestModuleMap_WatermarkDriftDetected`, and the regression `TestModuleCompile_MockLaneSpends`'s own tail assertion (a real compiled, unbroken anchor reports ZERO gaps — the exact case the Fetch-without-Discover bug broke).

- [x] `fathom module compile`: thin wrapper over the EXISTING `compileViaStore`/`pkg/client.CompileLayerWithEscape` machinery, reading service/layer identity from the manifest instead of ad-hoc flags; a `--mock`/`--mock-entries-file` dev lane calls the same production entry point with a scripted `MockClient` {id: J-5, symbol: cmdModuleCompile, package: cmd/fathom}
  - `--dry-run=true --json` (the default) is served WITHOUT ever touching `compile_runs` (a dry run is never persisted — `internal/semantic.Compile` returns before `RecordCompileRun`), via `moduleCompileDryRunJSON` calling `CompileLayerWithEscape` directly and printing the in-memory run; a real (non-dry-run) `--json` call routes through `compileViaStore` (preserving every A2-1/D3-1 guard unchanged) and re-reads the just-recorded `compile_runs` row. Does not fork `internal/semantic.Compiler` — zero new compile logic.
  - Tests: `TestModuleCompile_MockStoreDirect` (dry-run through the real guarded path), `TestModuleCompile_MockLaneSpends` (a real, non-dry-run mock compile through `pkg/client.CompileLayerWithEscape`, asserting `ClaimsEmitted == 1` AND the claim actually lands in the store).

- [x] `fathom module eval`: runs a module's eval fixtures dir (default: the sibling `eval/` dir next to `module.cue`) through the layer's compile identity, `--mock`/`--mock-entries-file` for a scripted, no-key lane {id: J-6, symbol: cmdModuleEval, package: cmd/fathom}
  - The precision/recall scoring machinery itself was EXTRACTED from `internal/semantic/eval_test.go`'s test-only `runLayerEval` into an exported, non-test function so a runtime CLI caller could reuse it without forking the scoring logic — `eval_test.go`'s `runLayerEval` is now a thin wrapper over the same `RunEval`, so the real-model gate (`FATHOM_EVAL_LLM=1`) and the CLI's mock lane are provably the SAME scorer. `PassesGate()` applies the `docs/design/semantic-layers.md` §4.6 bar (precision ≥0.90, recall ≥0.75) — `module eval` reports pass/fail, never fails the process on a failing gate (diagnostic, not enforcement; `module doctor`'s eval-passed criterion is what gates).
  - Tests (extraction, `internal/semantic/eval_mock_test.go`): `TestRunEval_MockLane_PassesGate`, `TestRunEval_MockLane_FalsePositiveFailsGate` (an unlabeled claim citing UNLABELED document text — not merely an unlabeled rule-name — actually drags precision below the gate), `TestRunEval_MissingLabelsJSON`. CLI tests: `TestModuleEval_MockLane` (asserts `passes_gate: true` via `--json`), `TestModuleEval_RequiresProfileOrMock` (refuses to run with neither, naming both ways out).

- [x] `fathom module doctor`: the readiness scorecard + ladder (draft → eval-passed → servable → published), every criterion machine-checked and named pass/fail with its evidence; never auto-promotes {id: J-7, symbol: runDoctor, package: cmd/fathom}
  - Criteria table implemented exactly as this plan's design section specifies. `highestSupportedLevel` walks the ladder from `published` down to `draft`, taking the highest level whose own AND every lower level's criteria all pass — a `published`-level criterion failing alone cannot mask a `servable`-level failure underneath it, and vice versa: every criterion at every level is always computed and reported.
  - Tests: `TestModuleDoctor_LadderProgression` (draft → eval-passed → verdict pinned at eval-passed for an empty, freshly-`store.Open`'d store — proving an UNCOMPILED store does NOT vacuously read as zero-broken-anchors/servable), `TestModuleDoctor_ManifestDoesNotLoad` (a syntactically broken manifest never reaches even draft). The end-to-end servable→published transitions are proven by `scripts/module-cold-run.sh` (J-9) rather than re-duplicated as a third Go test — the cold run IS that proof, real binary, real store, real pack artifact.

- [x] `fathom module pack`: the thin serializer — assemble a portable module dir {`module.sqlite` (safe-copy via WAL checkpoint), `module.cue`, eval evidence, `README.md` stub} and verify the copy's conformance markers post-copy; no network, no registry {id: J-8, symbol: cmdModulePack, package: cmd/fathom}
  - `checkpointWAL` + `store.MountReadOnly` post-copy verification per this plan's design section. Refuses a non-empty `--out` without `--force`. Copies `eval_evidence.path` into `out/eval/<basename>` when `evaled: true` and the file resolves on disk; the README stub names every manifest field plus the achieved store-conformance level (`L0` or `Writer`, whichever the copy actually satisfies).
  - Tests: `TestModulePack_RoundTrip` (asserts every artifact file exists AND the copy independently passes `store.MountReadOnly`), `TestModulePack_RefusesNonEmptyOutWithoutForce`.

- [x] `scripts/module-cold-run.sh`: headless end-to-end proof — scratch corpus → init → sources → ingest → compile (mock lane) → eval (mock lane) → doctor (`servable`) → pack → doctor (`published`); asserts each step's exit code plus the final verdict, zero real LLM calls {id: J-9, symbol: cmdModule, package: cmd/fathom, skip_test: true, skip: "the acceptance artifact for this row IS the script's own exit code, not a Test* function reaching cmdModule — every module verb it drives already has its own Go test (J-1..J-8) reaching cmdModule via the runModule test helper, which is what test_reaches actually finds; this row's own evidence is scripts/module-cold-run.sh's real, recorded run, not a duplicate unit test"}
  - `bash -n` clean (`task scripts-check` green); builds a FRESH `fathom` binary into scratch space (never `bin/fathom` — CLAUDE.md's binary-staleness discipline: that path is the daemon's installed copy); every LLM-shaped step uses `--mock --mock-entries-file`, never `ANTHROPIC_API_KEY` — runnable in CI with no key. All state lives under `workspace/tmp/module-cold-run-<pid>/` (the repo's `workspace/` convention), removed on success, left in place on failure for inspection.
  - **This IS the phase's terminal criterion** (`docs/vision/13-expert-suite.md` §9 row J: "a stranger-shaped test (scripted cold run) goes corpus → `servable` verdict in one sitting, every promotion criterion machine-checked"). Recorded run (2026-07-29, this plan's authoring session): every step exits 0; step 10's doctor verdict is `servable` (draft + eval-passed + all three servable criteria pass: store conforms, zero broken anchors, watermark fresh); step 12's post-pack doctor verdict is `published`. The corpus-scoped-watermark fix (design section) and the map anchor-health Fetch/Discover fix (J-4) were BOTH found by this script's real run, not by a unit test — the concrete payoff of building it as a real artifact instead of a paper checklist item.

- [x] Docs: `docs/spec/expert-module-v0.md` §7's "no manifest loader ships in this repo" UNENFORCED note updated to name `internal/modspec.LoadManifest`/`ValidateStrict` as the closing implementation {id: J-10, symbol: LoadManifest, package: internal/modspec, skip_test: true, skip: "documentation-only row; the loader's own test coverage is J-2's"}

## Deferred out of Phase J (named triggers, not silently dropped)

- **`module map`'s agentic half** (spec IDs cited-but-undefined, contradiction
  scan) — phase L's eval-gated advisor, per this plan's design section.
- **The coverage-map instrument's UI rendering, the readiness ladder's TUI
  panel, mouse/keyboard interaction** — phase K (`fathom-studio`, Bubble
  Tea), a separate repo per `docs/vision/13-expert-suite.md` §7's ownership
  table.
- **Tool-kind (yaml/cue-family) bucketing in `module sources`** — would
  require exporting `internal/fusion`'s unexported `siblingIndex`
  construction for a bucket that only matters to Helm/CUE-module corpora;
  scoped out of this pass, not forgotten (see J-3's evidence).
- **A `module` verb that patches `eval_evidence`/`recompile` fields back
  into `module.cue` after a passing eval** — no such verb exists; an author
  hand-edits `module.cue` after `module eval` passes (`scripts/module-
  cold-run.sh` step 9 does exactly this, in the open, as the honest
  current workflow). A future phase could add `module record-eval` if this
  proves to be a real friction point; not commissioned here.
