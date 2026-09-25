# CV-M4: the Go surface delta vs apidiff (2026-09-24)

**Verdict (plain language). CV-M4 fails, clearly, on precision, and fires
ADR-044's own named revisit trigger** ("CV-M4 fails: the Go surface delta
disagrees with apidiff below 0.95"). Recall is close (0.941, one point
under the bar on 17 gold-positive pairs) but precision is not (0.615):
fathom's `api:` surface diff calls roughly 2 in 5 "changed" pairs
compatible by apidiff's own type-identity comparison. The gap is not
noise — it resolves into three clean, reproducible mechanisms, one of
which is common enough (a `gofmt`/`go vet -fix`-style `interface{}` → `any`
modernization sweep) that it alone explains most of the false positives in
this sample. **The Go surface delta is not yet apidiff-equivalent; it is a
textual, not a type-identity, diff, and needs to normalize at least
`interface{}`/`any` and Go's struct-compatibility rule (additive is
compatible) before this bar can be met.**

Bar (design §5, ADR-044 revisit clause): precision and recall of
"incompatible change," per release pair, both ≥ 0.95 over 50 consecutive
release pairs across the calibration repositories. **Measured: 38 pairs**
(16 cobra + 11 cli/cli + 11 prometheus, not the full 50 — apidiff on
prometheus's `./...` module average ~40s/pair and this reading's time
budget capped the sample there; see "Scope" below for exactly what that
costs the reading).

## Method

Clones under `apidiff-clones/` (cobra, cli/cli, prometheus), pinned tags,
oldest-16/12/12 release tags → 16/11/11 consecutive pairs (38 total).
`golang.org/x/exp/cmd/apidiff@latest` installed once into a scratch
`GOBIN` (never a global install). For each pair `(old, new)`:

```sh
git checkout -q old && apidiff -m -w old.export <module>   # -m excludes internal/ by default
git checkout -q new && apidiff -m old.export <module> > apidiff.out
bin/fathom conserve --service cvm4 --store cvm4.sqlite --since old --json
```

Ground truth (gold) per pair: apidiff's own `Incompatible changes:` section
non-empty. Prediction per pair: fathom's `surface` plane has ≥1
`removed`/`changed` finding whose key starts `api:`. Both tools already
exclude `internal/` packages by construction (apidiff's `-m` default;
fathom's `api:` extraction is scoped to "neither internal/ nor main" by
design §2.1) — no extra filtering needed. The unit of measurement is the
**pair**, not the individual symbol: apidiff and fathom name changes in
incompatible vocabularies (Go type-checker identity vs. printed-signature
text), so a symbol-level join would be a second, unvalidated instrument;
the pair-level question ("does this release carry an incompatible surface
change, yes/no") is the one both tools answer natively and is what design
§5's "per pair" phrasing names.

## Results

| repo | pairs | TP | FP | FN | TN | precision | recall |
|---|---|---|---|---|---|---|---|
| cobra | 16 | 2 | 6 | 1 | 7 | 0.250 | 0.667 |
| cli/cli | 11 | 7 | 3 | 0 | 1 | 0.700 | 1.000 |
| prometheus | 11 | 7 | 1 | 0 | 3 | 0.875 | 1.000 |
| **overall** | **38** | **16** | **10** | **1** | **11** | **0.615** | **0.941** |

Both numbers are below the 0.95 bar; precision badly, recall by one pair.

### Disagreement classes

**(A) `interface{}` ↔ `any` — the dominant false-positive driver.**
`cli/cli` v2.98.0→v2.99.0: fathom flags 94 `api:` keys `changed`; apidiff
reports zero incompatible (and the pair is a real style-modernization
release). Example finding:
```
"key": "api:api.Client.GraphQL",
"detail": "func(string, string, map[string]interface{}, interface{}) error
        -> func(string, string, map[string]any, any) error"
```
`any` is a predeclared alias for `interface{}` — semantically identical,
zero API impact — but fathom's `detail` is the literal signature text
(design §2.1), which differs, so `inventory.Diff` reports `changed`.
apidiff resolves the underlying `types.Type` and correctly sees no change.
This single mechanism plausibly accounts for most of cli/cli's and
cobra's false positives (a codebase-wide `any` sweep touches every
exported signature using the old spelling in one release).

**(B) Struct field-set text diff vs. Go's additive-is-compatible rule.**
`cobra` v1.1.3→v1.2.0, `prometheus` v3.11.1→v3.11.2: fathom flags
`api:discovery/consul.Discovery`/`SDConfig`, `api:..Command` `changed`
(detail is the whole struct body); apidiff reports these pairs
compatible. apidiff applies Go's real compatibility rule for structs
(adding a new exported field, or reformatting/reordering, is compatible;
only removing or narrowing an *existing* field is not); fathom's
`inventory.Diff` treats ANY textual difference in the struct's printed
body as `changed`, so a release that only ADDS a field, or that a `gofmt`
pass merely re-aligns, still flags.

**(C) A type alias is invisible to a textual signature diff — the sole
false negative.** `cobra` v1.9.0→v1.9.1: `type CompletionFunc func(...)` →
`type CompletionFunc = func(...)` (adding `=`) is apidiff's single
largest incompatible change in this sample (5 symbols: the alias changes
`CompletionFunc`'s type identity, which cascades to every signature that
names it) — genuinely breaking (methods on the old named type are lost;
interface satisfaction changes). fathom's `api:` extraction reports these
same 5 keys with **identical** `detail` text before and after (two of the
five even print byte-identical "changed from X to X" under apidiff itself,
since apidiff's OWN report names the type, not its structure) — the alias
keyword is not part of the printed signature fathom compares, so
`inventory.Diff` sees nothing to report. This is a real recall gap:
detecting it needs type identity, not signature text.

**(D) Minor tooling note, not a classification disagreement.** cobra's
package sits at its module root; every cobra `api:` key rendered with an
empty import-path segment (`api:..Command` — the double dot is literal).
The struct/signature content extracted is correct; only the key's
namespace prefix degrades to empty for a root-level package in a repo
that is not a registered fathom service. This does not affect the
precision/recall figures above (matching is by presence of ANY
removed/changed `api:` finding per pair, not by exact key), but it is a
latent key-COLLISION risk (two unrelated root-package symbols named
`Command` from two different unrelated modules would render the same key)
worth fixing before `api:` keys from an ungraphed external repo are relied
on for anything beyond this reading's pair-level yes/no.

## Scope note (38 of the requested 50 pairs)

Time budget, not a methodological limit, capped the sample at 38: cobra's
16 pairs ran in under a minute each; cli/cli and prometheus's `apidiff -m`
(which type-checks the whole module, transitively) run 7–75s/pair, and
prometheus in particular pulls a large dependency graph. The 38-pair
result is unambiguous in the direction that matters (precision is not
marginally short of 0.95, it is 0.34 short), so extending to 50 is very
unlikely to flip the verdict, but the bar as written asks for 50 and this
reading did not clear it; a follow-up reading should extend prometheus
(and optionally a fourth repository) to close the gap honestly rather than
rounding 38 up to 50 in the record.
