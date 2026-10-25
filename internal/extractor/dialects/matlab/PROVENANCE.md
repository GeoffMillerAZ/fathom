# matlab — provenance

- Grammar: `acristoffers/tree-sitter-matlab v1.3.1` (`f03d0347acd8bb05d4edd8c845ac1718729e1fad`, MIT, licence checked at the pin: the
  top-level `LICENSE` only, `queries/` and `src/` carry none), ABI 15, external scanner, built into
  `internal/extractor/tsengine/grammars/matlab.wasm.gz` (482,912 bytes raw, 121,531 gzipped, 433 peak module pages on chebfun); pin and
  digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`. `src/parser.c` (3.2 MB) ships at the tag, so the lock says
  `"generate": false`.
- Parse coverage (every `.m` file of both calibration subjects, 3,180): 8 have an ERROR node (0.25%, all in chebfun v5.7.0:
  `@chebfun3/get.m`, `@chebfun3/times.m`, `@chebfun3t/get.m`, `@spherefun/get.m`, `@spinoperator/subsref.m`,
  `tests/chebop/test_cellOperator.m`, `tests/chebop/test_linearize_init_fails.m`, `tests/diskfun/test_diag.m`), far under design section
  14.2's 5% line. A file with an error still yields the definitions the tree recovers.
- `tags.scm`: starts from the grammar's own `queries/neovim/tags.scm` (MIT; header on the file), which tags a class, every function and
  every call. This one tags `classdef`, each `property` of any `properties` block, a `function` in a `methods` block as a method, a
  `function` at file level or nested in another function's block as a function, and a `methods` signature with no body as a method (an
  abstract method, or one whose implementation lives in its own file of an `@Class` folder: chebfun declares 547 that way).
  - No call is captured: `f(x)` is a call and an index in the same syntax, so a tags query cannot tell them apart; calls belong to L2.
  - A callable's signature runs from its start to the start of `@body`, so `@body` must be the first node after the header: a leading help
    comment, else the block, else nothing (the signature is then its first line). An anchor `.` after an optional argument list is not
    exclusive when the list is absent (a function without parentheses matched once per leading comment and once more for its block; found
    on export_fig's `ghostscript.m`, locked in by `testdata/funcs.m`), so every function context spells out the six header shapes —
    arguments or none, then comment, block or neither — each anchored between nodes that are there.
  - A getter or setter (`function r = get.Secret(obj)`) is named by its property, `Account.Secret`, because the grammar keeps `get.` as an
    anonymous token no capture can join to the name; its signature carries the prefix.
  - Events and enumeration members are not extracted (ctags has no kind for either).
  - A function with no `end` keyword ends where the next function starts, and the grammar's node for it runs through the blank lines
    before that function, so its `EndLine` can over-read by those lines; `testdata/noend.m` keeps its functions adjacent so its golden reads
    the true range. ctags writes no end line for MATLAB, so `range_agreement` is not measured either way.
  - Package: the directory rule (the file stem); a `+pkg` or `@Class` folder is not a package here.
- `match.sniff`: `.m` is claimed by no extension (Objective-C shares it, and `unsupported_language.go` never lists it). The pattern
  `^\s*(function|classdef)\b|^\s*%` over the first 1 KiB, priority 50, leaves room below it for Objective-C's own sniff (design section
  5 orders `@interface|@implementation|#import` first). On the calibration subjects it claims 3,179 of 3,180 files; the one it leaves
  unclassified is chebfun's `ODEformats.m`, a script that opens with `set(0, ...)`. An Objective-C `.m` matches none of it and is counted as
  `ambiguous_unclassified` (`internal/fusion/matlab_ingest_test.go`, `TestIngest_MatlabSniffLeavesObjectiveCUnclaimed`).
- `testdata/`: six fixtures written in this repository; goldens written by hand from each fixture before any query existed
  (`TestDialectGoldens/matlab` red), then the queries were written against `indexbench lang-probe`.

## L1 oracle: a MISS_HIT parser dump (LB-Q-matlab, LB-Q-matlab-rest)

universal-ctags 6.2.1 cannot measure the four required kinds: its MatLab parser has class, function and variable only (a method is an
unscoped function; a property is a `variable` among 45,154 assignment tags; it misses 12 classes whose `classdef` line carries
`(InferiorClasses = {?class})`, and writes no end line). The oracle is therefore `calibration/oracles/miss-hit/miss_hit_dump.py`, an original
driver over `miss-hit-core` 0.9.44's parser (tier B on the design 1.5 D2 precedent: an independent parser, as luaparse, php-parser and pglast),
`oracle_kinds.tool` mapping class, function, method and property one to one. Subjects: export_fig v3.58 (18 files), chebfun v5.7.0 (3,162 `.m`
files, 3,156 scored) and matlab2tikz v1.1.0 (64 files; BSD-2-Clause).

Reading (`docs/bench/readings/lang/matlab.json`, 3,238 files measured): sym_recall .9898, sym_precision 1, kind_agreement .9899, range_agreement
.9998 (5,235 of 5,236 pairs with an oracle end), signature_coverage 1, 0 faults, max_file_ms 338, 490 peak module pages. MISS_HIT rejects 5
chebfun files (0.16%) with a parse error; they are left out with `-only` and counted in the reading's notes. Throughput is the one bar not met:
0.94 MB/CPU-s against 1.0 on the VM that took the reading (see the `LB-Q-matlab-rest` row).

Scope choices the oracle and the dialect share: a getter or setter is the property's name on both sides; an `arguments` block is not a property;
events and enumeration members are in neither; a function with no `end` has oracle end 0 and is not range-compared.

## Engine changes this dialect required (LB-Q-matlab)

MATLAB is the first dialect to claim an extension only through `match.sniff`:

- `internal/fusion` registered only extensions, basenames and variants, so a sniffed file reached no extractor even when
  `connector.ClassifyDialect` classified it. `Pipeline.sniffExts` (dialect id to extractor, built in `New` for a manifest with a sniff
  and closed in `Close`) and `sniffedExtractor` (called last in `resolveExtractor` and in `ReingestFile`, so ingest, the one-file refresh
  and the bench dispatch agree) route a file the sniff settled to its dialect; a file no sniff settles is counted, as before.
- `internal/langbench.Manifest.Owns` ignored `match.sniff`, so `lang-oracle` measured zero files for it; a sniff extension now makes a
  file a candidate, and ingest's own gate skips (and the reading's notes disclose) the ones no sniff settles.
- `connector.LanguageForPath` still answers `unclassified` for a `.m` file: it reads the path alone, and a sniff needs content.
