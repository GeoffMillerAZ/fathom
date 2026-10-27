#!/usr/bin/env python3
"""Build the pinned released GNU Bash with only official published patches."""
import argparse,hashlib,json,os
from pathlib import Path
import shutil,subprocess,sys,tarfile,time


def ready():
    while True:
        subprocess.run(['df','-h',str(Path.home())],check=True)
        if shutil.disk_usage(Path.home()).free<20*(1<<30):raise RuntimeError('need at least 20 GiB free')
        if os.getloadavg()[0]<=6:return
        print('wait load<=6',flush=True);time.sleep(30)


def download(url,path,sha):
    ready()
    if not path.exists():subprocess.run(['curl','-fsSL','--retry','2',url,'-o',str(path)],check=True)
    assert hashlib.sha256(path.read_bytes()).hexdigest()==sha,'published checksum mismatch'


def main():
    if sys.version_info<(3,13):raise SystemExit('Python 3.13+ required')
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);args=p.parse_args()
    if os.getpriority(os.PRIO_PROCESS,0)<10:raise SystemExit('nice -n 10 required')
    base=args.out.resolve();base.mkdir(parents=True,exist_ok=True)
    pins=json.loads(Path(__file__).with_name('bash-pins.json').read_text())
    archive=base/'bash-5.3.tar.gz'
    download('https://ftp.gnu.org/gnu/bash/bash-5.3.tar.gz',archive,pins['archive_sha256'])
    source=base/'bash-5.3'
    if source.exists():raise ValueError('source build must be fresh')
    ready()
    with tarfile.open(archive) as data:data.extractall(base,filter='data')
    patches=base/'patches';patches.mkdir()
    for number,sha in pins['patches'].items():
        patch=patches/('bash53-'+number)
        download('https://ftp.gnu.org/gnu/bash/bash-5.3-patches/bash53-'+number,patch,sha)
        subprocess.run(['patch','-p0','-i',str(patch)],cwd=source,check=True)
    ready()
    with (base/'configure.log').open('w') as log:
        subprocess.run([str(source/'configure'),'--prefix='+str(base/'install'),'--without-bash-malloc','--disable-nls'],cwd=source,stdout=log,stderr=log,check=True)
    ready()
    with (base/'build.log').open('w') as log:subprocess.run(['make','-j2'],cwd=source,stdout=log,stderr=log,check=True)
    binary=source/'bash'
    version=subprocess.check_output([str(binary),'--version'],text=True).splitlines()[0]
    assert '5.3.20' in version
    subprocess.run([str(binary),'-c','shopt -s bash_source_fullpath; shopt -q bash_source_fullpath'],check=True)
    (base/'bash-build.json').write_text(json.dumps({'version':version,'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),'pins':pins,'fullpath_shopt':'available, enabled only by the probe','licence':'GPL-3.0-or-later; benchmark CLI only, never linked'},indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
