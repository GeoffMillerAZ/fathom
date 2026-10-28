"""Reproduce the pinned private Jenkins diagnostic bundle and canonical inputs."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

HERE=Path(__file__).resolve().parent
REPO=HERE.parents[2]

def floor(out):
    if shutil.disk_usage(out).free<20*1024**3:raise ValueError('20 GiB disk floor')
    if os.getloadavg()[0]>6:raise ValueError('load above 6')

def fetch(row,path):
    floor(path.parent)
    subprocess.run(['curl','-fsSL','--max-time','300','--max-filesize',str(row['bytes']),'-o',str(path),row['url']],check=True)
    if path.stat().st_size!=row['bytes'] or hashlib.sha256(path.read_bytes()).hexdigest()!=row['sha256']:
        raise ValueError('download identity')

def sdk(out):
    bundle=json.loads((HERE/'native-bundle-r1.json').read_text())
    (out/'plugins').mkdir()
    for row in bundle['sdk_downloads']:fetch(row,out/row['name'])
    # This is the publisher's checksum-pinned JDK archive, never a subject archive.
    with tarfile.open(out/'jdk.tar.gz') as archive:
        if sum(item.size for item in archive.getmembers())>1024**3:raise ValueError('JDK expanded cap')
        for item in archive.getmembers():
            target=(out/item.name).resolve()
            if not target.is_relative_to(out.resolve()):raise ValueError('JDK path escape')
            if not (item.isfile() or item.isdir() or item.issym() or item.islnk()):raise ValueError('unsupported JDK entry')
            if item.islnk() and not (out/item.linkname).resolve().is_relative_to(out.resolve()):raise ValueError('JDK hardlink escape')
            if item.issym() and not (target.parent/item.linkname).resolve().is_relative_to(out.resolve()):raise ValueError('JDK symlink escape')
        archive.extractall(out)
    for row in bundle['plugins']:fetch(row,out/'plugins'/(row['id']+'.jpi'))
    print(json.dumps(dict(sdk_files=len(bundle['sdk_downloads']),plugins=len(bundle['plugins']))))

def manifests(out,phase):
    # Execute the exact committed recipe from design section 13.3.1.
    text=(REPO/'docs/design/language-breadth.md').read_text()
    start=text.index('SUBJECT=workspace/calibration/<subject>')
    start=text.index('\n',start)+1
    end=text.index('\nPY\n```',start)
    recipe=text[start:end]
    for key,scope in [('examples','declarative-examples'),('library','Jenkinsfile')]:
        env=dict(os.environ,SUBJECT=str(out/('subject-'+key)),SUBPATH=scope,RECEIPT=str(out/(key+'-'+phase)))
        subprocess.run(['python3','-B','-c',recipe],env=env,check=True)

def subjects(out):
    protocol=json.loads((HERE/'native-protocol-r1.json').read_text())
    (out/'inputs').mkdir()
    for row in protocol['population']:
        floor(out)
        key=row['subject'];root=out/('subject-'+key);root.mkdir()
        def git(*args):return subprocess.check_output(['git','-c','credential.helper=','-C',str(root),*args],env=dict(os.environ,GIT_TERMINAL_PROMPT='0'),stderr=subprocess.DEVNULL)
        git('init');git('remote','add','origin','https://github.com/'+row['repository'])
        git('fetch','--filter=blob:none','--depth=1','origin',row['sha'])
        git('sparse-checkout','init','--no-cone')
        git('sparse-checkout','set','--no-cone','/declarative-examples/' if key=='examples' else '/Jenkinsfile')
        git('checkout','--detach',row['sha'])
        files=sorted((root/'declarative-examples').rglob('*.groovy')) if key=='examples' else [root/'Jenkinsfile']
        if len(files)!=row['expected_files']:raise ValueError('canonical population changed')
        for source in files:
            relative=source.relative_to(root)
            if key=='examples':relative=relative.with_suffix('.jenkinsfile')
            target=out/'inputs'/key/relative;target.parent.mkdir(parents=True,exist_ok=True)
            target.write_bytes(source.read_bytes())
    manifests(out,'before')

def tool_declarations(log,out):
    found=set()
    for match in re.finditer(r'Tool type "([^"]+)" does not have an install of "([^"]+)"',log.read_text()):
        kind,name=match.groups()
        if kind not in ('maven','jdk') or not 1<=len(name)<=256:raise ValueError('unsupported tool declaration')
        found.add((kind,name))
    if len(found)>64:raise ValueError('tool declaration cap')
    out.write_text(json.dumps([dict(type=kind,name=name) for kind,name in sorted(found)],indent=2)+'\n')
    print(json.dumps(dict(declarations=len(found),tools_installed=False)))

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('mode',choices=['sdk','subjects','after','tools'])
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--log',type=Path)
    args=parser.parse_args();out=args.out.resolve()
    if not out.is_relative_to((REPO/'workspace').resolve()):raise ValueError('output must be under workspace')
    if args.mode in ('sdk','subjects'):out.mkdir(parents=True,exist_ok=False)
    if args.mode=='sdk':sdk(out)
    elif args.mode=='subjects':subjects(out)
    elif args.mode=='after':manifests(out,'after')
    else:tool_declarations(args.log,out)
