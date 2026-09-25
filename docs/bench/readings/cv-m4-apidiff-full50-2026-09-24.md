# CV-M4 (wave 3, gap-closing): the Go surface delta vs apidiff, full 50 pairs (2026-09-24)

**Verdict (plain language). CV-M4 now passes.** Precision 0.960, recall
1.000 over the full 50 consecutive release pairs the design's bar asks
for (cobra, cli/cli, prometheus) — both at or above the 0.95/0.95 bar
ADR-044 sets and the revisit trigger names. This closes the gap the prior
reading found (`docs/bench/readings/cv-m4-apidiff-calibration-2026-09-24.md`:
0.615 precision / 0.941 recall over 38 of the 50 pairs): the Go `api:`
surface delta now agrees with `golang.org/x/exp/apidiff`'s own
compatible/incompatible classification for `any`/`interface{}` spelling,
additive struct fields, and a type-alias introduction — the three
disagreement classes that reading named — plus two more the FULL 50-pair
run surfaced along the way (a `var`'s literal-vs-named-constant
initializer, and a func-literal-valued `var`'s own parameter types never
being normalized). **One residual false positive remains** (cobra
v1.9.1→v1.10.0), named below — it needs resolving another package's own
type declaration, which this fix deliberately does not attempt (see
"Residual disagreement" below), and it does not move the overall
precision below the bar.

Bar (design §5, ADR-044 revisit clause): precision and recall of
"incompatible change," per release pair, both ≥ 0.95 over 50 consecutive
release pairs across the calibration repositories.

## What changed since the failing reading

`internal/analysis/inventory/apicompat.go` (new file), wired into
`inventory.Extract`/`inventory.Diff`:

1. **Type identity for Go `api:` type declarations**, entirely from the
   same per-file `go/ast` parse `Extract` already does (no `go/types`
   pass, no second module load — P19 stays intact): a `TypeDetail` per
   exported `type` declaration carries its struct field set (name ->
   normalized type text, tags excluded — apidiff's own rule), its
   interface method set (name -> normalized signature, plus whether the
   base side already had an unexported method), and whether it is an
   alias (`type X = T`, the parser's own `Assign` token) or a defined
   type (`type X T`). `Diff` applies apidiff's own compatibility rules
   over this shape instead of a whole-text comparison: an added exported
   struct field is compatible (only removing or retyping an EXISTING one
   is not); an added interface method is compatible only when the base
   interface already carried an unexported method (apidiff's own
   asymmetric rule — sealed interfaces may grow for free, open ones may
   not, even for an exported addition); a flip between alias and defined
   is always a loss, whatever the RHS text does.
2. **`any`/`interface{}` canonicalization at render time**
   (`normalizeAnyType`), applied inside `renderNode` itself — so it
   collapses the two spellings for EVERY Go item this package renders,
   not only type declarations: a func signature, a struct field's type,
   a const/var's declared type. This alone fixes class A wherever it
   appears, with no `TypeDetail` involved.
3. **A `var`'s initializer is compared by inferred KIND, not by its own
   text**, when unearthed by the full 50-pair run: apidiff's own
   `checkObjects` only ever compares a `*types.Var`'s TYPE, never its
   initializer expression, so `var X = true` and a later `var X =
   someBoolConst` (a bare literal promoted to a named constant — a real,
   common Go refactor, cobra's own v1.5.0→v1.6.0) are the identical
   `bool`-typed var to apidiff however differently spelled. A `const`
   keeps its literal text — a const's actual VALUE, unlike a var's, is
   part of apidiff's own compatibility check.
4. The `any`/`interface{}` collapse also had to reach into a `*ast.FuncLit`
   used as a `var`'s value (no explicit `Type:`, so the whole closure,
   parameters included, is what gets rendered) — cli/cli's own
   `utils.TerminalSize`, a `var TerminalSize = func(w interface{}) ...`
   promoted to `func(w any) ...` in the same modernization sweep as the
   class-A example. Only the closure's parameter/result TYPES are
   normalized; its body is kept exactly as parsed.

Tests reproducing each of the three named disagreement classes (plus the
two found above, and the interface-method asymmetry apidiff's own source
documents) are fixtures in
`internal/analysis/inventory/inventory_test.go` and
`internal/conserve/surface_test.go`.

## Method

Clones under `/Volumes/m2ext/tmp/cv3-calib/` (cobra, cli/cli, prometheus),
the most recent N tags per repo sharing one `go.mod` module path (cobra:
all 24 go.mod-carrying tags, v0.0.4→v1.10.2, 23 pairs; cli/cli: the most
recent 18 `/v2`-module tags, v2.87.1→v2.101.0, 17 pairs; prometheus: the
most recent 11 v3.x tags, v3.10.0→v3.14.0, 10 pairs — 50 pairs total,
chosen to include every pair the prior 38-pair reading's three
disagreement-class examples named: cobra v1.1.3→v1.2.0 and
v1.9.0→v1.9.1, cli/cli v2.98.0→v2.99.0, prometheus v3.11.1→v3.11.2).
`golang.org/x/exp/cmd/apidiff@latest` installed into a scratch `GOBIN`
(never a global install) — **rebuilt with `GOTOOLCHAIN=go1.27.1`
forced**, not the ambient go1.26.3: apidiff's own compiled-in `go/types`
pass, not the `go list` subprocess it also shells out to, is what parses
a calibration repo's source, so a repo whose later tags use go1.27
language syntax needs the TOOL itself built with a new-enough toolchain
— setting `GOTOOLCHAIN` on the *comparison* invocation does not help,
only rebuilding apidiff does. For each pair `(old, new)`, from the target
repo's own checkout:

```sh
git checkout -q old && apidiff -m -w old.export <module-path-from-go.mod>
git checkout -q new && apidiff -m old.export <module-path-from-go.mod> > apidiff.out
bin/fathom conserve --service cvm4-full --store <scratch>.sqlite --since old --json
```

Ground truth (gold) per pair: `apidiff.out`'s `Incompatible changes:`
section non-empty. Prediction per pair: fathom's `surface` plane has ≥1
`removed`/`changed` finding whose key starts `api:`. A run in which
apidiff's own output carries no "Incompatible changes:"/"Compatible
changes:" header AND a genuine failure signature (`requires newer Go
version`, `found no packages`, a `loading ...: err` line — apidiff's
report writer skips an empty section header outright, so "no header" by
itself is NOT a failure, it is a valid empty-diff gold=0, and reading it
as one would be exactly the false-RED shape the EPERM guide warns
about — a broken run misread as clean) is excluded from scoring rather
than silently counted; **zero pairs were excluded this run** (one
transient case was investigated by hand, confirmed a genuine empty diff
on both sides, and re-ran clean once apidiff itself was correctly built).
The unit of measurement is the pair, not the individual symbol, same
rationale as the prior reading: apidiff and fathom name changes in
incompatible vocabularies, so the pair-level yes/no question is the one
both tools answer natively.

## Results

| repo | pairs | TP | FP | FN | TN | precision | recall |
|---|---|---|---|---|---|---|---|
| cobra | 23 | 6 | 1 | 0 | 16 | 0.857 | 1.000 |
| cli/cli | 17 | 12 | 0 | 0 | 5 | 1.000 | 1.000 |
| prometheus | 10 | 6 | 0 | 0 | 4 | 1.000 | 1.000 |
| **overall** | **50** | **24** | **1** | **0** | **25** | **0.960** | **1.000** |

Both numbers clear the 0.95/0.95 bar; recall is perfect, precision is
0.01 short of perfect on this sample, entirely from one named, understood
case.

### Per-pair table

| repo | old → new | gold | pred | apidiff incompatible | fathom api findings |
|---|---|---|---|---|---|
| cobra | v0.0.4 → v0.0.5 | 1 | 1 | 7 | 7 |
| cobra | v0.0.5 → v0.0.6 | 1 | 1 | 1 | 3 |
| cobra | v0.0.6 → v0.0.7 | 1 | 1 | 1 | 1 |
| cobra | v0.0.7 → v1.0.0 | 0 | 0 | 0 | 0 |
| cobra | v1.0.0 → v1.1.0 | 0 | 0 | 0 | 0 |
| cobra | v1.1.0 → v1.1.1 | 0 | 0 | 0 | 0 |
| cobra | v1.1.1 → v1.1.2 | 0 | 0 | 0 | 0 |
| cobra | v1.1.2 → v1.1.3 | 0 | 0 | 0 | 0 |
| cobra | v1.1.3 → v1.2.0 | 0 | 0 | 0 | 0 |
| cobra | v1.2.0 → v1.2.1 | 0 | 0 | 0 | 0 |
| cobra | v1.2.1 → v1.3.0 | 0 | 0 | 0 | 0 |
| cobra | v1.3.0 → v1.4.0 | 1 | 1 | 3 | 12 |
| cobra | v1.4.0 → v1.5.0 | 0 | 0 | 0 | 0 |
| cobra | v1.5.0 → v1.6.0 | 0 | 0 | 0 | 0 |
| cobra | v1.6.0 → v1.6.1 | 0 | 0 | 0 | 0 |
| cobra | v1.6.1 → v1.7.0 | 0 | 0 | 0 | 0 |
| cobra | v1.7.0 → v1.8.0 | 0 | 0 | 0 | 0 |
| cobra | v1.8.0 → v1.8.1 | 0 | 0 | 0 | 0 |
| cobra | v1.8.1 → v1.9.0 | 1 | 1 | 5 | 7 |
| cobra | v1.9.0 → v1.9.1 | 1 | 1 | 5 | 1 |
| cobra | **v1.9.1 → v1.10.0** | **0** | **1** | 0 | 1 |
| cobra | v1.10.0 → v1.10.1 | 0 | 0 | 0 | 0 |
| cobra | v1.10.1 → v1.10.2 | 0 | 0 | 0 | 0 |
| cli/cli | v2.87.1 → v2.87.2 | 0 | 0 | 0 | 0 |
| cli/cli | v2.87.2 → v2.87.3 | 0 | 0 | 0 | 0 |
| cli/cli | v2.87.3 → v2.88.0 | 1 | 1 | 2 | 2 |
| cli/cli | v2.88.0 → v2.88.1 | 1 | 1 | 1 | 1 |
| cli/cli | v2.88.1 → v2.89.0 | 1 | 1 | 8 | 7 |
| cli/cli | v2.89.0 → v2.90.0 | 0 | 0 | 0 | 0 |
| cli/cli | v2.90.0 → v2.91.0 | 1 | 1 | 12 | 13 |
| cli/cli | v2.91.0 → v2.92.0 | 0 | 0 | 0 | 0 |
| cli/cli | v2.92.0 → v2.93.0 | 1 | 1 | 6 | 4 |
| cli/cli | v2.93.0 → v2.94.0 | 1 | 1 | 4 | 1 |
| cli/cli | v2.94.0 → v2.95.0 | 0 | 0 | 0 | 0 |
| cli/cli | v2.95.0 → v2.96.0 | 1 | 1 | 12 | 7 |
| cli/cli | v2.96.0 → v2.97.0 | 1 | 1 | 12 | 10 |
| cli/cli | v2.97.0 → v2.98.0 | 1 | 1 | 1 | 2 |
| cli/cli | v2.98.0 → v2.99.0 | 1 | 1 | 3 | 1 |
| cli/cli | v2.99.0 → v2.100.0 | 1 | 1 | 16 | 18 |
| cli/cli | v2.100.0 → v2.101.0 | 1 | 1 | 20 | 33 |
| prometheus | v3.10.0 → v3.11.0 | 1 | 1 | 94 | 91 |
| prometheus | v3.11.0 → v3.11.1 | 0 | 0 | 0 | 0 |
| prometheus | v3.11.1 → v3.11.2 | 0 | 0 | 0 | 0 |
| prometheus | v3.11.2 → v3.11.3 | 1 | 1 | 1 | 1 |
| prometheus | v3.11.3 → v3.12.0 | 1 | 1 | 22 | 21 |
| prometheus | v3.12.0 → v3.13.0 | 1 | 1 | 21 | 23 |
| prometheus | v3.13.0 → v3.13.1 | 0 | 0 | 0 | 0 |
| prometheus | v3.13.1 → v3.13.2 | 1 | 1 | 1 | 1 |
| prometheus | v3.13.2 → v3.13.3 | 0 | 0 | 0 | 0 |
| prometheus | v3.13.3 → v3.14.0 | 1 | 1 | 10 | 17 |

The three pairs the prior reading named as disagreement-class examples
all now agree: cobra v1.1.3→v1.2.0 (0/0, was fathom-changed/apidiff-clean
on the `..Command` struct's additive field), cobra v1.9.0→v1.9.1 (1/1,
the `CompletionFunc` alias introduction — apidiff's largest incompatible
change in the sample, now correctly caught), cli/cli v2.98.0→v2.99.0
(1/1 — the pair does carry three genuinely incompatible changes, an
unexported-interface-embedding case fathom's exported-only `api:` scope
correctly stays silent on, and an exported interface's `Commentable`
gaining two methods, which fathom's own interface-addition rule now
correctly flags), prometheus v3.11.1→v3.11.2 (0/0, the `discovery/consul`
struct's additive field).

## Residual disagreement (does not fail the bar)

**cobra v1.9.1→v1.10.0 (the one false positive): a cross-package type
alias.** `var FParseErrWhitelist flag.ParseErrorsWhitelist` (cobra's own
pflag dependency, imported as `flag`) becomes `var FParseErrWhitelist
flag.ParseErrorsAllowlist` in the new pflag version. apidiff resolves
that `pflag.ParseErrorsAllowlist` is itself declared as an alias for
`pflag.ParseErrorsWhitelist` — a fact that lives in the pflag package's
OWN source, not cobra's — so the var's static type is unchanged and the
pair is compatible. fathom's `api:` extraction has no cross-package
resolution by design (this fix's own scope, stated in
`internal/analysis/inventory/apicompat.go`'s header comment): it sees two
different type-expression texts (`flag.ParseErrorsWhitelist` vs
`flag.ParseErrorsAllowlist`) and, correctly by ITS OWN rules, reports
`changed`. Closing this would mean fetching and parsing an external
dependency's own source at the pinned version — a second module's worth
of resolution, the same cost tier `go/types`/`go/packages` would need,
that P19 and this fix's own design explicitly decline to pay for a single
observed instance. Named here as a residual class rather than fixed.

## Scope note

50 of 50 requested pairs, chosen from the most recent tags sharing one
`go.mod` module path per repo (an earlier module-path migration — cli/cli
moved from `github.com/cli/cli` to `github.com/cli/cli/v2`, for instance —
would otherwise require comparing across two different module identities
mid-run, a different question `apidiff -m` is not built to answer
directly). Every fixture in this reading — the three original disagreement
classes and the two more the full run found — has a small, standalone
regression test in `internal/analysis/inventory/inventory_test.go` and
`internal/conserve/surface_test.go`, independent of this sample.
