"""T-R13a vendor child; tracer stays outside its fresh PID/proc view.

Private scratch remains the sole writable filesystem subtree. Only the vendor
child raises its inherited soft file-size bound. Output pipes and receipt have
independent smaller bounds. No trace writer is inherited into its PID view.
"""
import os
import resource
import subprocess
import sys
from pathlib import Path
from filesystem_isolation import mount_readonly, restricted_command, require_no_privileges

VENDOR_LIMIT = 256 * 1024**2


def close_extra_fds():
    maximum=resource.getrlimit(resource.RLIMIT_NOFILE)[0]
    os.closerange(3,int(maximum if maximum!=resource.RLIM_INFINITY else 1048576))


def namespace_command(home,payload,authored=False):
    return ['/usr/bin/unshare','--mount','--pid','--fork','--kill-child','--mount-proc','--',
            sys.executable,'-B',str(Path(__file__).resolve()),str(home),str(payload)]+(['--authored-control'] if authored else [])


def vendor_command(home,payload,authored=False):
    # strace opens the original trace fd before forking this authored launcher.
    # Close ALL inherited extra fds before creating the vendor PID namespace.
    # Its procfs then cannot expose the outside tracer or any trace writer.
    return [sys.executable,'-B',str(Path(__file__).resolve()),'--launch',str(home),str(payload)]+(['--authored-control'] if authored else [])


def child(home,payload,authored=False):
    if os.getpid()!=1: raise ValueError('private scratch setup requires fresh PID1/procfs')
    mount_readonly(home.parent)
    command=[sys.executable,'-B',str(Path(__file__).resolve()),'--exec',str(payload)]
    if authored: command+=['--authored-control']
    # Keep an authored init: vendor PID1 would have special signal semantics.
    # PID1 has no trace descriptor; vendor PID2 receives ordinary SIGXFSZ.
    close_extra_fds()
    # mount_readonly changes cwd to the scratch root. Both the authored
    # control and vendor must instead start in their private HOME.
    process=subprocess.Popen(restricted_command(command),cwd=home)
    try:
        code=process.wait(timeout=170)
    finally:
        if process.poll() is None: process.kill()
        process.wait()
    raise SystemExit(code if code>=0 else 128-code)


def execute(payload,authored=False):
    require_no_privileges()
    if os.getpid()==1 or os.getppid()!=1:
        raise ValueError('vendor requires authored PID1 and ordinary signal semantics')
    close_extra_fds()
    resource.setrlimit(resource.RLIMIT_CORE,(0,0))
    resource.setrlimit(resource.RLIMIT_FSIZE,(VENDOR_LIMIT,VENDOR_LIMIT))
    binary=payload.parent/'home'/'authored-write-control' if authored else payload/'PolicyLanguageServer'
    os.execv(str(binary),[str(binary)])


if __name__=='__main__':
    authored=sys.argv[-1]=='--authored-control'
    if sys.argv[1]=='--exec': execute(Path(sys.argv[2]),authored)
    elif sys.argv[1]=='--launch':
        close_extra_fds()
        command=namespace_command(Path(sys.argv[2]),Path(sys.argv[3]),authored)
        os.execv(command[0],command)
    else: child(Path(sys.argv[1]),Path(sys.argv[2]),authored)
