#!/usr/bin/env python3
"""COV-1 checkout preparation and unchanged Python-oracle dispatch.

No subject code is executed. Truth goes to the caller's pipe, never a file.
"""
import argparse
import contextlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent


def subjects():
    python = json.loads((HERE.parent / 'deps-python/pins.json').read_text())['subjects']
    for p in python:
        p.update(dialect='python', prefixes=[''], suffixes=['.py'])
    dev = python + json.loads((HERE.parent / 'deps-tsjs/pins.json').read_text())['subjects']
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


def python_truth(args):
    spec = importlib.util.spec_from_file_location('cov_pinned_python', HERE.parent / 'deps-python/modulefinder_oracle.py')
    oracle = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(oracle)
    oracle.pin()
    p = next(p for p in subjects()[args.pool] if p['name'] == args.subject and p['dialect'] == 'python')
    root = args.root.resolve(strict=True)
    names, before = oracle.contents(root, p)
    roots = [(root / r).resolve(strict=True) for r in p['roots']]
    # Existing adapter diagnostics can contain filenames. Never propagate them,
    # even on development: only aggregate gap codes leave this wrapper.
    with open(os.devnull, 'w') as sink, contextlib.redirect_stderr(sink):
        rows = [oracle.read_entry(root, roots, name) for name in names]
    if (names, before) != oracle.contents(root, p):
        raise ValueError('oracle input drift')
    json.dump({'manifest': before, 'rows': rows}, sys.stdout, sort_keys=True, separators=(',', ':'))
    print()


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
    args = ap.parse_args()
    if args.command == 'prepare':
        prepare(args)
    else:
        python_truth(args)


if __name__ == '__main__':
    os.environ.update(GIT_CONFIG_COUNT='2', GIT_CONFIG_KEY_0='core.hooksPath', GIT_CONFIG_VALUE_0='/dev/null',
                      GIT_CONFIG_KEY_1='init.templateDir', GIT_CONFIG_VALUE_1='/dev/null', GIT_TERMINAL_PROMPT='0')
    try:
        main()
    except Exception:
        # Neither traceback nor native stderr may expose holdout per-file data.
        print('COV runner failed; no per-file diagnostics emitted', file=sys.stderr)
        sys.exit(1)
