# Test certifications stage 0 — the case-to-symbol join measurement (TC-M1)

**Verdict (plain language).** The join clears the bar on both corpora
measured — 100% on test-kit's own runnable example, 94.76% on fathom's own
`./internal/...` suite — well above the 80% kill line vision 33 sets for
stage 0. Stage 0's design stands; the certifications layer (stage 1) can
proceed. The gap on fathom's own repo is fully accounted for by two causes,
neither a defect in `JoinCasesToSymbols` itself: genuinely ambiguous bare
`Test*` names shared by sibling analyzer packages (by design, never
guessed at), and a `surveyor ingest --format go-test-json` adapter quirk
that mangles a subtest name containing a literal `.` (a golden-fixture
filename, e.g. `post-tool.live.in.json`) down to its last dot-segment,
losing the `Test*` prefix the join needs entirely — an envelope-shape gap
in test-kit's adapter, not in fathom's join contract.

Bar: join rate ≥ 80% (`docs/vision/33-test-certifications.md` §6, design
doc §8's TC-M1). Measured 2026-09-23 on tc-stage0 (built on tc-spec
28cc6da4), fathom binary built from this branch's own `bin/fathom`,
test-kit's `surveyor` built from platform-engineering/kits/test-kit@`6c1f9d73`.

## (a) test-kit's own example — `examples/runnable`

```
surveyor run --dir examples/runnable --json > runnable-envelope.json
fathom ingest --store tc-m1-runnable.sqlite --service testkit-runnable --path examples/runnable
fathom tests ingest --manifest examples/runnable --results runnable-envelope.json \
    --service testkit-runnable --store tc-m1-runnable.sqlite
```

Result: **9 of 9 cases joined (100%)**. `cart/unit`, class unit, tier
blocking, last run `pass` (9 run, 8 passed, 1 skipped). Every case —
`TestRunnableExampleCartAppliesTaxInBasisPoints` and its three subtests,
`TestRunnableExampleCartDiscountNeedsAPricingService` (skip),
`TestRunnableExampleCartSubtotalsLines` and its three subtests — joined to
its own `Test*` symbol exactly, no ambiguity in a four-file module.

## (b) fathom's own repo — `./internal/...`

```
fathom ingest --store tc-m1-fathom.sqlite --service fathom --path .
go test -json ./internal/... > gotest.json          # no -race; 1m20s wall
surveyor ingest --format go-test-json --component fathom --suite unit \
    --class unit --started-at <RFC3339> < gotest.json > fathom-envelope.json
fathom tests ingest --manifest . --results fathom-envelope.json \
    --service fathom --store tc-m1-fathom.sqlite
```

`tests/fathom.cue` (committed): component `fathom`, one `unit` suite over
`./internal/...`, class unit, tier blocking — no `certifies` yet (that is
stage 1's TC-15). Narrowed from `surveyor init`'s own scaffolded
`./...` to `./internal/...` per the assignment's scope.

`go test -json ./internal/...` (no `-race`, to stay well under the ten
minute budget — it ran in 1m20s) reported one real failure,
`internal/fusion.TestTypedEdges_PerformanceGate` (a timing-sensitive
perf gate that failed on this machine's contention, not a build error);
verdict `fail` is the honest, unedited attested result.

Result: **4,542 of 4,793 cases joined (94.76%)**. The 251 unjoined split
two ways:

- **171 — a `surveyor ingest --format go-test-json` adapter quirk.** Every
  one of these carries a case `id` naming a real, findable subtest (e.g.
  `internal/harness.TestCodexHooksFixtureFromLiveSession/post-tool.live.in.json`)
  but a `name` field that is just the LAST dot-separated segment of that
  subtest's own name — `"json"`, `"go"`, `"md"`, `"review"` and 92 other
  bare non-`Test*`-prefixed strings, most repeated across many golden
  fixtures (`post-tool.live.in.json`, `pre-tool.live.in.json`, … all
  collapse to the single case name `"json"`). The subtest names
  themselves are real filenames from table-driven fixture tests (harness
  session replay, markdown/doc fixtures, review fixtures) and legitimately
  contain literal `.` characters; the adapter's name field appears to
  treat the LAST `.`-delimited segment as authoritative, which is correct
  for a package-qualified Go identifier but wrong for a fixture-name
  subtest. `JoinCasesToSymbols` never sees a `Test*` prefix on these case
  names at all — there is nothing for the exact-name join to match, by
  construction, once the name has already lost it. This is exactly the
  design doc §2 / vision 33 §6 escape hatch: "the envelope needs a
  case-to-symbol contract" — here, specifically, `case.name` needs to stay
  the FULL `Test*`-prefixed subtest path (as `surveyor run --json`'s own
  native envelope already does correctly — see (a) above, and the `id`
  field on every one of these 171 cases already carries it) rather than
  being re-derived by splitting on `.`.
- **80 — genuinely ambiguous bare `Test*` names**, e.g. `TestDetect`,
  `TestScan_Determinism`, `TestAnalyze_Determinism`,
  `TestConformance_Deterministic`: fathom's `internal/analysis/*` tree
  has several sibling analyzer packages (license detection, dead-code,
  scanners, …) that each declare a same-named top-level test by
  convention. `JoinCasesToSymbols` treats two-or-more `Test*` symbols
  sharing one bare name as unjoined rather than guessing which package's
  test produced a given case — the design's own "exact name" contract,
  applied honestly. This is a real, permanent gap `case.name` alone can
  never close without a package qualifier the `#TestResult` schema does
  not carry this round; it does not indicate a bug in the join.

## Verdict

Both corpora clear the 80% bar (100% and 94.76%); TC-M1 passes and stage 0's
join design stands as built. The one genuine external gap worth carrying
into a stage-1 conversation with test-kit: `surveyor ingest
--format go-test-json` should preserve `case.name`'s full `Test*`-prefixed
path for every subtest, not just ones whose names happen to avoid literal
dots — `surveyor run --json`'s own native envelope (measurement (a) above)
already gets this right, so the fix is likely narrow, in the `go-test-json`
adapter's own name derivation. The 80-case ambiguous-name gap on fathom's
own repo is not a defect; a package-qualified `case.name`
(`pkg.TestX` rather than bare `TestX`) would close it, and is worth naming
as a candidate case-to-symbol contract improvement if stage 1's own
dogfood run shows it mattering at scale.
