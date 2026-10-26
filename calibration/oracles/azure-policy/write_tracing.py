"""Bounded pipe capture; its destination is outside the vendor writable mount."""
import io
import threading
import hashlib
import re
import subprocess
from pathlib import Path
from write_diagnostics import CALLS


class BoundedCapture:
    def __init__(self, source, target, limit):
        self.source, self.target, self.limit = source, target, limit
        self.overflow, self.error = False, None
        self.thread = threading.Thread(target=self.read, daemon=True)
        self.thread.start()

    def read(self):
        size = 0
        try:
            while True:
                data = self.source.read(65536)
                if not data: break
                room = self.limit - size
                self.target.write(data[:room]); self.target.flush()
                size += len(data)
                if size > self.limit:
                    self.overflow = True
                    break
        except Exception as error:
            self.error = error
        finally:
            self.source.close()

    def check(self):
        self.thread.join(timeout=1)
        if self.thread.is_alive() or self.error or self.overflow:
            raise ValueError('bounded capture incomplete or exceeded cap')


def trace_command(strace, fd, command):
    return [str(strace), '-f', '-q', '-yy', '-s', '0', '-o', '/proc/self/fd/'+str(fd),
            '-e', 'trace='+','.join(sorted(CALLS)), '--', *map(str,command)]


def trace_identity():
    binary=Path('/usr/bin/strace')
    with binary.open('rb') as source: value=source.read(16*1024**2+1)
    if len(value)>16*1024**2: raise ValueError('strace identity cap exceeded')
    process=subprocess.Popen([str(binary),'-V'],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    output=io.BytesIO();capture=BoundedCapture(process.stdout,output,8192)
    try:
        process.wait(timeout=2)
    finally:
        if process.poll() is None: process.kill(); process.wait()
    capture.check()
    line=output.getvalue().splitlines()[0]+b'\n'
    match=re.fullmatch(rb'strace -- version (\d+)\.(\d+)(?:\.(\d+))?\n',line)
    if not match or process.returncode: raise ValueError('strace version identity refused')
    return dict(sha256=hashlib.sha256(value).hexdigest(),version=[int(v) for v in match.groups() if v is not None])
