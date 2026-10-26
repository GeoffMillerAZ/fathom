import sys
sys.dont_write_bytecode = True
"""Lock-bound preparation and offline trials; configuration supplies package-native commands."""
from bench_paths import job_dir
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import datetime
import uuid

IMAGES = {
    'TypeScript': 'node@sha256:63538dfe497cc29dfafdda25ada09b46db68a3f4cc205abf1f5d6bf0913f3384',
    'Python': 'python@sha256:14a4933e6cef421424f7a66d89a3f4eafcbd0d90e0a822251f298e05132dc735',
    'Go': 'golang@sha256:972b4110ba0f47539a975c31e52c64765e1ff19f2d74469ae4a595c6676e80a5',
}

def shell_command(language, stage, command):
    # Keep all caller-supplied semicolon commands inside the last AND operand.
    # Otherwise a staging/cd failure can skip one export and still run setup.
    if stage == 'setup':
        command = '(test -d /work/source/.git || ( mkdir -p /work/source && git -C /subject ls-files -z > /work/tracked-files && tar -C /subject --null -T /work/tracked-files -cf /work/source.tar && tar --touch --no-same-owner -C /work/source -xf /work/source.tar && cp -R /subject/.git /work/source/.git )) && cd /work/source && ( ' + command + ' )'
    elif stage != 'network-probe':
        command = 'cd /work/source && ( ' + command + ' )'
    if language == 'TypeScript':
        command = 'corepack enable && ( ' + command + ' )'
    return 'set -e; ' + command

def measure(config, job, trials_only=False):
    name = config['id']
    source = Path(config['source']).resolve()
    work = job / 'builds' / name
    work.mkdir(parents=True, exist_ok=True)
    (work / 'tmp').mkdir(exist_ok=True)
    def run(stage, command, offline):
        container_name = 'bench-trial-' + uuid.uuid4().hex
        argv = ['docker', 'run', '--rm', '--name', container_name, '--network', 'none' if offline else 'bridge',
                '--cpus', '4', '--tmpfs', '/work/tmp:rw,exec', '-v', str(source) + ':/subject:ro', '-v', str(work) + ':/work',
                '-v', str(Path(__file__).resolve().parents[2] / 'bench.d') + ':/bench-inputs:ro',
                '-w', '/subject', '-e', 'TMPDIR=/work/tmp', '-e', 'COREPACK_HOME=/work/corepack',
                '-e', 'UV_PROJECT_ENVIRONMENT=/work/venv', '-e', 'UV_CACHE_DIR=/work/uv-cache',
                '-e', 'UV_PYTHON_DOWNLOADS=never', '-e', 'GIT_CONFIG_COUNT=3',
                '-e', 'GIT_CONFIG_KEY_0=safe.directory', '-e', 'GIT_CONFIG_VALUE_0=/subject',
                '-e', 'GIT_CONFIG_KEY_1=safe.directory', '-e', 'GIT_CONFIG_VALUE_1=/subject/.git',
                '-e', 'GIT_CONFIG_KEY_2=safe.directory', '-e', 'GIT_CONFIG_VALUE_2=/work/source',
                '-e', 'PIP_CACHE_DIR=/work/pip-cache', '-e', 'GOPATH=/work/gopath',
                '-e', 'GOTOOLCHAIN=local', '-e', 'GOFLAGS=-mod=readonly', '-e', 'GOMAXPROCS=4']
        if config['language'] == 'Go':
            argv += ['-e', 'GOMODCACHE=/work/gomodcache', '-e', 'GOCACHE=/work/gocache']
        if offline:
            argv += ['-e', 'GOPROXY=off', '-e', 'PIP_NO_INDEX=1', '-e', 'UV_OFFLINE=1',
                     '-e', 'npm_config_offline=true', '-e', 'PNPM_CONFIG_OFFLINE=true',
                     '-e', 'pnpm_config_verify_deps_before_run=warn', '-e', 'COREPACK_ENABLE_NETWORK=0',
                     '-e', 'HTTP_PROXY=', '-e', 'HTTPS_PROXY=', '-e', 'ALL_PROXY=']
        command = shell_command(config['language'], stage, command)
        argv += [config.get('image', IMAGES[config['language']]), 'sh', '-c', command]
        lease = (job / 'offline-trials.lock').open('w')
        if offline:
            fcntl.flock(lease, fcntl.LOCK_EX)
        started_at = datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z')
        start = time.monotonic()
        log = job / 'logs' / (name + '-' + stage + '.log')
        with log.open('wb') as stream:
            try:
                code = subprocess.run(argv, stdout=stream, stderr=stream, timeout=900).returncode
            except subprocess.TimeoutExpired:
                subprocess.run(['docker', 'kill', container_name], capture_output=True)
                code = 124
            except KeyboardInterrupt:
                subprocess.run(['docker', 'kill', container_name], capture_output=True)
                raise
        result = {'command': command, 'seconds': round(time.monotonic() - start, 3),
                  'started_at': started_at, 'exit_code': code, 'network': 'none' if offline else 'bridge',
                  'log': log.name, 'log_sha256': hashlib.sha256(log.read_bytes()).hexdigest()}
        lease.close()
        print(name, stage, result['seconds'], code, flush=True)
        return result
    pin = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
    lock = source / config['lock']
    result = {k: v for k, v in config.items() if k != 'source'}
    result.update(sha=pin, image=config.get('image', IMAGES[config['language']]), platform='linux/arm64', cpus=4,
                  lockfile_sha256=hashlib.sha256(lock.read_bytes()).hexdigest())
    if trials_only:
        prior = json.loads((job / 'receipts' / (name + '-suite.json')).read_text())
        assert prior['sha'] == pin and prior['lockfile_sha256'] == result['lockfile_sha256']
        result['setup'] = prior['setup']
    else:
        result['setup'] = run('setup', config['setup'], False)
    result['runs'] = []
    receipt = job / 'receipts' / (name + '-suite.json')
    def checkpoint():
        receipt.write_text(json.dumps(result, indent=2) + '\n')
    checkpoint()
    if result['setup']['exit_code'] == 0:
        result['network_probe'] = run('network-probe', 'curl --noproxy "*" -I --connect-timeout 2 --max-time 3 https://1.1.1.1', True)
        result['warmup'] = run('warmup', config['test'], True)
        checkpoint()
        if result['warmup']['exit_code'] != 0 and config.get('stop_on_failed_warmup'):
            result['stopped_after_failed_warmup'] = True
        else:
            for i in range(1, 4):
                result['runs'].append(run('run' + str(i), config['test'], True))
                checkpoint()
    result['tracked_diff'] = subprocess.check_output(['git', '-C', str(source), 'diff', '--name-only', pin], text=True).splitlines()
    assert hashlib.sha256(lock.read_bytes()).hexdigest() == result['lockfile_sha256'], 'lock changed'
    (job / 'receipts' / (name + '-suite.json')).write_text(json.dumps(result, indent=2) + '\n')
    return result

if __name__ == '__main__':
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('config', type=Path)
    parser.add_argument('--trials-only', action='store_true', help='reuse successful setup receipt at the same pin and lock; rerun warm-up and three trials')
    args = parser.parse_args()
    job = job_dir()
    for config in json.loads(args.config.read_text()):
        measure(config, job, args.trials_only)
