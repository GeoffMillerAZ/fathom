#!/usr/bin/env python3
"""Pinned original Bash subjects, parser binary and independent query runtime."""
import argparse,hashlib,json,os
from pathlib import Path
import subprocess
import tarfile
from build_bash import ready,download

SUBJECTS={'nvm':('nvm-sh/nvm','1889911f0841e669de0be5bd02c737a3f1fd20fa'),
          'bats':('bats-core/bats-core','713504bc0224a19b3d7c7958c18dc07f64f54b44')}


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
    base=args.out.resolve();base.mkdir(parents=True,exist_ok=True)
    for name,(repository,pin) in SUBJECTS.items():
        root=base/name
        if not(root/'.git').exists():
            ready();subprocess.run(['git','init','-q',str(root)],check=True)
            subprocess.run(['git','-C',str(root),'remote','add','origin','https://github.com/'+repository],check=True)
            subprocess.run(['git','-C',str(root),'fetch','-q','--depth=1','origin',pin],check=True)
            subprocess.run(['git','-C',str(root),'checkout','-q','--detach','FETCH_HEAD'],check=True)
        assert subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()==pin
    binary=base/'shfmt'
    download('https://github.com/mvdan/sh/releases/download/v3.14.1/shfmt_v3.14.1_darwin_arm64',binary,
             'b7c872db63553ccffc7253aba3ed7d4885a27d83f1ba567b1138c6315a5847e5')
    binary.chmod(0o755)
    archive=base/'urchin.tgz'
    download('https://registry.npmjs.org/urchin/-/urchin-0.0.5.tgz',archive,'caa836710230bf0b82caa54375cd5609c7083336910b83202156e2424b4139d0')
    harness=base/'urchin'
    if not harness.exists():
        ready();harness.mkdir()
        with tarfile.open(archive) as f:f.extractall(harness,filter='data')
    fixture_pin=subprocess.check_output(['git','-C',str(base/'nvm'),'rev-parse','HEAD:test/fixtures/nvmrc'],text=True).strip()
    fixture=base/'nvmrc-fixture'
    if not(fixture/'.git').exists():
        ready();subprocess.run(['git','init','-q',str(fixture)],check=True)
        subprocess.run(['git','-C',str(fixture),'remote','add','origin','https://github.com/nvm-sh/nvmrc'],check=True)
        subprocess.run(['git','-C',str(fixture),'fetch','-q','--depth=1','origin',fixture_pin],check=True)
        subprocess.run(['git','-C',str(fixture),'checkout','-q','--detach','FETCH_HEAD'],check=True)
    assert subprocess.check_output(['git','-C',str(fixture),'rev-parse','HEAD'],text=True).strip()==fixture_pin
    (base/'fetch.json').write_text(json.dumps({'subjects':{n:{'revision':pin,'scope':'entire original Git checkout'} for n,(_,pin) in SUBJECTS.items()},
        'shfmt':{'version':'3.14.1','sha256':hashlib.sha256(binary.read_bytes()).hexdigest()},
        'declared_runtime_harness':{'urchin':'0.0.5','archive_sha256':'caa836710230bf0b82caa54375cd5609c7083336910b83202156e2424b4139d0',
            'nvmrc_gitlink_revision':fixture_pin,'load_only':'copied into disposable runtime copies only; original measurement membership unchanged'}},indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
