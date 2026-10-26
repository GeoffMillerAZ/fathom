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


def snapshot(subject, require_pristine=False):
    def git(*args):
        return subprocess.check_output(['git', '-C', str(subject), *args])
    if git('rev-parse', 'HEAD').decode().strip() != SUBJECT_COMMIT:
        raise ValueError('subject commit mismatch')
    status_args = ['status', '--porcelain', '--untracked-files=all']
    if require_pristine:
        status_args.append('--ignored')
    if git(*status_args).strip():
        raise ValueError('subject checkout is not pristine' if require_pristine else 'subject checkout changed outside ignored build outputs')
    paths = sorted(p for p in git('ls-files', '-z').split(b'\0') if p.endswith((b'.kt', b'.kts')))
    digest = hashlib.sha256()
    for name in paths:
        digest.update(name + b'\0' + bytes.fromhex(sha(subject / os.fsdecode(name))))
    return {'commit': SUBJECT_COMMIT, 'kotlin_files': len(paths), 'manifest_sha256': digest.hexdigest(),
            'manifest_recipe': 'byte-sorted git ls-files .kt/.kts; SHA256(path bytes + NUL + raw SHA256(file bytes)) concatenated'}


def run(command, cwd, env, log, seconds, grace_seconds=5):
    started = time.monotonic()
    with log.open('wb') as output:
        child = subprocess.Popen(command, cwd=cwd, env=env, stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
        timed_out = False
        try:
            code = child.wait(timeout=seconds)
        except subprocess.TimeoutExpired:
            timed_out = True
            try:
                os.killpg(child.pid, signal.SIGTERM)
            except (ProcessLookupError, PermissionError):
                pass
            grace_end = time.monotonic() + grace_seconds
            while time.monotonic() < grace_end:
                child.poll()
                try:
                    os.killpg(child.pid, 0)
                except (ProcessLookupError, PermissionError):
                    break
                time.sleep(.05)
            # The launcher may have exited before a descendant. Kill the group
            # independently of launcher wait status.
            try:
                os.killpg(child.pid, signal.SIGKILL)
            except (ProcessLookupError, PermissionError):
                pass
            child.wait()
            code = 124
    return {'exit_code': code, 'timed_out': timed_out, 'wall_seconds': round(time.monotonic() - started, 3), 'log_sha256': sha(log)}


def self_test():
    import tempfile
    global SUBJECT_COMMIT
    with tempfile.TemporaryDirectory() as scratch:
        root = Path(scratch)
        (root/'release').write_text('JAVA_RUNTIME_VERSION=21\n')
        try:
            jdk_identity(root)
        except ValueError:
            pass
        else:
            raise AssertionError('wrong JDK accepted')
        subject = root/'subject'
        subject.mkdir()
        (subject/'.gitignore').write_text('build/\n')
        (subject/'source.kt').write_text('class Example\n')
        subprocess.run(['git', 'init', '-q', str(subject)], check=True)
        subprocess.run(['git', '-C', str(subject), 'add', '.'], check=True)
        subprocess.run(['git', '-C', str(subject), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@test.invalid',
                        'commit', '-qm', 'fixture'], check=True)
        saved_commit = SUBJECT_COMMIT
        try:
            SUBJECT_COMMIT = subprocess.check_output(['git', '-C', str(subject), 'rev-parse', 'HEAD'], text=True).strip()
            source_before = snapshot(subject, require_pristine=True)
            (subject/'build').mkdir()
            (subject/'build'/'warmup.bin').write_bytes(b'warmup')
            try:
                snapshot(subject, require_pristine=True)
            except ValueError:
                pass
            else:
                raise AssertionError('ignored warmup output passed pristine initial check')
            assert snapshot(subject) == source_before, 'expected ignored warmup output changed source snapshot'
        finally:
            SUBJECT_COMMIT = saved_commit
        result = run([sys.executable, '-c', 'print("ok")'], root, os.environ.copy(), root/'ok.log', 5)
        assert result['exit_code'] == 0 and not result['timed_out']
        for iteration in range(20):
            result = run([sys.executable, '-c', 'import time; time.sleep(30)'], root, os.environ.copy(), root/f'timeout-{iteration}.log', .02, grace_seconds=.02)
            assert result['exit_code'] == 124 and result['timed_out'], result
            marker = root/f'descendant-{iteration}.ready'
            child_code = ('import signal,time,pathlib; '
                          'signal.signal(signal.SIGTERM,signal.SIG_IGN); '
                          f'pathlib.Path({str(marker)!r}).write_text("ready"); time.sleep(30)')
            code = f'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c",{child_code!r}]); time.sleep(30)'
            result = run([sys.executable, '-c', code], root, os.environ.copy(), root/f'descendant-{iteration}.log', .25, grace_seconds=.02)
            assert marker.exists(), 'descendant did not start before timeout'
            assert result['exit_code'] == 124 and result['timed_out'] and result['wall_seconds'] < 1, result
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
    before = snapshot(subject, require_pristine=True)
    if scratch.is_relative_to(subject) or scratch.exists():
        raise ValueError('scratch must be a fresh path outside the subject; existing outputs are preserved')
    scratch.mkdir(parents=True)
    env = os.environ.copy()
    for key in ['GRADLE_OPTS', 'JDK_JAVA_OPTIONS', '_JAVA_OPTIONS']:
        env.pop(key, None)
    env.update(JAVA_HOME=str(jdk), PATH=str(jdk/'bin') + os.pathsep + env['PATH'], JAVA_TOOL_OPTIONS='-XX:ActiveProcessorCount=2', JAVA_OPTS='-Xmx256m -XX:ActiveProcessorCount=2', GRADLE_USER_HOME=str(scratch/'gradle'), COURSIER_CACHE=str(scratch/'coursier'), TMPDIR=str(scratch/'tmp'))
    (scratch/'tmp').mkdir(exist_ok=True)
    receipt = {'jdk': identity, 'route': args.route, 'source': before, 'pristine_initial': True,
               'producer_sha256': PRODUCER_SHA,
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
