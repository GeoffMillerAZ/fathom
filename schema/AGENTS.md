# schema/ — CUE schema conventions

- A new schema is a `schema.cue` package with `<package>/testdata/valid.cue`
  (positive) and a commented `<package>/testdata/invalid.cue` (negative) —
  placeholder written as a pattern: these live under each schema package,
  not at a fixed path.
- `invalid.cue` stays fully commented out: `task cue-vet` runs
  `cue vet ./...` over every schema package (+ its testdata), so a live
  negative would fail the gate. Negatives are asserted by Go tests instead.
- Pre-commit also vets any staged-`.cue` directory containing a
  `schema.cue` (`lefthook.yml`); `task cue-vet` is part of `task validate`.
