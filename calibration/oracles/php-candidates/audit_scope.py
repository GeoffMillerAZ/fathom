#!/usr/bin/env python3
"""Account for unindexed files as well as the harness's within-index blind pairs."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

from fetch import ready


def summarize_scope(pairs, docs, local, matched):
    outside = {p for p in pairs if p[0] not in docs}
    if not pairs or local < 0 or matched < 0 or matched+local+len(outside) > len(pairs):
        raise ValueError('inconsistent complete-population accounting')
    return {
        'written_pairs': len(pairs), 'scip_documents': len(docs),
        'documents_sha256': hashlib.sha256(json.dumps(sorted(docs), separators=(',', ':')).encode()).hexdigest(),
        'written_pairs_in_unindexed_files': len(outside),
        'within_index_blind_pairs': local, 'full_unattested_pairs': local+len(outside),
        'full_unattested_fraction': (local+len(outside))/len(pairs),
        'matched_fraction_lower_bound_with_unattested_as_misses': matched/len(pairs),
        'harness_scope': 'native SCIP documents only; full scope audit additionally counts absent documents',
        'syntax_population_source': 'complete pinned php-parser dump; denominator only, never native pair truth',
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--measurement', required=True, type=Path)
    args = ap.parse_args()
    base, out = args.tools.resolve(), args.measurement.resolve()
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    php = base/'runtime/php'
    summary = json.loads((out/'summary.json').read_text())
    for name, expected in [('guzzle', 2571), ('monolog', 4558)]:
        ready()
        syntax = out/(name+'-full-syntax.json')
        with syntax.open('w') as target, (out/(name+'-syntax.log')).open('w') as log:
            subprocess.run([str(php), '-n', str(repo/'calibration/oracles/php-parser/PhpParserDump.php'),
                            str(base/'callsite-fork/vendor/autoload.php'), str(base/name)], check=True,
                           stdout=target, stderr=log)
        raw = json.loads(syntax.read_text())
        pairs = {(p['path'], p['from'], p['to']) for p in raw['calls']}
        if len(pairs) != expected:
            raise ValueError('written population differs from the established complete syntax reading')
        for candidate, result in summary['candidates'].items():
            ready()
            with (out/(candidate+'-'+name+'-docs.log')).open('w') as log:
                run = subprocess.run([str(php), '-n', '-d', 'memory_limit=512M', '-d', 'display_errors=stderr', str(here/'scip_documents.php'),
                                      str(base/'callsite-fork/vendor/autoload.php'),
                                      str(out/(candidate+'-'+name+'.scip'))], check=True,
                                     capture_output=False, stdout=subprocess.PIPE, stderr=log, text=True)
            docs = set(json.loads(run.stdout))
            r = result['subjects'][name]
            local = r['counts'].get('calls_oracle_blind', 0)
            r['full_scope_audit'] = summarize_scope(pairs, docs, local, r['counts']['call_fathom_in_truth'])
            print(candidate, name, 'full unattested', r['full_scope_audit']['full_unattested_pairs'], '/', len(pairs), flush=True)
    (out/'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')


if __name__ == '__main__':
    main()
