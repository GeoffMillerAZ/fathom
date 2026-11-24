HLP-2 verdict: FAIL (python, typescript)

# HLP-2: file outline size on the outline corpus

Measured at `88bd0a216` (the reader and `pkg/lang/view` at that commit). The measuring test is
`TestOutlineCorpusReading` in `pkg/lang/view/outline_corpus_test.go`. Toolchain `go1.27.4`, `darwin/arm64`.

Aggregate SHA-256 of the printed JSON: `6f2601ae5d62b511964b63e5de29c6ab070da2eb2266fb08757cf9d7bdf22a94`
(two runs gave the same digest).

The bar: the median of outline bytes divided by file bytes is at or under 20 percent for each of the four
dialects. Go and JavaScript meet it. Python and TypeScript do not, so each of those is a FAIL for that dialect and
the row is a FAIL. The bar was fixed before the reading and does not move (ESCALATE-28 R7). It is reported as measured and was not changed.

This measures size, not whether an agent does better with an outline (CH-R3). Whether an outline helps an agent
belongs to the bench program, and nothing here claims it.

The HLP-2 proof tests (`go test -race -count=1 ./pkg/lang/view ./cmd/fathom-lang`: byte-equal output for equal and
shuffled input, every string from the Result, nesting and `MaxNodes` tables, goldens for both formats, the schema
tests, the API snapshot) pass. They are not what fails: the verdict is the size bar alone.

## Rerun

```sh
FATHOM_LP8_CORPUS=<manifest.json> go test -race -count=1 -run TestOutlineCorpusReading -v ./pkg/lang/view
```

The manifest, its pins and its selection checks are those of the HLP-1 reading, and the test uses the same loader
(`corpus_helpers_test.go`, shared with `TestCorpusReading` and `TestRowsCorpusReading`, whose published digests
reproduce through it). The test fails unless each checkout's HEAD equals its pin, the tree is clean, the selected
file count equals the frozen count and, where the manifest freezes one, the selection digest matches. It skips when
the variable is unset, so the gate never runs it.

## What is measured

The corpus is the one of the HLP-1 reading: the COV-0 development pools for Python, JavaScript and TypeScript and
the three Go subjects of COV-M3. A measured file is a selected file whose read has at least one symbol. Per file,
the ratio is the byte length of `view.Outline(path, result, OutlineOptions{}).Text()` (no `MaxNodes`) divided by the
byte length of the file. Dialect is the manifest's dialect for the subject.

The median of an even count is the mean of the two middle ratios. The 90th percentile is the nearest rank,
the value at position ceil(0.9 n) of the sorted ratios. A measured file of zero bytes has no ratio: it is counted
in `files` and in `empty files`, and it is left out of every ratio statistic. Python has 98 of them (files of no bytes that
still read with a symbol) and TypeScript has one. Counting them as 0 percent would not change a verdict:
1009 of Python's 1314 files and 754 of TypeScript's 1224 are over 20 percent, which is more than half of each.

## Per dialect

| Dialect | Files | Empty files | Median | 90th percentile | Max | Files over 20% | Bar |
|---|---|---|---|---|---|---|---|
| go | 1386 | 0 | 18.50% | 36.77% | 149.64% | 619 | PASS |
| javascript | 1508 | 0 | 13.14% | 33.43% | 175.00% | 426 | PASS |
| python | 1314 | 98 | 58.31% | 123.96% | 407.84% | 1009 | FAIL |
| typescript | 1224 | 1 | 24.17% | 75.19% | 454.72% | 754 | FAIL |

The text form is one line per node: a Go Signature carries raw newlines and tabs (80 fields in 48 development corpus
Go files: 13 files in cli, 35 in prometheus), and `Text` writes each run of whitespace that holds a newline or CR as one
space (JSON and node strings are unchanged). The reading fails if any file's text has other than nodes plus two
lines, and none did. That rule moved only Go: outline bytes 1999254 to 1998904, median 18.4997 to 18.4981. The
other dialects, every verdict and the per-subject rows are unchanged.

All outlines were complete reads (no incomplete file) and no node was omitted.

Total bytes, for context only (the bar is on the per-file median):

| Dialect | File bytes | Outline bytes | Outline over file |
|---|---|---|---|
| go | 15014614 | 1998904 | 13.31% |
| javascript | 15292844 | 1476200 | 9.65% |
| python | 4530302 | 934252 | 20.62% |
| typescript | 7543238 | 1312569 | 17.40% |

## By file size (description, no bar)

Added by ESCALATE-28 R7 to tell a consumer when an outline pays. It sets no bar and changes no verdict. File size is
the file's line count (newlines, plus one for an unterminated last line). Files is the count with a ratio. Median is
the median of per-file ratios, pooled is outline bytes over file bytes summed within the bucket.

| Dialect | Lines | Files | Median | Pooled |
|---|---|---|---|---|
| go | 1-49 | 193 | 27.16% | 28.92% |
| go | 50-199 | 579 | 20.82% | 20.95% |
| go | 200-999 | 517 | 13.23% | 13.96% |
| go | 1000 and over | 97 | 8.70% | 10.10% |
| javascript | 1-49 | 282 | 33.01% | 30.84% |
| javascript | 50-199 | 619 | 14.66% | 14.33% |
| javascript | 200-999 | 524 | 9.36% | 9.46% |
| javascript | 1000 and over | 83 | 7.33% | 7.66% |
| python | 1-49 | 796 | 83.65% | 68.00% |
| python | 50-199 | 279 | 26.58% | 23.09% |
| python | 200-999 | 120 | 14.40% | 17.89% |
| python | 1000 and over | 21 | 12.45% | 10.27% |
| typescript | 1-49 | 475 | 46.68% | 40.64% |
| typescript | 50-199 | 457 | 18.72% | 19.75% |
| typescript | 200-999 | 251 | 17.86% | 18.14% |
| typescript | 1000 and over | 40 | 11.65% | 10.92% |

The ratio falls as files grow in every dialect. Below 50 lines the outline is a quarter to four fifths of the file.

Fixed bytes, the header line and the footer line of the text form per file (the header carries the file path, so it
varies with path length):

| Dialect | Median | Range |
|---|---|---|
| go | 107 | 86 to 163 |
| javascript | 116 | 95 to 158 |
| python | 124 | 93 to 169 |
| typescript | 118 | 96 to 178 |

## Per subject

| Subject | Commit | Selected | Files | Empty files | Median | 90th percentile |
|---|---|---|---|---|---|---|
| cli | `45ffa3c668f7` | 737 | 737 | 0 | 19.08% | 43.67% |
| cobra | `40b5bc1437a5` | 36 | 36 | 0 | 16.02% | 24.40% |
| prometheus | `9ec59baffb54` | 618 | 613 | 0 | 18.12% | 34.25% |
| eslint | `a438ec39512c` | 389 | 389 | 0 | 9.45% | 20.23% |
| openlayers | `88fc44564552` | 373 | 373 | 0 | 11.91% | 37.12% |
| webpack | `5a106753839f` | 746 | 746 | 0 | 17.09% | 35.04% |
| attrs | `c8bb9bfaa284` | 56 | 56 | 0 | 35.78% | 163.16% |
| fastapi | `864b569cf845` | 1175 | 1175 | 95 | 61.85% | 123.08% |
| flask | `22d924701a6a` | 83 | 83 | 3 | 38.77% | 182.69% |
| fp-ts | `669cd3ed7cb5` | 204 | 204 | 0 | 23.53% | 41.59% |
| headlessui | `d13526d02a2d` | 201 | 201 | 0 | 36.61% | 127.54% |
| react-bootstrap | `8b1cb721001c` | 233 | 233 | 0 | 23.22% | 52.16% |
| rxjs | `72bc92191ab9` | 586 | 586 | 1 | 22.37% | 69.40% |
