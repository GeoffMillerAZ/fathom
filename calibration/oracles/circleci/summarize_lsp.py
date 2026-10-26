"""Reproduce the measured vendor-LSP comparison summary with identity checks."""
import argparse
import json
from pathlib import Path

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-results', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads(Path(__file__).with_name('lsp-protocol-r2.json').read_text())
    subjects = []
    totals = dict(local_sites=0, local_pairs=0, definition_agrees=0, references_agree=0,
                  command_sites=0, executor_sites=0, orb_sites=0, orb_external_resolutions=0)
    for subject in protocol['subjects']:
        data = json.loads((args.results / (subject['name'].split('/')[-1] + '-lsp.json')).read_text())
        if data['subject_sha'] != subject['sha'] or data['binary_sha256'] != protocol['binary_sha256']:
            raise SystemExit('comparison identity mismatch: ' + subject['name'])
        sites = [s for file in data['files'] for s in file['sites']]
        local = [s for s in sites if s['scope'] == 'local']
        orbs = [s for s in sites if s['scope'] == 'orb']
        counts = dict(local_sites=len(local), local_pairs=len({(f['path'], s['owner'], s['target']) for f in data['files'] for s in f['sites'] if s['scope'] == 'local'}),
                      definition_agrees=sum(s['definition_agrees'] for s in local), references_agree=sum(s['references_agree'] for s in local),
                      command_sites=sum(s['family'] == 'command' for s in local), executor_sites=sum(s['family'] == 'executor' for s in local),
                      orb_sites=len(orbs), orb_external_resolutions=sum(s['external_locations'] > 0 for s in orbs))
        for key in totals:
            totals[key] += counts[key]
        subjects.append({'name': subject['name'], 'source_shard': subject['source_shard'], 'counts': counts, 'native_report': data})
    diagnostic = json.loads((args.results / 'cypress-diagnostic.json').read_text())
    if diagnostic['subject_sha'] != next(s['sha'] for s in protocol['subjects'] if s['name'] == 'cypress-io/cypress'):
        raise SystemExit('diagnostic identity mismatch')
    differences = [s for f in diagnostic['files'] for s in f['sites'] if s['scope'] == 'local' and not s['definition_agrees']]
    report = {'tool': protocol['tool'], 'version': protocol['version'], 'protocol_commit': '3a7d13e34',
              'diagnostic_commit': 'b4e4ba91d', 'tier': 'pending controller W4 ruling',
              'totals': totals, 'subjects': subjects, 'definition_disagreement_diagnostic': differences}
    args.out.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(totals, sort_keys=True))
