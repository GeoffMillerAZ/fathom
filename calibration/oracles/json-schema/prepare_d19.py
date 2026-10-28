"""Pinned private SDK/subjects for the preregistered JSON Schema D19 replay."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import time

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
PROTOCOL=HERE/'d19-protocol-r1.json'

def floor(path):
 while shutil.disk_usage(path).free<20*1024**3 or os.getloadavg()[0]>6:
  print('waiting for disk>=20GiB/load<=6',flush=True);time.sleep(30)
 subprocess.run(['df','-h',str(Path.home())],check=True)

def inside_workspace(path):
 path=path.resolve()
 if not path.is_relative_to(ROOT/'workspace'):raise ValueError('private workspace output required')
 return path

def check_inputs(root,scopes,limits):
 files=[];total=0
 for scope in scopes:
  for path in sorted((root/scope).rglob('*.json')):
   info=path.lstat()
   if not stat.S_ISREG(info.st_mode):continue
   if info.st_size>limits['source_file_bytes']:raise ValueError('source-file cap')
   total+=info.st_size;files.append(path)
   if len(files)>limits['files_per_subject'] or total>limits['source_bytes_per_subject']:raise ValueError('source population cap')
 return dict(files=len(files),bytes=total)

def manifests(out,phase):
 text=(ROOT/'docs/design/language-breadth.md').read_text();start=text.index('SUBJECT=workspace/calibration/<subject>');start=text.index('\n',start)+1;end=text.index('\nPY\n```',start);recipe=text[start:end]
 rows=json.loads(PROTOCOL.read_text())['population'];summaries=[]
 for index,row in enumerate(rows):
  for number,scope in enumerate(row['scopes']):
   receipt=out/(str(index)+'-'+str(number)+'-'+phase)
   subprocess.run(['python3','-B','-c',recipe],env=dict(os.environ,SUBJECT=str(out/f'subject-{index}'),SUBPATH=scope,RECEIPT=str(receipt)),check=True,timeout=60)
   summaries.append(dict(subject=index,scope=number,summary=json.loads(receipt.with_suffix('.json').read_text())))
 return summaries

def sdk(out):
 floor(out)
 tools=out/'tools';tools.mkdir(exist_ok=False)
 package=dict(name='json-schema-d19-oracle',private=True,type='module',dependencies={'@apidevtools/json-schema-ref-parser':'16.0.5','yaml':'2.8.2'})
 (tools/'package.json').write_text(json.dumps(package,indent=2)+'\n')
 with (out/'sdk-install.log').open('wb') as log:
  subprocess.run(['npm','--prefix',str(tools),'install','--ignore-scripts','--no-audit','--no-fund','--cache',str(out/'npm-cache')],stdout=log,stderr=subprocess.STDOUT,check=True,timeout=180)
 lock=json.loads((tools/'package-lock.json').read_text());protocol=json.loads(PROTOCOL.read_text())['oracle']
 for name,pin in [('@apidevtools/json-schema-ref-parser',protocol),('yaml',protocol['positions'])]:
  entry=lock['packages']['node_modules/'+name]
  if entry['version']!=pin['version'] or entry['integrity']!=pin['integrity']:raise ValueError('SDK integrity identity')
 (out/'sdk-receipt.json').write_text(json.dumps(dict(package_lock_sha256=hashlib.sha256((tools/'package-lock.json').read_bytes()).hexdigest(),node=subprocess.check_output(['node','--version'],text=True).strip(),npm=subprocess.check_output(['npm','--version'],text=True).strip()),indent=2)+'\n')

def subjects(out):
 protocol=json.loads(PROTOCOL.read_text());rows=[]
 for index,row in enumerate(protocol['population']):
  floor(out);subject=out/f'subject-{index}';subject.mkdir(exist_ok=True)
  def git(*args):return subprocess.check_output(['git','-c','credential.helper=','-C',str(subject),*args],env=dict(os.environ,GIT_TERMINAL_PROMPT='0'),stderr=subprocess.DEVNULL,timeout=180)
  if not (subject/'.git').exists():
   git('init');git('remote','add','origin','https://github.com/'+row['repository'])
   git('fetch','--filter=blob:none','--depth=1','origin',row['sha'])
   git('sparse-checkout','init','--no-cone');git('sparse-checkout','set','--no-cone',*['/'+scope+'/' for scope in row['scopes']])
   git('checkout','--detach',row['sha'])
  if git('rev-parse','HEAD').decode().strip()!=row['sha']:raise ValueError('subject pin')
  if git('remote','get-url','origin').decode().strip()!='https://github.com/'+row['repository']:raise ValueError('subject origin')
  license_bytes=None
  for name in ['LICENSE','LICENSE.txt','LICENSE.md']:
   try:license_bytes=git('show',row['sha']+':'+name);break
   except subprocess.CalledProcessError:pass
  if license_bytes is None:raise ValueError('license absent')
  expected={'Apache-2.0':b'Apache License','MIT':b'Permission is hereby granted','BSD-3-Clause':b'Redistribution and use'}[row['license']]
  if expected not in license_bytes:raise ValueError('license requires review')
  rows.append(dict(subject=index,sha=row['sha'],license=row['license'],license_sha256=hashlib.sha256(license_bytes).hexdigest(),**check_inputs(subject,row['scopes'],protocol['limits'])))
 (out/'subjects-receipt.json').write_text(json.dumps(rows,indent=2)+'\n');manifests(out,'before')

if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['sdk','subjects','after']);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=inside_workspace(args.out);out.mkdir(parents=True,exist_ok=True)
 if args.mode=='sdk':sdk(out)
 elif args.mode=='subjects':subjects(out)
 else:manifests(out,'after')
