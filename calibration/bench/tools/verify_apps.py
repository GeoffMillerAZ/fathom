"""Verify an application-screen receipt and every archived byte (no extraction)."""
import sys
sys.dont_write_bytecode = True
import argparse
from collections import Counter
import getpass
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import socket
import tarfile
from recount import product_path


def verify(receipt_path):
    receipt = json.loads(receipt_path.read_text())
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
    # Inspect local identity without a reverse-DNS lookup.
    tokens = {getpass.getuser(), socket.gethostname()}
    tokens = {x.encode() for x in tokens if len(x) >= 4}
    home_path = re.compile(rb'(?<![/\w])/(?:Users|home)/[^/\s"\']+')
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
            assert not home_path.search(data), 'home path in ' + member.name
            assert not any(re.search(rb'(?<![\w])' + re.escape(t) + rb'(?![\w])', data, re.I)
                           for t in tokens), 'local identity in ' + member.name
    for path, digest in receipt['instruments'].items():
        data = contents['preparation/remeasure-executed.py'] if path.endswith('/remeasure.py') else Path(path).read_bytes()
        assert hashlib.sha256(data).hexdigest() == digest, path
    for path, digest in receipt['post_measurement_instruments'].items():
        assert hashlib.sha256(Path(path).read_bytes()).hexdigest() == digest, path
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
            assert suite['platform'] == 'linux/arm64'
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
        assert manifest.is_file(), str(manifest)
        assert hashlib.sha256(manifest.read_bytes()).hexdigest() == subject['manifest_sha256']
        data = json.loads(manifest.read_text())
        assert data['sha'] == subject['sha'] and data['licence'] in {x['spdx'] for x in subject['licences']}
        assert data['bench']['flake_runs'] == 3
        assert data['bench']['toolchain']['lockfile_sha256'] == subject['lock_sha256']
        assert 'split' not in data['bench'], 'Q2/controller assignment remains pending'
    print('PASS: 15 subjects; archive membership, hashes, local-identity scan, D26 counts and kept-screen receipts')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('receipt', type=Path,
                        nargs='?', default=Path('docs/bench/readings/bench-prep/apps-r1.json'))
    verify(parser.parse_args().receipt)
