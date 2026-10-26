#!/usr/bin/env python3
"""Historical throwaway compiler capability check, never a grading oracle.

Do not extend this JSON-AST reader. Use compdb.py with the existing clang-calls,
clang-ast-defs and scip-clang/preproc tools, following the CUDA pipeline. Raw AST
CallExpr counts below include repeated macro expansions, not written call sites.
"""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess

BINARY_SHA256 = '1590ac950a3d627817d09ade5cb60b2115f17a72182a3141e010b4bcc482a0c9'
HEADERS = {
    'opencl-c.h': 'aedaac009c73aa9ce4fad770ebdddbf11bbce42effc4c40cb54330f31af49c50',
    'opencl-c-base.h': 'e0b5b97e38951211312bfe946c956927a78da52a9e31bc66ec426e17fa5c23c5',
}
SDK_PIN = '6090378a113db8dab20ae278c8fa57ab10b92300'
KERNEL_ATTR_KINDS = frozenset({'OpenCLKernelAttr', 'DeviceKernelAttr'})
TRIAGE_SPEC = importlib.util.spec_from_file_location('opencl_c_triage', Path(__file__).with_name('triage.py'))
TRIAGE = importlib.util.module_from_spec(TRIAGE_SPEC)
TRIAGE_SPEC.loader.exec_module(TRIAGE)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def children(node):
    return node.get('inner', [])


def walk(node):
    stack = [node]
    while stack:
        current = stack.pop()
        yield current
        stack.extend(reversed(children(current)))


def is_kernel(node):
    return any(child.get('kind') in KERNEL_ATTR_KINDS for child in children(node))


def summarize_ast(ast):
    definitions = [node for node in ast.get('inner', [])
                   if node.get('kind') == 'FunctionDecl'
                   and any(child.get('kind') == 'CompoundStmt' for child in children(node))]
    names = {node['name'] for node in definitions}
    calls = []
    for declaration in definitions:
        for node in walk(declaration):
            if node.get('kind') != 'CallExpr':
                continue
            refs = [item['referencedDecl'] for item in walk(node.get('inner', [{}])[0])
                    if item.get('kind') == 'DeclRefExpr'
                    and item.get('referencedDecl', {}).get('kind') == 'FunctionDecl']
            calls.append(refs[0]['name'] if refs else None)
    return {
        'function_definitions': len(definitions),
        'kernel_definitions': sum(is_kernel(node) for node in definitions),
        'call_sites': len(calls),
        'source_function_call_sites': sum(callee in names for callee in calls),
        'external_call_sites': sum(callee is not None and callee not in names for callee in calls),
        'unresolved_call_sites': calls.count(None),
        'definition_end_ranges': sum(all(key in node.get('range', {}).get('end', {})
                                         for key in ('offset', 'col', 'tokLen')) for node in definitions),
        'vector_literal_nodes': sum(node.get('kind') == 'CompoundLiteralExpr'
                                    for declaration in definitions for node in walk(declaration)),
    }


def measure(compiler, resource, source, private, label):
    input_sha256 = digest(source)
    command = [str(compiler), '--no-default-config', '-x', 'cl', '--target=spir64-unknown-unknown',
               '-cl-std=CL3.0', '-resource-dir', str(resource), '-fsyntax-only', '-w',
               '-Xclang', '-finclude-default-header', '-Xclang', '-ast-dump=json', '--', str(source)]
    astfile, errors = private / (label + '.ast.json'), private / (label + '.stderr')
    with astfile.open('w') as stdout, errors.open('w') as stderr:
        result = subprocess.run(command, stdout=stdout, stderr=stderr, timeout=30)
    if digest(source) != input_sha256:
        raise ValueError('private compiler input changed during measurement')
    record = {'exit_code': result.returncode, 'source_sha256': input_sha256,
              'ast_sha256': digest(astfile), 'stderr_sha256': digest(errors)}
    if result.returncode == 0:
        record.update(summarize_ast(json.loads(astfile.read_text())))
    return record


def snapshot_sources(checkout, private):
    snapshot = private / 'sdk-inputs'
    snapshot.mkdir()
    for source, content in checkout.eligible_sources.items():
        target = snapshot / source.relative_to(checkout.root)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
        target.chmod(0o444)
    return snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--clang', type=Path, required=True)
    parser.add_argument('--resource-dir', type=Path, required=True)
    parser.add_argument('--sdk', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if digest(args.clang) != BINARY_SHA256:
        raise ValueError('compiler pin mismatch')
    for name, expected in HEADERS.items():
        if digest(args.resource_dir / 'include' / name) != expected:
            raise ValueError('resource header pin mismatch')
    before_checkout = TRIAGE.capture_checkout(args.sdk, SDK_PIN)
    args.out.mkdir(parents=True, exist_ok=False)
    private = args.out / 'private'
    private.mkdir()
    fixture = private / 'authored.cl'
    fixture.write_bytes(Path(__file__).with_name('probe.cl').read_bytes())
    fixture.chmod(0o444)
    authored = measure(args.clang, args.resource_dir, fixture, private, 'authored')
    expected = {'exit_code': 0, 'function_definitions': 2, 'kernel_definitions': 1, 'call_sites': 2,
                'source_function_call_sites': 1, 'external_call_sites': 1,
                'unresolved_call_sites': 0, 'definition_end_ranges': 2, 'vector_literal_nodes': 1}
    if not all(authored.get(key) == value for key, value in expected.items()):
        raise ValueError('authored compiler capability mismatch: ' + json.dumps(authored))
    files = [path.relative_to(before_checkout.root).as_posix() for path in before_checkout.eligible]
    records = {}
    snapshot = snapshot_sources(before_checkout, private)
    for i, name in enumerate(files):
        records[name] = measure(args.clang, args.resource_dir, snapshot / name, private, 'sdk-' + str(i))
    after_checkout = TRIAGE.capture_checkout(args.sdk, SDK_PIN)
    TRIAGE.assert_checkout_unchanged(before_checkout, after_checkout)
    (private / 'sdk-results.json').write_text(json.dumps(records, indent=2) + '\n')
    fields = ('function_definitions', 'kernel_definitions', 'source_function_call_sites',
              'external_call_sites', 'unresolved_call_sites')
    summary = {
        'compiler_version': 'Apple clang 21.0.0 (clang-2100.3.34.2)',
        'clt_package_version': '27.0.0.0.1788430756', 'compiler_binary_sha256': BINARY_SHA256,
        'resource_header_sha256': HEADERS, 'oracle_tier': 'B',
        'mode': 'CL3.0 SPIR64 reconstructed syntax-only JSON AST, no subject context flags',
        'authored_probe': authored,
        'sdk': dict({'commit': SDK_PIN, 'files': len(files),
                     'tracked': before_checkout.public['tracked'], 'untracked': before_checkout.public['untracked'],
                     'checkout_unchanged': True, 'input_snapshot': 'Verified captured kernel bytes in a private read-only tree, preserving relative paths',
                     'compiled': sum(record['exit_code'] == 0 for record in records.values()),
                     'failed': sum(record['exit_code'] != 0 for record in records.values())},
                    **{field: sum(record.get(field, 0) for record in records.values()) for field in fields}),
        'grade': None,
        'notes': 'Capability probe only. Failed files stay in the denominator. No source-truth adapter, independent mutation probe or quality grade is asserted. External calls are not yet classified as builtins; header/source origin mapping remains future adapter work.',
    }
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary))


if __name__ == '__main__':
    main()
