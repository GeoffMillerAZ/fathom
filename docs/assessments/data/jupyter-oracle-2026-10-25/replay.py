#!/usr/bin/env python3
"""Replay the native notebook probes and modern diagnostic comparison, offline."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
ap = argparse.ArgumentParser(description=__doc__)
for name in ['server','basedpyright','scip-python','ml','handbook','modern','modern-python','ctags','indexbench','out']:
    ap.add_argument('--'+name, required=True)
a = ap.parse_args(); out = Path(a.out).resolve(); out.mkdir(parents=True, exist_ok=False)
here = Path(__file__).resolve().parent
if os.getpriority(os.PRIO_PROCESS,0) < 10: raise ValueError('run with nice -n 10')

def run(args, **kwargs):
    if os.getloadavg()[0] > 6: raise ValueError('load above 6; stop and replay into a fresh output directory')
    subprocess.run([str(x) for x in args], check=True, **kwargs)

run([sys.executable,here/'probe.py','--basedpyright',a.basedpyright,'--scip-python',a.scip_python,'--out',out/'probe'])
for label, subject, python in [('probe',out/'probe/notebook',sys.executable),('ml',a.ml,sys.executable),('modern',a.modern,a.modern_python)]:
    run([sys.executable,here/'native_lsp.py','--server',a.server,'--subject',subject,'--python',python,'--out',out/(label+'-native.json')])
run([sys.executable,here/'compare_native.py','--native',out/'modern-native.json','--subject',a.modern,
     '--ctags',a.ctags,'--indexbench',a.indexbench,'--out',out/'modern-comparison'])
run([sys.executable,here/'roundtrip.py','--subject','ML-For-Beginners='+a.ml,
     '--subject','PythonDataScienceHandbook='+a.handbook,'--out',out/'roundtrip.json'])
# Capture exact installed environments without requiring pip in the target venv.
query = 'import importlib.metadata,json; print(json.dumps(dict(sorted((d.metadata["Name"],d.version) for d in importlib.metadata.distributions())),indent=2,sort_keys=True))'
for label, python in [('tools',sys.executable),('modern',a.modern_python)]:
    with (out/(label+'-environment.json')).open('w') as target: run([python,'-c',query],stdout=target)
manifest = {}
for label, root in [('ml',a.ml),('handbook',a.handbook),('modern',a.modern)]:
    root = Path(root).resolve()
    sha = subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()
    manifest[label] = {'commit':sha,'notebooks':{p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
        for p in sorted(root.rglob('*.ipynb')) if 'translations' not in p.relative_to(root).parts}}
    run(['git','-C',root,'diff','--exit-code','--','*.ipynb','pyproject.toml','requirements.txt','environment.yml'])
manifest['oracle_code_sha256'] = {p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(here.glob('*.py'))}
(out/'manifest.json').write_text(json.dumps(manifest,indent=2,sort_keys=True)+'\n')
print('Replay complete; modern diagnostic remains outside canonical corpus and reading.')
