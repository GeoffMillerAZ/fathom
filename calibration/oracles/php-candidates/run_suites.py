#!/usr/bin/env python3
"""Run the original PHPUnit suites serially with private bounded native traces."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import time

from fetch import ready


def listening(port):
    with socket.socket() as sock:
        sock.settimeout(.2)
        return sock.connect_ex(('127.0.0.1', port)) == 0


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    base, out = args.tools.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    for name in ['guzzle', 'monolog']:
        ready()
        server = None
        with (out/(name+'-integration-server.log')).open('w') as log:
            try:
                if name == 'guzzle':
                    if any(listening(port) for port in (8126, 10000)):
                        raise ValueError('declared test port occupied; leave existing service untouched')
                    receipt = json.loads((base/'runtime-instrumentation.json').read_text())
                    if hashlib.sha256((base/'guzzle/tests/Server.php').read_bytes()).hexdigest() != receipt['instrumented_sha256']:
                        raise ValueError('prepare workspace-only server log redirection first')
                    server = subprocess.Popen([str(base/'runtime/php'), '-n', '-S', '127.0.0.1:10000', '-t',
                                               str(base/'guzzle/vendor/guzzle/client-integration-tests/fixture')],
                                              stdout=log, stderr=subprocess.STDOUT)
                    for _ in range(50):
                        if server.poll() is not None:
                            raise ValueError('own integration server exited during startup')
                        if listening(10000):
                            break
                        time.sleep(.1)
                    else:
                        raise ValueError('own integration server failed to start')
                subprocess.run([sys.executable, '-B', str(Path(__file__).with_name('trace.py')),
                                '--tools', str(base), '--subject', str(base/name), '--out', str(out/name)], check=True)
            finally:
                if server is not None:
                    server.terminate()
                    try:
                        server.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        server.kill()
                        server.wait()


if __name__ == '__main__':
    main()
