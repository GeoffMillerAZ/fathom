#!/usr/bin/env python3
"""Reproduce SQL syntax-oracle blind spots; no subject source is printed."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import sqlfluff

ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--sqitch', required=True)
ap.add_argument('--pagila', required=True)
ap.add_argument('--out', required=True)
a = ap.parse_args()
repo = Path(__file__).resolve().parents[4]
results = {}
for item, root in [('sqitch', a.sqitch), ('pagila', a.pagila)]:
    run = subprocess.run([sys.executable, str(repo / 'calibration/oracles/sqlglot/sql_refs_oracle.py'), root],
                         capture_output=True, text=True, check=True)
    result = json.loads(run.stdout)
    results[item] = {'sqlglot_version': result['version'], 'pairs': result['calls'],
                     'command_fallback_warnings': run.stderr.count("Falling back to parsing as a 'Command'")}
text = (Path(a.pagila) / 'pagila-temporal-setup.sql').read_bytes().decode()
from sqlfluff.core import Linter
parsed = Linter(dialect='postgres').parse_string(text)
results['sqlfluff'] = {'version': sqlfluff.__version__, 'violation_codes': [v.rule_code() for v in parsed.violations],
                       'contract': 'syntax parse tree, not native evaluation or resolved FK catalog'}
Path(a.out).write_text(json.dumps(results, indent=2, sort_keys=True) + '\n')
print(json.dumps({k: len(v.get('pairs', [])) if k != 'sqlfluff' else v for k, v in results.items()}))
