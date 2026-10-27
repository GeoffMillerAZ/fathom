#!/usr/bin/env python3
"""Record the released compiler's pretty-print contract on an authored fixture."""
import argparse
import hashlib
import json
import os
import shlex
from pathlib import Path
import subprocess


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--tools', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    bash = (a.tools / 'bash-5.3/bash').resolve()
    marker = out / 'executed'
    source = out / 'input.sh'
    # Authored source and its absolute scratch path remain private.
    body = 'driver() { printf executed > ' + shlex.quote(str(marker)) + '; }; driver\n'
    source.write_text(body)
    env = {k: v for k, v in os.environ.items() if k != 'BASH_ENV'}
    env['LC_ALL'] = 'C'
    printed = subprocess.check_output([str(bash), '--noprofile', '--norc', '--pretty-print', str(source)], env=env)
    assert not marker.exists() and b'driver' in printed and b'printf' in printed
    subprocess.run([str(bash), '--noprofile', '--norc', '--pretty-print', '-c', body], env=env, check=True)
    assert marker.read_text() == 'executed'
    public = dict(version=subprocess.check_output([str(bash), '--version'], env=env, text=True).splitlines()[0],
                  binary_sha256=hashlib.sha256(bash.read_bytes()).hexdigest(),
                  file_mode_executes=False, command_string_mode_executes=True,
                  file_mode_output='normalized Bash source text; no structured source-position or resolved-target records',
                  code_sha256={'probe_contracts.py': hashlib.sha256(Path(__file__).read_bytes()).hexdigest()})
    (out / 'summary.json').write_text(json.dumps(public, indent=2, sort_keys=True) + '\n')
    print(json.dumps(public, sort_keys=True))


if __name__ == '__main__':
    main()
