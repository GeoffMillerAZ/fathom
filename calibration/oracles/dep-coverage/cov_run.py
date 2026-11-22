#!/usr/bin/env python3
"""COV-1 checkout preparation and pinned Python-oracle dispatch.

No subject code is executed. Per-file truth goes to the caller's pipe.
"""
import argparse
from collections import Counter
import contextlib
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent


def python_subjects():
    pins = json.loads((HERE.parent / 'deps-python/pins.json').read_text())['subjects']
    return [dict(p, dialect='python', prefixes=[''], suffixes=['.py']) for p in pins]


def subjects():
    dev = python_subjects() + json.loads((HERE.parent / 'deps-tsjs/pins.json').read_text())['subjects']
    return {'development': dev, 'holdout': json.loads((HERE / 'cov_holdout.json').read_text())}


def git(*args):
    return subprocess.check_output(['git', *map(str, args)], stderr=subprocess.DEVNULL)


def prepare(args):
    dest = args.destination.resolve()
    dest.mkdir(parents=True, exist_ok=True)
    for pool, pins in subjects().items():
        for p in pins:
            source = (args.holdout_source / p['name'] if pool == 'holdout' else
                      args.headless_source if p['name'] == 'headlessui' else args.cache / (p['name'] + '.git'))
            for arm in ('primary', 'relocated'):
                root = dest / pool / arm / p['name']
                root.parent.mkdir(parents=True, exist_ok=True)
                if not root.exists():
                    git('clone', '--quiet', '--shared', '--no-checkout', source.resolve(), root)
                    git('-C', root, 'remote', 'set-url', 'origin', p['url'])
                    git('-C', root, 'checkout', '--quiet', '--detach', p['commit'])
                if git('-C', root, 'rev-parse', 'HEAD').decode().strip() != p['commit']:
                    raise ValueError('checkout pin mismatch')
                if git('-C', root, 'remote', 'get-url', 'origin').decode().strip() != p['url']:
                    raise ValueError('checkout origin mismatch')
                if subprocess.run(['git', '-C', str(root), 'symbolic-ref', '-q', 'HEAD'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode != 1:
                    raise ValueError('checkout must be detached')
                if git('-C', root, 'status', '--porcelain', '--untracked-files=all', '--ignored=matching'):
                    raise ValueError('checkout is not clean')
    (dest / 'subjects.json').write_text(json.dumps(subjects(), sort_keys=True, indent=2) + '\n')
    print('COV checkouts prepared: development=10 holdout=5 arms=2')


def python_oracle():
    spec = importlib.util.spec_from_file_location('cov_pinned_python', HERE.parent / 'deps-python/modulefinder_oracle.py')
    oracle = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(oracle)
    return oracle


def python_rows(oracle, root, subject, names, *, declarative_fromlist):
    # Native diagnostics can contain filenames. Keep them inside this wrapper.
    with open(os.devnull, 'w') as sink, contextlib.redirect_stderr(sink):
        return oracle.read_rows(root, subject, names, declarative_fromlist=declarative_fromlist)


def python_truth(args):
    if args.declarative_fromlist and args.pool != 'development':
        raise ValueError('declarative from-list truth is development-only')
    oracle = python_oracle()
    oracle.pin(declarative_fromlist=args.declarative_fromlist)
    pins = python_subjects() if args.pool == 'development' else subjects()[args.pool]
    subject = next(p for p in pins if p['name'] == args.subject and p['dialect'] == 'python')
    root = args.root.resolve(strict=True)
    names, before = oracle.contents(root, subject)
    rows = python_rows(oracle, root, subject, names, declarative_fromlist=args.declarative_fromlist)
    if (names, before) != oracle.contents(root, subject):
        raise ValueError('oracle input drift')
    print(canonical({'manifest': before, 'rows': rows}))


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'))


def compare_python(frozen, extension):
    paths = [r['path'] for r in frozen]
    if paths != [r['path'] for r in extension] or len(paths) != len(set(paths)):
        raise ValueError('oracle modes returned different file populations')
    counts = dict.fromkeys(('agree', 'disagree', 'lost', 'gained', 'neither'), 0)
    gaps = Counter()
    disagreements = []
    for old, new in zip(frozen, extension, strict=True):
        before, after = old['gap'], new['gap']
        gaps[before, after] += 1
        if not before and not after:
            if old == new:
                counts['agree'] += 1
            else:
                counts['disagree'] += 1
                disagreements.append(old['path'])
        elif not before:
            counts['lost'] += 1
        elif not after:
            counts['gained'] += 1
        else:
            counts['neither'] += 1
    return {
        'files': len(paths), 'counts': counts, 'disagreements': disagreements,
        'gap_transitions': [[old, new, count] for (old, new), count in sorted(gaps.items())],
        'frozen_sha256': hashlib.sha256(canonical(frozen).encode()).hexdigest(),
        'extension_sha256': hashlib.sha256(canonical(extension).encode()).hexdigest(),
    }


def python_overlap(args):
    oracle = python_oracle()
    pin = oracle.pin(declarative_fromlist=True)
    totals, gaps = Counter(), Counter()
    reports = {}
    for subject in python_subjects():
        name = subject['name']
        root = (args.root / name).resolve(strict=True)
        names, manifest = oracle.contents(root, subject)
        frozen = python_rows(oracle, root, subject, names, declarative_fromlist=False)
        extension = python_rows(oracle, root, subject, names, declarative_fromlist=True)
        if (names, manifest) != oracle.contents(root, subject):
            raise ValueError('oracle input drift')
        report = compare_python(frozen, extension)
        reports[name] = dict(report, commit=subject['commit'], roots=subject['roots'], manifest=manifest)
        totals.update(report['counts'])
        gaps.update({(old, new): count for old, new, count in report['gap_transitions']})

    print(canonical({
        'pin': {key: pin[key] for key in ('implementation', 'version', 'modulefinder_sha256')},
        'files': sum(totals.values()), 'counts': totals, 'subjects': reports,
        'gap_transitions': [[old, new, count] for (old, new), count in sorted(gaps.items())],
    }))
    return int(bool(totals['disagree'] or totals['lost']))


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    sub = ap.add_subparsers(dest='command', required=True)
    prep = sub.add_parser('prepare')
    prep.add_argument('destination', type=Path)
    prep.add_argument('--cache', type=Path, required=True)
    prep.add_argument('--holdout-source', type=Path, required=True)
    prep.add_argument('--headless-source', type=Path, required=True)
    truth = sub.add_parser('python-truth')
    truth.add_argument('pool', choices=['development', 'holdout'])
    truth.add_argument('subject')
    truth.add_argument('root', type=Path)
    truth.add_argument('--declarative-fromlist', action='store_true')
    overlap = sub.add_parser('python-overlap')
    overlap.add_argument('root', type=Path, help='pinned development checkout cohort')
    args = ap.parse_args()
    if args.command == 'prepare':
        prepare(args)
    elif args.command == 'python-truth':
        python_truth(args)
    else:
        return python_overlap(args)


if __name__ == '__main__':
    os.environ.update(GIT_CONFIG_COUNT='2', GIT_CONFIG_KEY_0='core.hooksPath', GIT_CONFIG_VALUE_0='/dev/null',
                      GIT_CONFIG_KEY_1='init.templateDir', GIT_CONFIG_VALUE_1='/dev/null', GIT_TERMINAL_PROMPT='0')
    try:
        sys.exit(main())
    except Exception:
        # Neither traceback nor native stderr may expose holdout per-file data.
        print('COV runner failed; no per-file diagnostics emitted', file=sys.stderr)
        sys.exit(1)
