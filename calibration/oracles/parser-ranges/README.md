# parser-ranges — tier-B definition ranges (Bash, SQL)

universal-ctags 6.2.1 writes no end line for a Sh function or a SQL definition (`ctags --fields=+ne` on bats-core
713504bc and nvm 1889911f: 1,063 Sh functions, none with `end:`, only its 21 heredocs have one; on pagila 9baf49c4 and
sqitch ce0550c4: 1,152 SQL tags, none with `end:`), so the bash and sql readings' L1 records `range_agreement` as
not_measured and caps L1 at good (language-breadth §1.2). An independent parser that reports where a definition ends
measures it. Each script here writes the range companion `indexbench lang-oracle` reads beside a tool oracle: a sibling
named `<tool-oracle>.ranges.json` (`bash-bats-core.json` -> `bash-bats-core.ranges.json`).

| Script | Parser | Language | Definitions |
|---|---|---|---|
| `shfmt_ranges.py` | shfmt `--to-json` (mvdan.cc/sh/v3, BSD-3-Clause), `-ln bash` | Bash | each `syntax.FuncDecl`, nested ones included |
| `pglast_ranges.py` | pglast (libpg_query, the PostgreSQL server's parser; pglast is GPL-3.0-or-later, run as a tool, never linked) | SQL | each CREATE TABLE, VIEW, MATERIALIZED VIEW, FUNCTION, PROCEDURE and INDEX statement |

    GOBIN=<tools>/bin go install mvdan.cc/sh/v3/cmd/shfmt@v3.14.1
    python3 -I shfmt_ranges.py --shfmt <tools>/bin/shfmt --ext .sh --ext .bash --shebang bash --shebang sh <clone> > out/bash-<subject>.ranges.json

    python3 -m venv <tools>/pyenv && <tools>/pyenv/bin/pip install pglast==8.5
    <tools>/pyenv/bin/python -I pglast_ranges.py <clone> [file ...] > out/sql-<subject>.ranges.json

Both are tier B: an independent parser's syntax tree, not the language's evaluator. Output:
`{tool, tier, version, units, errored, unparsed, kinds, scope, ranges: [{path, name, kind, line, end}]}`, sorted.
`kinds` maps the script's kind to the manifest's canonical kind (SQL's `index` is `key`), and `scope` says which
definitions the ranges cover; the harness quotes it in the reading's note.

Facts each script relies on:

- shfmt: a node's `End` is the position after its last byte, so a declaration whose `End` sits at column 1 of a later
  line ends on the line before. A file is selected as the bash manifest selects it: a listed extension, or no
  extension and a `#!` line naming a listed interpreter (env's first non-flag argument, a version suffix allowed).
  A file shfmt rejects is named in `unparsed` and counted in `errored`.
- pglast: `stmt_location` and `stmt_len` are character offsets of the statement, its terminating semicolon excluded
  (`stmt_len` 0 means the rest of the input). The start line is that of the first token PostgreSQL's own scanner
  (`pglast.parser.scan`) reads that is not a comment; the last line that of the statement's last non-blank character.
  A file the parser rejects whole (another engine's syntax: SQLite's `AUTOINCREMENT`, a `PRAGMA`) is split by the
  scanner alone (`pglast.parser.split(..., with_parser=False)`) and each statement parsed by itself; a statement that
  still does not parse is counted per file in `unparsed` and is not scored. A column definition has a start
  (`location`) but no end, so columns are not in the companion.

## How the harness reads it

`cmd/indexbench/langoracle_imports.go` (`scoreRangeCompanion`) pairs fathom's definitions with the companion's by
`langbench.ScoreL1`'s own rule (same file and name, |Δline| <= 2, nearest first) and takes only the range counts:
`range_agreement` is the share of paired definitions whose last line is within one line of the companion's. Recall,
precision and kind agreement stay the L1 oracle's (universal-ctags). A companion with no `kinds` and no `scope` is
`../clang-ast-defs`' (canonical kinds, its own note).
