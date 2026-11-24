HLP-3 verdict: PASS

# HLP-3: row export, what the rows hold on the outline corpus

Measured at `9c99d4c88` (the reader and `view.Rows` at that commit). The measuring test is
`TestRowsCorpusReading` in `pkg/lang/view/rows_corpus_test.go`, added with the row export. Toolchain `go1.27.4`,
`darwin/arm64`.

Aggregate SHA-256 of the printed JSON: `870a4a9c3c229720a062956f5417d4fa8f7c34bd5a937713942f5d594bf69289` (two runs gave the same digest).

PASS means the HLP-3 proof tests pass and the lossless check below holds on every measured file. The proof tests
are `go test -race -count=1 ./pkg/lang/view ./cmd/fathom-lang`: the lossless test on the binary goldens and the
golden source tree, schema acceptance of every golden output and rejection of each invalid fixture, the drift
guard over the JSON tags of `lang.Result`, `lang.Symbol`, `lang.Reference`, `lang.Unresolved` and `lang.Fact`, the
published schema copies, binary goldens for `extract -rows` and each `schema` name, byte-equal output for equal
input, and one executed SQL example in in-memory SQLite. There is no bar on the row counts.

## Rerun

```sh
FATHOM_LP8_CORPUS=<manifest.json> go test -race -count=1 -run TestRowsCorpusReading -v ./pkg/lang/view
```

The manifest, its pin checks and its selection rules are those of the HLP-1 reading (the test shares that test's
subject type, git checks and `selectFiles`). It skips when the variable is unset, so the gate never runs it.

## Corpus

The outline corpus of HLP-2, as in the HLP-1 reading: the COV-0 development pools for Python, JavaScript and
TypeScript, and the three Go subjects of COV-M3. A measured file is a selected file whose read has at least one
symbol, read with `lang.New()` defaults. Each measured file was exported with `view.Rows`, rebuilt into the nested
document from its rows alone, and the rebuilt bytes compared with the nested bytes.

## Row counts per dialect

Header rows: one per export, not counted below. The reader returned no unresolved reference, fact or note on any
measured file, so those three kinds are zero here. They are covered by the unit and golden tests, not by corpus
files.

| Dialect | Files | file | symbol | reference | unresolved | fact | note | Lossless | Lossy |
|---|---|---|---|---|---|---|---|---|---|
| go | 1386 | 1386 | 25766 | 95091 | 0 | 0 | 0 | 1386 | 0 |
| javascript | 1508 | 1508 | 22805 | 54480 | 0 | 0 | 0 | 1508 | 0 |
| python | 1314 | 1314 | 14630 | 17981 | 0 | 0 | 0 | 1314 | 0 |
| typescript | 1224 | 1224 | 20367 | 65220 | 0 | 0 | 0 | 1224 | 0 |
| **total** | 5432 | 5432 | 83568 | 232772 | 0 | 0 | 0 | 5432 | 0 |

## Row counts per subject

| Subject | Files | file | symbol | reference | unresolved | fact | note | Lossless | Lossy |
|---|---|---|---|---|---|---|---|---|---|
| cli | 737 | 737 | 9158 | 33202 | 0 | 0 | 0 | 737 | 0 |
| cobra | 36 | 36 | 775 | 3165 | 0 | 0 | 0 | 36 | 0 |
| prometheus | 613 | 613 | 15833 | 58724 | 0 | 0 | 0 | 613 | 0 |
| eslint | 389 | 389 | 3363 | 8079 | 0 | 0 | 0 | 389 | 0 |
| openlayers | 373 | 373 | 4840 | 12768 | 0 | 0 | 0 | 373 | 0 |
| webpack | 746 | 746 | 14602 | 33633 | 0 | 0 | 0 | 746 | 0 |
| attrs | 56 | 56 | 2916 | 3781 | 0 | 0 | 0 | 56 | 0 |
| fastapi | 1175 | 1175 | 9705 | 10910 | 0 | 0 | 0 | 1175 | 0 |
| flask | 83 | 83 | 2009 | 3290 | 0 | 0 | 0 | 83 | 0 |
| fp-ts | 204 | 204 | 9283 | 15213 | 0 | 0 | 0 | 204 | 0 |
| headlessui | 201 | 201 | 2528 | 9019 | 0 | 0 | 0 | 201 | 0 |
| react-bootstrap | 233 | 233 | 1962 | 6351 | 0 | 0 | 0 | 233 | 0 |
| rxjs | 586 | 586 | 6594 | 34637 | 0 | 0 | 0 | 586 | 0 |

## Lossless result

5432 of 5432 measured files rebuilt to the same bytes as the nested document, and 0 did not. Every symbol, reference, unresolved reference, fact and note row is a copy of a value in the
Result, and the file row carries `has_result` and `null_arrays`, so a null array is told from an empty one.

## What it means for a consumer

- A corpus file costs about 15.4 symbol rows and 42.9 reference rows on average, plus one file row. Row counts follow the Result, so a
  consumer can size a SQLite or DuckDB load from `extract` output it already has.
- Reference rows dominate. A consumer loading a large tree should expect about 2.8 reference rows per symbol row on these dialects.
- Not measured: load time or query time in any database, rows for dialects outside the corpus, and whether the
  row shape serves a consumer better than the nested document. The row shape and the schemas are provisional.
- `schema outline` exits 1 as not yet available: the outline shape belongs to HLP-2 and does not exist yet.
