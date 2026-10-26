"""Native target-only upstream-reference ablation, with no C fallback."""
import argparse
import hashlib
import json
import posixpath
import re
import shutil
import subprocess
from pathlib import Path


def rename_target(source, target, line):
    # Only an unambiguous original upstream declaration is eligible. Preserve
    # every other source byte, including newlines and every reference.
    pattern = re.compile(r'(?m)^([ \t]*upstream[ \t]+)([\"\']?)' + re.escape(target) + r'\2(?=[ \t]*\{)')
    matches = list(pattern.finditer(source))
    if len(matches) != 1 or source[:matches[0].start()].count('\n') + 1 != line:
        raise ValueError('target declaration is ambiguous or not on its recorded line')
    match = matches[0]
    replacement = match.group(1) + match.group(2) + 'zz_oracle_unreferenced' + match.group(2)
    return source[:match.start()] + replacement + source[match.end():]


def attests_error(message, target, expected_path, lines):
    if 'host not found in upstream "' + target + '"' not in message:
        return False
    found = re.fullmatch(r'(.+):(\d+)\s*', message.rsplit(' in ', 1)[-1])
    return bool(found and found.group(1) == expected_path and int(found.group(2)) in lines)


def check_subject_links(source):
    # Refuse external/directory symlinks instead of following paths into the host or a cycle.
    for path in source.rglob('*'):
        if path.is_symlink() and (source.resolve() not in path.resolve().parents or path.resolve().is_dir()):
            raise ValueError('subject symlink leaves its checkout')


def copy_subject(source, dest):
    if source.resolve() == dest.resolve() or source.resolve() in dest.resolve().parents:
        raise ValueError('scratch copy must be outside subject')
    check_subject_links(source)
    shutil.copytree(source, dest, ignore=shutil.ignore_patterns('.git'), dirs_exist_ok=False)


def measure(subject, files, nginx, work):
    if subject.resolve() == work.resolve() or subject.resolve() in work.resolve().parents:
        raise ValueError('scratch must be outside subject')
    check_subject_links(subject)
    import nginx_oracle as old

    class LocatedWalk(old.FileWalk):
        def __init__(self, rel, parsed):
            self.locations = {}
            self.declarations = {}
            super().__init__(rel, parsed)

        def walk(self, ds, server, owner, upstream, data):
            for node in ds:
                if not data and node['directive'] == 'upstream' and node.get('block') is not None and node['args']:
                    self.declarations.setdefault(node['args'][0], []).append(node['line'])
                if not data and node['directive'] in old.PASS and node['args']:
                    host, port = old.pass_host(node['args'][0])
                    if not port and host in self.upstreams:
                        self.locations.setdefault((owner, host), set()).add(node['line'])
                super().walk([node], server, owner, upstream, data)

    definitions, references, audit = [], [], []
    for index, rel in enumerate(files):
        parsed, error = old.parse(str(subject), rel)
        if parsed is None:
            raise ValueError('previous scoped source is not parseable: ' + rel)
        walk = LocatedWalk(rel, parsed)
        positive, _, error, _, mode = old.attest(str(subject), rel, str(nginx), str(work / 'positive'))
        if not positive:
            raise ValueError('previous native scope no longer loads: ' + rel + ': ' + error)
        definitions.extend(walk.defs)
        source = (subject / rel).read_text()
        for ref in walk.refs:
            locations = walk.locations[(ref['from'], ref['to'])]
            declarations = walk.declarations[ref['to']]
            if len(declarations) != 1:
                raise ValueError('ambiguous upstream declaration')
            probe = work / ('probe-' + str(index) + '-' + str(len(audit)))
            copy_subject(subject, probe)
            (probe / rel).write_text(rename_target(source, ref['to'], declarations[0]))
            prefix = old.prefix_of(str(subject), rel)
            if prefix is None:
                prefix = posixpath.dirname(rel)
            ok, _, message, _ = old.nginx_check(str(probe), prefix, str(nginx), str(work / 'negative'), mode, rel)
            expected_path = posixpath.relpath(rel, prefix) if prefix else rel
            if ok or not attests_error(message, ref['to'], expected_path, locations):
                raise ValueError('native target-only ablation did not attest this pair: ' + rel + ': ' + message)
            references.append(ref)
            audit.append({'path': rel, 'from': ref['from'], 'to': ref['to'], 'native_error': message,
                          'source_sha256': hashlib.sha256(source.encode()).hexdigest(), 'positive_context': mode})
            shutil.rmtree(probe)
    return definitions, references, audit


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-subject', type=Path, required=True)
    parser.add_argument('-subject-sha', required=True)
    parser.add_argument('-files', type=Path, required=True, help='original committed reading scope, unchanged')
    parser.add_argument('-nginx', type=Path, required=True)
    parser.add_argument('-build-receipt', type=Path, required=True)
    parser.add_argument('-work', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    subject, native = args.subject.resolve(), args.nginx.resolve()
    if subprocess.check_output(['git', '-C', str(subject), 'rev-parse', 'HEAD'], text=True).strip() != args.subject_sha:
        raise SystemExit('subject pin mismatch')
    if subprocess.check_output(['git', '-C', str(subject), 'status', '--porcelain'], text=True).strip():
        raise SystemExit('dirty subject')
    receipt = json.loads(args.build_receipt.read_text())
    if hashlib.sha256(native.read_bytes()).hexdigest() != receipt['binary_sha256'] or receipt['version'] != 'nginx/1.31.6':
        raise SystemExit('native identity mismatch')
    args.work.mkdir(parents=True, exist_ok=True)
    files = args.files.read_text().splitlines()
    if not files or files != sorted(set(files)) or any(Path(f).is_absolute() or '..' in Path(f).parts for f in files):
        raise SystemExit('invalid original scope')
    definitions, references, audit = measure(subject, files, native, args.work)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    for suffix, tier, defs, refs in [('defs', 'C', definitions, []), ('refs', 'A', [], references)]:
        output = {'tool': 'nginx native target-only upstream ablation' if suffix == 'refs' else 'independent crossplane definition placement',
                  'version': receipt['version'], 'tier': tier, 'definitions': defs, 'references': refs}
        args.out.with_name(args.out.stem + '-' + suffix + '.json').write_text(json.dumps(output, indent=2) + '\n')
    # Raw responses stay private; the committed summary is counts and hashes.
    args.out.with_name(args.out.stem + '-raw.json').write_text(json.dumps(audit, indent=2) + '\n')
    summary = {'kind': 'native L2 reference ablation only; no L3 measurement', 'subject_sha': args.subject_sha,
               'configs': len(files), 'definitions': len(definitions), 'references': len(references),
               'binary_sha256': receipt['binary_sha256'], 'scope_sha256': hashlib.sha256(args.files.read_bytes()).hexdigest(),
               'native_evidence_sha256': hashlib.sha256(json.dumps(audit, sort_keys=True).encode()).hexdigest(),
               'mutations': 'target upstream declaration only; references untouched', 'server_started': False, 'backend_requests': 0}
    args.out.write_text(json.dumps(summary, indent=2) + '\n')
