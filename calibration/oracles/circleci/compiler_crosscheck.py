"""T-R7 spot checks: compiler expansion corroborates three vendor bindings.

Only config process runs. No pipeline, shell body, container or Docker runs.
The complete reference population is measured separately by native_refs.py.
"""
import argparse
import json
import os
import subprocess
from pathlib import Path
import yaml

HERE = Path(__file__).resolve().parent

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-cli', type=Path, required=True)
    parser.add_argument('-work', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads((HERE / 'lsp-protocol-r2.json').read_text())
    checks = []
    for name in ('CircleCI-Public/circleci-cli', 'cypress-io/cypress'):
        subject = next(s for s in protocol['subjects'] if s['name'] == name)
        checkout = Path(subject['checkout']).resolve()
        if subprocess.check_output(['git', '-C', str(checkout), 'rev-parse', 'HEAD'], text=True).strip() != subject['sha']:
            raise SystemExit('subject identity mismatch')
        if subprocess.check_output(['git', '-C', str(checkout), 'status', '--porcelain', '--', '.circleci'], text=True).strip():
            raise SystemExit('dirty subject')
        home = (args.work / name.split('/')[-1]).resolve()
        home.mkdir(parents=True, exist_ok=True)
        env = {k: os.environ[k] for k in ('PATH', 'TMPDIR') if k in os.environ}
        env['HOME'] = str(home)
        compiled_text = subprocess.check_output([str(args.cli.resolve()), 'config', 'process', '.circleci/config.yml'], cwd=checkout, env=env, text=True)
        compiled = yaml.safe_load(compiled_text)
        source = yaml.safe_load((checkout / '.circleci/config.yml').read_text())
        if name.endswith('/circleci-cli'):
            expected = source['executors']['linux']
            actual = {k: compiled['jobs']['check'].get(k) for k in expected}
            checks.append(dict(subject=name, sha=subject['sha'], owner='check', target='linux', family='executor', expected=expected, actual=actual, agrees=expected == actual))
            expected = source['commands']['store_results']['steps'][0]
            actual = next((x for x in compiled['jobs']['check']['steps'] if isinstance(x, dict) and 'store_test_results' in x), None)
            checks.append(dict(subject=name, sha=subject['sha'], owner='check', target='store_results', family='command', expected=expected, actual=actual, agrees=expected == actual))
        else:
            expected = next(x for x in source['commands']['pack-workflows']['steps'] if isinstance(x, dict) and 'run' in x)
            actual = next((x for x in compiled['jobs']['pack-workflows']['steps'] if isinstance(x, dict) and isinstance(x.get('run'), dict) and x['run'].get('name') == expected['run']['name']), None)
            checks.append(dict(subject=name, sha=subject['sha'], owner='pack-workflows', target='pack-workflows', family='command', expected=expected, actual=actual, agrees=expected == actual))
    version = subprocess.check_output([str(args.cli.resolve()), 'version'], text=True).strip()
    result = dict(tool='circleci config process', version=version, checks=checks, agreements=sum(c['agrees'] for c in checks))
    args.out.write_text(json.dumps(result, indent=2) + '\n')
    if not all(c['agrees'] for c in checks):
        raise SystemExit('compiler divergence; retain report and investigate')
    print('compiler cross-check:', result['agreements'], 'of', len(checks))
