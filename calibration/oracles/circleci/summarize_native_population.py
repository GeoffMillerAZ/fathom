"""Publish aggregate r3 vendor population evidence without subject contents."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-results', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads(Path(__file__).with_name('lsp-protocol-r3.json').read_text())
    summaries = []
    total = Counter()
    for subject in protocol['subjects']:
        raw = (args.results / (subject['name'].split('/')[-1] + '-lsp.json')).read_bytes()
        data = json.loads(raw)
        if data['subject_sha'] != subject['sha'] or data['binary_sha256'] != protocol['binary_sha256']:
            raise SystemExit('native subject/binary identity mismatch')
        if not all(f['all_declarations_queried'] for f in data['files']):
            raise SystemExit('incomplete declaration population')
        declarations = [d for f in data['files'] for d in f['declarations']]
        refs = [r for d in declarations for r in d['references']]
        classifications = Counter(r['classification'] for r in refs)
        if set(classifications) - {'local-use', 'workflow-executor-parameter', 'same-name-workflow-job-oracle-error'}:
            raise SystemExit('unclassified native answer')
        counts = dict(declarations=len(declarations), returned_locations=len(refs), **classifications)
        total.update(counts)
        summaries.append(dict(subject=subject['name'], sha=subject['sha'], source_shard=subject['source_shard'],
                              counts=counts, native_report_sha256=hashlib.sha256(raw).hexdigest()))
    compiler_raw = (args.results / 'compiler-adjudication.json').read_bytes()
    compiler = json.loads(compiler_raw)
    if not all(c['agrees'] for c in compiler['checks']):
        raise SystemExit('compiler disagreement')
    result = dict(protocol='r3', tool=protocol['tool'], version=protocol['version'],
                  binary_sha256=protocol['binary_sha256'], totals=dict(total), subjects=summaries,
                  compiler_checks=len(compiler['checks']), compiler_agreements=compiler['agreements'],
                  compiler_report_sha256=hashlib.sha256(compiler_raw).hexdigest(),
                  owner_rule='workflow executor parameter belongs to the invoked job declaration; deduplicate owner-target pairs',
                  namespace_error='vendor command references also return the same-name workflow job entry; compiler adjudicates',
                  tier='A for the complete evaluated reference family, T-R7/P-T1')
    args.out.write_text(json.dumps(result, indent=2)+'\n')
