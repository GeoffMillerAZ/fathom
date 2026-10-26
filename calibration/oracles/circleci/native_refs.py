"""T-R7: run both vendor instruments on every registered subject.

The existing independent walk supplies query positions; it supplies no fallback
truth. Every local use must be returned by the pinned vendor references method.
Compiler workflow/job/requires pairs retain the compiler's native authority.
"""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path

HERE = Path(__file__).resolve().parent


def native_output(raw, report, protocol, sha, compiler_checks=()):
    if report['subject_sha'] != sha or report['binary_sha256'] != protocol['binary_sha256']:
        raise ValueError('native report subject/binary identity mismatch')
    if not report['files']:
        raise ValueError('empty native population')
    pairs = {(r['path'], r['from'], r['to']) for r in raw['references']}
    native_pairs, walk_pairs = set(), set()
    for file in report['files']:
        if file['version'] != protocol['version']:
            raise ValueError('unexpected server version')
        local = [s for s in file['sites'] if s['scope'] == 'local']
        if any(s['references_agree'] is not True for s in local):
            raise ValueError('native reference miss; source-walk fallback forbidden')
        local_pairs = {(file['path'], s['owner'], s['target']) for s in local}
        if not local_pairs <= pairs:
            raise ValueError('native population differs from adapter pairs')
        if file['yaml_v3_pairs'] != len(local_pairs):
            raise ValueError('incomplete native population')
        walk_pairs.update(local_pairs)
        if not file.get('all_declarations_queried') or 'declarations' not in file:
            raise ValueError('native declarations were not completely queried')
        for declaration in file['declarations']:
            for ref in declaration['references']:
                classification = ref['classification']
                if classification == 'same-name-workflow-job-oracle-error':
                    if not any(c['sha'] == sha and c['family'] == 'command' and c['target'] == ref['target'] and c['agrees'] for c in compiler_checks):
                        raise ValueError('namespace error lacks native compiler adjudication')
                elif classification in ('local-use', 'workflow-executor-parameter'):
                    if classification == 'workflow-executor-parameter' and not any(
                            c['sha'] == sha and c['family'] == 'executor-parameter' and
                            c['owner'] == ref['owner'] and c['target'] == ref['target'] and c['agrees']
                            for c in compiler_checks):
                        raise ValueError('executor parameter lacks native compiler cross-check')
                    native_pairs.add((file['path'], ref['owner'], ref['target']))
                else:
                    raise ValueError('unclassified native location')
        if not local_pairs <= native_pairs:
            raise ValueError('native truth omits a source walk pair')
    result = dict(raw)
    result.update(tool='circleci config process and CircleCI vendor textDocument/references',
                  version=raw['version'] + '; circleci-yaml-lsp ' + protocol['version'], tier='A',
                  references=[dict(path=p, **{'from': f, 'to': t}) for p, f, t in sorted((pairs - walk_pairs) | native_pairs)])
    return result


def run_subject(subject, protocol, adapter, cli, lsp, work, compiler_checks):
    checkout = Path(subject['checkout']).resolve()
    sha = subprocess.check_output(['git', '-C', str(checkout), 'rev-parse', 'HEAD'], text=True).strip()
    if sha != subject['sha']:
        raise ValueError('subject identity mismatch: ' + subject['name'])
    if subprocess.check_output(['git', '-C', str(checkout), 'status', '--porcelain', '--', '.circleci'], text=True).strip():
        raise ValueError('dirty subject configuration')
    name = subject['name'].split('/')[-1]
    home = (work / (name + '-home')).resolve()
    home.mkdir(parents=True, exist_ok=True)
    command = [str(adapter), '-subject', str(checkout), '-cli', str(cli), '-scratch', str(home)]
    if subject['source_shard'] != 'circleci.yaml':
        command += ['-matrix', str(HERE / 'parameters-t-r3.json'), '-subject-name', subject['name']]
    for mode in ('defs', 'yaml-refs'):
        with (work / (name + '-' + mode + '.json')).open('w') as stream:
            subprocess.run(command + (['-refs'] if mode == 'yaml-refs' else []), stdout=stream, check=True)
    raw = json.loads((work / (name + '-yaml-refs.json')).read_text())
    subprocess.run([__import__('sys').executable, str(HERE / 'lsp_compare.py'), '-subject', str(checkout),
                    '-lsp', str(lsp), '-work', str(work / name), '-yaml-refs', str(work / (name + '-yaml-refs.json')),
                    '-out', str(work / (name + '-lsp.json'))], check=True)
    report = json.loads((work / (name + '-lsp.json')).read_text())
    result = native_output(raw, report, protocol, sha, compiler_checks)
    (work / (name + '-refs.json')).write_text(json.dumps(result, indent=2) + '\n')
    print('native reference oracle:', subject['name'], len(result['references']), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-adapter', type=Path, required=True)
    parser.add_argument('-cli', type=Path, required=True)
    parser.add_argument('-lsp', type=Path, required=True)
    parser.add_argument('-out-dir', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads((HERE / 'lsp-protocol-r3.json').read_text())
    if hashlib.sha256(args.lsp.read_bytes()).hexdigest() != protocol['binary_sha256']:
        raise SystemExit('vendor binary checksum mismatch')
    args.out_dir.mkdir(parents=True, exist_ok=True)
    adjudication = args.out_dir / 'compiler-adjudication.json'
    subprocess.run([__import__('sys').executable, str(HERE / 'compiler_crosscheck.py'),
                    '-cli', str(args.cli.resolve()), '-work', str(args.out_dir / 'compiler'), '-out', str(adjudication)], check=True)
    checks = json.loads(adjudication.read_text())['checks']
    for subject in protocol['subjects']:
        run_subject(subject, protocol, args.adapter.resolve(), args.cli.resolve(), args.lsp.resolve(), args.out_dir, checks)
