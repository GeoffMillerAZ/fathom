## SymbolNode shape

Queries and navigation tools return `SymbolNode` objects:

```json
{
  "service":   "payment-service",
  "name":      "ValidateToken",
  "kind":      "GoFunc",
  "file_path": "internal/auth/token.go",
  "line":      42,
  "end_line":  46,
  "signature": "func ValidateToken(ctx context.Context, tok string) error",
  "tier":      "verified"
}
```

`tier` is the evidence tier: `verified`, `pre-scan`, `doc-claimed`, `sme-stated`, or
`assumed`. See [architecture.md](../architecture.md#evidence-tiers-adr-007-adr-009).

`end_line` (T5-5, docs/plans/trust-and-tokenomics.md) is the last line covered by this
symbol, when its extractor computed a real extent — `DocChunk` headings (line before the
next heading, or EOF for the last one); the Go extractor, for every func/method/type/
const/var (the declaration's own end position); most tree-sitter/AST-backed extractors
(C, C++, C#, Java, Lua, PHP, Rust, Scala, CUE, HCL/Terraform, CMake, Templ). Omitted
(zero) for a symbol from an extractor that doesn't compute it yet — see
`core.SymbolNode.EndLine`'s doc comment for the current list. Pair it with `read_source`
(`start_line: line, end_line: end_line`) to pull a symbol's or doc section's real,
line-numbered text instead of guessing its extent by arithmetic off the start line — the
fix for the T08 class in `docs/plans/home-bench.md`'s H3-1 row.

---
