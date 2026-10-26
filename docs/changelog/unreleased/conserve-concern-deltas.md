### Added

- `fathom conserve --since BASE --concern-deltas FILE` writes experimental `fathom.concern-deltas/v1` Kubernetes fact deltas with workload addresses, before/after values, commit anchors, index basis and skipped files. Deltas are informational and never affect `--strict`. The schema remains experimental until PLT-M4 and is not yet part of the compatibility surface. With this flag, `--json`'s `fathom.conserve/v1` gains the additive `concern_deltas` field.
