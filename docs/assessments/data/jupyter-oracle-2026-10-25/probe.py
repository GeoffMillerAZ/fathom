#!/usr/bin/env python3
"""Compare native basedpyright notebook checking and scip-python batch discovery."""
import argparse
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--basedpyright', required=True)
ap.add_argument('--scip-python', required=True)
ap.add_argument('--out', required=True)
a = ap.parse_args()
here = Path(__file__).resolve().parent
out = Path(a.out).resolve(); out.mkdir(parents=True, exist_ok=False)
result = {'python': sys.version.split()[0], 'runs': {}}
for kind, source in [('notebook', 'probe.ipynb'), ('python', 'control.py')]:
    root = out / kind; root.mkdir()
    shutil.copyfile(here / source, root / source)
    checker = subprocess.run([str(Path(a.basedpyright).resolve()), '--outputjson', source], cwd=root,
                             capture_output=True, text=True, timeout=120)
    assert checker.returncode in (0, 1), checker.stderr
    diagnostics = json.loads(checker.stdout)
    diagnostics.pop('time', None)
    for row in diagnostics.get('generalDiagnostics', []):
        row['file'] = row['file'].replace(str(root), '<probe>')
    (out / (kind + '-basedpyright.json')).write_text(json.dumps(diagnostics, indent=2, sort_keys=True) + '\n')
    indexer = subprocess.run([str(Path(a.scip_python).resolve()), 'index', '.', '--project-name', 'notebook-oracle-probe',
                             '--project-version', '0', '--environment', str(here / 'environment.json'),
                             '--output', str(root / 'index.scip')], cwd=root,
                            capture_output=True, text=True, timeout=120)
    assert indexer.returncode == 0, indexer.stderr
    # ANSI escapes are irrelevant to the semantic discovery count.
    output = re.sub(r'\x1b\[[0-9;]*m', '', indexer.stdout + indexer.stderr)
    count = re.search(r'Total Project Files\s*:?\s*(\d+)', output)
    assert count, output
    result['runs'][kind] = {'scip_total_project_files': int(count[1]),
                           'basedpyright_version': diagnostics['version'],
                           'basedpyright_summary': diagnostics['summary'],
                           'basedpyright_keys': sorted(diagnostics),
                           'scip_command': 'scip-python index . --project-name notebook-oracle-probe --project-version 0 --environment environment.json --output index.scip'}
assert result['runs']['notebook']['scip_total_project_files'] == 0
assert result['runs']['python']['scip_total_project_files'] == 1
(out / 'summary.json').write_text(json.dumps(result, indent=2, sort_keys=True) + '\n')
print(json.dumps(result, sort_keys=True))
