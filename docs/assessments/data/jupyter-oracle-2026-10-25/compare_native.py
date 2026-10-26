#!/usr/bin/env python3
"""Compare fathom to native-answered notebook pairs; disclose all incomplete truth.

This is a diagnostic holdout reading, not a canonical support-grade promotion.
Unknown native answers and scanner gaps remain in the separate full-site receipt.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--native', required=True)
ap.add_argument('--subject', required=True)
ap.add_argument('--ctags', required=True)
ap.add_argument('--indexbench', required=True)
ap.add_argument('--out', required=True)
a = ap.parse_args()
root = Path(a.subject).resolve(); out = Path(a.out).resolve(); out.mkdir(parents=True, exist_ok=False)
repo = Path(__file__).resolve().parents[4]
native = json.loads(Path(a.native).read_text())
pairs = sorted({(r['path'], c['from'], c['to']) for r in native['records'] for c in r['calls']
                if c['definitions'] and c['to']})
unknown = sum(not c['definitions'] for r in native['records'] for c in r['calls'])
truth = {'tool':'basedpyright native notebook definitions (answered subset; diagnostic only)',
         'version':native['server'], 'tier':'A',
         'references':[{'path':p, 'from':f, 'to':t} for p,f,t in pairs],
         'disclose':[f"Diagnostic holdout, not a canonical support promotion: {native['resolved_calls']}/{native['calls']} enumerable sites answered; {unknown} unknown; {native['rejected_cells']} scanner-rejected cells; {native.get('opaque_magic_lines',0)} opaque magic lines; all gaps retained in native receipt. Only native-answered pairs enter this diagnostic truth."]}
(out / 'native-pairs.json').write_text(json.dumps(truth, indent=2, sort_keys=True)+'\n')
with (out / 'definitions.json').open('w') as target:
    subprocess.run([sys.executable, str(repo / 'calibration/oracles/jupyter/jupyter_oracle.py'), '--subject', str(root),
                    '--ctags', str(Path(a.ctags).resolve())], stdout=target, check=True)
with (out / 'syntax-pairs.json').open('w') as target:
    subprocess.run([sys.executable, str(repo / 'calibration/oracles/jupyter/jupyter_oracle.py'), '--subject', str(root), '--refs'],
                   stdout=target, check=True)
sha = subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()
for label, oracle in [('syntax', 'syntax-pairs.json'), ('native-diagnostic', 'native-pairs.json')]:
    subprocess.run([str(Path(a.indexbench).resolve()), 'lang-oracle', '-dialect', 'jupyter', '-subject', str(root),
                    '-subject-sha', sha, '-subject-name', 'wesm/pydata-book (separate modern diagnostic holdout)',
                    '-tool-oracle', str(out / 'definitions.json'), '-tool-oracle', str(out / oracle),
                    '-unmatched', str(out / (label + '-misses.json')), '-out', str(out / (label + '-reading.json')),
                    '-note', 'OL-R4 modern subject; kept outside canonical corpus and reading; native subset is diagnostic, unknowns and scanner gaps forbid a full-population tier-A claim'], check=True)
summary = {'subject_sha':sha, 'native_sites':native['calls'], 'native_answered':native['resolved_calls'],
           'unknown_sites':unknown, 'scanner_rejected_cells':native['rejected_cells'],
           'opaque_magic_lines':native.get('opaque_magic_lines',0), 'native_distinct_pairs':len(pairs),
           'scope':'all notebooks; separate diagnostic holdout, never canonical corpus', 'readings':{}}
for label in ['syntax','native-diagnostic']:
    r = json.loads((out / (label + '-reading.json')).read_text())
    summary['readings'][label] = {k:r[k] for k in ['metrics','counts','grade','tenancy']}
(out / 'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')
print(json.dumps(summary, sort_keys=True))
