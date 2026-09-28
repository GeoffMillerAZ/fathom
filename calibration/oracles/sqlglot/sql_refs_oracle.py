#!/usr/bin/env python3
"""Tier-C SQL call-pair oracle: foreign-key references by sqlglot.

Owner decision D5 (docs/design/language-breadth.md §1.5): DerekStride's
tree-sitter-sql ships no tags.scm, so SQL's L2 truth comes from an
independent parser library (design §1.3, tier C): sqlglot parses every
CREATE TABLE and emits, as the neutral oracle file's `calls`,
  (column, referenced table) for an inline `col ... REFERENCES t(...)`,
  (table,  referenced table) for a table-level `[CONSTRAINT c] FOREIGN KEY (...) REFERENCES t`,
names bare (schema and quotes dropped). A statement sqlglot cannot parse is
skipped and counted on stderr, never guessed.

usage: sql_refs_oracle.py [--dialect GLOB=DIALECT ...] [--default postgres] <subject> [files...]
  files: subject-relative paths (default: every *.sql under the subject)
"""
import argparse, fnmatch, json, os, sys

import sqlglot
from sqlglot import exp


def bare(name):
    return name.split(".")[-1].strip('"`[]')


def pairs_for(sql, dialect):
    out, failed = [], 0
    for chunk in sqlglot.parse(sql, read=dialect, error_level=sqlglot.ErrorLevel.IGNORE):
        if chunk is None or not isinstance(chunk, exp.Create) or (chunk.args.get("kind") or "").upper() != "TABLE":
            continue
        schema = chunk.this
        if not isinstance(schema, exp.Schema):
            continue
        table = bare(schema.this.name if isinstance(schema.this, exp.Table) else schema.this.sql())
        for e in schema.expressions:
            if isinstance(e, exp.ColumnDef):
                for c in e.constraints:
                    ref = c.kind if isinstance(c, exp.ColumnConstraint) else None
                    if isinstance(ref, exp.Reference):
                        out.append((bare(e.name), bare(ref.this.this.name if isinstance(ref.this, exp.Schema) else ref.this.name)))
            else:
                fks = [e] if isinstance(e, exp.ForeignKey) else list(e.find_all(exp.ForeignKey))
                for fk in fks:
                    ref = fk.args.get("reference")
                    if isinstance(ref, exp.Reference):
                        t = ref.this.this if isinstance(ref.this, exp.Schema) else ref.this
                        out.append((table, bare(t.name)))
    return out, failed


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dialect", action="append", default=[])
    ap.add_argument("--default", default="postgres")
    ap.add_argument("subject")
    ap.add_argument("files", nargs="*")
    a = ap.parse_args()
    rules = [r.split("=", 1) for r in a.dialect]
    files = a.files or sorted(
        os.path.relpath(os.path.join(d, f), a.subject).replace(os.sep, "/")
        for d, _, fs in os.walk(a.subject) for f in fs if f.endswith(".sql"))
    calls = []
    for rel in files:
        dialect = next((d for g, d in rules if fnmatch.fnmatch(rel, g)), a.default)
        with open(os.path.join(a.subject, rel), encoding="utf-8", errors="replace") as f:
            sql = f.read()
        try:
            pairs, _ = pairs_for(sql, dialect)
        except Exception as e:  # a file sqlglot rejects whole
            print(f"skip {rel}: {e}", file=sys.stderr)
            continue
        for frm, to in sorted(set(pairs)):
            calls.append({"path": rel, "from": frm, "to": to})
    json.dump({"tool": "sqlglot", "version": sqlglot.__version__, "tier": "C", "calls": calls}, sys.stdout, indent=1)
    print()


if __name__ == "__main__":
    main()
