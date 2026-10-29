"""Private pinned compiler/source preparation for the preregistered D19 replay."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import time
import urllib.request
import zipfile

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[3]
PROTOCOL=HERE/'protocol-r1.json'


def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def floor(path):
 while shutil.disk_usage(path).free<20*1024**3 or os.getloadavg()[0]>6:
  print('waiting for disk>=20GiB/load<=6',flush=True);time.sleep(30)
 subprocess.run(['df','-h',str(Path.home())],check=True)


def private(path):
 path=path.resolve()
 if not path.is_relative_to(ROOT/'workspace'):raise ValueError('private workspace output required')
 return path


def check_sources(root,limits):
 files=[];total=0
 for path in sorted(root.rglob('*.proto')):
  info=path.lstat()
  if not stat.S_ISREG(info.st_mode):continue
  if info.st_size>limits['source_file_bytes']:raise ValueError('source-file cap')
  total+=info.st_size;files.append(path)
  if len(files)>limits['files_per_subject'] or total>limits['source_bytes_per_subject']:raise ValueError('source population cap')
 return dict(proto_files=len(files),proto_bytes=total)


def unpack(archive,target,limits):
 with zipfile.ZipFile(archive) as source:
  entries=source.infolist()
  if len(entries)>limits['archive_members']:raise ValueError('archive member cap')
  if sum(row.file_size for row in entries)>limits['archive_expanded_bytes']:raise ValueError('archive expanded cap')
  for row in entries:
   components=row.filename.rstrip('/').split('/')
   if any(part in ('','.','..') for part in components) or row.filename.startswith('/') or '\\' in row.filename or stat.S_ISLNK(row.external_attr>>16):raise ValueError('archive path refused')
   if row.file_size>limits['archive_member_bytes']:raise ValueError('archive file cap')
  target.mkdir(exist_ok=False)
  for row in entries:
   path=target/row.filename
   if row.is_dir():path.mkdir(parents=True,exist_ok=True);continue
   path.parent.mkdir(parents=True,exist_ok=True)
   with source.open(row) as data:path.write_bytes(data.read(limits['archive_member_bytes']+1))
   if path.stat().st_size!=row.file_size:raise ValueError('archive member size')
 return dict(members=len(entries),expanded_bytes=sum(row.file_size for row in entries))


def manifests(out,phase):
 text=(ROOT/'docs/design/language-breadth.md').read_text();start=text.index('SUBJECT=workspace/calibration/<subject>');start=text.index('\n',start)+1;end=text.index('\nPY\n```',start);recipe=text[start:end]
 rows=[]
 for index,row in enumerate(json.loads(PROTOCOL.read_text())['population']):
  # A sparse index still lists unhydrated C++/Go files. Hash the complete
  # preregistered proto/config pathspecs, never a whole unhydrated repository.
  scopes=['src/google/protobuf/**/*.proto'] if row['subpath']=='src' else row['scopes']
  for number,pattern in enumerate(scopes):
   scope=':(glob)'+pattern;subject=out/f'subject-{index}'
   tracked=subprocess.check_output(['git','-C',str(subject),'ls-files','-z','--',scope],timeout=30)
   if not tracked:
    rows.append(dict(subject=index,scope=scope,absent=True));continue
   receipt=out/f'{index}-{number}-{phase}'
   subprocess.run(['python3','-B','-c',recipe],env=dict(os.environ,SUBJECT=str(subject),SUBPATH=scope,RECEIPT=str(receipt)),check=True,timeout=60)
   rows.append(dict(subject=index,scope=scope,summary=json.loads(receipt.with_suffix('.json').read_text())))
 return rows


def verify_sdk(archive,target):
 with zipfile.ZipFile(archive) as source:
  entries=source.infolist()
  for row in entries:
   path=target/row.filename
   if row.is_dir():
    if not stat.S_ISDIR(path.lstat().st_mode):raise ValueError('compiler payload directory identity')
   elif not stat.S_ISREG(path.lstat().st_mode) or path.stat().st_size!=row.file_size or digest(path)!=hashlib.sha256(source.read(row)).hexdigest():raise ValueError('compiler payload file identity')
 return dict(members=len(entries),expanded_bytes=sum(row.file_size for row in entries))


def download(url,path,cap):
 created=False
 try:
  with urllib.request.urlopen(url,timeout=30) as source,path.open('xb') as target:
   created=True;total=0
   while chunk:=source.read(1024**2):
    total+=len(chunk)
    if total>cap:raise ValueError('archive download cap')
    target.write(chunk)
 except Exception:
  if created:path.unlink(missing_ok=True)
  raise


def prepare(out):
 protocol=json.loads(PROTOCOL.read_text());limits=protocol['limits'];floor(out)
 archive=out/'compiler.zip'
 if not archive.exists():download(protocol['oracle']['url'],archive,limits['archive_bytes'])
 if archive.stat().st_size>limits['archive_bytes'] or digest(archive)!=protocol['oracle']['sha256']:raise ValueError('compiler archive identity')
 sdk=out/'protoc'
 if not sdk.exists():unpack(archive,sdk,limits)
 members=verify_sdk(archive,sdk)
 binary=sdk/'bin'/'protoc';binary.chmod(0o700)
 version=subprocess.check_output([str(binary),'--version'],text=True,timeout=10).strip()
 if version!='libprotoc '+protocol['oracle']['version']:raise ValueError('compiler version')
 rows=[]
 for index,row in enumerate(protocol['population']):
  floor(out);subject=out/f'subject-{index}';subject.mkdir(exist_ok=True)
  def git(*args):return subprocess.check_output(['git','-c','credential.helper=','-C',str(subject),*args],env=dict(os.environ,GIT_TERMINAL_PROMPT='0'),stderr=subprocess.DEVNULL,timeout=180)
  if not (subject/'.git').exists():
   git('init');git('remote','add','origin','https://github.com/'+row['repository']);git('fetch','--filter=blob:none','--depth=1','origin',row['sha'])
   git('sparse-checkout','init','--no-cone');git('sparse-checkout','set','--no-cone',*['/'+scope for scope in row['scopes']]);git('checkout','--detach',row['sha'])
  if git('rev-parse','HEAD').decode().strip()!=row['sha'] or git('remote','get-url','origin').decode().strip()!='https://github.com/'+row['repository']:raise ValueError('subject pin or origin')
  license_bytes=None
  for name in ['LICENSE','LICENSE.txt','LICENSE.md']:
   try:license_bytes=git('show',row['sha']+':'+name);break
   except subprocess.CalledProcessError:pass
  expected={'Apache-2.0':b'Apache License','BSD-3-Clause':b'Redistribution and use'}[row['license']]
  if license_bytes is None or expected not in license_bytes:raise ValueError('license requires review')
  rows.append(dict(subject=index,repository=row['repository'],sha=row['sha'],license=row['license'],license_sha256=hashlib.sha256(license_bytes).hexdigest(),**check_sources(subject/row['subpath'],limits)))
 result=dict(protocol_sha256=digest(PROTOCOL),compiler_version=version,archive_sha256=digest(archive),compiler_sha256=digest(binary),compiler_payload=members,subjects=rows,manifests=manifests(out,'before'))
 (out/'prepare-receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=private(args.out);out.mkdir(parents=True,exist_ok=True);prepare(out)
