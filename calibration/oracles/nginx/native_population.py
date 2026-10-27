"""Whole-input native upstream identities first; lexical owner labels second.

The independent walk supplies definition placement and enclosing entity names,
never the reference population. All native local pairs survive comparison.
"""
import argparse
import hashlib
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys
from pathlib import Path

from native_upstream import attests_error, check_subject_links, copy_subject, rename_target
from population import PASS, local_population
from population_capture import NativeCapture

HERE = Path(__file__).resolve().parent


def verify_identity(binary, receipt):
    protocol = json.loads((HERE / 'native-protocol-r3.json').read_text())
    if receipt.get('version') != protocol['version'] or hashlib.sha256(binary.read_bytes()).hexdigest() != receipt.get('binary_sha256'):
        raise ValueError('native binary/version identity mismatch')
    observer = receipt.get('native_population_observer', {})
    expected = {'src/http/ngx_http_upstream.c', 'src/stream/ngx_stream_upstream.c'}
    sources = observer.get('sources', [])
    if (receipt.get('build_inputs') != protocol['build_inputs'] or
            observer.get('shim_sha256') != hashlib.sha256((HERE / 'observer_shim.c.in').read_bytes()).hexdigest() or
            len(sources) != 2 or {row['path'] for row in sources} != expected or
            any(not re.fullmatch('[0-9a-f]{64}', row.get(key, ''))
                for row in sources for key in ('original_sha256', 'observed_sha256'))):
        raise ValueError('native observer identity mismatch')


def normalized_evidence(evidence):
    """Preserve native object equality without publishing ASLR heap addresses."""
    import copy
    normalized = copy.deepcopy(evidence)
    for profile in normalized:
        events = profile['native_events']
        identities = {}
        for event in events:
            key = event['domain'], event['pointer']
            if event['flags'] & 1 and key not in identities:
                identities[key] = 'declaration:' + str(len(identities) + 1)
        for event in events:
            key = event['domain'], event['pointer']
            if key not in identities:
                identities[key] = 'implicit:' + str(len(identities) + 1)
            event['pointer'] = identities[key]
    return normalized


def measure(subject, files, nginx, work, native_definitions=False):
    import nginx_oracle as old
    subject, nginx, work = subject.resolve(), nginx.resolve(), work.resolve()
    if subject == work or subject in work.parents:
        raise ValueError('scratch must be outside subject')
    check_subject_links(subject)
    work.mkdir(parents=True, exist_ok=True)

    class LocatedWalk(old.FileWalk):
        def __init__(self, rel, parsed):
            self.sites = {}
            super().__init__(rel, parsed)

        def walk(self, nodes, server, owner, upstream, data):
            for node in nodes:
                if not data and node['directive'] in PASS and node['args']:
                    # No known-host/port/variable test here: this is only a
                    # lexical source-to-owner map. Ambiguity refuses the run.
                    key = node['line'], node['directive'], node['args'][0]
                    self.sites.setdefault(key, set()).add(owner)
                super().walk([node], server, owner, upstream, data)

        def owner(self, event):
            owners = self.sites.get((event['line'], event['command'], event['argument']), set())
            return next(iter(owners)) if len(owners) == 1 else None

    definitions, references, audit = [], [], []
    for index, rel in enumerate(files):
        parsed, error = old.parse(str(subject), rel)
        if parsed is None:
            raise ValueError('original input is no longer parseable: ' + rel + ': ' + error)
        walk = LocatedWalk(rel, parsed)
        capture = NativeCapture(rel)
        definition_capture = None
        if native_definitions:
            from native_definitions import DefinitionCapture
            definition_capture = DefinitionCapture(rel)
        def observe(record):
            capture(record)
            if definition_capture is not None:
                definition_capture(record)
        ok, _, error, _, mode = old.attest(str(subject), rel, str(nginx), str(work / 'positive'), observer=observe)
        if not ok:
            raise ValueError('original native input no longer loads: ' + rel + ': ' + error)
        events = capture.selected()['events']
        owner = definition_capture.owner if definition_capture is not None else walk.owner
        refs, counts = local_population(events, rel, owner)
        native_set = {(r['path'], r['from'], r['to']) for r in refs}
        scanner_set = {(r['path'], r['from'], r['to']) for r in walk.refs}
        # Comparison follows native enumeration and never filters it.
        declarations = {(e['domain'], e['pointer']): e for e in events if e['flags'] & 1}
        controls = []
        for ref in refs:
            sites = []
            for event in events:
                declaration = declarations.get((event['domain'], event['pointer']))
                if (not event['flags'] & 1 and event['path'] == rel and declaration and
                        declaration['path'] == rel and declaration['host'] == ref['to'] and owner(event) == ref['from']):
                    sites.append((event, declaration))
            if not sites or len({d['line'] for _, d in sites}) != 1:
                raise ValueError('native target declaration is ambiguous')
            source = (subject / rel).read_text()
            probe = work / ('negative-' + str(index) + '-' + str(len(controls)))
            copy_subject(subject, probe)
            try:
                (probe / rel).write_text(rename_target(source, ref['to'], sites[0][1]['line']))
                prefix = old.prefix_of(str(subject), rel)
                if prefix is None:
                    prefix = posixpath.dirname(rel)
                loaded, _, message, _ = old.nginx_check(str(probe), prefix, str(nginx), str(work / 'negative'), mode, rel)
                expected = posixpath.relpath(rel, prefix) if prefix else rel
                if loaded or not attests_error(message, ref['to'], expected, {e['line'] for e, _ in sites}, allow_no_port=True):
                    raise ValueError('native target-only ablation did not attest pair: ' + message)
                controls.append(dict(reference=ref, native_error=message))
            finally:
                shutil.rmtree(probe)
        positioned = definition_capture.selected() if definition_capture is not None else None
        definitions.extend(positioned['definitions'] if positioned is not None else walk.defs)
        references.extend(refs)
        audit.append(dict(path=rel, mode=mode, native_events=events, classification=counts,
                          native_positions={key: value for key, value in positioned.items() if key != 'definitions'} if positioned else None,
                          failed_contexts=capture.failed_contexts, negative_controls=controls,
                          native_only_pairs=sorted(native_set - scanner_set), scanner_only_pairs=sorted(scanner_set - native_set)))
    return definitions, references, audit


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for name in ('subject', 'files', 'nginx', 'build-receipt', 'work', 'out'):
        parser.add_argument('-' + name, type=Path, required=True)
    parser.add_argument('-subject-sha', required=True)
    parser.add_argument('-native-definitions', action='store_true')
    args = parser.parse_args()
    if subprocess.check_output(['git', '-C', str(args.subject), 'rev-parse', 'HEAD'], text=True).strip() != args.subject_sha:
        raise SystemExit('subject pin mismatch')
    if subprocess.check_output(['git', '-C', str(args.subject), 'status', '--porcelain'], text=True).strip():
        raise SystemExit('dirty subject')
    receipt = json.loads(args.build_receipt.read_text())
    verify_identity(args.nginx, receipt)
    version = subprocess.run([str(args.nginx.resolve()), '-v'], capture_output=True, text=True, check=True)
    if (version.stdout + version.stderr).strip() != 'nginx version: ' + receipt['version']:
        raise SystemExit('native runtime version mismatch')
    # Every CLI measurement requires fresh authored native controls on this
    # exact binary. Importing this module or running skipped tests earns no A.
    control_env = dict(os.environ, NGINX_OBSERVER=str(args.nginx.resolve()))
    subprocess.run([sys.executable, '-B', '-m', 'unittest', 'discover', '-s', str(HERE),
                    '-p', 'test_native_population.py'], env=control_env, check=True)
    if args.native_definitions:
        from instrument_tokens import SOURCE_SHA256
        token_identity = receipt.get('native_token_observer', {})
        if (token_identity.get('original_sha256') != SOURCE_SHA256 or
                token_identity.get('shim_sha256') != hashlib.sha256((HERE / 'token_observer.c.in').read_bytes()).hexdigest()):
            raise SystemExit('native lexer observer identity mismatch')
        control_env['NGINX_TOKENS'] = str(args.nginx.resolve())
        subprocess.run([sys.executable, '-B', '-m', 'unittest', 'discover', '-s', str(HERE),
                        '-p', 'test_native_token*.py'], env=control_env, check=True)
        subprocess.run([sys.executable, '-B', '-m', 'unittest', 'discover', '-s', str(HERE),
                        '-p', 'test_native_definition_integration.py'], env=control_env, check=True)
    files = args.files.read_text().splitlines()
    if not files or files != sorted(set(files)) or any(Path(f).is_absolute() or '..' in Path(f).parts for f in files):
        raise SystemExit('invalid original input scope')
    definitions, references, evidence = measure(args.subject, files, args.nginx, args.work, args.native_definitions)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    for suffix, tier, defs, refs in [('defs', 'A' if args.native_definitions else 'C', definitions, []), ('refs', 'A', [], references)]:
        output = dict(tool='nginx native parser upstream population' if suffix == 'refs' else 'native nginx lexer definition population and ranges' if args.native_definitions else 'independent crossplane definition placement',
                      version=receipt['version'], tier=tier, definitions=defs, references=refs)
        args.out.with_name(args.out.stem + '-' + suffix + '.json').write_text(json.dumps(output, indent=2) + '\n')
    raw = json.dumps(evidence, sort_keys=True).encode()
    args.out.with_name(args.out.stem + '-raw.json').write_bytes(raw + b'\n')
    summary = dict(kind='native whole-input lexer definitions/ranges and upstream population; L1/L2 only' if args.native_definitions else 'native whole-input parser population; same-file L2 only', subject_sha=args.subject_sha,
                   binary_sha256=receipt['binary_sha256'], reference_tier='A', configs=len(files),
                   definition_tier='A' if args.native_definitions else 'C',
                   native_position_tokens=sum(a['native_positions']['native_tokens_target'] for a in evidence) if args.native_definitions else 0,
                   native_tokens_all_evaluations=sum(a['native_positions']['native_tokens_all_sources'] for a in evidence) if args.native_definitions else 0,
                   native_source_evaluations=sum(a['native_positions']['native_source_evaluations'] for a in evidence) if args.native_definitions else 0,
                   native_sources_summed_per_profile=sum(a['native_positions']['native_sources'] for a in evidence) if args.native_definitions else 0,
                   native_target_evaluations=sum(a['native_positions']['native_target_evaluations'] for a in evidence) if args.native_definitions else 0,
                   definitions=len(definitions), references=len(references), native_events=sum(len(a['native_events']) for a in evidence),
                   classification={key: sum(a['classification'].get(key, 0) for a in evidence)
                                   for key in sorted({key for a in evidence for key in a['classification']})},
                   negative_controls=sum(len(a['negative_controls']) for a in evidence),
                   native_only_pairs=sum(len(a['native_only_pairs']) for a in evidence),
                   scanner_only_pairs=sum(len(a['scanner_only_pairs']) for a in evidence),
                   native_evidence_sha256=hashlib.sha256(json.dumps(normalized_evidence(evidence), sort_keys=True).encode()).hexdigest(),
                   native_evidence_digest_format='pointer-normalized-v1', scope_sha256=hashlib.sha256(args.files.read_bytes()).hexdigest(),
                   server_started=False, backend_requests=0)
    args.out.write_text(json.dumps(summary, indent=2) + '\n')
