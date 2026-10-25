# miss_hit_dump.py — tier-B MATLAB L1 oracle (syntax only)

ADR-048 amendment D2: universal-ctags 6.2.1 is deficient as a MATLAB L1 oracle. It has no method kind and no
property kind (a method is an unscoped `function`; a property is a `variable` among every assignment of every
function, 45,154 of 49,920 tags on chebfun v5.7.0 and export_fig v3.58), it misses 12 classes whose `classdef` line
carries `(InferiorClasses = {?class})`, and it writes no end line. MISS_HIT (`miss-hit-core` 0.9.44, GPL-3, PyPI
2024-09-02), a MATLAB parser written in Python that shares no code with tree-sitter, reports all four kinds and the
ranges.

`miss_hit_dump.py` is an original driver that imports only `miss_hit_core`'s parser classes (`MATLAB_Lexer`,
`Token_Buffer`, `MATLAB_Parser`, `AST_Visitor`, `Message_Handler`, `Config`); none of MISS_HIT's own command-line tools
is imported or copied. Bench-only: a Python process run beside the harness from a scratch venv, never a product
dependency. Tier B on the D2 precedent (luaparse, php-parser, pglast): an independent syntactic parser, so L1 can be
excellent and L2 at most good. The tier is a claim for Opus review, not a harness fact.

    python3 -m venv <tools>/mh && <tools>/mh/bin/pip install miss-hit-core==0.9.44
    <tools>/mh/bin/python -I miss_hit_dump.py <clone> --out out/matlab-defs.json --only-list out/matlab-only.txt
    indexbench lang-oracle -dialect matlab -subject <clone> -tool-oracle out/matlab-defs.json \
        -only "$(cat out/matlab-only.txt)" -out docs/bench/readings/lang/matlab.json

Run it with `python -I`. It prints the files, parsed files and definitions to stderr and writes
`{tool,version,tier,definitions,disclose}`; `disclose` counts what it could not read. A file MISS_HIT rejects (a
`Error` or `ICE`) gets no definitions and is left out of the measured scope with `-only` (the reading's notes name the
count); the restriction is allowed only while the rejected files are at most 5% of a subject's `.m` files.

What it counts (the parser is independent; the scope is the dialect author's rule, stated here):

- `class`: the `classdef` line; `end` is the line of the class's closing `end` keyword.
- `function`: a `Function_Definition` that is not in a `methods` block (main, local, nested, `@Class` and `+pkg` files);
  `line` is the `function` keyword's line, `end` the closing `end` keyword's line, 0 for a function with no `end`.
- `method`: a `Function_Definition` in a `methods` block (`end` as for a function), or a `Function_Signature` there
  (a method declared without a body: `end` 0).
- `property`: each `Entity_Constraints` item of a class's `properties` blocks (`end` 0). An `arguments` block is never a
  property. Events and enumeration members are not reported.
- Names are bare: the last `.` segment (a getter `get.Secret` is `Secret`).
- Files that are not UTF-8 are decoded with replacement characters, so line numbers stay true; the count is disclosed.
- Octave's `__parse_file__` dumps no AST and is not used.
