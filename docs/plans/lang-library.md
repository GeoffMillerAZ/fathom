# The language library: `pkg/lang`, resource reads and packs

Grammar v2 (`docs/plans/README.md`). Prefix: LL- (LL-n work, LL-Mn readings). Design: `docs/design/lang-library.md` (section numbers are its). Decision:
ADR-092. Goal: G50. Evidence: `docs/assessments/2026-11-04-standalone-capabilities.md`.

Rows flip only through `fathom verify --service fathom --plan docs/plans/lang-library.md`. Readings live under `docs/bench/readings/lang-library/`, one file
per row, each opening with a verdict line. A row names the function a product entry point runs. `cmd/fathom-lang` and `cmd/fathom` are the entry points.

A commit that moves code behind a claimed row re-anchors that row in the same commit (design section 4). A `skip_test` waiver must land in an earlier push than its claim (I45, O1516), which this plan's first landing does. Rows that only pass together, a door and the verb that reaches it, flip in one commit.

## Phase 1: the cuts

- [x] The plan-row extractor leaves `internal/extractor`, so the package no longer imports `internal/verify`; the product registers it {id: LL-1, symbol: NewPlanRowsExtractor, package: internal/planrowsdialect}
- [x] The symbol snapshot and graph store leave `internal/memo` for `internal/graphmemo`, so `memo` imports only `tenancy`, `tsengine` reaches no call graph, and I51's shed still evicts grammars with no install step {id: LL-2, symbol: GraphFor, package: internal/graphmemo}
- [x] `tsengine` names no fathom path: the compilation cache takes a directory the caller computes {id: LL-3, symbol: EnableCompilationCacheDir, package: internal/extractor/tsengine}
- [ ] Routing leaves the Pipeline: `Router.Resolve` owns dispatch and content gates, returns construction errors, and `fusion.Pipeline` and `ReingestFile` delegate {id: LL-4, symbol: Router.Resolve, package: internal/langroute}
- [x] File classifiers move to the leaf `internal/langclass`, importing only `core` and the standard library, with forwarding functions left in `internal/connector` {id: LL-5, symbol: LanguageForPath, package: internal/langclass}
- [ ] `readout` builds a router, not a Pipeline, and no longer imports `internal/fusion` {id: LL-6, symbol: newOutlineRouter, package: internal/readout, needs: LL-4}
- [x] CUE extraction moves behind a registration, so `internal/extractor` compiles without `cuelang.org/go`, and the product registers it {id: LL-7, symbol: RegisterCUE, package: internal/extractor/cuelang, needs: LL-4}
- [ ] (op) ctl: routing parity: nodes for every dialect fixture and the fauxtown corpus at the base commit match byte for byte through `ExtractForBench` and `ReingestFile`, and `DispatchTable` is unchanged {id: LL-M2, oracle: lexical, path: docs/bench/readings/lang-library/m2-parity.md, pattern: "^LL-M2 verdict:", needs: "LL-4,LL-5,LL-6,LL-7"}

## Phase 2: the door

- [ ] `lang.New` builds a `Reader` from options, returning errors and never panicking {id: LL-10, symbol: New, package: pkg/lang, needs: LL-M2}
- [ ] `Reader.Extract` returns facts for one file through public types, with a stated skip reason {id: LL-11, symbol: Reader.Extract, package: pkg/lang, needs: LL-10}
- [ ] `Reader.Dialects` reports every dialect with its measured level, from a generated table a test holds to the readings {id: LL-12, symbol: Reader.Dialects, package: pkg/lang, needs: LL-10}
- [ ] `lang.WithDialects` adds a per-Reader pack layer, merged with the built-ins before validation, its queries compiled at `New`, refused whole with a reason, injections refused, reached through `fathom-lang --pack DIR` {id: LL-13, symbol: WithDialects, package: pkg/lang, needs: LL-10}
- [ ] `lang.WithExtractor` registers a Go extractor under the same routing rules {id: LL-14, symbol: WithExtractor, package: pkg/lang, library: true, reason: "the deliverable is the extension API Go consumers import, and a CLI cannot load Go code", needs: LL-10}
- [ ] `langtest.CheckPack` runs a pack's validation, required kinds and goldens {id: LL-15, symbol: CheckPack, package: pkg/lang/langtest, library: true, reason: "the deliverable is the conformance API pack authors import", needs: LL-13}
- [ ] `pkg/lang/cue` adds CUE and Timoni extraction by import {id: LL-16, symbol: Option, package: pkg/lang/cue, library: true, reason: "the deliverable is the import that opts in to CUE", needs: "LL-7,LL-10"}
- [ ] `cmd/fathom-lang extract|dialects` prints the door as JSON, with stdlib flags only {id: LL-17, symbol: cmdExtract, package: cmd/fathom-lang, needs: LL-10}
- [ ] Release builds publish `fathom-lang` beside `fathom` {id: LL-18, oracle: lexical, path: .goreleaser.yaml, pattern: "cmd/fathom-lang", needs: LL-17}
- [ ] `task deploy` installs `fathom-lang` beside `fathom` {id: LL-19, oracle: lexical, path: Taskfile.yml, pattern: "fathom-lang", needs: LL-17}

## Phase 3: standing alone

- [ ] A closure test fails if `pkg/lang` reaches store, fusion, connector, MCP or `net/http`, or passes its package budget {id: LL-20, symbol: TestLangClosure, package: pkg/lang, skip_test: true, skip: "the test is the anchor: it fails on any forbidden dependency", needs: LL-11}
- [ ] A test fails if the door opens a socket or writes a file without a cache directory {id: LL-21, symbol: TestLangNeverDialsOrWrites, package: pkg/lang, skip_test: true, skip: "the test is the anchor: it fails on any dial or write", needs: LL-11}
- [ ] The exported API is pinned by a `go doc -all` snapshot {id: LL-22, symbol: TestLangAPISnapshot, package: pkg/lang, skip_test: true, skip: "the test is the anchor: it fails on an unreviewed API change", needs: LL-11}
- [ ] An outside module builds against the door offline {id: LL-23, symbol: TestOutsideModuleBuilds, package: pkg/lang, skip_test: true, skip: "the test is the anchor: it builds the outside module", needs: LL-11}
- [ ] Fuzz seeds run in the normal test run: `Extract` never panics and holds its limits {id: LL-24, symbol: FuzzExtract, package: pkg/lang, skip_test: true, skip: "the fuzz target is the anchor", needs: LL-11}
- [ ] Equal inputs give byte-equal JSON under concurrent use and `-race` {id: LL-25, symbol: TestExtractConcurrentDeterministic, package: pkg/lang, skip_test: true, skip: "the test is the anchor: it fails on any difference", needs: LL-11}
- [ ] Docs: `docs/use-cases/lang-library.md` with `## Maintained`, godoc examples, `docs/usage.md`, the compatibility surface, a CLI review in `docs/app-kit.md` and a change note {id: LL-26, oracle: lexical, path: docs/use-cases/lang-library.md, pattern: "^## Maintained", needs: "LL-11,LL-17"}
- [ ] `fathom-lang`'s JSON is pinned by a golden test, as compatibility surface {id: LL-27, symbol: TestFathomLangJSONGolden, package: cmd/fathom-lang, skip_test: true, skip: "the test is the anchor: it fails on an unreviewed JSON change", needs: LL-17}
- [ ] (op) An outside consumer's closure and binary size read against the design's bar {id: LL-M1, oracle: lexical, path: docs/bench/readings/lang-library/m1-closure.md, pattern: "^LL-M1 verdict:", needs: "LL-20,LL-23"}
- [ ] (op) A module outside this repository builds and runs against a tagged release through the Go proxy {id: LL-M7, oracle: lexical, path: docs/bench/readings/lang-library/m7-outside-module.md, pattern: "^LL-M7 verdict:", needs: "LL-M1,LL-18"}

## Phase 4: resource reads

- [ ] `ReadTerraformFile` reads resources, data sources and module calls with attribute paths, classes and spans {id: LL-30, symbol: ReadTerraformFile, package: internal/iacfacts}
- [ ] `ReadTerraformModule` adds variable defaults and locals across one directory, as `defaulted`, without expanding modules {id: LL-31, symbol: ReadTerraformModule, package: internal/iacfacts, needs: LL-30}
- [ ] `ReadCloudFormationFile` reads `Properties`, intrinsic functions, `Condition` and parameter defaults, in YAML and JSON {id: LL-32, symbol: ReadCloudFormationFile, package: internal/iacfacts}
- [ ] `ReadPulumiYAMLFile` reads resources and properties {id: LL-33, symbol: ReadPulumiYAMLFile, package: internal/iacfacts}
- [ ] `iac.ReadFile` and `iac.ReadModule` are the public door, with values empty unless `WithValues` is passed {id: LL-34, symbol: ReadFile, package: pkg/lang/iac, needs: LL-10}
- [ ] `fathom-lang resources` prints reads as JSON {id: LL-35, symbol: cmdResources, package: cmd/fathom-lang, needs: LL-10}
- [ ] A sensitive or write-only literal stays withheld under `WithValues` when a contract is bound, and the product never passes `WithValues`, with a planted sentinel in no store row {id: LL-36, symbol: TestResourceValuesNeverReachTheStore, package: internal/iacfacts, skip_test: true, skip: "the test is the anchor: it fails on a call outside the door or a sentinel in a row", needs: LL-34}
- [ ] Fuzz seeds on the readers {id: LL-37, symbol: FuzzReadFile, package: pkg/lang/iac, skip_test: true, skip: "the fuzz target is the anchor", needs: LL-34}
- [ ] (op) Terraform reads against `tofu show -json -config` on public subjects chosen before reading, under the design's three oracle classes {id: LL-M3, oracle: lexical, path: docs/bench/readings/lang-library/m3-terraform-reads.md, pattern: "^LL-M3 verdict:", needs: LL-31}
- [ ] (op) CloudFormation reads against cfn-lint's decode on the dialect's subjects {id: LL-M4, oracle: lexical, path: docs/bench/readings/lang-library/m4-cloudformation-reads.md, pattern: "^LL-M4 verdict:", needs: LL-32}
- [ ] (op) Pulumi YAML reads against its own loader {id: LL-M5, oracle: lexical, path: docs/bench/readings/lang-library/m5-pulumi-yaml-reads.md, pattern: "^LL-M5 verdict:", needs: LL-33}
- [ ] (op) ctl: the unknown rate per design section 10, with the subject rule and the 20 rules committed before reading {id: LL-M6, oracle: lexical, path: docs/bench/readings/lang-library/m6-unknown-rate.md, pattern: "^LL-M6 verdict:", needs: "LL-M3,LL-M4,LL-M5"}

## Deferred

- [-] A pack may bring a new grammar as a wasm blob with an ABI check {id: LL-40, disposition: deferred, reason: "no pack needs one yet, and running a third party's parser is a separate trust decision", trigger: "a named pack needs a grammar fathom does not embed (F29)"}
- [-] The product loads repository-local dialect packs {id: LL-41, disposition: deferred, reason: "a repository-controlled query in a daemon is a separate trust decision", trigger: "a named adopter needs a dialect fathom should not ship, and queries are budgeted per tenant"}
- [-] A nested Go module for the library {id: LL-42, disposition: deferred, reason: "every language commit would become tag then bump, and the self-containment test refuses it", trigger: "an outside consumer reports a real version conflict"}
