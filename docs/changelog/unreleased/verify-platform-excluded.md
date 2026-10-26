### Changed

- `fathom verify`: a symbol row whose symbol is declared only in Go files the index's build context excludes (a darwin-only file on a Linux index) reads Unknown with `platform: defined only in <file>, excluded on <os>/<arch>` and exits 2 when every unproven row is one, instead of failing the claim. A symbol missing everywhere, or declared in an included file, behaves as before.
