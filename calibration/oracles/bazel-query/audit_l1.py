"""Classify L1 mismatches by source callee; never changes oracle truth or grading."""
import argparse
import collections
import json
import re
import subprocess
from pathlib import Path


def classify(subject, unmatched):
    groups = collections.defaultdict(list)
    for extra in unmatched['dialect_only']:
        lines = (subject / extra['Path']).read_text().splitlines()
        # The harness anchor is the name literal, which may follow the call opener.
        prior = '\n'.join(lines[max(0, extra['Line'] - 6):extra['Line']])
        calls = list(re.finditer(r'(?m)^\s*([\w.]+)\s*\(', prior))
        callee = calls[-1][1] if calls else '(unknown)'
        groups[callee].append(extra)
    return {'counts': {k: len(groups[k]) for k in sorted(groups)},
            'items': {k: groups[k] for k in sorted(groups)}}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-subject', type=Path, required=True)
    parser.add_argument('-unmatched', type=Path, required=True)
    args = parser.parse_args()
    sha = subprocess.check_output(['git', '-C', str(args.subject), 'rev-parse', 'HEAD'], text=True).strip()
    if sha != '3252a89f10d8e92997862167ca7d095ecda85973':
        raise SystemExit('this audit requires the recorded grpc subject pin')
    dirty = subprocess.check_output(['git', '-C', str(args.subject), 'status', '--porcelain'], text=True)
    if dirty:
        raise SystemExit('audit subject must be clean')
    print(json.dumps(classify(args.subject, json.loads(args.unmatched.read_text())), indent=2))
