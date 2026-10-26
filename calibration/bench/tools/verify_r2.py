"""Verify the committed round-2 receipts against the returned offline-log archive."""
import argparse
import hashlib
import json
from pathlib import Path
import tarfile

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('--archive', required=True, type=Path)
args = parser.parse_args()
root = Path(__file__).resolve().parents[3]
reading = root / 'docs/bench/readings/bench-prep'
manifest = json.loads((reading / 'offline-logs-r2.json').read_text())
assert hashlib.sha256(args.archive.read_bytes()).hexdigest() == manifest['sha256'], 'archive checksum differs'
ts = json.loads((reading / 'typescript-r2.json').read_text())['candidates']
py = json.loads((reading / 'python-r2.json').read_text())['subjects']
bands = json.loads((reading / 'size-bands-r2.json').read_text())['candidates']
snapshot = json.loads((reading / 'click-snapshot-r2.json').read_text())
suites = [row['suite'] for row in ts] + py + [row['suite'] for row in bands] + [snapshot]
expected = {}
timed = green = 0
for suite in suites:
    stages = [suite.get('network_probe'), suite.get('warmup'), *suite.get('runs', [])]
    for stage in stages:
        if stage and stage.get('log_sha256'):
            name, digest = stage['log'], stage['log_sha256']
            assert name not in expected or expected[name] == digest
            expected[name] = digest
    for trial in suite.get('runs', []):
        assert trial['network'] == 'none'
        assert trial['started_at'].endswith('Z')
        assert trial['seconds'] > 0
        timed += 1
        green += trial['exit_code'] == 0
assert timed == manifest['timed_trials'] == 42
assert green == manifest['green_trials'] == 39
assert len(expected) == manifest['members']
with tarfile.open(args.archive) as archive:
    members = archive.getmembers()
    assert len(members) == len(expected)
    assert {member.name for member in members} == set(expected)
    for member in members:
        assert member.isfile()
        with archive.extractfile(member) as stream:
            assert hashlib.sha256(stream.read()).hexdigest() == expected[member.name], member.name
print(f'PASS: {timed} timed trials ({green} green), {len(expected)} raw offline logs and archive checksum')
