"""Linux-only authored mount and write controls; never executes vendor code."""
import errno
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path


def restricted_command(command):
    return ['/usr/bin/setpriv', '--bounding-set=-all', '--inh-caps=-all',
            '--ambient-caps=-all', '--no-new-privs', '--', *map(str,command)]


def mount_table():
    rows=Path('/proc/self/mountinfo').read_text().splitlines()
    if len(rows)>1024:
        raise ValueError('mount count cap exceeded')
    # mountinfo escapes whitespace/backslashes with octal sequences.
    return [(Path(re.sub(r'\\([0-7]{3})',lambda m:chr(int(m[1],8)),row.split()[4])),
             row.split()[5]) for row in rows]


def mount_readonly(scratch):
    def mount(*args):
        subprocess.run(['/usr/bin/mount',*map(str,args)],check=True,timeout=10,
                       stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    mount('--make-rprivate','/')
    mount('--bind',scratch,scratch)
    mount('--rbind','/','/')
    # Reattach root/cwd to the new view; retain no old writable directory
    # handle that /proc/self/root or relative lookups could expose later.
    os.chroot('/')
    os.chdir(scratch)
    # A top-level remount alone leaves writable submounts. Freeze each visible
    # mount, deepest first, retaining only the fresh worker-scratch submount.
    for path,options in sorted(set(mount_table()),key=lambda row:len(row[0].parts),reverse=True):
        if path==scratch or scratch in path.parents or 'ro' in options.split(','):
            continue
        mount('-o','remount,bind,ro',path)


def require_no_privileges():
    values=dict(line.split(':',1) for line in Path('/proc/self/status').read_text().splitlines() if ':' in line)
    if (any(int(values[key].strip(),16) for key in ('CapInh','CapPrm','CapEff','CapBnd','CapAmb')) or
            int(values['NoNewPrivs'].strip())!=1):
        raise ValueError('vendor capabilities or new privileges remain enabled')


def require_readonly(directory):
    path=directory/('.azure-write-control-'+str(os.getpid()))
    try:
        fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    except OSError as error:
        if error.errno!=errno.EROFS:
            raise
    else:
        os.close(fd)
        path.unlink()
        raise ValueError('protected directory is writable')


def write_controls(scratch,protected):
    require_no_privileges()
    for directory in protected:
        require_readonly(directory)
    # Opening the running launcher for write must fail too, without modifying
    # it if isolation is broken. All tested paths are launcher-supplied.
    script=Path(__file__).with_name('run_probe_workflow.sh')
    try:
        fd=os.open(script,os.O_WRONLY|os.O_APPEND|os.O_NOFOLLOW)
    except OSError as error:
        if error.errno!=errno.EROFS:
            raise
    else:
        os.close(fd)
        raise ValueError('launcher script is writable')
    with tempfile.TemporaryFile(dir=scratch) as positive:
        positive.write(b'authored writable scratch control')
        positive.flush()
    return dict(filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,
                filesystem_write_probes=len(protected)+1)


def isolate_filesystem(scratch,protected):
    if os.getpid()!=1:
        raise ValueError('filesystem setup requires namespace PID1')
    if not protected or len(protected)<2 or any(p==scratch or scratch in p.parents for p in protected):
        raise ValueError('launcher-supplied protected paths required')
    mount_readonly(scratch)
    code=('import sys,json;sys.path.insert(0,sys.argv[1]);'
          'from filesystem_isolation import write_controls;from pathlib import Path;'
          'print(json.dumps(write_controls(Path(sys.argv[2]),[Path(p) for p in sys.argv[3:]])))')
    child=subprocess.run(restricted_command([sys.executable,'-B','-c',code,str(Path(__file__).parent),
                                            str(scratch),*map(str,protected)]),
                         check=True,timeout=10,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    if len(child.stdout)>1024:
        raise ValueError('write-control result exceeds cap')
    flags=json.loads(child.stdout)
    expected=dict(filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,
                  filesystem_write_probes=len(protected)+1)
    if flags!=expected or any(type(flags[key]) is not type(value) for key,value in expected.items()):
        raise ValueError('write-control flags invalid')
    return flags
