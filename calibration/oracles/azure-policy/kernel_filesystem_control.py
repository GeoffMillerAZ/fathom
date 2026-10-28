"""Real Linux mount regression on authored directories; no vendor or container."""
import argparse
import errno
import json
import os
import signal
import subprocess
import sys
from pathlib import Path
from filesystem_isolation import isolate_filesystem,restricted_command


def inside(root,hidden):
    mount=['/usr/bin/mount']
    subprocess.run(mount+['--make-rprivate','/'],check=True)
    if hidden:
        # This namespace-only mount becomes an unreachable old-proc child.
        subprocess.run(mount+['-t','tmpfs','tmpfs','/proc/sys/fs/binfmt_misc'],check=True)
    subprocess.run(mount+['-t','proc','-o','nosuid,nodev,noexec','proc','/proc'],check=True)
    protected=[root/'checkout',root/'runner-temp']
    scratch=root/'worker'
    flags=isolate_filesystem(scratch,protected)
    # Independently test persistent markers, rather than accepting helper flags.
    # Do not normalize the dot-dot spellings. Only authored directories occur.
    code='''import os,sys,errno
count=0
for directory in sys.argv[1:]:
    for prefix in ('','/..','/proc/..'):
        path=prefix+directory+'/unexpected-persistent-write'
        try:
            fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
        except OSError as error:
            if error.errno!=errno.EROFS:raise
            count+=1
        else:
            os.write(fd,b'authored regression marker');os.close(fd)
            raise RuntimeError('write persisted outside scratch')
assert count==6
'''
    subprocess.run(restricted_command([sys.executable,'-B','-c',code,*map(str,protected)]),check=True)
    assert flags['filesystem_dotdot_writes_refused']
    print(json.dumps(dict(completed=True,write_denials=6,hidden_submount=hidden)))


def run(out):
    if not sys.platform.startswith('linux'):
        raise ValueError('Linux kernel control required')
    out=out.resolve();out.mkdir(parents=True,exist_ok=False)
    for index,hidden in enumerate((False,True)):
        root=out/('case-'+str(index));root.mkdir()
        for name in ('checkout','runner-temp','worker'):(root/name).mkdir(mode=0o700)
        command=['/usr/bin/unshare','--user','--map-root-user','--net','--pid','--fork',
                 '--kill-child','--mount','--',sys.executable,'-B',str(Path(__file__).resolve()),
                 '-out',str(root),'--inside']+(['--hidden-submount'] if hidden else [])
        with (root/'stdout.log').open('wb') as stdout,(root/'stderr.log').open('wb') as stderr:
            process=subprocess.Popen(command,stdout=stdout,stderr=stderr,start_new_session=True)
            try:
                code=process.wait(timeout=45)
            finally:
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                process.wait()
        if code or any((root/name/'unexpected-persistent-write').exists() for name in ('checkout','runner-temp')):
            raise ValueError('Linux namespace write/overmount control failed')
        raw=(root/'stdout.log').read_bytes()
        if len(raw)>1024:
            raise ValueError('kernel control output exceeds cap')
        value=json.loads(raw)
        if value!=dict(completed=True,write_denials=6,hidden_submount=hidden):
            raise ValueError('kernel control receipt invalid')
    return dict(linux_namespace_executed=True,scenarios=2,write_denials=12,native_executed=False)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True)
    parser.add_argument('--inside',action='store_true');parser.add_argument('--hidden-submount',action='store_true')
    args=parser.parse_args()
    if args.inside:inside(args.out,args.hidden_submount)
    else:print(json.dumps(run(args.out),sort_keys=True))
