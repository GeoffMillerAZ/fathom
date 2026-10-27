"""Verify an application-screen receipt and every archived byte (no extraction)."""
import sys
sys.dont_write_bytecode = True
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tarfile
from recount import product_path


def check_privacy(data, label):
    # Fixed shapes make the scan independent of the verifier's machine.
    assert not re.search(rb'/(?:Users)/|(?<![\w/])/(?:home)/[^/\s"\']+', data), 'home path in ' + label
    emails = re.findall(rb'[A-Za-z0-9_.+%-]+@(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}', data)
    for email in emails:
        domain = email.rsplit(b'@', 1)[1].lower()
        if re.fullmatch(rb'(?:[0-9]+\.)+tgz', domain):
            continue  # Package-version archive notation, rather than an address.
        assert any(domain == d or domain.endswith(b'.' + d)
                   for d in [b'example.com', b'example.org', b'example.net', b'example.test', b'example.invalid']), 'email in ' + label


def reading_tables(path, receipt):
    source = path.read_text()
    subjects = receipt['subjects']
    assert f"SHA-256 `{receipt['archive']['sha256']}`" in source, 'reading archive hash'
    tables = []
    for line in source.splitlines():
        if line.startswith('|'):
            tables.append([cell.strip().replace('`', '') for cell in line.strip('|').split('|')])
    def row(first):
        found = [r for r in tables if r[0] == first and len(r) == 7 and r[1].isdigit()]
        assert len(found) == 1, ('reading row', first)
        return found[0]
    for subject in subjects:
        r = [r for r in tables if len(r) == 7 and r[1] == subject['name'] + ' / ' + subject['sha']]
        assert len(r) == 1, ('reading screen row', subject['id'])
        r = r[0]
        assert r[0] == subject['language']
        assert r[2] == f"{subject['primary_lines']:,} / {subject['band']}"
        assert [int(x) for x in r[3:6]] == [subject['product_directories'], subject['primary_fix_co_changes'], subject['c6_qualifying_symbols']]
        assert r[6] == ('kept for preparation' if subject['status'] == 'kept' else '; '.join(subject['refusals']))
        suite = subject.get('suite')
        r = [r for r in tables if len(r) == 4 and r[0] == subject['name']]
        assert len(r) == 1, ('reading suite row', subject['id'])
        r = r[0]
        def phase(value):
            return 'unrun' if not value else f"{value['seconds']} / {value['exit_code']}"
        assert r[1:] == [phase(suite.get('setup') if suite else None), phase(suite.get('warmup') if suite else None),
                          ', '.join(phase(x) for x in suite['runs']) if suite and suite['runs'] else 'unrun']
        r = [r for r in tables if len(r) == 5 and r[0] == subject['name']]
        assert len(r) == 1
        r = r[0]
        meta = subject['metadata']
        assert r[1:4] == [meta['created_at'], meta['latest_commit_at'], f"{meta['stars']:,}"]
        assert r[4] == f"{len(subject['c7_hits'])}: {subject['c7_hit_classification']}"
    for language, carried in [('Go', 0), ('Python', 1), ('TypeScript', 1)]:
        rows = [s for s in subjects if s['language'] == language]
        kept = sum(s['status'] == 'kept' for s in rows)
        dev = sum(s['status'] == 'kept' and bool(s['prior_reads']) for s in rows)
        r = row(language)
        assert r[1:] == [str(kept), str(len(rows)-kept), str(dev), str(carried+kept), str(6-carried-kept), '3 / 3']
    hashes = {r[0]: r[1] for r in tables if len(r) == 2 and r[0].startswith('logs/')}
    assert hashes == {k: v for k, v in receipt['archive']['members'].items() if k.startswith('logs/')}, 'reading log hashes'


def verify(receipt_path):
    receipt = json.loads(receipt_path.read_text())
    reading_tables(receipt_path.with_suffix('.md'), receipt)
    subjects = receipt['subjects']
    assert Counter(x['language'] for x in subjects) == {'Go': 5, 'Python': 5, 'TypeScript': 5}
    assert len({x['id'] for x in subjects}) == 15
    assert all(re.fullmatch('[0-9a-f]{40}', x['sha']) for x in subjects)
    assert all(x['product_directories'] >= 10 for x in subjects)
    assert all(x['status'] in {'kept', 'refused'} for x in subjects)
    assert Counter(x['name'].split('/')[0] for x in subjects).most_common(1)[0][1] <= 2
    archive_path = receipt_path.parent / receipt['archive']['path']
    assert hashlib.sha256(archive_path.read_bytes()).hexdigest() == receipt['archive']['sha256']
    expected = receipt['archive']['members']
    assert all(re.fullmatch('[0-9a-f]{64}', h) for h in expected.values())
    contents = {}
    with tarfile.open(archive_path, 'r:gz') as archive:
        members = archive.getmembers()
        assert len(members) == len(expected)
        assert {m.name for m in members} == set(expected)
        assert len({m.name for m in members}) == len(members)
        for member in members:
            path = PurePosixPath(member.name)
            assert member.isfile() and not path.is_absolute() and '..' not in path.parts
            assert member.uid == member.gid == 0 and not member.uname and not member.gname
            data = archive.extractfile(member).read()
            contents[member.name] = data
            assert hashlib.sha256(data).hexdigest() == expected[member.name], member.name
            check_privacy(data, member.name)
    for path, digest in receipt['instruments'].items():
        data = contents['preparation/remeasure-executed.py'] if path.endswith('/remeasure.py') else Path(path).read_bytes()
        assert hashlib.sha256(data).hexdigest() == digest, path
    for path, digest in receipt['post_measurement_instruments'].items():
        assert hashlib.sha256(contents['preparation/' + Path(path).stem + '-corrected.py']).hexdigest() == digest, path
    for subject in subjects:
        assert subject['full_history'] is True
        assert subject['c5_window'] == ['2024-10-25T00:00:00Z', '2026-10-26T00:00:00Z']
        assert subject['replayed'] == 0
        assert subject['prior_reads'] == [], 'prior subject read admitted to application pool'
        assert subject['licences'] and any(x['spdx'] in {'MIT', 'Apache-2.0', 'BSD-2-Clause', 'BSD-3-Clause', 'CC0-1.0'}
                                           for x in subject['licences'])
        for licence in subject['licences']:
            assert re.fullmatch('[0-9a-f]{64}', licence['sha256'])
        static = json.loads(contents[subject['static_artifact']])
        for key in ['sha', 'primary_lines', 'primary_fix_co_changes', 'product_directories']:
            assert subject[key] == static[key], (subject['id'], key)
        if subject['c6_artifact'] is not None:
            rows = json.loads(contents[subject['c6_artifact']])
            for row in rows:
                paths = [site.rsplit(':', 2)[0] for site in row['sites']]
                directories = {str(PurePosixPath(p).parent) for p in paths}
                assert row['calls'] == len(set(row['sites'])) == len(row['sites'])
                assert len(row['units']) == len(directories)
                assert all(product_path(p) for p in paths)
                defining = row['defining_unit']
                if subject['language'] == 'Go':
                    defining = defining.removeprefix(subject['module']).lstrip('/') or '.'
                assert defining not in directories
                assert row['qualifies'] == (row['calls'] >= 5 and len(row['units']) >= 3)
            assert subject['c6_qualifying_symbols'] == sum(x['qualifies'] for x in rows)
        suite = subject.get('suite')
        if suite is not None:
            assert suite['sha'] == subject['sha'] and suite['cpus'] == 4
            assert suite == json.loads(contents['receipts/' + subject['id'] + '-suite.json']), 'archived suite differs: ' + subject['id']
            assert suite['tracked_diff'] == []
            for stage in [suite['setup']] + [suite[k] for k in ['network_probe', 'warmup'] if k in suite] + suite['runs']:
                assert expected['logs/' + stage['log']] == stage['log_sha256']
                assert stage['seconds'] > 0 and stage['started_at'].endswith('Z')
            assert all(x['network'] == 'none' for x in suite['runs'])
        if subject['status'] != 'kept':
            assert subject['refusals'], subject['id']
            continue
        assert 5000 <= subject['primary_lines'] <= 150000
        assert subject['c6_qualifying_symbols'] >= 3
        assert subject['primary_fix_co_changes'] >= 30
        assert suite['setup']['exit_code'] == suite['warmup']['exit_code'] == 0
        assert suite['network_probe']['exit_code'] != 0
        assert len(suite['runs']) == 3
        assert all(r['exit_code'] == 0 and r['network'] == 'none' and 0 < r['seconds'] < 900
                   for r in suite['runs'])
        assert subject['lock_sha256'] == subject['staged_lock_sha256']
        manifest = Path(subject['manifest'])
        manifest_bytes = contents['manifests/' + manifest.name]
        assert hashlib.sha256(manifest_bytes).hexdigest() == subject['manifest_sha256']
        data = json.loads(manifest_bytes)
        assert data['sha'] == subject['sha'] and data['licence'] in {x['spdx'] for x in subject['licences']}
        assert data['bench']['flake_runs'] == 3
        assert data['bench']['toolchain']['lockfile_sha256'] == subject['lock_sha256']
        assert 'split' not in data['bench'], 'Q2/controller assignment remains pending'
    print('PASS: 15 subjects; archive membership, hashes, fixed-shape privacy scan, D26 counts and kept-screen receipts')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('receipt', type=Path,
                        nargs='?', default=Path('docs/bench/readings/bench-prep/apps-r1.json'))
    verify(parser.parse_args().receipt)
