"""Linux proof of the production private-scratch child and exact SIGXFSZ trace.

Only authored Python executes. All logs/trace remain private on the runner.
"""
import argparse
import hashlib
import json
import os
import signal
import subprocess
import resource
import sys
from pathlib import Path
from vendor_exec import VENDOR_LIMIT, vendor_command
from filesystem_isolation import mount_readonly
from write_tracing import BoundedCapture, trace_command
from write_diagnostics import diagnostics, MAX_TRACE


def run(out):
    if not sys.platform.startswith('linux'):
        raise ValueError('Linux write control required')
    out=out.resolve();out.mkdir(parents=True,exist_ok=False)
    root=out/'worker';home=root/'home';home.mkdir(parents=True)
    (home/'tmp').mkdir();payload=root/'payload';payload.mkdir()
    target=home/'authored-write-control'
    target.write_text('#!/usr/bin/python3\n'+'''import os,resource,signal
assert resource.getrlimit(resource.RLIMIT_FSIZE)==(268435456,268435456)
assert os.getpid()>1 and os.getppid()==1
assert {int(p) for p in os.listdir('/proc') if p.isdigit()}=={1,os.getpid()}
for fd in [int(p) for p in os.listdir('/proc/self/fd') if int(p)>=3]:
    try:os.fstat(fd)
    except OSError:pass
    else:raise RuntimeError('extra descriptor inherited by authored child')
for path in [os.path.dirname(os.path.dirname(os.getcwd()))]:
    try:fd=os.open(path+'/unexpected-write',os.O_CREAT|os.O_WRONLY,0o600)
    except OSError as e:assert e.errno==30
    else:raise RuntimeError('outside scratch was writable')
signal.signal(signal.SIGXFSZ,signal.SIG_DFL)
resource.setrlimit(resource.RLIMIT_FSIZE,(16384,16384))
fd=os.open('tmp/limit-control',os.O_CREAT|os.O_WRONLY,0o600)
os.write(fd,b'hello')
os.ftruncate(fd,32768)
raise RuntimeError('file-size limit failed to stop extension')
''')
    target.chmod(0o700)
    reader,writer=os.pipe()
    command=['/usr/bin/unshare','--user','--map-root-user','--net','--pid','--fork',
             '--kill-child','--mount','--mount-proc','--',sys.executable,'-B',str(Path(__file__).resolve()),
             '-out',str(root),'--inside','--trace-fd',str(writer)]
    with (out/'trace.log').open('wb') as trace, (out/'stderr.log').open('wb') as stderr:
        capture=BoundedCapture(os.fdopen(reader,'rb'),trace,MAX_TRACE)
        process=None
        try:
            process=subprocess.Popen(command,pass_fds=(writer,),stdout=subprocess.DEVNULL,
                                     stderr=subprocess.PIPE,start_new_session=True)
            os.close(writer);writer=None
            errors=BoundedCapture(process.stderr,stderr,MAX_TRACE)
            process.wait(timeout=45)
        finally:
            if writer is not None:os.close(writer)
            if process:
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                process.wait()
        capture.check();errors.check()
    result=diagnostics((out/'trace.log').read_text(),root,target)
    expected=hashlib.sha256(b'limit-control').hexdigest()
    limit=result['limit_file']
    if (not result['vendor_exec_observed'] or result['native_returncode']!=-25 or
            not result['sigxfsz_observed'] or not limit or limit['area']!='tmp' or
            limit['path_sha256']!=expected or limit['requested_extent_bytes']!=32768):
        raise ValueError('exact file-size control trace failed')
    row=next(row for row in result['files'] if row['path_sha256']==expected)
    if row['observed_write_bytes']!=5 or row['final_size_bytes']!=5:
        raise ValueError('per-path written-byte control failed')
    return dict(linux_write_control_executed=True,private_scratch_boundary=True,trace_writer_hidden=True,
                sigxfsz_observed=True,exact_limit_file_identified=True,
                native_executed=False,vendor_limit_bytes=VENDOR_LIMIT)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True)
    parser.add_argument('--inside',action='store_true');parser.add_argument('--trace-fd',type=int)
    args=parser.parse_args()
    if args.inside:
        os.chdir(args.out/'home')
        mount_readonly(args.out)
        resource.setrlimit(resource.RLIMIT_CORE,(0,0))
        resource.setrlimit(resource.RLIMIT_FSIZE,(8*1024**2,VENDOR_LIMIT))
        command=trace_command('/usr/bin/strace',args.trace_fd,vendor_command(args.out/'home',args.out/'payload',authored=True))
        process=subprocess.Popen(command,pass_fds=(args.trace_fd,))
        os.close(args.trace_fd)
        raise SystemExit(process.wait(timeout=30))
    else:print(json.dumps(run(args.out),sort_keys=True))
