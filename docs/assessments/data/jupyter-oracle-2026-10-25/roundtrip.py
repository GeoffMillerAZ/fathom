#!/usr/bin/env python3
"""Check every nontranslation notebook with jupytext --test --to py:percent."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import warnings
import nbformat
import jupytext
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--subject', action='append', required=True, help='NAME=ROOT')
ap.add_argument('--out', required=True)
a = ap.parse_args()
records = []
for entry in a.subject:
    name, root = entry.split('=', 1); root = Path(root).resolve()
    for path in sorted(root.rglob('*.ipynb')):
        rel = path.relative_to(root).as_posix()
        if 'translations' in path.relative_to(root).parts: continue
        before = hashlib.sha256(path.read_bytes()).hexdigest()
        result = subprocess.run([sys.executable, '-m', 'jupytext', '--test', '--to', 'py:percent', str(path)],
                                capture_output=True, text=True, timeout=60)
        assert before == hashlib.sha256(path.read_bytes()).hexdigest(), 'roundtrip modified original notebook'
        record = {'subject':name, 'path':rel, 'sha256':before, 'exit':result.returncode}
        if result.returncode:
            record['error'] = (result.stdout + result.stderr).strip().splitlines()[-1].replace(str(root), '<subject>')
        try:
            with warnings.catch_warnings():
                warnings.simplefilter('ignore')
                original = nbformat.reads(path.read_text(), as_version=4)
                converted = jupytext.reads(jupytext.writes(original, fmt='py:percent'), fmt='py:percent')
            record['cells_before'] = len(original.cells)
            record['cells_after'] = len(converted.cells)
            record['code_sources_equal'] = ([c.source for c in original.cells if c.cell_type=='code'] ==
                                            [c.source for c in converted.cells if c.cell_type=='code'])
        except Exception as error:
            record['roundtrip_unreadable'] = type(error).__name__
        records.append(record)
result = {'jupytext_version': subprocess.check_output([sys.executable, '-m', 'jupytext', '--version'], text=True).strip(),
          'python':sys.version.split()[0], 'files':len(records), 'passed':sum(r['exit']==0 for r in records), 'records':records}
Path(a.out).write_text(json.dumps(result, indent=2, sort_keys=True)+'\n')
print(json.dumps({k:v for k,v in result.items() if k!='records'}, sort_keys=True))
