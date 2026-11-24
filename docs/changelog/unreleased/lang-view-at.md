### Added

- `pkg/lang/view` (provisional): `NewIndex`, `Index.Enclosing`, `Index.EnclosingRange`, `Index.Touched` and `Slice` answer which symbol holds a line or range and which symbols a range touches, from one `lang.Result`. A symbol with no end line or an incomplete read gives `unknown`, never a guess. A consumer of `view` links one package more than one of `pkg/lang` alone.
- `fathom-lang at [-root DIR] [-deadline D] [-max-memory SIZE] [-in-process] PATH:LINE[-END]...` (provisional) prints `{"schema_version":1,"queries":[...]}` with status, enclosing symbol, touched symbols and completeness per query. It reads each path once under the supervised boundary and prints no source text.
