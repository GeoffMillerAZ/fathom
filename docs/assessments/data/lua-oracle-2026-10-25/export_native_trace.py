#!/usr/bin/env python3
"""Export only executed literal sites; native lookup supplies the frozen denominator."""
import argparse
import json
from pathlib import Path

MAX_BYTES = 8 * 1024 * 1024
MAX_EVENTS = 100000


def read(path):
    with Path(path).open('rb') as stream:
        data = stream.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise ValueError('whole trace input refused: byte cap exceeded')
    return data


def site(row):
    return (row['path'], row['line'], row['module'], row['to_path'])


def export(population, events, frozen):
    expected = sorted(map(site, frozen['imports']))
    actual = sorted(map(site, population['imports']))
    if actual != expected or len(actual) != 237 or len(set(actual)) != 237:
        raise ValueError('frozen 237-site population changed')
    if len(events) > MAX_EVENTS:
        raise ValueError('whole trace refused: event cap exceeded')
    seen = {site(row) for row in events if row.get('accepted') is True and row.get('to_path')}
    imports = [dict(zip(('path', 'line', 'module', 'to_path'), key)) for key in actual if key in seen]
    covered = len(imports)
    return {'tool': 'neovim-require-trace', 'tier': 'A' if covered * 10 >= 237 * 9 else 'B',
            'version': 'Neovim 0.12.6; luaparse 0.3.1 literal-site population',
            'disclose': [f'{covered}/237 literal in-repo sites executed; at least 90% (214 sites) required for tier A; '
                         f'{237-covered} unexecuted sites retain their denominator membership and have no emitted truth; '
                         'original subject runtime/loaders and platform guards retained'],
            'imports': imports}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--trace', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    root = Path(args.trace)
    events = []
    total = 0
    for path in sorted(root.glob('*.jsonl')):
        data = read(path)
        total += len(data)
        if total > MAX_BYTES:
            raise ValueError('whole trace refused: aggregate byte cap exceeded')
        for line in data.splitlines():
            if len(events) >= MAX_EVENTS:
                raise ValueError('whole trace refused: event cap exceeded')
            events.append(json.loads(line))
    result = export(json.loads(read(root / 'population.json')), events,
                    json.loads(read(Path(__file__).with_name('telescope-native-population.json'))))
    with Path(args.out).open('x') as out:
        out.write(json.dumps(result, sort_keys=True, indent=2) + '\n')


if __name__ == '__main__':
    main()
