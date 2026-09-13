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

## Known gap: `SymbolNode.EndLine` is unset for Go

Found while building this feature: the Go extractor (`internal/extractor/goextract.go`) never sets `core.SymbolNode.EndLine` — only the Markdown extractor computes it. Since `JoinSymbols`/`JoinLayeredSymbols` need a symbol's END line to decide which profiled statement blocks belong to it, an unconditional `EndLine>0` requirement would make the join dead-on-arrival for every real Go function in this index today.

**Fallback used (RC-37/38, `internal/analysis/coverage/join.go`):** when a coverage-eligible symbol's `EndLine` is 0, it is synthesized as the line immediately before the NEXT coverage-eligible symbol's own `Line` in the same file (sorted by `Line`), or "the rest of the file" for the last symbol in it — the same technique `internal/extractor/mdextract.go` already uses for a Markdown heading's own `EndLine`. This is a reasonable approximation (Go functions rarely have meaningful trailing dead space between them) but is exactly that: an approximation standing in for a real extractor fix. It has not been filed as its own plan row yet.

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

## Maintained

This doc covers: `review_diff` (`coverage_profile`, `coverage_profiles` args), `fathom review` (`--coverage-profile`), `fathom tests coverage` (`--profile`, `--service`, `--diff`, `--json`). Update it when these change.
