"""Source-only X-R4 audit of pinned vendor packages and runner loader.

Does not execute a workflow or turn source inspection into a native reading.
"""
import argparse
import hashlib
import io
import json
import re
import tarfile
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent


def rejects_local_dot(source):
    signature = 'export function parseActionReference(uses) {'
    if signature not in source:
        return False
    body = source.split(signature, 1)[1].split('\nexport function ', 1)[0]
    for match in re.finditer(r'\bif\s*\(([^{}]*?)\)\s*\{\s*return\s+undefined\s*;\s*\}', body, re.S):
        if 'uses.startsWith("./")' in match.group(1):
            return True
    return False

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    expected = json.loads((HERE / 'candidate-identities-r1.json').read_text())
    results = []
    source = {}
    for package in expected['npm']:
        raw = urllib.request.urlopen(package['url'], timeout=60).read()
        if hashlib.sha256(raw).hexdigest() != package['tarball_sha256']:
            raise SystemExit('package digest mismatch')
        with tarfile.open(fileobj=io.BytesIO(raw), mode='r:gz') as archive:
            manifest = json.load(archive.extractfile('package/package.json'))
            if manifest['version'] != package['version'] or manifest['name'] != package['name']:
                raise SystemExit('package identity mismatch')
            for member in archive.getmembers():
                if member.isfile() and member.name in (
                    'package/dist/action.js', 'package/dist/validate-action-reference.js',
                    'package/dist/cli.bundle.cjs', 'package/dist/index.d.ts'):
                    source[package['name'] + ':' + member.name] = archive.extractfile(member).read().decode()
        results.append({'name': package['name'], 'version': package['version'], 'sha256': hashlib.sha256(raw).hexdigest()})
    service = '@actions/languageservice:package/dist/'
    action = source[service + 'action.js']
    validation = source[service + 'validate-action-reference.js']
    bundle = source['@actions/languageserver:package/dist/cli.bundle.cjs']
    caps = bundle.split('connection.onInitialize((params) => {', 1)[1].split('connection.onInitialized', 1)[0]
    checks = {'local_dot_reference_rejected_by_action_parser': rejects_local_dot(action),
              'native_self_repository_validation_uses_dollar_slash': 'uses.startsWith("$/")' in validation,
              'definition_provider_advertised': 'definitionProvider' in caps,
              'reference_provider_advertised': 'referencesProvider' in caps}
    if checks != {'local_dot_reference_rejected_by_action_parser': True,
                  'native_self_repository_validation_uses_dollar_slash': True,
                  'definition_provider_advertised': False, 'reference_provider_advertised': False}:
        raise SystemExit('vendor contract changed: reassess rather than retain the ceiling')
    for item in expected['runner']['sources']:
        raw = urllib.request.urlopen(item['url'], timeout=60).read()
        if hashlib.sha256(raw).hexdigest() != item['sha256']:
            raise SystemExit('runner source digest mismatch')
    act = {}
    for item in expected['act']['sources']:
        raw = urllib.request.urlopen(item['url'], timeout=60).read()
        if hashlib.sha256(raw).hexdigest() != item['sha256']:
            raise SystemExit('act source digest mismatch')
        act[item['path']] = raw.decode()
    root, local, loader = act['cmd/root.go'], act['pkg/runner/step_action_local.go'], act['pkg/runner/action.go']
    act_checks = {
        'list_returns_plan_before_runner': root.index('if list {') < root.index('runner.New(config)') and
            'return plannerErr' in root.split('if list {', 1)[1].split('if graph {', 1)[0],
        'dryrun_returns_before_local_action_load': bool(re.search(
            r'if common\.Dryrun\(ctx\) \{\s*return nil\s*\}\s*actionDir :=', local)) and
            local.index('common.Dryrun(ctx)') < local.index('sal.readAction(ctx'),
        'metadata_order_yml_before_yaml': loader.index('readFile("action.yml")') < loader.index('readFile("action.yaml")'),
    }
    if not all(act_checks.values()):
        raise SystemExit('act source contract changed: reassess')
    result = {'kind': 'source interface audit only; no workflow execution or oracle grade',
              'npm': results, 'checks': checks, 'runner': expected['runner'],
              'act': dict(expected['act'], checks=act_checks, runtime_executed=False),
              'next_native_route': 'runner ActionManager.LoadAction and ActionManifestManager.Load; full-population harness unmeasured, held for D19'}
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, indent=2) + '\n')
