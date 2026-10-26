#!/usr/bin/env python3
"""Pinned owner runtime, released subject modules and original Git populations."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import time
import zipfile

RUNTIME=('https://github.com/PowerShell/PowerShell/releases/download/v7.6.9/powershell-7.6.9-osx-arm64.tar.gz',
         'a1c7e98ac403da4be5ba75c1c5855af272de7787a91d478c4267466fe58fc1af')
MODULES=[
    ('Pester','6.2.0','e6ac7418d4f12500269aaca58ae56cf1caafbbf1afa2cec334e289d1cf50a239'),
    ('Pester','5.7.1','4a27904c6814a5fbe4758f8e49861f6a1994aee77b71165a5c43c0371ba6c580'),
    ('PSScriptAnalyzer','1.25.0','14e634c828eb98efb9f40b2918ba90f139ed5eccdf663a2a747736d996995d60'),
]
SUBJECTS={
    'pester':('pester/Pester','98e56d65da1642a7158992b50ded240c42a3ac18','.'),
    'psa':('PowerShell/PSScriptAnalyzer','f05704df81b2aca17dc027ee39b3fce106d418fc','Tests'),
}


def ready():
    while True:
        subprocess.run(['df','-h',str(Path.home())],check=True)
        if shutil.disk_usage(Path.home()).free<20*(1<<30):raise RuntimeError('need at least 20 GiB free')
        if os.getloadavg()[0]<=6:return
        print('waiting for load <=6',flush=True)
        time.sleep(30)


def environment(base):
    env=os.environ.copy()
    env.update(DOTNET_CLI_HOME=str(base/'dotnet-home'),DOTNET_CLI_TELEMETRY_OPTOUT='1',
               DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1',POWERSHELL_TELEMETRY_OPTOUT='1',
               XDG_CACHE_HOME=str(base/'xdg-cache'),XDG_CONFIG_HOME=str(base/'xdg-config'),
               XDG_DATA_HOME=str(base/'xdg-data'),PSModulePath=str(base/'runtime/Modules')+os.pathsep+str(base/'modules'))
    return env


def fetch(url,path,expected):
    ready()
    if not path.exists():subprocess.run(['curl','-fsSL','--retry','2',url,'-o',str(path)],check=True)
    if hashlib.sha256(path.read_bytes()).hexdigest()!=expected:raise ValueError('pinned checksum mismatch')


def main():
    if sys.version_info<(3,13):raise SystemExit('Python 3.13+ required')
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS,0)<10:raise SystemExit('run with nice -n 10')
    base=args.out.resolve()
    base.mkdir(parents=True,exist_ok=True)
    archive=base/'powershell.tar.gz'
    fetch(RUNTIME[0],archive,RUNTIME[1])
    if not(base/'runtime/pwsh').exists():
        ready()
        (base/'runtime').mkdir(exist_ok=True)
        with tarfile.open(archive) as source:source.extractall(base/'runtime',filter='data')
    version=subprocess.check_output([str(base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive','-Command',
                                     '$PSVersionTable.PSVersion.ToString()'],env=environment(base),text=True).strip()
    assert version=='7.6.9'
    receipt={'pwsh':{'version':version,'archive_sha256':RUNTIME[1]},'modules':{},'subjects':{}}
    for name,version,digest in MODULES:
        filename=name.lower()+'.'+version+'.nupkg'
        package=base/filename
        fetch('https://cdn.powershellgallery.com/packages/'+filename,package,digest)
        module=base/'modules'/name/version
        if not module.exists():
            ready()
            with zipfile.ZipFile(package) as source:
                assert len(source.infolist())<=10000
                assert sum(x.file_size for x in source.infolist())<=512<<20
                source.extractall(module)
        receipt['modules'][name+' '+version]={'version':version,'package_sha256':digest,'source':'matching released PowerShell Gallery module package'}
    for name,(repository,pin,scope) in SUBJECTS.items():
        target=base/name
        if not(target/'.git').exists():
            ready()
            subprocess.run(['git','init','--quiet',str(target)],check=True)
            subprocess.run(['git','-C',str(target),'remote','add','origin','https://github.com/'+repository],check=True)
            if scope!='.':
                subprocess.run(['git','-C',str(target),'config','remote.origin.promisor','true'],check=True)
                subprocess.run(['git','-C',str(target),'config','remote.origin.partialclonefilter','blob:none'],check=True)
                subprocess.run(['git','-C',str(target),'sparse-checkout','set','--no-cone','/Tests/'],check=True)
            command=['git','-C',str(target),'fetch','--quiet','--depth=1']
            if scope!='.':command+=['--filter=blob:none']
            subprocess.run(command+['origin',pin],check=True)
            subprocess.run(['git','-C',str(target),'checkout','--quiet','--detach','FETCH_HEAD'],check=True)
        actual=subprocess.check_output(['git','-C',str(target),'rev-parse','HEAD'],text=True).strip()
        assert actual==pin
        receipt['subjects'][name]={'revision':pin,'subpath':scope,'checkout':'complete Git tree' if scope=='.' else 'complete original Tests subtree; no other repository content is claimed'}
    (base/'fetch.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
