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

## L1 is not proven: the oracle cannot report two of the four kinds (LB-Q-matlab-rest)

`required_kinds` is class, function, method and property: the L1 kinds the queue names. universal-ctags 6.2.1's MatLab parser reports
three kinds (`ctags --list-kinds-full=MatLab`: class, function, variable), and on the calibration subjects (3,179 files) it reports
45,154 variables, 4,705 functions and 61 classes:

- A method is a `function` with no scope and no method kind (chebfun has 1,090 method definitions the dialect emits).
- A property is a `variable`, only when it has a default value, among every assignment `x = ...` at any depth of any function or
  script (chebfun and export_fig: 45,154 of the 49,920 tags). Mapping `variable` to `property` would score a property's recall against
  every local variable of the repository; leaving it out puts `property` and `method` outside the kind intersection, which design
  section 1.4 refuses (`lang-oracle -dialect matlab` says `required kind "method" is outside the kind intersection`).
- ctags also misses 12 classes whose `classdef` line carries `(InferiorClasses = {?class})` (chebfun's `@adchebfun`, `@chebop`,
  `@domain` and nine more), names a signature's line continuation `...` as a function 21 times, and writes a getter as `get.name`,
  which the harness's last-segment rule never pairs with the dialect's `Class.name` (12 accessors in each direction).

`required_kinds` is left whole, so no reading is written. For the record, on an uncommitted scratch manifest that compares only classes and
functions (methods and properties outside the intersection, ctags `variable` unmapped; not a reading, no level claimed), 3,179 files
(chebfun v5.7.0 3,161, export_fig v3.58 18) score sym_recall .8758 (the methods with a body are ctags functions the scratch left
unmatched), sym_precision .9907, kind_agreement 1, signature_coverage 1 over 4,140 callables, 2.45 MB/CPU-s, 433 peak module pages and 0
faults; on export_fig alone recall and precision are 1.0 (95 of 95). With methods counted as functions chebfun's precision is .8904 (5,208
definitions, 4,637 matched): 547 declared-only method signatures ctags cannot see, and 24 definitions of the 12 classes it misses.

## Engine changes this dialect required (LB-Q-matlab)

MATLAB is the first dialect to claim an extension only through `match.sniff`:

- `internal/fusion` registered only extensions, basenames and variants, so a sniffed file reached no extractor even when
  `connector.ClassifyDialect` classified it. `Pipeline.sniffExts` (dialect id to extractor, built in `New` for a manifest with a sniff
  and closed in `Close`) and `sniffedExtractor` (called last in `resolveExtractor` and in `ReingestFile`, so ingest, the one-file refresh
  and the bench dispatch agree) route a file the sniff settled to its dialect; a file no sniff settles is counted, as before.
- `internal/langbench.Manifest.Owns` ignored `match.sniff`, so `lang-oracle` measured zero files for it; a sniff extension now makes a
  file a candidate, and ingest's own gate skips (and the reading's notes disclose) the ones no sniff settles.
- `connector.LanguageForPath` still answers `unclassified` for a `.m` file: it reads the path alone, and a sniff needs content.
