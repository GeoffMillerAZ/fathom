#!/usr/bin/env python3
"""Bounded, checksum-pinned Kotlin producer audit. All artifacts stay in scratch."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

PRODUCER_SHA = '5248136592d19ea2c468f64ebd18587d32de7c961536ff19346f8a025af1d13c'
SUBJECT_COMMIT = 'a68a26884952875722c1802ef42b382876d7d056'
MODULES = [':exposed-core:compileKotlin', ':exposed-dao:compileKotlin', ':exposed-jdbc:compileKotlin']
GRADLE = ['--no-daemon', '--max-workers=2', '-Dorg.gradle.jvmargs=-Xmx640m -XX:ActiveProcessorCount=2', '-Pkotlin.compiler.execution.strategy=in-process']


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def jdk_identity(jdk):
    release = dict(line.split('=', 1) for line in (jdk/'release').read_text().splitlines() if '=' in line)
    expected = {'IMPLEMENTOR': 'Eclipse Adoptium', 'IMPLEMENTOR_VERSION': 'Temurin-17.0.21+9', 'JAVA_RUNTIME_VERSION': '17.0.21+9', 'OS_ARCH': 'aarch64', 'OS_NAME': 'Darwin'}
    actual = {key: release.get(key, '').strip('"') for key in expected}
    if actual != expected:
        raise ValueError('JDK distribution/version mismatch')
    version = subprocess.run([str(jdk/'bin/java'), '-version'], capture_output=True, text=True, timeout=10, check=True)
    if '17.0.21' not in version.stderr or 'Temurin-17.0.21+9' not in version.stderr:
        raise ValueError('JDK executable version mismatch')
    return {'release': actual, 'java_version': version.stderr.strip(), 'java_binary_sha256': sha(jdk/'bin/java')}


def snapshot(subject):
    def git(*args):
        return subprocess.check_output(['git', '-C', str(subject), *args])
    if git('rev-parse', 'HEAD').decode().strip() != SUBJECT_COMMIT:
        raise ValueError('subject commit mismatch')
    if git('status', '--porcelain', '--untracked-files=all').strip():
        raise ValueError('subject checkout is not clean')
    paths = sorted(p for p in git('ls-files', '-z').split(b'\0') if p.endswith((b'.kt', b'.kts')))
    digest = hashlib.sha256()
    for name in paths:
        digest.update(name + b'\0' + bytes.fromhex(sha(subject / os.fsdecode(name))))
    return {'commit': SUBJECT_COMMIT, 'kotlin_files': len(paths), 'manifest_sha256': digest.hexdigest(),
            'manifest_recipe': 'byte-sorted git ls-files .kt/.kts; SHA256(path bytes + NUL + raw SHA256(file bytes)) concatenated'}


def run(command, cwd, env, log, seconds):
    started = time.monotonic()
    with log.open('wb') as output:
        child = subprocess.Popen(command, cwd=cwd, env=env, stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        timed_out = False
        try:
            code = child.wait(timeout=seconds)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(child.pid, signal.SIGTERM)
            grace_end = time.monotonic() + 5
            while time.monotonic() < grace_end:
                child.poll()
                try:
                    os.killpg(child.pid, 0)
                except ProcessLookupError:
                    break
                time.sleep(.05)
            # The launcher may have exited before a descendant. Kill the group
            # independently of launcher wait status.
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            child.wait()
            code = 124
    return {'exit_code': code, 'timed_out': timed_out, 'wall_seconds': round(time.monotonic() - started, 3), 'log_sha256': sha(log)}


def self_test():
    import tempfile
    with tempfile.TemporaryDirectory() as scratch:
        root = Path(scratch)
        (root/'release').write_text('JAVA_RUNTIME_VERSION=21\n')
        try:
            jdk_identity(root)
        except ValueError:
            pass
        else:
            raise AssertionError('wrong JDK accepted')
        result = run([sys.executable, '-c', 'print("ok")'], root, os.environ.copy(), root/'ok.log', 5)
        assert result['exit_code'] == 0 and not result['timed_out']
        result = run([sys.executable, '-c', 'import time; time.sleep(30)'], root, os.environ.copy(), root/'timeout.log', .05)
        assert result['exit_code'] == 124 and result['timed_out']
        code = 'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c", "import signal,time; signal.signal(signal.SIGTERM,signal.SIG_IGN); time.sleep(30)"]); time.sleep(30)'
        result = run([sys.executable, '-c', code], root, os.environ.copy(), root/'descendant.log', .3)
        assert result['exit_code'] == 124 and result['timed_out'] and result['wall_seconds'] < 7
    print('RUNNER SELF-TEST PASS')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--self-test', action='store_true')
    parser.add_argument('--producer', type=Path)
    parser.add_argument('--jdk', type=Path)
    parser.add_argument('--subject', type=Path)
    parser.add_argument('--scratch', type=Path)
    parser.add_argument('--route', choices=['direct', 'prebuilt'], default='direct')
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not all([args.producer, args.jdk, args.subject, args.scratch]):
        parser.error('--producer, --jdk, --subject and --scratch required')
    producer, jdk, subject, scratch = (p.resolve() for p in [args.producer, args.jdk, args.subject, args.scratch])
    if sha(producer) != PRODUCER_SHA:
        raise ValueError('producer checksum mismatch')
    identity = jdk_identity(jdk)
    before = snapshot(subject)
    scratch.mkdir(parents=True, exist_ok=True)
    if (scratch/'index.scip').exists():
        raise ValueError('scratch already contains an index; use fresh scratch')
    env = os.environ.copy()
    for key in ['GRADLE_OPTS', 'JDK_JAVA_OPTIONS', '_JAVA_OPTIONS']:
        env.pop(key, None)
    env.update(JAVA_HOME=str(jdk), PATH=str(jdk/'bin') + os.pathsep + env['PATH'], JAVA_TOOL_OPTIONS='-XX:ActiveProcessorCount=2', JAVA_OPTS='-Xmx256m -XX:ActiveProcessorCount=2', GRADLE_USER_HOME=str(scratch/'gradle'), COURSIER_CACHE=str(scratch/'coursier'), TMPDIR=str(scratch/'tmp'))
    (scratch/'tmp').mkdir(exist_ok=True)
    receipt = {'jdk': identity, 'route': args.route, 'source': before, 'producer_sha256': PRODUCER_SHA,
               'caps': {'timeout_seconds_combined': 1200, 'indexer_heap_mb': 256, 'gradle_heap_mb': 640, 'active_processors': 2, 'gradle_workers': 2, 'nice': 10}, 'phases': []}
    budget = 1200.0
    commands = []
    if args.route == 'prebuilt':
        commands.append(('warmup', ['./gradlew', 'help', *GRADLE]))
    index_command = [str(producer), 'index', '--output', str(scratch/'index.scip'), '--', *GRADLE]
    if args.route == 'prebuilt':
        index_command += ['--init-script', str(Path(__file__).resolve().with_name('skip-prebuilt-buildsrc.gradle'))]
    commands.append(('index', index_command + MODULES))
    try:
        for name, command in commands:
            while os.getloadavg()[0] > 9:
                time.sleep(10)
            if budget <= 0:
                raise TimeoutError('combined active command budget exhausted')
            result = run(['nice', '-n', '10', *command], subject, env, scratch/(name+'.log'), budget)
            budget -= result['wall_seconds']
            # Exact argv contains caller paths; local receipt remains in scratch.
            result.update(name=name, argv=['nice', '-n', '10', *command])
            receipt['phases'].append(result)
            if result['exit_code']:
                break
    finally:
        receipt['source_after'] = snapshot(subject)
        receipt['source_unchanged'] = receipt['source_after'] == before
        receipt['scip_index_emitted'] = (scratch/'index.scip').is_file()
        (scratch/'receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
    if not receipt['source_unchanged']:
        raise ValueError('source changed during audit')
    sys.exit(receipt['phases'][-1]['exit_code'])


if __name__ == '__main__':
    main()
