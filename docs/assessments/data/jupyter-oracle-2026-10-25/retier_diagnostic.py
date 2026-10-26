#!/usr/bin/env python3
"""Re-emit the unchanged written-pair diagnostic at its actual tier C.

No extraction, matching, LSP query, notebook or environment is rerun. Recorded
counts are retained; lang-regrade recomputes the tier/grade consequences.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
from compare_native import answered_syntax_oracle


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--data', required=True)
    parser.add_argument('--indexbench', required=True)
    args = parser.parse_args()
    data = Path(args.data)
    pairs_path = data / 'modern-native-pairs.json'
    for name in ['native-probe-results.json', 'native-ml-results.json', 'native-modern-results.json']:
        path = data / name
        coverage = json.loads(path.read_text())
        coverage.update(analyzer_tier='C',
                        written_pair_source='Python AST caller/callee names, not definition target references',
                        resolved_target_truth=False,
                        claim='direct notebook LSP coverage only; independent analyzer C; no resolved target-pair truth or grade')
        path.write_text(json.dumps(coverage, indent=2, sort_keys=True) + '\n')
    receipt = json.loads((data / 'native-modern-results.json').read_text())
    truth = answered_syntax_oracle(receipt)
    previous = json.loads(pairs_path.read_text())
    if previous['references'] != truth['references']:
        raise ValueError('Pair content changed; a fresh lang-oracle measurement is required')
    pairs_path.write_text(json.dumps(truth, indent=2, sort_keys=True) + '\n')
    reading_path = data / 'modern-native-diagnostic-reading.json'
    reading = json.loads(reading_path.read_text())
    before_counts, before_metrics = reading['counts'], reading['metrics']
    for oracle in reading['oracles']:
        if oracle['tool'].startswith(('basedpyright native', 'Python AST written')):
            oracle.update({key: truth[key] for key in ['tool', 'version', 'tier']})
    for subject in reading['subjects']:
        subject['families']['refs'] = 'C'
    for metric in ['ref_precision', 'ref_recall']:
        reading['metric_tiers'][metric] = 'C'
    reading['notes'] = [note for note in reading['notes']
                        if not any(word in note for word in ['native', 'tier-A'])]
    reading['notes'].extend(truth['disclose'])
    reading['notes'].append('Review correction only: original pair content, counts, extraction and timing retained; no new native target truth or performance run.')
    reading_path.write_text(json.dumps(reading, indent=2) + '\n')
    subprocess.run([str(Path(args.indexbench).resolve()), 'lang-regrade', str(reading_path)], check=True)
    corrected = json.loads(reading_path.read_text())
    if corrected['counts'] != before_counts or corrected['metrics'] != before_metrics:
        raise ValueError('Counts or metrics changed during tier-only regrade')
    summary_path = data / 'modern-comparison-summary.json'
    summary = json.loads(summary_path.read_text())
    summary.update(pair_source=truth['pair_source'], resolved_target_truth=False,
                   filtered_precision_interpretation='lower bound; unknown sites counted as misses')
    summary['readings']['native-diagnostic'] = {
        key: corrected[key] for key in ['metrics', 'metric_tiers', 'counts', 'grade', 'tenancy']
    }
    syntax = json.loads((data / 'modern-syntax-reading.json').read_text())
    summary['readings']['syntax']['metric_tiers'] = syntax['metric_tiers']
    summary_path.write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
    manifest_path = data / 'manifest.json'
    manifest = json.loads(manifest_path.read_text())
    manifest['review_tier_correction'] = {
        'operation': 'unchanged written pair content; retained recorded counts; tier-C regrade only',
        'fresh_lsp_or_environment_run': False,
        'client_metadata_only_sha256': hashlib.sha256((data / 'native_lsp.py').read_bytes()).hexdigest(),
        'code_sha256': {name: hashlib.sha256((data / name).read_bytes()).hexdigest()
                        for name in ['compare_native.py', 'retier_diagnostic.py', 'test_comparison.py']},
    }
    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + '\n')
    print('C syntax-filter diagnostic re-emitted; counts/metrics unchanged; no native target truth claimed.')


if __name__ == '__main__':
    main()
