#!/usr/bin/env python3
"""Native SQL inline-FK oracle; original statements, catalog truth, no daemon.

See README.md for pins, scope, execution order, coverage and replay commands.
"""
import argparse
import hashlib
import json
from pathlib import Path
import sqlite3
import subprocess
import sys


def code_line(text, start, piece):
    from pglast import parser
    tokens = [t for t in parser.scan(piece) if t.name not in ('SQL_COMMENT', 'C_COMMENT')]
    offset = tokens[0].start if tokens else 0
    return text.count('\n', 0, start + offset) + 1


def sqlite_statements(text):
    start = 0
    for i, ch in enumerate(text):
        if ch == ';' and sqlite3.complete_statement(text[start:i + 1]):
            yield code_line(text, start, text[start:i + 1]), text[start:i + 1]
            start = i + 1
    if text[start:]:
        yield code_line(text, start, text[start:]), text[start:]


def pg_statements(text):
    from pglast import parser
    for span in parser.split(text, with_parser=False, only_slices=True):
        yield code_line(text, span.start, text[span.start:span.stop]), text[span.start:span.stop]


def statement_kind(text):
    from pglast import parser
    # The native scanner classifies errors; it never supplies a FK target.
    tokens = [t.name for t in parser.scan(text) if t.name not in ('SQL_COMMENT', 'C_COMMENT')]
    if not tokens:
        return 'empty'
    if tokens[0] == 'CREATE' and 'TABLE' in tokens[1:4]:
        return 'create_table'
    return tokens[0].lower()


def receipt(path, line, text, kind):
    return {'path': path, 'line': line, 'sha256': hashlib.sha256(text.encode()).hexdigest(), 'kind': kind}


def sqlite_file(path, text):
    db = sqlite3.connect(':memory:')
    db.enable_load_extension(False)
    created = []

    def authorize(action, a, b, _db, _trigger):
        if action in (sqlite3.SQLITE_ATTACH, sqlite3.SQLITE_DETACH):
            return sqlite3.SQLITE_DENY
        if action == sqlite3.SQLITE_PRAGMA and a.lower() not in ('foreign_keys', 'foreign_key_list'):
            return sqlite3.SQLITE_DENY
        if action in (sqlite3.SQLITE_CREATE_TABLE, sqlite3.SQLITE_CREATE_TEMP_TABLE):
            created.append((a, _db))
        return sqlite3.SQLITE_OK

    db.set_authorizer(authorize)
    calls, errors, receipts = set(), [], []
    failed = 0
    try:
        for line, piece in sqlite_statements(text):
            kind = statement_kind(piece)
            if kind == 'empty':
                continue
            event = receipt(path, line, piece, kind)
            created.clear()
            try:
                db.execute(piece)
            except sqlite3.Error as exc:
                failed += kind == 'create_table'
                error = {'path': path, 'line': line, 'kind': kind, 'code': exc.sqlite_errorname,
                         'message': str(exc)}
                errors.append(error)
                event['error'] = error['code']
            else:
                event['accepted'] = True
                for table, schema in created:
                    # Parameterized pragma table-valued function: names are never SQL text.
                    for (target,) in db.execute('SELECT "table" FROM pragma_foreign_key_list(?, ?)', (table, schema)):
                        calls.add((path, table, target))
            receipts.append(event)
    finally:
        db.close()
    return {'calls': [{'path': p, 'from': f, 'to': t} for p, f, t in sorted(calls)],
            'errors': errors, 'create_table_failed': failed, 'statements': len(receipts), 'receipts': receipts, 'source_sha256': hashlib.sha256(text.encode()).hexdigest(), 'source_bytes': len(text.encode())}


def read_sql(path):
    # TextIO's default newline handling would rewrite CRLF input.
    return path.read_bytes().decode('utf-8')


def input_file(root, rel, engine):
    p = root / rel
    if p.stat().st_size > 2 * 1024 * 1024 or not p.resolve().is_relative_to(root.resolve()):
        raise ValueError('file outside scope or over 2 MiB: ' + rel)
    text = read_sql(p)
    statements = []
    for line, piece in pg_statements(text):
        kind = statement_kind(piece)
        if kind != 'empty':
            statements.append(dict(receipt(rel, line, piece, kind), sql=piece))
    return {'path': rel, 'engine': engine, 'statements': statements, 'source_sha256': hashlib.sha256(p.read_bytes()).hexdigest(), 'source_bytes': p.stat().st_size}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--subject', required=True)
    ap.add_argument('--item', choices=['sqitch', 'pagila'], required=True)
    ap.add_argument('--pglite-root', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    root = Path(args.subject)
    if args.item == 'pagila':
        scenarios = [['pagila-schema.sql', 'pagila-temporal-setup.sql'], ['pagila-schema-jsonb.sql']]
        sqlite_paths = []
    else:
        scenarios = [[
            't/engine/deploy/users.sql', 't/engine/deploy/widgets.sql',
            't/engine/deploy/func/add_user.sql', 't/sql/verify/users.sql',
            't/engine/revert/func/add_user.sql', 't/engine/revert/widgets.sql',
            't/engine/revert/users.sql', 't/engine/reworked/deploy/users@alpha.sql',
            't/engine/reworked/revert/users@alpha.sql',
        ]]
        pg_paths = set(scenarios[0])
        scoped = set(['lib/App/Sqitch/Engine/sqlite.sql', 't/lib/upgradable_registries/sqlite.sql'])
        for pattern in ['lib/App/Sqitch/Engine/Upgrade/sqlite-*.sql', 't/engine/**/*.sql', 't/sql/**/*.sql']:
            scoped.update(p.relative_to(root).as_posix() for p in root.glob(pattern))
        if not pg_paths <= scoped:
            raise ValueError('missing declared PostgreSQL fixture')
        sqlite_paths = sorted(scoped - pg_paths)
    requests = [[input_file(root, p, 'postgresql') for p in scenario] for scenario in scenarios]
    helper = Path(__file__).with_name('pg_catalog.mjs')
    run = subprocess.run(['node', str(helper), str(Path(args.pglite_root).resolve())],
                         input=json.dumps(requests), text=True, capture_output=True, timeout=180, check=True)
    pg = json.loads(run.stdout)
    results = list(pg['files'])
    for rel in sqlite_paths:
        results.append(dict(sqlite_file(rel, read_sql(root / rel)), path=rel, engine='sqlite'))
    results.sort(key=lambda r: r['path'])
    if any(r['create_table_failed'] for r in results):
        # Never promote a subset by silently ignoring rejected CREATE TABLEs.
        Path(args.out + '.refused.json').write_text(json.dumps(results, indent=2, sort_keys=True) + '\n')
        raise ValueError('native CREATE TABLE rejected; no tier-A oracle emitted')
    pairs = sorted({(c['path'], c['from'], c['to']) for r in results for c in r['calls']})
    errors = sum(len(r['errors']) for r in results)
    out = {'tool': 'sql-native-catalog', 'version': f"SQLite {sqlite3.sqlite_version}; PGlite {pg['pglite']} PostgreSQL {pg['postgresql']}; pgvector {pg['pgvector']}",
           'tier': 'A', 'calls': [{'path': p, 'from': f, 'to': t} for p, f, t in pairs],
           'disclose': [f"native catalog: {len(results)} unchanged scoped files, {len(pairs)} distinct inline CREATE TABLE FK pairs; every CREATE TABLE accepted; {errors} rejected statements outside CREATE TABLE named in the companion report; ALTER TABLE and inherited-only keys excluded"],
           }
    Path(args.out + '.report.json').write_text(json.dumps({'files': results, 'calls': out['calls'], 'version': out['version']}, indent=2, sort_keys=True) + '\n')
    Path(args.out).write_text(json.dumps(out, indent=2, sort_keys=True) + '\n')
    print(f"{args.item}: {len(results)} files, {len(pairs)} inline FK pairs, {errors} named errors outside CREATE TABLE")


if __name__ == '__main__':
    main()
