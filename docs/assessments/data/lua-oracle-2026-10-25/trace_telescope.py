#!/usr/bin/env python3
"""Run Telescope's native Plenary test entrypoint with a require observation hook."""
import argparse
import json
import os
from pathlib import Path
import subprocess
ap = argparse.ArgumentParser(description=__doc__)
ap.add_argument('--nvim', required=True)
ap.add_argument('--subject', required=True)
ap.add_argument('--out', required=True)
ap.add_argument('--luaparse', required=True, help='local node_modules containing luaparse 0.3.1')
a = ap.parse_args()
root, out = Path(a.subject).resolve(), Path(a.out).resolve()
out.mkdir(parents=True, exist_ok=False)
env = dict(os.environ, LUA_TRACE_SUBJECT=str(root), LUA_TRACE_OUT=str(out))
for key in ['XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_STATE_HOME', 'XDG_CACHE_HOME']:
    env[key] = str(out / key.lower())
here = Path(__file__).resolve().parent
hook = here / 'trace_init.lua'
env['NODE_PATH'] = str(Path(a.luaparse).resolve())
with (out / 'sites.json').open('w') as target:
    subprocess.run(['node', str(here / 'literal_sites.cjs'), str(root)], env=env, stdout=target, check=True)
env['LUA_TRACE_SITES'] = str(out / 'sites.json')
env['LUA_TRACE_POPULATION'] = str(out / 'population.json')
subprocess.run([str(Path(a.nvim).resolve()), '--headless', '--noplugin', '-u', 'NONE', '-l', str(here / 'denominator.lua')], cwd=root, env=env, capture_output=True, timeout=60, check=True)
command = 'PlenaryBustedDirectory lua/tests/automated/ { minimal_init = ' + json.dumps(str(hook)) + ', sequential = true, timeout = 10000 }'
try:
    p = subprocess.run([str(Path(a.nvim).resolve()), '--headless', '--noplugin', '-u', str(hook), '-c', command],
                       cwd=root, env=env, capture_output=True, text=True, timeout=240)
    exit_code = p.returncode
    (out / 'tests.log').write_text(p.stdout + p.stderr)
except subprocess.TimeoutExpired as e:
    exit_code = 'timeout'
    (out / 'tests.log').write_bytes((e.stdout or b'') + (e.stderr or b''))
# Ruby-trace precedent: also load each original file in its own native process.
# The subject's startup and loader paths are retained; no globals are stubbed.
loads = []
for source in sorted(root.rglob('*.lua')):
    if '.git' in source.parts: continue
    command = 'lua local ok,err=pcall(dofile,' + json.dumps(str(source)) + '); if not ok then io.stderr:write(tostring(err)); vim.cmd("cq") end'
    try:
        result = subprocess.run([str(Path(a.nvim).resolve()), '--headless', '--noplugin', '-u', str(hook), '-c', command, '-c', 'qa!'],
                                cwd=root, env=env, capture_output=True, text=True, timeout=5)
        loads.append({'path': source.relative_to(root).as_posix(), 'exit': result.returncode})
    except subprocess.TimeoutExpired:
        loads.append({'path': source.relative_to(root).as_posix(), 'exit': 'timeout'})
events = [json.loads(line) for f in sorted(out.glob('*.jsonl')) for line in f.read_text().splitlines()]
imports = sorted({(r['path'], r['line'], r['module'], r.get('to_path')) for r in events if 'path' in r}, key=str)
population = json.loads((out / 'population.json').read_text())
seen = {(p, line, m) for p,line,m,to in imports if to}
covered = [r for r in population['imports'] if (r['path'], r['line'], r['module']) in seen]
summary = {'file_loads': loads, 'literal_in_subject_statements': len(population['imports']), 'covered_literal_statements': len(covered),
           'coverage': round(len(covered) / len(population['imports']), 4),
           'scanner_counts': population['counts'],
           'plenary_commit': subprocess.check_output(['git', '-C', str(root.parent / 'plenary.nvim'), 'rev-parse', 'HEAD'], text=True).strip(),
           'nvim_version': subprocess.check_output([a.nvim, '--version'], text=True).splitlines()[0],
           'test_exit': exit_code, 'observed_unique_statements': len(imports),
           'observed_in_subject_statements': sum(bool(r[3]) for r in imports),
           'observed': [dict(zip(['path','line','module','to_path'], row)) for row in imports],
           'claim': 'feasibility run; tier A requires >=90% executed literal-statement coverage; no grade changed'}
(out / 'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
print(json.dumps({k:v for k,v in summary.items() if k != 'observed'}, sort_keys=True))
