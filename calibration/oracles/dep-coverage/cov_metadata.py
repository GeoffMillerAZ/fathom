#!/usr/bin/env python3
"""Aggregate input-contract audit. Never emits a subject path or filename."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
import subprocess

PATH = re.compile(r'[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*\Z')


def classify(name):
    classes = []
    if not PATH.fullmatch(name):
        if any(ord(c) > 127 for c in name):
            classes.append('non-ASCII')
        if '[' in name or ']' in name:
            classes.append('square brackets')
        if '(' in name or ')' in name:
            classes.append('parentheses')
        if any(ord(c) < 128 and c not in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._@+~ -/[]()' for c in name):
            classes.append('other excluded punctuation')
    if len(name.encode()) > 512:
        classes.append('over 512 bytes')
    if any(p in ('', '.', '..') for p in name.split('/')):
        classes.append('invalid path segment')
    return classes


def audit(root, subjects):
    result = {}
    for pool, pins in subjects.items():
        rows = {}
        for s in pins:
            repo = root / pool / 'primary' / s['name']
            records = subprocess.check_output(['git', '-C', str(repo), 'ls-tree', '-rz', s['commit']], stderr=subprocess.DEVNULL)
            selected = []
            for record in records.split(b'\0'):
                if b'\t' not in record:
                    continue
                meta, name = record.split(b'\t', 1)
                if meta.split()[0] not in (b'100644', b'100755'):
                    continue
                name = name.decode()
                if any(name.startswith(p) for p in s['prefixes']) and any(name.endswith(p) for p in s['suffixes']):
                    selected.append(name)
            selected.sort()
            digest = hashlib.sha256(b''.join(p.encode() + b'\0' for p in selected)).hexdigest()
            if len(selected) != s['files'] or s.get('selectionSHA256') and digest != s['selectionSHA256']:
                raise ValueError('selection differs from pin')
            row = rows.setdefault(s['dialect'], {'selected': 0, 'rejected_by_path_metadata': 0, 'classes': Counter()})
            row['selected'] += len(selected)
            for name in selected:
                classes = classify(name)
                row['rejected_by_path_metadata'] += bool(classes)
                row['classes'].update(classes)
        result[pool] = rows
    return result


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('root', type=Path)
    ap.add_argument('--subjects', type=Path, help='alternate pinned manifest, including future Go subjects')
    args = ap.parse_args()
    try:
        manifest = args.subjects or args.root / 'subjects.json'
        result = audit(args.root, json.loads(manifest.read_text()))
        print(json.dumps(result, sort_keys=True, separators=(',', ':')))
    except Exception:
        raise SystemExit('COV metadata audit failed; no per-file diagnostics emitted')
