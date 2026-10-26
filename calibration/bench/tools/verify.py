"""Historical round-1 verifier; --archive checks the returned trial log bundle."""
import os
import sys
import runpy
from pathlib import Path
if '--archive' in sys.argv:
    runpy.run_path(str(Path(__file__).with_name('verify_logs.py')), run_name='__main__')
    sys.exit(0)
import pathlib,json,subprocess,hashlib,re,collections
root=pathlib.Path.cwd();job=pathlib.Path(os.environ['BENCH_JOB'])
ms=json.loads((job/'manifests.json').read_text());assert len(ms)==10
owners=collections.Counter();paths=[]
for m in ms:
 b=m['bench'];e=b['evidence'];repo=m['url'].removeprefix('https://github.com/');rid=repo.replace('/','--');r=json.loads((job/'receipts'/f'{rid}.json').read_text());s=json.loads((job/'receipts'/f'{rid}-suite.json').read_text());clone=pathlib.Path(os.environ['BENCH_SUBJECTS'])/rid
 assert re.fullmatch('[0-9a-f]{40}',m['sha']) and m['sha']==r['sha']
 assert m['name']==repo.replace('/','-').lower()
 assert b['flake_runs']==3 and b['suite_seconds']==[x['seconds'] for x in s['runs']]
 assert len(s['runs'])==3 and s['warmup']['exit_code']==0 and all(x['exit_code']==0 and x['network']=='none' and x['seconds']<900 for x in s['runs'])
 assert 5000<=e['primary_language_lines']['total']<=150000 and e['primary_language_lines']==r['lines']
 lic=subprocess.check_output(['git','-C',str(clone),'show',m['sha']+':'+e['licence_file']]);assert hashlib.sha256(lic).hexdigest()==e['licence_sha256']
 assert m['licence'] in ['MIT','Apache-2.0','BSD-2-Clause','BSD-3-Clause','CC0-1.0']
 for f,sha in b['toolchain']['lockfile_sha256'].items():assert hashlib.sha256((clone/f).read_bytes()).hexdigest()==sha
 for i,x in enumerate(s['runs']):assert e['suite']['runs'][i]['log_sha256']==hashlib.sha256((job/'logs'/x['log']).read_bytes()).hexdigest()
 assert len(e['fan_in']['qualifying_symbols'])==3 and all(x['caller_modules']>=3 and x['call_sites']>=5 for x in e['fan_in']['qualifying_symbols'])
 assert b['l0_reading'] is None and b['contamination']['fresh_after'] is None and b['split']=='unassigned' and b['task_sources']==[]
 assert e['fix_yield_estimate']['replayed']==0 and e['fix_yield_estimate']['test_and_non_test_commits']==r['yield']['test_and_non_test_commits']
 owners[e['independence']['organization']]+=1
 paths.append(root/'calibration/bench.d'/(m['name']+'.yaml'))
assert max(owners.values())<=2
reading=root/'docs/bench/readings/bench-prep/candidates.md';text=reading.read_text();rows=[x for x in text.splitlines() if x.startswith(('| Go |','| Python |','| TypeScript |'))]
assert len(rows)==15 and all(sum(x.startswith('| '+l+' |') for x in rows)==5 for l in ['Go','Python','TypeScript'])
for f in paths+[reading]:
 t=f.read_text()
 assert not re.search(r'/(?:Users|home|private|tmp|var|work|subject)/',t),f
 assert not re.search(r'file://|(?:HOSTNAME|hostname)\s*[:=]',t),f
 assert not any(line.rstrip()!=line for line in t.splitlines()),f
 assert t.endswith('\n')
 p=subprocess.run(['git','diff','--no-index','--check','/dev/null',str(f)],capture_output=True,text=True)
 assert not p.stdout and not p.stderr,(f,p.stdout,p.stderr)
assert len(list((root/'calibration/bench.d').glob('*.yaml')))==10
print('PASS: 10 YAML drafts match pins, licences, lines, lock hashes, fan-in, timings and log hashes; 30 green isolated trials; 15 reading rows; owner cap; draft unknowns; portable paths; whitespace.')
