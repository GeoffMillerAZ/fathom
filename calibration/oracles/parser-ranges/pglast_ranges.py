#!/usr/bin/env python3
"""pglast_ranges.py: SQL definition ranges from PostgreSQL's own parser (tier B).

universal-ctags 6.2.1 writes no end line for a SQL definition, so the sql
reading's L1 caps range_agreement at not_measured. pglast binds libpg_query,
the PostgreSQL server's parser: each top-level statement it parses carries
stmt_location and stmt_len (where the statement's first token starts and how
long it is, the terminating semicolon excluded). This script writes each
defining statement's first and last line as the range companion
`indexbench lang-oracle` reads beside a tool oracle (<name>.ranges.json):

    CREATE TABLE (with or without PARTITION OF)  -> table     (relation name)
    CREATE VIEW, CREATE MATERIALIZED VIEW        -> view      (view name)
    CREATE FUNCTION, CREATE PROCEDURE            -> function  (last name part)
    CREATE INDEX                                 -> index     (index name)

    python3 -I pglast_ranges.py <clone> [file ...] > out/<tool-oracle>.ranges.json

With no files, every .sql file of the clone. A file the parser rejects whole is
split into statements by PostgreSQL's own scanner (pglast.parser.split without
the parser) and each statement parsed alone: a statement that does not parse
(another engine's syntax, a psql variable) is counted in `statements_errored`
and named per file in `unparsed`, never guessed. The start line is the first
token that is not a comment (the scanner's), the last line the statement's last
non-blank character's. Offsets are pglast's (characters).
"""

import json
import os
import sys

import pglast
from pglast import parser


def kind_and_name(stmt):
    t = type(stmt).__name__
    if t == "CreateStmt":
        return "table", stmt.relation.relname
    if t == "ViewStmt":
        return "view", stmt.view.relname
    if t == "CreateTableAsStmt" and getattr(stmt.objtype, "name", str(stmt.objtype)) == "OBJECT_MATVIEW":
        return "view", stmt.into.rel.relname
    if t == "CreateFunctionStmt":
        return "function", stmt.funcname[-1].sval
    if t == "IndexStmt" and stmt.idxname:
        return "index", stmt.idxname
    return None, None


def first_token(text, start, end):
    """The offset of the first non-comment token in text[start:end]."""
    for tok in parser.scan(text[start:end]):
        if tok.name not in ("SQL_COMMENT", "C_COMMENT"):
            return start + tok.start
    return start


def line_of(text, offset):
    return text.count("\n", 0, offset) + 1


def span(s, base, limit):
    """A RawStmt's [start, stop) in the file: stmt_len 0 means the rest of what was parsed."""
    start = base + s.stmt_location
    return start, (limit if s.stmt_len == 0 else start + s.stmt_len)


def statements(text):
    """(stmt, start, stop) for every statement, and the count of pieces that did not parse."""
    try:
        return [(s.stmt, *span(s, 0, len(text))) for s in pglast.parse_sql(text)], 0
    except pglast.parser.ParseError:
        pass
    out, bad = [], 0
    for sl in parser.split(text, with_parser=False, only_slices=True):
        piece = text[sl.start:sl.stop]
        try:
            parsed = pglast.parse_sql(piece)
        except pglast.parser.ParseError:
            bad += 1
            continue
        for s in parsed:
            out.append((s.stmt, *span(s, sl.start, sl.stop)))
    return out, bad


def main():
    root = sys.argv[1]
    files = sys.argv[2:]
    if not files:
        for dirpath, dirnames, filenames in os.walk(root):
            dirnames[:] = sorted(d for d in dirnames if d != ".git")
            for f in sorted(filenames):
                if f.lower().endswith(".sql"):
                    files.append(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    ranges, unparsed, errored = [], {}, 0
    for rel in sorted(files):
        with open(os.path.join(root, rel), encoding="utf-8", errors="replace") as fh:
            text = fh.read()
        stmts, bad = statements(text)
        if bad:
            unparsed[rel] = bad
            errored += bad
        for stmt, loc, stop in stmts:
            kind, name = kind_and_name(stmt)
            if kind is None:
                continue
            start = first_token(text, loc, stop)
            last = stop
            while last > start and text[last - 1].isspace():
                last -= 1
            ranges.append({"path": rel, "name": name, "kind": kind,
                           "line": line_of(text, start), "end": line_of(text, max(last - 1, start))})
    ranges.sort(key=lambda r: (r["path"], r["line"], r["name"]))
    major, minor = parser.get_postgresql_version()
    json.dump({
        "tool": "pglast (libpg_query, the PostgreSQL parser)",
        "tier": "B",
        "version": f"pglast {pglast.__version__}, PostgreSQL {major}.{minor} parser",
        "units": len(files),
        "errored": len(unparsed),
        "statements_errored": errored,
        "unparsed": unparsed,
        "kinds": {"table": "table", "view": "view", "function": "function", "index": "key"},
        "scope": "the first and last line of each CREATE TABLE, VIEW, MATERIALIZED VIEW, FUNCTION, PROCEDURE and INDEX statement "
                 "(stmt_location and stmt_len, the semicolon excluded; a column has no end and is not scored); "
                 f"{errored} statements in {len(unparsed)} files did not parse alone and are not scored",
        "ranges": ranges,
    }, sys.stdout, indent=1, sort_keys=True)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
