#!/usr/bin/env python3
"""Run the original Bats driver without hooks in the same lane write sandbox."""
from runtime_copy import copy_subject,runtime_environment
import argparse, hashlib, json, os, re, shutil, signal, subprocess, sys
from pathlib import Path

from build_bash import ready
from collect import child_limits
from fetch import SUBJECTS


def outcomes(path):
    text = path.read_text(errors='replace')
    rows = re.findall(r'^(not ok|ok) ([0-9]+) (.*)$', text, re.M)
    plans = re.findall(r'^1\.\.([0-9]+)\s*$', text, re.M)
    if len(plans) != 1 or not rows or [int(n) for _, n, _ in rows] != list(range(1, int(plans[0]) + 1)):
        raise ValueError('whole-unit refusal: incomplete original TAP outcomes')
    failures = {name for status, _, name in rows if status == 'not ok'}
    return dict(tests=len(rows), passed=sum(s == 'ok' and not re.search(r'#\s*skip\b', n, re.I) for s, _, n in rows),
                skipped=sum(s == 'ok' and bool(re.search(r'#\s*skip\b', n, re.I)) for s, _, n in rows),
                failed=sum(s == 'not ok' for s, _, _ in rows)), failures


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ['subject', 'bash', 'out', 'cache', 'observed']:
        p.add_argument('--' + name, type=Path, required=True)
    a = p.parse_args()
    ready()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise SystemExit('nice -n 10 required')
    root, out, cache, bash = (x.resolve() for x in [a.subject, a.out, a.cache, a.bash])
    out.mkdir(parents=True, exist_ok=False)
    cache.mkdir(parents=True, exist_ok=True)
    here = Path(__file__).resolve().parent
    assert subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip() == SUBJECTS['bats'][1]
    observed = json.loads((a.observed / 'summary.json').read_text())
    for relative, digest in observed['source_sha256'].items():
        assert hashlib.sha256((root / relative).read_bytes()).hexdigest() == digest
    child = out / 'child'
    copy_subject(root, child)
    profile = out / 'writes.sb'
    profile.write_text('(version 1)(allow default)(deny file-write*)(allow file-write* ' +
                       ' '.join('(subpath ' + json.dumps(str(x)) + ')' for x in [out, cache, '/dev/null']) + ')')
    env = runtime_environment(out/'environment',{k: v for k, v in os.environ.items() if k != 'BASH_ENV' and not k.startswith('ORACLE_')})
    env.update(LC_ALL='C', PATH=str(bash.parent) + os.pathsep + env.get('PATH', ''),
               NVM_DIR=str(child), TEST_SHELL=str(bash))
    status, log = out / 'driver-status.json', out / 'child.log'
    with log.open('wb') as fh:
        result = subprocess.run([sys.executable, '-B', str(here / 'supervise.py'), '--status', str(status),
                                 '--timeout', '600', '--', '/usr/bin/sandbox-exec', '-f', str(profile),
                                 str(bash), str(child / 'bin/bats'), 'test'],
                                cwd=child, env=env, stdout=fh, stderr=fh, start_new_session=True,
                                preexec_fn=child_limits, timeout=660)
    if result.returncode != -signal.SIGKILL or not status.exists():
        raise ValueError('whole-unit refusal: owned supervisor cleanup failed')
    completion = json.loads(status.read_text())
    if completion['timeout'] or completion['driver_exit'] is None or completion['driver_exit'] < 0:
        raise ValueError('whole-unit refusal: control timed out or was signalled')
    changed = sum(not (child / r).is_file() or hashlib.sha256((child / r).read_bytes()).hexdigest() != h
                  for r, h in observed['source_sha256'].items())
    if changed:
        raise ValueError('whole-unit refusal: control changed original source')
    control, failed = outcomes(log)
    traced, traced_failed = outcomes(a.observed / 'child.log')
    public = dict(control=control, observed=traced, control_exit=completion['driver_exit'],
                  shared_failed_tests=len(failed & traced_failed), only_control_failed_tests=len(failed - traced_failed),
                  only_observed_failed_tests=len(traced_failed - failed), source_changed=False,
                  control_failed_names_sha256=hashlib.sha256('\n'.join(sorted(failed)).encode()).hexdigest(),
                  code_sha256={n: hashlib.sha256((here / n).read_bytes()).hexdigest()
                               for n in ['control.py', 'collect.py', 'supervise.py', 'runtime_copy.py']},
                  policy='Untraced original bin/bats test; same interpreter path and write sandbox; no BASH_ENV or oracle hooks; failures retained')
    shutil.rmtree(child)
    (out / 'summary.json').write_text(json.dumps(public, indent=2, sort_keys=True) + '\n')
    print(json.dumps(public, sort_keys=True))


if __name__ == '__main__':
    main()
