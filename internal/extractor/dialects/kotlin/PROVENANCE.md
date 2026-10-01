# kotlin — provenance

- Grammar: `fwcd/tree-sitter-kotlin` at commit `1852ea17b7f60fb3f9d84e0b1555d56b46b39fb1` (MIT; the
  repository's untagged main, `tree-sitter.json` version 0.4.0), built into
  `internal/extractor/tsengine/grammars/kotlin.wasm.gz` (3,054,455 bytes, 567,216 gzipped); pins and
  digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`. The 0.3.8 tag the dialect first
  shipped on parsed 1,137 of 1,363 real Kotlin files without an ERROR node (`fun interface` did not
  parse at all, a getter or setter after a newline detached, an object expression with two supertypes
  lost its body); this commit parses 1,348 (98.9%). The 15 files left with ERROR nodes (14 in
  JetBrains/Exposed, one build script in square/okhttp) still yield every definition outside the
  error.
- `tags.scm`: the definition shapes follow the grammar's own `queries/tags.scm` at that commit (MIT)
  and the previous version of this file (written in this repository during the tree-sitter migration,
  ADR-021 §4, moved unchanged from `internal/extractor/tsqueries/kotlin.scm` by LB-F1). Written for
  LB-Q-kotlin: `@body` signatures, member and primary-constructor properties (the name anchored to the
  child right after `val`/`var`, so a default value that is a bare identifier is not a second
  property), enum entries, type aliases, secondary constructors, the package clause and `@name`
  inside every call. The previous file's call capture took the whole callee expression as text
  (`repository.find(id).await`); it now takes the callee's last identifier (`await`, `find`).
- `imports.scm`: written for LB-Q-kotlin; the shape follows Java's (`dialects/java/imports.scm`).
- `testdata/`: fixtures written in this repository. `shapes.kt`, `build.kts` and `broken.kt` predate
  LB-Q-kotlin (their goldens were written by the pre-manifest engine at fdbc2ee4; every symbol they
  listed is still held by `TestKotlinDialectKeepsLegacySymbols`); `members.kt`, `calls.kt` and
  `imports.kt` were written for it, with their goldens typed by hand from reading each fixture. The
  engine's output matched them except for the order of same-row siblings (`class User(val name,
  var age)` lists `age` first, `enum class Color { RED, GREEN }` lists `GREEN` first), which is
  tree-sitter's match order, recorded as it is.
- Oracle: `calibration/oracles/kotlin-psi/` (tier B), with the reasons universal-ctags and scip-java
  could not be the L1 oracle; the reading is `docs/bench/readings/lang/kotlin.json`.
- `references.scm` (LB-F74, LB-F78, wave lc): written in this repository; no upstream query adapted
  (user types, navigation heads and value references).
