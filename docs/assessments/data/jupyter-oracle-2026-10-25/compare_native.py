#!/usr/bin/env python3
"""Compare tier-C written syntax, optionally filtered by analyzer answer presence.

Legacy output filenames contain 'native'; neither pair content nor tier is native
resolved-reference truth. basedpyright answers here are a coverage signal only.
"""
import argparse
import json
from pathlib import Path
import subprocess
import sys


def answered_syntax_oracle(receipt):
    """Keep the pair's actual source/tier explicit; never infer an answer target."""
    pairs = sorted({(r['path'], c['from'], c['to'])
                    for r in receipt['records'] for c in r['calls']
                    if c['definitions'] and c['to']})
    unknown = sum(not c['definitions'] for r in receipt['records'] for c in r['calls'])
    return {
        'tool': 'Python AST written pairs filtered by basedpyright answer presence (diagnostic C)',
        'version': receipt['server'] + '; Python ' + receipt.get('python', 'unknown') + ' AST, existing jupyter_oracle.CallWalker written-name projection',
        'tier': 'C',
        'pair_source': 'Python AST written caller/callee names; not basedpyright target references',
        'resolved_target_truth': False,
        'references': [{'path': p, 'from': f, 'to': t} for p, f, t in pairs],
        'disclose': [
            f"Tier-C syntax filter, not resolved-reference truth: {receipt['resolved_calls']}/{receipt['calls']} enumerable sites answered; {unknown} unknown; {receipt['rejected_cells']} scanner-rejected cells; {receipt.get('opaque_magic_lines', 0)} opaque magic lines. Pair content comes from the AST walk, not definition targets. basedpyright remains C under P-T1/OL-R2/OL-R7. Precision is a lower bound with unknown sites counted as misses, not a native binding error rate."
        ],
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ['native', 'subject', 'ctags', 'indexbench', 'out']:
        ap.add_argument('--' + name, required=True)
    args = ap.parse_args()
    root = Path(args.subject).resolve()
    out = Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    repo = Path(__file__).resolve().parents[4]
    receipt = json.loads(Path(args.native).read_text())
    truth = answered_syntax_oracle(receipt)
    (out / 'native-pairs.json').write_text(json.dumps(truth, indent=2, sort_keys=True) + '\n')
    with (out / 'definitions.json').open('w') as target:
        subprocess.run([sys.executable, str(repo / 'calibration/oracles/jupyter/jupyter_oracle.py'),
                        '--subject', str(root), '--ctags', str(Path(args.ctags).resolve())],
                       stdout=target, check=True)
    with (out / 'syntax-pairs.json').open('w') as target:
        subprocess.run([sys.executable, str(repo / 'calibration/oracles/jupyter/jupyter_oracle.py'),
                        '--subject', str(root), '--refs'], stdout=target, check=True)
    sha = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    for label, oracle in [('syntax', 'syntax-pairs.json'), ('native-diagnostic', 'native-pairs.json')]:
        subprocess.run([str(Path(args.indexbench).resolve()), 'lang-oracle', '-dialect', 'jupyter',
                        '-subject', str(root), '-subject-sha', sha,
                        '-subject-name', 'wesm/pydata-book (separate modern diagnostic holdout)',
                        '-tool-oracle', str(out / 'definitions.json'), '-tool-oracle', str(out / oracle),
                        '-unmatched', str(out / (label + '-misses.json')),
                        '-out', str(out / (label + '-reading.json')),
                        '-note', 'OL-R4 separate modern holdout; written syntax and answer-presence filter are C under P-T1/OL-R2/OL-R7, not resolved native target truth; unknown sites count as misses, making filtered precision a lower bound'], check=True)
    summary = {
        'subject_sha': sha, 'native_sites': receipt['calls'],
        'native_answered': receipt['resolved_calls'],
        'unknown_sites': receipt['calls'] - receipt['resolved_calls'],
        'scanner_rejected_cells': receipt['rejected_cells'],
        'opaque_magic_lines': receipt.get('opaque_magic_lines', 0),
        'native_distinct_pairs': len(truth['references']),
        'pair_source': truth['pair_source'], 'resolved_target_truth': False,
        'filtered_precision_interpretation': 'lower bound; unknown sites counted as misses',
        'scope': 'all notebooks; separate diagnostic holdout, never canonical corpus',
        'readings': {},
    }
    for label in ['syntax', 'native-diagnostic']:
        reading = json.loads((out / (label + '-reading.json')).read_text())
        summary['readings'][label] = {k: reading[k] for k in ['metrics', 'metric_tiers', 'counts', 'grade', 'tenancy']}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
    print(json.dumps(summary, sort_keys=True))


if __name__ == '__main__':
    main()
