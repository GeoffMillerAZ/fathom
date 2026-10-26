"""Verify round-3 readonly Testify receipts, harness identity and committed raw logs."""
import hashlib
import json
from pathlib import Path
import tarfile

root = Path(__file__).resolve().parents[3]
reading = root / 'docs/bench/readings/bench-prep'
suite = json.loads((reading / 'testify-r3.json').read_text())
assert suite['sha'] == '87a7b9d57689f6579db2da795ab8deeab29cb724'
assert suite['setup']['exit_code'] == suite['warmup']['exit_code'] == 0
assert suite['network_probe']['exit_code'] == 7
assert len(suite['runs']) == 3
assert suite['tracked_diff'] == suite['staged_tracked_diff'] == []
for run in [suite['warmup'], *suite['runs']]:
    assert run['exit_code'] == 0 and run['network'] == 'none'
    assert run['seconds'] > 0 and run['started_at'].endswith('Z')
    for flag in ['GOFLAGS=-mod=readonly', 'GOPROXY=off', 'go test -count=1 -timeout=14m ./...']:
        assert flag in run['command']
for name, digest in suite['instrument_sha256'].items():
    assert hashlib.sha256((root / 'calibration/bench/tools' / name).read_bytes()).hexdigest() == digest
expected = {stage['log']: stage['log_sha256'] for stage in [suite['setup'], suite['network_probe'], suite['warmup'], *suite['runs']]}
archive = reading / suite['archive']['path']
assert hashlib.sha256(archive.read_bytes()).hexdigest() == suite['archive']['sha256']
with tarfile.open(archive) as tar:
    assert len(tar.getmembers()) == suite['archive']['members'] == 6
    assert {m.name for m in tar.getmembers()} == set(expected)
    for member in tar.getmembers():
        assert member.isfile() and not member.uname and not member.gname
        assert hashlib.sha256(tar.extractfile(member).read()).hexdigest() == expected[member.name]
print('PASS: readonly/offline Testify warm-up and three green trials, unchanged source, exact edited harness, six raw log hashes')
