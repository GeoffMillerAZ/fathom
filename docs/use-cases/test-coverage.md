# Test Coverage

Join one or more Go cover profiles — one per test layer — to indexed symbols, so a diff review can name exactly which changed code has no covering test, and in which layer(s), without an agent reading a raw `coverage.out` file.

---

## When to use this

- You want per-symbol statement coverage, not just per-file/package percentages `go tool cover` reports.
- You want to know, for a diff, which changed functions/methods have NO covering test in ANY layer — the review kit's gate signal, not fathom's.
- You run unit/integration/e2e (or your own vocabulary) as SEPARATE `go test` invocations and want coverage broken out by which of those layers actually exercises each symbol.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `review_diff` (`coverage_profiles` / `coverage_profile` args) | MCP tool (investigation) | The composed verb's `coverage` dimension: per-changed-symbol `{symbol, file, line, layers, uncovered_at, percent, uncovered}`, `checked:false` with reason `no coverage profile supplied` when neither arg is given |
| `fathom review --coverage-profile [layer=]<file>` (repeatable) | CLI | The CLI mirror's own `coverage` dimension — same fields, same honesty rule, resolved via the identical `internal/analysis/review.ResolveCoverage` composition so both surfaces never drift |
| `fathom tests coverage --profile [layer=]<file>` (repeatable) | CLI | Standalone per-symbol coverage table (one column per layer, plus `ANY`) or `--json`, independent of a diff; `--diff <base>` narrows to changed symbols with no covering test in any layer |

---

## By layer: a caller-declared label, never a guess

**How layers are attributed:** fathom NEVER inspects a profile's contents to infer which test layer produced it, and never guesses from per-test data. Layer attribution is exactly what the CALLER labels each profile with — because that's how layers actually work in practice: they are run SEPARATELY.

```
go test -coverprofile=workspace/cov/unit.out ./...
go test -tags integration -coverprofile=workspace/cov/integration.out ./...
# an e2e suite writes its own workspace/cov/e2e.out
```

Pass each one labeled by layer:

```
fathom tests coverage \
  --profile unit=workspace/cov/unit.out \
  --profile integration=workspace/cov/integration.out \
  --service myapp
```

A bare `--profile <path>` (no `layer=` prefix) defaults to layer `"unit"` — `internal/testlayer.Classify`'s own default for an unlabeled test file. There is no signal inside a cover profile that could confirm or override that default; it is the caller's honest fallback, not an inference fathom makes.

For each symbol, fathom reports:

- **`layers`** — which of the SUPPLIED layers actually cover it (statement count > 0 in that layer's own profile);
- **`uncovered_at`** — which supplied layers ran over it but recorded zero executions (as distinct from a layer with no data for it at all, which is simply absent from both lists);
- **`percent` / `uncovered`** — the UNION across every supplied layer: a statement counts as covered if ANY layer covered it.

---

## The boundary

**fathom asserts a fact. The review kit judges it.**

fathom asserts, from the Go cover profiles you supply:

- which function/method symbols have statement-coverage data, from which layer(s), and how many of their statements ran;
- whether a symbol has ZERO covering executions in a given layer, or in the union of every layer supplied — a fact, not a verdict;
- nothing about which tests SHOULD exist, whether a percentage is "enough", or any pass/fail threshold — no blocking behavior of any kind.

A review kit consuming `fathom review --json`'s `coverage` block (or `review_diff`'s) owns the threshold, the blocking decision, and any "this needs a test" judgment. fathom's job stops at the fact.

---

## Former gap, fixed: `SymbolNode.EndLine` for Go (T5-5)

Found while building this feature: the Go extractor (`internal/extractor/goextract.go`) did not set `core.SymbolNode.EndLine` — only the Markdown extractor computed it. Since `JoinSymbols`/`JoinLayeredSymbols` need a symbol's END line to decide which profiled statement blocks belong to it, an unconditional `EndLine>0` requirement would have made the join dead-on-arrival for every real Go function in this index.

The Go extractor now sets `EndLine` for every func/method/type/const/var it emits, from the declaration's own `ast.Node.End()` — a real end line, not an approximation. Several tree-sitter/AST-backed extractors (C, C++, C#, Java, Lua, PHP, Rust, Scala, CUE, HCL/Terraform, CMake, Templ) do the same.

**Fallback kept for what's still unset (RC-37/38, `internal/analysis/coverage/join.go`):** when a coverage-eligible symbol's `EndLine` is still 0 (an unrefreshed index, or an extractor that doesn't compute it — see `core.SymbolNode.EndLine`'s doc comment for the current list), it is synthesized as the line immediately before the NEXT coverage-eligible symbol's own `Line` in the same file (sorted by `Line`), or "the rest of the file" for the last symbol in it — the same technique `internal/extractor/mdextract.go` uses for a Markdown heading's own `EndLine`. `join.go` prefers a real, positive `EndLine` over this synthesis.

---

## Example

```
$ go test -coverprofile=workspace/cov/unit.out ./internal/widget/...
$ go test -tags integration -coverprofile=workspace/cov/integration.out ./internal/widget/...
$ fathom review --service myapp --since origin/main \
    --coverage-profile unit=workspace/cov/unit.out \
    --coverage-profile integration=workspace/cov/integration.out \
    --json
```

```json
{
  "dimensions": [
    {
      "name": "coverage",
      "checked": true,
      "coverage": [
        {"symbol": "myapp::internal/widget.Parse", "file": "internal/widget/widget.go", "line": 42, "layers": ["unit"], "uncovered_at": ["integration"], "percent": 100, "uncovered": false},
        {"symbol": "myapp::internal/widget.newHelper", "file": "internal/widget/widget.go", "line": 61, "layers": [], "uncovered_at": ["unit", "integration"], "percent": 0, "uncovered": true}
      ]
    }
  ]
}
```

`fathom tests coverage --profile unit=workspace/cov/unit.out --profile integration=workspace/cov/integration.out --service myapp --diff origin/main` prints the same `newHelper` gap as a standalone table, without running the rest of the review.

---

## Selecting tests

`fathom tests select --since <ref>` answers a different question than
coverage does: not "what ran", but "what should I run for THIS diff" —
which `Test*` functions to actually invoke, ranked, plus the changed
symbols no test reaches at all (VB-33, `docs/vision/29-verbs-not-lookups.md`
§3 wave J).

**Certifying-first.** The output leads with the minimal set of tests that
re-proves every LIVE certificate the diff touched (T4, TQ-15w; a greedy set
cover, ties broken by name), the tests it skipped, the touched certificates no
live test certifies, and the CI time the selection saves — summed from each
selected and skipped test's last attested run duration, printed as
`time saved: N%` and appended to
`workspace/artifacts/tests-select/time-saved.jsonl` (TQ-16w; with no attested
duration on record it says `unknown` and records nothing). `--safe` adds every
test that statically reaches a changed symbol. In `--json` this is the
`selection` object. Selection is advice to CI: it never makes a certificate
live, only an attested passing run does. What follows is the reach ranking of
every test that touches the diff, the set `--safe` selects.

It reuses the SAME diff→changed-symbols composition `fathom review` and
`review_diff`'s `test_impact` dimension already use
(`internal/analysis/impact.TestGap`, built on the call graph's
`TestImpactDetail`): every ranked test is a `Test*` function that statically
reaches at least one changed symbol.

```
fathom tests select --since origin/main --service myapp \
  --profile unit=workspace/cov/unit.out
```

Ranking, in priority order:

1. **Direct reach before dispatch-mediated reach.** A test whose call site
   literally names the changed symbol outranks one whose only path crossed
   a typed interface-dispatch edge (ADR-021 §3) — real evidence, but weaker.
2. **WHEN `--profile` is supplied:** descending mean statement-coverage
   percent over the changed symbols the test reaches — a reach corroborated
   by a real, executed run outranks one that is only a static (possibly
   dead) edge. A standard Go cover profile carries no per-test attribution
   (see `internal/analysis/coverage`'s own doc comments), so this is a
   symbol-level corroboration signal, never a per-test one, and it has NO
   effect on ranking when no profile is given.
3. **Test layer** (`internal/testlayer.Classify`): unit before integration
   before e2e — run the cheap tests first.
4. **Reach breadth**, descending — a test touching more of the change first.
5. **Test name**, for a total deterministic order.

The output is a ranked list, a `go test` invocation PER PACKAGE (grouped —
`-run` only makes sense within one package's test binary), and the test
gap:

```json
{
  "since": "origin/main",
  "ranked_tests": [
    {"test": "myapp::internal/widget.TestParse", "file": "internal/widget/widget_test.go", "citation": "internal/widget/widget_test.go#L10-L18", "layer": "unit", "reaches": ["myapp::internal/widget.Parse"]}
  ],
  "invocations": [
    {"package": "internal/widget", "tests": ["TestParse"], "command": "go test ./internal/widget -run '^(TestParse)$'"}
  ],
  "test_gap": [
    {"symbol": "myapp::internal/widget.newHelper", "file": "internal/widget/widget.go", "citation": "internal/widget/widget.go#L61"}
  ]
}
```

**Predicted catch-rate — never a promised number.** When a replay ledger
exists at `workspace/artifacts/tests-select/replay.jsonl` (newline-
delimited JSON, one historical replay per line —
`{"commit":"<sha>","selected":<n>,"total":<n>,"caught":<bool>}`), the
report adds a `replay` block: the measured fraction of past replays where
the selection would have included a test that actually failed. No ledger,
no block — fathom never fabricates a catch-rate it hasn't measured.

---

## Mutation: covered is not the same as asserted

Statement coverage answers "did SOME test execute this line". It says
nothing about whether that test would have NOTICED the line breaking — a
symbol can be 100% covered with every assertion in its reaching test
disconnected from the mutated behavior. Mutation testing (VB-34) closes
that gap: `internal/analysis/coverage.ParseMutationReport` joins a mutation
run to symbols exactly like a cover profile does, producing per-symbol
`{mutants, killed, survived, score}`.

Two input shapes are accepted, detected by content:

- **go-mutesting's own log output** (`github.com/zimmski/go-mutesting`) —
  its per-mutant outcome line, `PASS "<tmpfile>" with checksum <hex>` (the
  mutant was KILLED: the exec step's tests failed) or `FAIL "..."` (the
  mutant SURVIVED: tests still passed) or `SKIP "..."` (didn't compile — no
  evidence either way, counted separately, never joined). A FAIL record
  always carries the `diff -u` block go-mutesting prints before it, which is
  how the file and line are recovered; a PASS/SKIP record only carries one
  under `go-mutesting --debug` — **run it with `--debug` for symbol-level
  attribution** (`go-mutesting --debug ./... > workspace/artifacts/mutation.txt`).
  Without `--debug`, a killed mutant's location can still often be recovered
  from its own temp-file path, but never its line — such a mutant is kept
  (never dropped) and counted in `--json`'s `mutation_summary.unattributed`
  (`mutation_summary.skipped` counts the SKIP records; the table form
  prints the same two counts as a line when either is non-zero).
  The trailing `The mutation score is F (P passed, F failed, D duplicated,
  S skipped, total is T)` summary line is cross-checked against what was
  parsed; a mismatch, or its absence when records were found, is a parse
  error, not a silent partial report.
- **fathom's own JSON shape**, for any other tool a caller adapts to it:
  `{"mutants":[{"file":"path/to/x.go","line":42,"killed":true}]}` (a bare
  top-level array of the same records is accepted too).

```
fathom tests coverage --profile workspace/cov/unit.out --mutation workspace/artifacts/mutation.txt --service myapp
```

adds a `MUTATION` score column (0.0-1.0) next to the coverage table, and a
`"mutation": {"mutants": N, "killed": N, "survived": N, "score": 0.0-1.0}`
field per symbol in `--json`.

**The verify gate.** `fathom verify --mutation <report>` (or the
`FATHOM_MUTATION_REPORT` env var; `--store`/direct mode only) re-evaluates
every already-Pass'd `test_reaches` criterion: a claimed row whose symbol IS
reached by a test but whose mutants ALL survived (score 0) gets a
`covered-not-asserted` note appended to that criterion's detail — advisory
by default, so it never fails the gate on its own. `--strict-mutation`
turns that finding into a gate-blocking `Fail` (label
`covered-not-asserted`), the same advisory/strict split every other verify
criterion uses.

---

## Maintained

This doc covers: `review_diff` (`coverage_profile`, `coverage_profiles` args), `fathom review` (`--coverage-profile`), `fathom tests coverage` (`--profile`, `--service`, `--diff`, `--json`, `--mutation`), `fathom tests select` (`--since`, `--safe`, `--profile`, `--json`), `fathom verify` (`--mutation`, `--strict-mutation`). Update it when these change.
