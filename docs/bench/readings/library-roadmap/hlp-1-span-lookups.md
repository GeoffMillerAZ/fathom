HLP-1 verdict: PASS

# HLP-1: span lookups, what they answer on the outline corpus

Measured at `653d0a75f` (the reader and `pkg/lang/view` at that commit, tie-break by the largest Key). The measuring test is
`pkg/lang/view/corpus_test.go`, added with this reading. Toolchain `go1.27.4`, `darwin/arm64`.

Aggregate SHA-256 of the printed JSON: `edc6a221463aea0f253f08fc8a2d4374a22be3e9f95b0449ca4a01103cf99aee`
(two runs gave the same digest).

PASS means the HLP-1 tests pass (`go test -race -count=1 ./pkg/lang/view` and the `at` tests and goldens in
`./cmd/fathom-lang`) and the shares below are published. There is no bar on the shares.

## Rerun

```sh
FATHOM_LP8_CORPUS=<manifest.json> go test -race -count=1 -run TestCorpusReading -v ./pkg/lang/view
```

The manifest is a JSON list of `{dialect, subject, commit, path, selection}`. The test fails unless each
checkout's HEAD equals `commit`, the tree is clean (untracked files included), the selected file count equals the
`files=` in `selection`, and, for JavaScript and TypeScript and Go, the SHA-256 of the sorted NUL-terminated
selected paths equals the frozen `selectionSHA256`. It skips when the variable is unset, so the gate never runs it.

## Corpus

The outline corpus of HLP-2: the COV-0 development pools for Python, JavaScript and TypeScript, and the three Go
subjects of COV-M3. A measured file is a selected file whose read has at least one symbol. Every selected file
was read with `lang.New()` defaults and measured as the whole file: lines are 1 to the last line.

| Dialect | Subject | Commit | Selected | Measured |
|---|---|---|---|---|
| go | cli | `45ffa3c668f7` | 737 | 737 |
| go | cobra | `40b5bc1437a5` | 36 | 36 |
| go | prometheus | `9ec59baffb54` | 618 | 613 |
| javascript | eslint | `a438ec39512c` | 389 | 389 |
| javascript | openlayers | `88fc44564552` | 373 | 373 |
| javascript | webpack | `5a106753839f` | 746 | 746 |
| python | attrs | `c8bb9bfaa284` | 56 | 56 |
| python | fastapi | `864b569cf845` | 1175 | 1175 |
| python | flask | `22d924701a6a` | 83 | 83 |
| typescript | fp-ts | `669cd3ed7cb5` | 204 | 204 |
| typescript | headlessui | `d13526d02a2d` | 201 | 201 |
| typescript | react-bootstrap | `8b1cb721001c` | 233 | 233 |
| typescript | rxjs | `72bc92191ab9` | 586 | 586 |

## Per dialect

Known span: a symbol with `EndLine >= Line > 0`. Line shares are over every line of every measured file, by the
status `Enclosing(line)` returns.

| Dialect | Files | Symbols | Known span | Lines | found | none | unknown |
|---|---|---|---|---|---|---|---|
| go | 1386 | 25766 | 100.00% | 504406 | 89.13% | 10.87% | 0.00% |
| javascript | 1508 | 22805 | 100.00% | 496773 | 100.00% | 0.00% | 0.00% |
| python | 1314 | 14630 | 100.00% | 137432 | 100.00% | 0.00% | 0.00% |
| typescript | 1224 | 20367 | 100.00% | 237214 | 100.00% | 0.00% | 0.00% |

Per subject:

| Subject | Files | Symbols | Known span | Lines | found | none | unknown |
|---|---|---|---|---|---|---|---|
| cli | 737 | 9158 | 100.00% | 183922 | 90.08% | 9.92% | 0.00% |
| cobra | 36 | 775 | 100.00% | 16549 | 87.21% | 12.79% | 0.00% |
| prometheus | 613 | 15833 | 100.00% | 303935 | 88.67% | 11.33% | 0.00% |
| eslint | 389 | 3363 | 100.00% | 99336 | 100.00% | 0.00% | 0.00% |
| openlayers | 373 | 4840 | 100.00% | 122146 | 100.00% | 0.00% | 0.00% |
| webpack | 746 | 14602 | 100.00% | 275291 | 100.00% | 0.00% | 0.00% |
| attrs | 56 | 2916 | 100.00% | 21026 | 100.00% | 0.00% | 0.00% |
| fastapi | 1175 | 9705 | 100.00% | 98517 | 100.00% | 0.00% | 0.00% |
| flask | 83 | 2009 | 100.00% | 17889 | 100.00% | 0.00% | 0.00% |
| fp-ts | 204 | 9283 | 100.00% | 77623 | 100.00% | 0.00% | 0.00% |
| headlessui | 201 | 2528 | 100.00% | 50733 | 100.00% | 0.00% | 0.00% |
| react-bootstrap | 233 | 1962 | 100.00% | 23352 | 100.00% | 0.00% | 0.00% |
| rxjs | 586 | 6594 | 100.00% | 85506 | 100.00% | 0.00% | 0.00% |

## End-line finding (register slot)

Symbols with no end line, by the reader's dialect name, over the measured files:

| Reader dialect | Files | Symbols | Without end line |
|---|---|---|---|
| go | 1386 | 25766 | 0 |
| javascript | 1508 | 22805 | 0 |
| python | 1314 | 14630 | 0 |
| typescript | 1224 | 20367 | 0 |

No dialect on this corpus has a symbol without an end line, and no measured file was `Incomplete`. This finding
covers these four dialects only. Dialects outside the corpus (line scanners, generic readers, config formats)
were not read here, so the register slot stays open for them.

## What it means for a consumer

- On Python, JavaScript and TypeScript, `Enclosing` is `found` for every line. This is structural: those
  dialects emit a file-spanning module symbol (for example `PythonModule <module>`), so no line is ever outside
  every span. `found` there often means the module, not a function or class. A consumer that wants the innermost
  declaration should check the returned symbol's kind, or `Touched`, rather than treat `found` as "inside a
  function".
- On Go, 89.13% of lines are `found` and 10.87% are `none`. `none` is package clause, imports and top-level
  whitespace and comments outside any declaration. It is a trustworthy "not inside a symbol", because every Go
  symbol has a known span and no read was incomplete.
- `unknown` was 0% everywhere. On this corpus the CH-D3 degradation paths (missing end line, incomplete read)
  never fired, so the disclosure behavior is proven by the unit and property tests, not exercised by real files.
- Span lookups work for a consumer on these four dialects with a 100% known-span share. Whether they work on
  any other dialect is unmeasured here. A consumer must still handle `unknown`.
- Not measured: whether the innermost symbol is the declaration a human would name, or span accuracy against an
  oracle. Known span means present and well ordered, not correct.
