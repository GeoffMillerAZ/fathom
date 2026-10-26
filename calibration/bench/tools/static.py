import sys
sys.dont_write_bytecode = True
from bench_paths import job_dir, workspace_path
import os
import pathlib, subprocess, json, re, hashlib, datetime, collections
ROOT=pathlib.Path.cwd(); JOB=job_dir()
def istest(p):
 return bool(re.search(r'(^|/)(tests?|specs?|__tests__)(/|$)|(^|/)(test_[^/]*|[^/]*_test)\.py$|_test\.go$|\.(test|spec)\.[cm]?[jt]sx?$',p))
for receipt in sorted((JOB/'receipts').glob('*.json')):
 d=json.loads(receipt.read_text());
 if 'language' not in d: continue
 root=pathlib.Path(os.environ['BENCH_SUBJECTS'])/d['id']
 def git(*args): return subprocess.check_output(['git','-C',str(root),*args])
 files=git('ls-files','-z').decode().split('\0')[:-1]
 extensions={'Go':['.go'],'Python':['.py'],'TypeScript':['.ts','.tsx','.mts','.cts']}[d['language']]
 counts={}; perfile=[]
 for ext in extensions:
  selected=[f for f in files if pathlib.Path(f).suffix==ext]; total=0
  for f in selected:
   b=git('show',d['sha']+':'+f); n=b.count(b'\n')+(bool(b) and not b.endswith(b'\n')); total+=n; perfile.append([f,n])
  counts[ext]={'files':len(selected),'lines':total}
 d['lines']={'extensions':counts,'total':sum(x['lines'] for x in counts.values()),'method':'tracked files at sha, including tests, fixtures and generated files; LF count plus unterminated final line'}
 (JOB/'receipts'/f"{d['id']}-lines.tsv").write_text(''.join(f'{f}\t{n}\n' for f,n in perfile))
 d['licences']=[]
 for f in files:
  if '/' not in f and re.match(r'(?i)licen[cs]e',f):
   b=git('show',d['sha']+':'+f); text=b.decode(errors='replace'); spdx=None
   if 'Permission is hereby granted, free of charge' in text: spdx='MIT'
   elif 'Apache License' in text and 'Version 2.0' in text: spdx='Apache-2.0'
   elif 'Redistribution and use in source and binary forms' in text:
    spdx='BSD-3-Clause' if ('Neither the name' in text or 'neither the name' in text) else 'BSD-2-Clause'
   d['licences'].append({'path':f,'spdx':spdx,'sha256':hashlib.sha256(b).hexdigest()})
 hist=git('log',d['sha'],'--since=2024-10-25T00:00:00Z','--until=2026-10-26T00:00:00Z','--no-merges','--format=@@%H%x09%cI%x09%s','--name-only').decode(errors='replace')
 both=[]
 for block in hist.split('@@')[1:]:
  ls=block.strip().splitlines(); sha,date,subject=ls[0].split('\t',2); paths=[p for p in ls[1:] if p]
  tests=[p for p in paths if istest(p)]; prod=[p for p in paths if not istest(p) and pathlib.Path(p).suffix in extensions]; other=[p for p in paths if not istest(p)]
  if tests and other: both.append({'sha':sha,'date':date,'subject':subject,'tests':tests,'non_tests':other,'primary_language_non_tests':prod,'fix_message':bool(re.search(r'(?i)\b(fix(es|ed)?|bugs?|regressions?)\b',subject))})
 d['yield']={'since':'2024-10-25T00:00:00Z','until':'2026-10-26T00:00:00Z','test_and_non_test_commits':len(both),'fix_message_subset':sum(x['fix_message'] for x in both),'primary_language_source_commits':sum(bool(x['primary_language_non_tests']) for x in both),'primary_language_source_fix_message_subset':sum(bool(x['primary_language_non_tests']) and x['fix_message'] for x in both),'replayed':0,'issue_examples':[x for x in both if x['fix_message'] and re.search(r'#\d+',x['subject'])][:3]}
 (JOB/'receipts'/f"{d['id']}-history.json").write_text(json.dumps(both,indent=2)+'\n')
 d['lockfiles']=[{'path':f,'sha256':hashlib.sha256(git('show',d['sha']+':'+f)).hexdigest()} for f in files if f in ['go.sum','package-lock.json','yarn.lock','pnpm-lock.yaml','uv.lock','poetry.lock','pdm.lock']]
 d['setup_files']=[f for f in files if f in ['pyproject.toml','setup.cfg','setup.py','requirements-dev.txt','requirements/test.txt','requirements/tests.txt','package.json','Makefile','go.mod']]
 receipt.write_text(json.dumps(d,indent=2)+'\n')
 print(d['name'],d['lines']['total'],d['licences'],d['yield']['test_and_non_test_commits'],d['yield']['fix_message_subset'],flush=True)
