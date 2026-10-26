"""Verify round-3 static corrections and optionally the full C5 recount receipts."""
import sys
sys.dont_write_bytecode = True
import argparse
import datetime
import json
from pathlib import Path
import subprocess
from bench_paths import workspace_path

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('--receipts', type=Path)
parser.add_argument('--subjects', type=Path)
a = parser.parse_args()
root = Path(__file__).resolve().parents[3]
reading = root / 'docs/bench/readings/bench-prep'
drafts = json.loads((reading / 'draft-status-r3.json').read_text())['drafts']
assert len(drafts) == 12 and len({r['repository'] for r in drafts}) == 12
assert len({r['reason'] for r in drafts}) == 1
assert all(r['admission_manifest'] == 'deferred' for r in drafts)
assert {'pytest-dev/pytest', 'reduxjs/redux-toolkit'} <= {r['repository'] for r in drafts}
cobra = json.loads((reading / 'c7-r3.json').read_text())
assert cobra['c7'] == 'fail' and cobra['prior_reads'] == ['calibration/corpus.yaml']
assert 'spf13/cobra' in (root / 'calibration/corpus.yaml').read_text()
assert 'sha256:' in (root / 'calibration/bench.d/uv-requirements.txt').read_text()
try:
    workspace_path('/output-outside-workspace')
except ValueError:
    pass
else:
    raise AssertionError('outside-workspace output accepted')
counts = json.loads((reading / 'c5-r3.json').read_text())['candidates']
assert len(counts) == 20 and all(r['replayed'] == 0 for r in counts)
if a.receipts:
    for row in counts:
        rid = row['repository'].replace('/', '--')
        inv = json.loads((a.receipts / (rid + '-inventory-r3.json')).read_text())
        for k, v in row.items():
            if k != 'repository':
                assert inv[k] == v, (rid, k)
        start, end = [datetime.datetime.fromisoformat(s.replace('Z', '+00:00')) for s in row['c5_window']]
        dates = {}
        if a.subjects:
            raw = subprocess.check_output(['git', '-C', str(a.subjects / rid), 'log', row['sha'], '--format=%H%x09%cI'], text=True)
            dates = dict(line.split('\t', 1) for line in raw.splitlines())
        for commit in inv['history']:
            date = datetime.datetime.fromisoformat(commit['date'].replace('Z', '+00:00'))
            assert start <= date <= end
            if dates:
                assert date == datetime.datetime.fromisoformat(dates[commit['sha']].replace('Z', '+00:00'))
print('PASS: twelve equal D23 deferrals, Cobra C7/prior read, uv hash pin, workspace refusal, twenty C5 recounts' + (' and committer dates' if a.subjects else ''))
