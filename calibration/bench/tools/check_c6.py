"""Validate all twenty D26 decisions and optional full recount output digests."""
import argparse
import hashlib
import json
from pathlib import Path

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('--receipts', type=Path)
a = parser.parse_args()
reading = Path(__file__).resolve().parents[3] / 'docs/bench/readings/bench-prep'
rows = json.loads((reading / 'c6-r2.json').read_text())['candidates']
assert len(rows) == len({r['repository'] for r in rows}) == 20
for r in rows:
    product = r['product_only']
    assert product['qualifying_symbols'] >= 0
    assert product['disposition'] == ('pass C6 only' if product['qualifying_symbols'] >= 3 else 'refused C6')
    for witness in product['examples']:
        assert witness['qualifies'] == (witness['calls'] >= 5 and len(witness['units']) >= 3)
        assert witness['defining_unit'] not in witness['units']
    if a.receipts:
        rid = r['repository'].replace('/', '--')
        path = a.receipts / (rid + '-d26.json')
        assert hashlib.sha256(path.read_bytes()).hexdigest() == product['recount_sha256']
        full = json.loads(path.read_text())
        assert sum(x['qualifies'] for x in full) == product['qualifying_symbols']
        for witness in full:
            assert witness['calls'] == len(witness['sites'])
            assert witness['qualifies'] == (len(witness['sites']) >= 5 and len(witness['units']) >= 3)
        d24 = a.receipts / (rid + '-d24.json')
        if d24.exists():
            expected = json.loads((reading / 'c6-d24-sha256.json').read_text())['outputs']
            digest = next(x['sha256'] for x in expected if x['subject_id'] == rid)
            assert hashlib.sha256(d24.read_bytes()).hexdigest() == digest
print('PASS: all twenty D26 counts, witnesses, refusals' + (' and complete recount digests' if a.receipts else ''))
