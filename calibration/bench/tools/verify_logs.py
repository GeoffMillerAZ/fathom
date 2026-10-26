"""Verify the content-addressed historical trial archive without an external service."""
import argparse
import hashlib
import json
from pathlib import Path
import tarfile

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('--archive', required=True, type=Path)
args = parser.parse_args()
root = Path(__file__).resolve().parents[3]
receipts = json.loads((root / 'docs/bench/readings/bench-prep/receipts-r1.json').read_text())
wanted = {subject['name'] + '-run' + str(i) + '.log': run['log_sha256']
          for subject in receipts['subjects'].values()
          for i, run in enumerate(subject['bench']['evidence']['suite']['runs'], 1)}
assert len(wanted) == 30
with tarfile.open(args.archive) as archive:
    members = archive.getmembers()
    assert len(members) == 30
    assert {member.name for member in members} == set(wanted)
    for member in members:
        assert member.isfile()
        with archive.extractfile(member) as stream:
            assert hashlib.sha256(stream.read()).hexdigest() == wanted[member.name]
print('PASS: all 30 historical trial logs match their committed receipt hashes')
