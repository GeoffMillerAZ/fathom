"""Regrade the two demonstrated cross-item regressions with independent truth.

Runs the existing OpenAPI and AWS-native CloudFormation instruments over all
pinned canonical subjects, then grades the same truth against baseline and
candidate binaries. No reading or generated tracker is overwritten. The
OpenAPI third-party instrument is C under P-T1 in both runs (legacy A is not
carried forward). L3 is replayed only for this binding regression check.
"""
import argparse
import json
import subprocess
from pathlib import Path
import yaml


def run(command, **kwargs):
    subprocess.run([str(x) for x in command], check=True, **kwargs)


def prepare_subject(repo, root):
    dest = root / repo['name'].replace('/', '--')
    if not dest.exists():
        dest.mkdir(parents=True)
        run(['git', '-C', dest, 'init', '-q'])
        run(['git', '-C', dest, 'remote', 'add', 'origin', repo['url']])
        run(['git', '-C', dest, 'config', 'remote.origin.promisor', 'true'])
        run(['git', '-C', dest, 'config', 'remote.origin.partialclonefilter', 'blob:none'])
        run(['git', '-C', dest, 'sparse-checkout', 'init', '--no-cone'])
        patterns = repo.get('sparse') or ([repo['subpath'] + '/**'] if repo.get('subpath') else ['/*'])
        run(['git', '-C', dest, 'sparse-checkout', 'set', '--no-cone', *patterns])
        run(['git', '-C', dest, '-c', 'protocol.version=2', 'fetch', '--depth=1', '--filter=blob:none', 'origin', repo['sha']])
        run(['git', '-C', dest, 'checkout', '--detach', '-q', 'FETCH_HEAD'])
    actual = subprocess.check_output(['git', '-C', str(dest), 'rev-parse', 'HEAD'], text=True).strip()
    if actual != repo['sha'] or subprocess.check_output(['git', '-C', str(dest), 'status', '--porcelain'], text=True).strip():
        raise ValueError('subject pin/cleanliness mismatch: ' + repo['name'])
    return dest  # the pinned sparse checkout itself is the complete measured scope


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-baseline', type=Path, required=True)
    parser.add_argument('-baseline-commit', required=True)
    parser.add_argument('-candidate', type=Path, required=True)
    parser.add_argument('-candidate-commit', required=True)
    parser.add_argument('-subjects', type=Path, required=True)
    parser.add_argument('-npm-prefix', type=Path, required=True)
    parser.add_argument('-cfn-python', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    if list(args.out.glob('*-baseline.json')) or list(args.out.glob('*-candidate.json')):
        raise SystemExit('fresh regrade output required')
    summary = {}
    for dialect in ('openapi', 'cloudformation'):
        shard = yaml.safe_load(Path('calibration/corpus.d/' + dialect + '.yaml').read_text())
        for repo in shard['repos']:
            subject = prepare_subject(repo, args.subjects)
            name = repo['name'].replace('/', '--')
            truth = args.out / (dialect + '-' + name + '-truth.json')
            xref = args.out / (dialect + '-' + name + '-xref.json')
            if dialect == 'openapi':
                run(['node', 'calibration/oracles/openapi/openapi_oracle.mjs', '-tools', args.npm_prefix,
                     '-subject', subject, '-out', truth, '-xref-out', xref])
                for file in (truth, xref):
                    if file.exists():
                        data = json.loads(file.read_text())
                        data['tier'] = 'C'
                        file.write_text(json.dumps(data, indent=2) + '\n')
            else:
                run([args.cfn_python, 'calibration/oracles/cfn-lint/cfn_oracle.py', '-subject', subject,
                     '-out', truth, '-xref-out', xref])
            for label, binary, commit in (('baseline', args.baseline, args.baseline_commit),
                                          ('candidate', args.candidate, args.candidate_commit)):
                output = args.out / (dialect + '-' + label + '.json')
                cmd = [binary.resolve(), 'lang-oracle', '-dialect', dialect, '-subject', subject,
                       '-subject-name', repo['name'], '-subject-sha', repo['sha'], '-tool-oracle', truth, '-out', output, '-commit', commit,
                       '-note', 'Binding Azure dispatch review regression replay; identical native/independent truth for baseline and candidate. OpenAPI third-party evidence is C under P-T1.']
                if xref.exists():
                    cmd += ['-tool-oracle', xref]
                run(cmd)
        before = json.loads((args.out / (dialect + '-baseline.json')).read_text())
        after = json.loads((args.out / (dialect + '-candidate.json')).read_text())
        keys = ('counts', 'metrics', 'metric_tiers', 'grade', 'level_met', 'excluded_by_kind')
        differences = [key for key in keys if before.get(key) != after.get(key)]
        summary[dialect] = {'subjects': len(shard['repos']), 'before': {k: before.get(k) for k in keys},
                            'after': {k: after.get(k) for k in keys}, 'differences': differences}
    (args.out / 'comparison.json').write_text(json.dumps(summary, indent=2) + '\n')
    if any(x['differences'] for x in summary.values()):
        raise SystemExit('cross-item regression: inspect comparison.json')
    print('Full canonical OpenAPI/CloudFormation regression regrades agree.')
