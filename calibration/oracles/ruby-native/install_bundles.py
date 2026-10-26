#!/usr/bin/env python3
"""Install each original subject's declared bundle in an isolated cache."""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import signal
import subprocess

from bootstrap import environment, ready


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', type=Path, required=True)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise SystemExit('run with nice -n 10')
    base = args.tools.resolve()
    state = json.loads((base/'bootstrap.json').read_text())
    ruby = base/state['ruby']['binary']
    receipt_path = base/'bundles.json'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}
    jobs = [('sinatra', 'sinatra'), ('sinatra-contrib', 'sinatra/sinatra-contrib'),
            ('rack-protection', 'sinatra/rack-protection'), ('jekyll', 'jekyll')]
    for name, path in jobs:
        if receipt.get(name, {}).get('exit') == 0:
            continue
        ready()
        root = base/path
        env = environment(base, ruby)
        env.update(BUNDLE_GEMFILE=str(root/'Gemfile'), BUNDLE_PATH=str(base/'bundles'/name),
                   BUNDLE_APP_CONFIG=str(base/'bundle-config'/name), BUNDLE_JOBS='2', BUNDLE_RETRY='2')
        if (base/'sdk/openssl/include/openssl/ssl.h').is_file():
            env['BUNDLE_BUILD__OPENSSL'] = '--with-openssl-dir='+str(base/'sdk/openssl')
        print('install declared bundle', name, flush=True)
        with (base/'logs'/(name+'-bundle.log')).open('w') as log:
            process = subprocess.Popen([str(ruby), str(ruby.parent/'bundle'), 'install', '--jobs', '2', '--retry', '2', '--prefer-local'],
                                       cwd=root, env=env, stdout=log, stderr=log, start_new_session=True)
            try:
                status = process.wait(timeout=1200)
            except subprocess.TimeoutExpired:
                status = 'timeout'
            finally:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
        lock = root/'Gemfile.lock'
        receipt[name] = {'exit': status, 'gemfile_sha256': hashlib.sha256((root/'Gemfile').read_bytes()).hexdigest(),
                         'resolver_preference': 'installed compatible gems first; original dependency constraints unchanged',
                         'lock_sha256': hashlib.sha256(lock.read_bytes()).hexdigest() if lock.exists() else None,
                         'packages':dict(re.findall(r'^    ([A-Za-z0-9_.-]+) \(([^)]+)\)$', lock.read_text(), re.M)) if lock.exists() else {},
                         'original_gemfile_changed': subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--', 'Gemfile'], cwd=root).returncode != 0}
        receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')
        if status != 0:
            print('declared bundle failed', name, status, '; private log retained', flush=True)
        else:
            print('declared bundle installed', name, flush=True)
    if any(item['exit'] != 0 for item in receipt.values()):
        raise SystemExit('one or more declared bundles failed; see receipt, no success claim')


if __name__ == '__main__':
    main()
