#!/usr/bin/env python3
"""Install isolated bench dependencies using the existing PHP subject recipe."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--runtime', action='store_true', help='restore and install the subjects complete declared test dependencies')
    args = ap.parse_args()
    base = args.tools.resolve()
    php = base/'runtime/php'
    composer = base/'composer.phar'
    env = os.environ.copy()
    env.update(COMPOSER_HOME=str(base/'composer-home'), COMPOSER_CACHE_DIR=str(base/'composer-cache'),
               COMPOSER_NO_INTERACTION='1', GIT_TERMINAL_PROMPT='0', PHP_INI_SCAN_DIR='',
               PATH=str(php.parent)+os.pathsep+env['PATH'])
    logs = base/'logs'
    logs.mkdir(exist_ok=True)
    common = ['--no-interaction', '--no-plugins', '--no-scripts', '--prefer-dist', '--ignore-platform-reqs']
    jobs = [('callsite-fork', ['install', '--no-dev', *common]),
            ('php84-fork', ['install', '--no-dev', *common]),
            ('guzzle', ['remove', '--dev', 'guzzle/client-integration-tests', *[x for x in common if x != '--prefer-dist']]),
            ('monolog', ['update', *common])]
    if args.runtime:
        jobs = [('guzzle', ['update', *common]), ('monolog', ['update', *common])]
    receipt_path = base/('runtime-install-receipt.json' if args.runtime else 'install-receipt.json')
    mode = 'runtime' if args.runtime else 'static'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}
    for name, command in jobs:
        while os.getloadavg()[0] > 6:
            print('waiting for load <=6', flush=True)
            time.sleep(30)
        root = base/name
        if args.runtime:
            original = subprocess.check_output(['git', 'show', 'HEAD:composer.json'], cwd=root)
            (root/'composer.json').write_bytes(original)
        lock = root/'composer.lock'
        if receipt.get(name, {}).get('mode') == mode and (root/'vendor/autoload.php').is_file() and lock.is_file():
            if (hashlib.sha256(lock.read_bytes()).hexdigest() == receipt[name]['composer_lock_sha256']
                    and hashlib.sha256((root/'composer.json').read_bytes()).hexdigest() == receipt[name].get('composer_json_sha256')):
                print('already installed', name, flush=True)
                continue
        if name == 'monolog':
            path = root/'composer.json'
            data = json.loads(path.read_text())
            data['config']['lock'] = True
            path.write_text(json.dumps(data, indent=4)+'\n')
        print('installing', name, flush=True)
        job_env = env.copy()
        # A shallow detached checkout has no local release tag for Composer to
        # infer. These are the subjects' actual pinned published versions.
        if name in ('guzzle', 'monolog'):
            job_env['COMPOSER_ROOT_VERSION'] = {'guzzle': '7.9.3', 'monolog': '3.9.0'}[name]
        with (logs/(name+'-composer.log')).open('w') as output:
            run = subprocess.run([str(php), '-n', str(composer), *command], cwd=root, env=job_env,
                                 stdout=output, stderr=subprocess.STDOUT)
        if run.returncode:
            raise ValueError(name+' Composer failed; inspect workspace log')
        data = json.loads(lock.read_text())
        receipt[name] = {'mode': mode, 'composer_lock_sha256': hashlib.sha256(lock.read_bytes()).hexdigest(),
                         'composer_json_sha256': hashlib.sha256((root/'composer.json').read_bytes()).hexdigest(),
                         'packages': {p['name']: p['version'] for p in data.get('packages', [])+data.get('packages-dev', [])},
                         'plugins': False, 'scripts': False,
                         'platform_requirements_ignored': True}
        receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')
        print('installed', name, flush=True)


if __name__ == '__main__':
    main()
