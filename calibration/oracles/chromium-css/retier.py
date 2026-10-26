#!/usr/bin/env python3
"""Copy a frozen CSS oracle bundle, correcting only its companion tier (C2-F1)."""
import argparse
import hashlib
import json
from pathlib import Path
import tempfile


def digest(data):
    return hashlib.sha256(data).hexdigest()


def classify(primary, destination):
    primary = Path(primary)
    destination = Path(destination)
    sources = {
        'primary': primary,
        'source_binding': primary.with_suffix('.source.json'),
        'ranges': primary.with_suffix('.ranges.json'),
    }
    blobs = {key: path.read_bytes() for key, path in sources.items()}
    original = json.loads(blobs['ranges'])
    if original.get('tier') != 'A' or original.get('tool') != 'Chromium DevTools Protocol CSS.setStyleTexts':
        raise ValueError('expected the frozen tier-A Chromium proposal companion')
    if not isinstance(original.get('ranges'), list):
        raise ValueError('missing frozen range entries')
    corrected = {**original, 'tier': 'C'}
    derived = {**blobs, 'ranges': (json.dumps(corrected, indent=2) + '\n').encode()}
    targets = {key: destination / path.name for key, path in sources.items()}
    if any(path.exists() for path in targets.values()):
        raise ValueError('refusing to overwrite classification copies')
    destination.mkdir(parents=True, exist_ok=True)
    for key, path in targets.items():
        with path.open('xb') as stream:
            stream.write(derived[key])
    return {
        'correction': 'C2-F1: tier C for the authored-proposal population; no oracle rerun',
        'original_sha256': {key: digest(body) for key, body in blobs.items()},
        'classification_copy_sha256': {key: digest(body) for key, body in derived.items()},
        'range_entries_unchanged': len(original['ranges']),
        'only_changed_companion_field': 'tier: A -> C',
    }


def self_test():
    with tempfile.TemporaryDirectory() as temporary:
        base = Path(temporary)
        primary = base / 'fixture.json'
        primary.write_text('{"tool":"synthetic","tier":"A","definitions":[]}\n')
        primary.with_suffix('.source.json').write_text('{"synthetic":"source binding"}\n')
        ranges = {'tool': 'Chromium DevTools Protocol CSS.setStyleTexts', 'tier': 'A',
                  'version': 'synthetic', 'ranges': [{'path': 'fixture.css', 'name': 'x', 'line': 1, 'end': 2}]}
        primary.with_suffix('.ranges.json').write_text(json.dumps(ranges))
        receipt = classify(primary, base / 'copy')
        output = json.loads((base / 'copy/fixture.ranges.json').read_text())
        if output != {**ranges, 'tier': 'C'}:
            raise AssertionError('classification changed range data')
        for key in ('primary', 'source_binding'):
            if receipt['original_sha256'][key] != receipt['classification_copy_sha256'][key]:
                raise AssertionError('classification changed frozen input')
        try:
            classify(primary, base / 'copy')
        except ValueError:
            pass
        else:
            raise AssertionError('classification copy overwrote output')
    print('classification-copy controls passed')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--primary', type=Path)
    parser.add_argument('--out-dir', type=Path)
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if args.primary is None or args.out_dir is None:
        parser.error('--primary and --out-dir are required')
    print(json.dumps(classify(args.primary, args.out_dir), indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
