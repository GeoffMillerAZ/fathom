#!/usr/bin/env python3
"""Replay the superseded whole-file SQLite probe; this is not the grading oracle."""
import argparse
import json
from pathlib import Path
import sqlite3
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--subject', required=True)
a = ap.parse_args()
root = Path(a.subject)
records = []
for p in sorted(root.rglob('*.sql')):
    rel = p.relative_to(root).as_posix()
    if not (rel in ('lib/App/Sqitch/Engine/sqlite.sql', 't/lib/upgradable_registries/sqlite.sql')
            or rel.startswith(('lib/App/Sqitch/Engine/Upgrade/sqlite-', 't/engine/', 't/sql/'))):
        continue
    db = sqlite3.connect(':memory:')
    try:
        db.executescript(p.read_bytes().decode())
        error = None
    except sqlite3.Error as e:
        error = str(e)
    pairs = sorted({(n, r[2]) for (n,) in db.execute("SELECT name FROM sqlite_master WHERE type='table'")
                    for r in db.execute('PRAGMA foreign_key_list(' + json.dumps(n) + ')')})
    records.append({'path': rel, 'executes': error is None, 'error': error, 'pairs': pairs})
    db.close()
print(json.dumps({'sqlite': sqlite3.sqlite_version, 'files': len(records),
                  'success': sum(r['executes'] for r in records), 'records': records}, indent=2))
