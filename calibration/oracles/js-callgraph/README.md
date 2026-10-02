# js-callgraph — a tier-B JavaScript call-graph oracle, measured and not a grader

LB-F204, O264, O481 (wave lg). `@persper/js-callgraph` 1.3.2 (npm `7abc6bb04fed9d82640716cc410db21fe1e9eb00`,
https://github.com/Persper/js-callgraph, EPL-2.0) is a field-based approximate call graph over an Esprima 4
parse: independent of fathom's grammar and of scip-typescript's type checker. It is used only as an unmodified
external behaviour oracle (run as a process, its output converted; never copied into the tree, never linked).

    npm install --prefix <tools>/jscg @persper/js-callgraph@1.3.2        # node v26.10.0, npm 11.19.1
    run.sh <tools> <subject-dir> out/jscg-<subject>.json test examples sandbox   # directories to skip, subject-relative

`run.sh` runs the tool offline over every `.js .cjs .mjs` file of the subject (node_modules, dist, build,
coverage and the named directories skipped) and `convert.js` writes the neutral tool-oracle JSON: tier B, one
`{path, from, to}` per edge whose caller is a named function and whose callee the tool resolved to a named
function of the subject (the tool's `global` module code, anonymous callers and `Native` targets are dropped).
It carries no `source` field: that marks a third-party query (tier C, ADR-048 D5), and this is a tool run.

Beside a SCIP index, the calls are taken with `lang-oracle -calls-from tool` (the index still grades
definitions, imports and typed edges), tier recorded B:

    indexbench lang-oracle -dialect javascript -subject <clone> -scip <index> -scip-defs \
        -calls-from tool -tool-oracle out/jscg-axios.json -out <reading>

Measured 2026-10-02 on axios cdcfd214 + express df0abc93 (lib/ and bin/; test, examples, sandbox skipped): 183 and
141 in-scope pairs; fathom's call_recall .608 (197 of 324) and call_precision .2472 (197 of 797), unmet at
tier B. It does not grade fathom's calls, so the committed reading does not use it:

- it names a callee by property name across every receiver, so `disable` calls `header` and `enable` calls
  `header` in express's application.js, pairs no run of the code makes;
- it writes no pair for a builtin, a library or a constructor (`Error`, `apply`, `new Map`), which fathom records;
- Esprima 4 fails on object spread and other ES2018+ syntax (webpack's `lib/Compiler.js`: "Unexpected token ..."),
  after which the tool exits on a stack overflow, so a modern typed subject gets no oracle at all.

The typed subject (webpack, `calibration/oracles/scip-typescript/README.md`) is the grader that can judge
JavaScript; this oracle stays so the next candidate (a tier-B tool that parses current syntax) has a runner and a
converter to replace, and so the choice of calls source beside `-scip` has a measured example.
