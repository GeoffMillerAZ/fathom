# verify spine — the gate's anchor and its blind language

> Governing sources: `.fathom/observations.md` O3 + O4 (root causes and
> fix shapes recorded there), `docs/vision/24-work-frontier.md` §7 (the
> verify gate is the proven spine — these two observations are its known
> erosion paths).
>
> **Covenant (ADR-0070 class):** failing test first, `task validate`
> green before commit, dated CHANGELOG entry, boxes flip only on
> `fathom verify --service fathom --plan docs/plans/verify-spine.md`.

## O4 — the annotation block cannot eat itself

- [x] one quote-aware tokenizer for the `{…}` annotation block: a `{` or `}` inside a double-quoted value no longer terminates the block, exactly as quoted commas already do not split values — one shared scanner, not two independent splitting rules; the O4 two-line repro (control row + braces-in-skip row) is a parser test case and both rows keep their author-owned ids {id: VS-1, symbol: ParseMarkdownPlan, package: internal/verify, reachable_from: TestQuotedBracesDoNotTerminateAnnotation}
- [x] a malformed annotation fails loudly: a checkbox line that contains a `{…}` block the parser cannot fully consume is a lint warning naming the row and the unparsed fragment — silently degrading to no-contract is the one outcome that hides the error, because an Unknown row is indistinguishable from an intentionally uncontracted one {id: VS-2, symbol: LintPlan, package: internal/verify, reachable_from: TestMalformedAnnotationLintsLoudly}

## O3 — CUE joins the call graph

- [x] the CUE extractor emits reference edges it currently discards: unification (`#Kit: #Block & {…}` is an edge #Kit → #Block), field type references (`profile: #Profile`), list element types (`uses: [...#Block]`), and cross-package imports — restoring `find_callers`/`references` for CUE definitions so "what composes this contract?" is answerable {id: VS-3, symbol: cueReferenceEdges, package: internal/extractor, reachable_from: TestCUEReferenceEdgesWiredDirect}
- [x] `testdata/*.cue` unification is the CUE analogue of test-reaches: a fixture that unifies a definition maps onto the existing test-reaches edge kind, so `reachable_from` becomes satisfiable for a CUE row with no new plan grammar — the twelve `skip_test: true` escapes in one platform-engineering plan stop being the only exit {id: VS-4, symbol: cueTestdataReaches, package: internal/extractor, reachable_from: TestCUETestdataReachesWiredDirect}

## The resolver — reachability that cannot flip

- [ ] a method call resolves by receiver type, or refuses loudly when same-named methods on different receivers are ambiguous — so adding a test to a package can never flip another row's reachability verdict (the fifth silent-void mode, 2026-08-23: CUE wire tests entering internal/extractor flipped the first-wins winner for `Extract` and voided cpp-toolchain CT-1/CT-2); the direct-wire-test workaround stays valid but stops being necessary {id: VS-5, symbol: Build, package: internal/analysis/callgraph, reachable_from: TestSameNamedMethodsDoNotFlipReachability}

## Deferred, named

- O3(c) — a CUE evidence tier (`vet_clean` / `export_golden`) so a row
  proves behaviour, not just presence: real design work on the tier
  vocabulary, not a patch; deferred until VS-3/VS-4 traffic shows the
  edge relation is right.
