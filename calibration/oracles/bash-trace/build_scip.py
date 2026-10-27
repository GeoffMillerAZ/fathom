#!/usr/bin/env python3
"""Build released scip-shell v0.1.0 with a lane-local pinned Rust toolchain."""
import argparse,hashlib,json,os
from pathlib import Path
import subprocess,tarfile
from build_bash import ready,download

PIN='bd95dfa882c2529fa83a29aba403a3caa5b4b267'


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
    base=a.out.resolve();base.mkdir(parents=True,exist_ok=True)
    pins=json.loads(Path(__file__).with_name('rust-pins.json').read_text())
    rust=base/'rust'
    for name,item in pins['components'].items():
        archive=base/(name+'.tar.xz');download(item['url'],archive,item['sha256'])
        source=base/(name+'-extract')
        if not source.exists():
            ready();source.mkdir()
            with tarfile.open(archive) as f:f.extractall(source,filter='data')
        marker=rust/(name+'.installed')
        if not marker.exists():
            ready();installer=next(source.glob('*/install.sh'))
            subprocess.run(['bash',str(installer),'--prefix='+str(rust),'--disable-ldconfig'],check=True)
            marker.write_text(item['sha256']+'\n')
    source=base/'scip-shell'
    if not(source/'.git').exists():
        ready();subprocess.run(['git','init','-q',str(source)],check=True)
        subprocess.run(['git','-C',str(source),'remote','add','origin','https://github.com/jelmer/scip-shell'],check=True)
        subprocess.run(['git','-C',str(source),'fetch','-q','--depth=1','origin','refs/tags/v0.1.0'],check=True)
        subprocess.run(['git','-C',str(source),'checkout','-q','--detach','FETCH_HEAD'],check=True)
    assert subprocess.check_output(['git','-C',str(source),'rev-parse','HEAD'],text=True).strip()==PIN
    env=dict(os.environ,CARGO_HOME=str(base/'cargo-home'),RUSTUP_HOME=str(base/'rustup-home'),
             CARGO_TARGET_DIR=str(base/'scip-target'),RUSTC=str(rust/'bin/rustc'),
             PATH=str(rust/'bin')+os.pathsep+os.environ['PATH'])
    ready();subprocess.run([str(rust/'bin/cargo'),'build','--locked','--release','-j','2','--bin','scip-shell'],cwd=source,env=env,check=True)
    binary=base/'scip-target/release/scip-shell'
    version=subprocess.check_output([str(binary),'--version'],text=True).strip()
    (base/'scip-build.json').write_text(json.dumps(dict(version=version,revision=PIN,rust=pins,
        license='MIT per released Cargo.toml',binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
        cargo_lock_sha256=hashlib.sha256((source/'Cargo.lock').read_bytes()).hexdigest()),indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
