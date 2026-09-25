"""Owned PTY session keeper; not a consumer command or general executor."""
import json
import os
import subprocess
import sys
import termios

from run import attrs_json


def emit(kind, value):
    print(f'APPKIT_PTY_{kind} {json.dumps(value, separators=(",", ":"))}', flush=True)


def main():
    emit('BEFORE', attrs_json(termios.tcgetattr(0)))
    # Stay the session leader after the actual application exits. Otherwise macOS
    # revokes the controlling PTY before restoration can be inspected.
    app = subprocess.Popen(sys.argv[1:])
    emit('PROCESS', app.pid)
    code = app.wait()
    emit('EXIT', code)
    emit('IMMEDIATE', attrs_json(termios.tcgetattr(0)))
    emit('READ_READY', True)
    line = os.read(0, 1024)
    emit('LINE', line.hex())
    emit('AFTER_READ', attrs_json(termios.tcgetattr(0)))
    # Keep the controlling session alive for the parent's independent snapshot.
    return 0 if os.read(0, 1024) == b'keeper-exit\n' else 1


if __name__ == '__main__':
    sys.exit(main())
