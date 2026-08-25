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

- [x] a method call resolves by receiver type, or refuses loudly when same-named methods on different receivers are ambiguous — so adding a test to a package can never flip another row's reachability verdict (the fifth silent-void mode, 2026-08-23: CUE wire tests entering internal/extractor flipped the first-wins winner for `Extract` and voided cpp-toolchain CT-1/CT-2); the direct-wire-test workaround stays valid but stops being necessary {id: VS-5, symbol: Build, package: internal/analysis/callgraph, reachable_from: TestSameNamedMethodsDoNotFlipReachability}

## Reachability truth — what a test can reach is what the code can reach (O18, O19; consumer-repo findings G14/G15)

- [x] a function referenced as a VALUE is an edge from the enclosing function, in both passes: `mux.HandleFunc(p, h)`, a route-table literal `{p, h}`, `http.HandlerFunc(h)`, `middleware(h)`, a method value `s.handleX` — typed pass resolves any `*types.Func` use outside call position (interface method values fan out like dispatch), syntax pass records package-local idents and `pkg.Func` selectors in value position; so an httptest that builds the server statically reaches every handler it registers and no webui row needs a direct-call test (O18 / G15) {id: VS-6, symbol: resolveFileEdges, package: internal/fusion, reachable_from: TestFuncValueReferencesAreEdges}
- [x] a call inside a function literal — `t.Run` body, `defer func`, goroutine, callback — is attributed to the enclosing named function in both passes, PROVEN by a fixture test rather than assumed: the "call it at the test's top level" folk rule is either retired with evidence or its real cause is fixed {id: VS-7, symbol: resolveFileEdges, package: internal/fusion, reachable_from: TestClosureCallsReachFromEnclosingTest}
- [x] `fathom verify --quiet` prints only failing (and, with --strict, unknown) rows plus the summary line, never the ✓ rows; the lefthook pre-commit/pre-push verify steps use it, so a gate run over 40 plans reads as its exceptions, not a wall of checkmarks (O19 / G14) {id: VS-8, symbol: cmdVerify, package: cmd/fathom, reachable_from: TestVerifyQuietPrintsFailingOnly}
- [x] the typed-edges pass counts and reports every edge it dropped because the target's declaring file was outside the connector's discovered set (`targetAllowed` false) — and when such a file EXISTS on disk, the ingest report names it as a discovery failure, never a clean "0 failed": a partially wedged discovery silently voiding edges into unread files is the likeliest cause of the 2026-08-23 cpp-toolchain flip (type-check errors are already counted; this is the gap beside it) {id: VS-9, symbol: EnrichTypedEdges, package: internal/fusion, reachable_from: TestTypedEdgesNameDroppedOutOfScopeTargets}
- [x] one deterministic reachability fixture (a small Go module under testdata with mux handlers, a route table, a HandlerFunc conversion, middleware, a t.Run-only caller, and two types sharing a method name) is ingested end-to-end and its expected TestImpact results asserted — the regression harness every future reachability claim runs against {id: VS-10, symbol: TestReachabilityFixtureEndToEnd, package: internal/fusion, skip_test: true, skip: "the row IS the harness test; a test cannot be reached by a test"}

## Deferred, named

- O3(c) — a CUE evidence tier (`vet_clean` / `export_golden`) so a row
  proves behaviour, not just presence: real design work on the tier
  vocabulary, not a patch; deferred until VS-3/VS-4 traffic shows the
  edge relation is right.
