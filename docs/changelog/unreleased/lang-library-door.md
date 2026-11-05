### Added

- `pkg/lang` reads a file into symbols, references, refused references and declared facts without the rest of fathom: `lang.New`, `Reader.Extract` (a skipped file says why: binary, oversize, generated, ambiguous, unclassified or not-linked) and `Reader.Dialects`, each dialect with the level its reading measured. `lang.EnableCompilationCache(dir)` is the one opt-in disk write. `fathom-lang extract|dialects` prints the same as JSON for tools not written in Go (ADR-092, LL-10 to LL-12, LL-17).
