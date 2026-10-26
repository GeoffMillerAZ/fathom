#!/usr/bin/env python3
"""Replay the existing clang oracle tools on the pinned SDK, without grading."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
spec = importlib.util.spec_from_file_location('opencl_compdb_replay', HERE / 'compdb.py')
compdb = importlib.util.module_from_spec(spec)
spec.loader.exec_module(compdb)
SDK = '6090378a113db8dab20ae278c8fa57ab10b92300'
PREPROC_SUMMARY_RE = re.compile(
    r'^translation_units (\d+) stopped (\d+) files (\d+) includes_in_subject (\d+)$',
    re.MULTILINE,
)


def digest(path):
    return compdb.sha256_file(path)


def tree_identity(root):
    """Hash every path, type, permission mode and regular-file byte sequence."""
    root = Path(root)
    metadata = root.lstat()
    if not stat.S_ISDIR(metadata.st_mode) or root.is_symlink():
        raise ValueError('identity root must be a real directory')
    rows = [['directory', '.', stat.S_IMODE(metadata.st_mode)]]

    def visit(directory, relative):
        with os.scandir(directory) as listing:
            entries = sorted(listing, key=lambda entry: os.fsencode(entry.name))
        for entry in entries:
            path = Path(entry.path)
            child = relative / entry.name
            child_metadata = entry.stat(follow_symlinks=False)
            mode = stat.S_IMODE(child_metadata.st_mode)
            if stat.S_ISDIR(child_metadata.st_mode):
                rows.append(['directory', child.as_posix(), mode])
                visit(path, child)
            elif stat.S_ISREG(child_metadata.st_mode):
                content = compdb.triage._read_regular_no_follow(path)
                rows.append([
                    'regular', child.as_posix(), mode, len(content),
                    hashlib.sha256(content).hexdigest(),
                ])
            else:
                raise ValueError('tree contains a symlink or special file')

    visit(root, Path())
    encoded = json.dumps(rows, ensure_ascii=True, separators=(',', ':')).encode('utf-8')
    return hashlib.sha256(encoded).hexdigest()


def compile_roots(database, subject):
    """Return the unique subject-relative files named by a compilation database."""
    try:
        entries = json.loads(
            compdb.triage._read_regular_no_follow(Path(database)).decode('utf-8'))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError('compilation database must be UTF-8 JSON') from exc
    if not isinstance(entries, list):
        raise ValueError('compilation database must contain a list')
    subject = Path(subject).resolve()
    roots = set()
    for entry in entries:
        if not isinstance(entry, dict):
            raise ValueError('compilation database entry must be an object')
        file = entry.get('file')
        directory = entry.get('directory')
        if not isinstance(file, str) or not isinstance(directory, str):
            raise ValueError('compilation database entry lacks file or directory')
        path = Path(file)
        if not path.is_absolute():
            path = Path(directory) / path
        try:
            relative = path.resolve().relative_to(subject).as_posix()
        except ValueError as exc:
            raise ValueError('compilation root is outside the subject snapshot') from exc
        if relative in roots:
            raise ValueError('compilation database contains a duplicate root')
        roots.add(relative)
    return roots


def validate_preprocessor_receipt(output, stderr_text, roots):
    """Require every compile root and zero stopped preprocessor invocations."""
    active = output.get('active_lines') if isinstance(output, dict) else None
    if not isinstance(active, dict):
        raise ValueError('preprocessor output lacks active_lines')
    matches = PREPROC_SUMMARY_RE.findall(stderr_text)
    if len(matches) != 1:
        raise ValueError('preprocessor stderr summary is missing or ambiguous')
    units, stopped, files, _ = (int(value) for value in matches[0])
    if stopped:
        raise ValueError('preprocessor stopped before completing every root')
    if units != len(roots):
        raise ValueError('preprocessor translation-unit count changed')
    if files != len(active):
        raise ValueError('preprocessor active-line file count changed')
    if set(active) != set(roots):
        raise ValueError('preprocessor active-line files do not match compile roots')


def make_private_layout(work):
    """Create private artifact directories and reserve the database path."""
    private = Path(work) / 'private'
    binaries = private / 'bin'
    logs = private / 'logs'
    outputs = private / 'outputs'
    for directory in (private, binaries, logs, outputs):
        directory.mkdir()
    return {
        'private': private,
        'database': private / 'database',
        'binaries': binaries,
        'logs': logs,
        'outputs': outputs,
    }


def run(command, *, cwd, out, err):
    while os.getloadavg()[0] > 9:
        print('waiting for load <=9', file=sys.stderr, flush=True)
        time.sleep(5)
    with out.open('wb') as stdout, err.open('wb') as stderr:
        subprocess.run(command, cwd=cwd, stdout=stdout, stderr=stderr, check=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sdk', type=Path, required=True)
    parser.add_argument('--clang', type=Path, required=True)
    parser.add_argument('--resource-dir', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        parser.error('caller must enforce nice 10 or lower scheduling priority')
    work = compdb._validated_out(args.out, parser, args.sdk)
    work.mkdir(parents=True)
    layout = make_private_layout(work)
    compiler = args.clang.resolve()
    pin, headers = compdb._load_compiler_pins()
    database = layout['database']
    capture = compdb.generate(subject=args.sdk, commit=SDK, clang=compiler,
                              clang_sha256=pin, resource_dir=args.resource_dir,
                              expected_header_sha256=headers, out=database)
    subject = database / 'subject'
    resource = database / 'resource'
    compile_database = database / 'compile_commands.json'
    roots = compile_roots(compile_database, subject)
    if len(roots) != 10 or capture['compile_units']['files'] != 10:
        raise ValueError('SDK compilation-root population changed')
    source_before = tree_identity(subject)
    resource_before = tree_identity(resource)
    database_before = tree_identity(database)
    db_before = digest(compile_database)
    tool_runs = [('clang-ast-defs', ROOT / 'calibration/oracles/clang-ast-defs', 'preproc.ranges.json'),
                 ('clang-calls', ROOT / 'calibration/oracles/clang-calls', 'clang-calls.json'),
                 ('preproc', ROOT / 'calibration/oracles/scip-clang/preproc', 'preproc.json')]
    for name, module, output in tool_runs:
        binary = layout['binaries'] / name
        run(['go', 'build', '-o', str(binary), '.'], cwd=module,
            out=layout['logs'] / (name + '-build.stdout'),
            err=layout['logs'] / (name + '-build.stderr'))
        compdb.verify_sha256(compiler, pin, 'compiler')
        compdb.hash_resource_headers(resource, headers)
        if (source_before != tree_identity(subject)
                or resource_before != tree_identity(resource)
                or database_before != tree_identity(database)
                or db_before != digest(compile_database)):
            raise ValueError('compiler input trees or database changed before replay')
        command = [str(binary), '-subject', str(subject), '-compdb', str(compile_database),
                   '-clang', str(compiler), '-tier', 'B']
        if name != 'preproc':
            command += ['-jobs', '2']
        run(command, cwd=ROOT, out=layout['outputs'] / output,
            err=layout['logs'] / (name + '.stderr'))
        compdb.verify_sha256(compiler, pin, 'compiler')
        compdb.hash_resource_headers(resource, headers)
        if (source_before != tree_identity(subject)
                or resource_before != tree_identity(resource)
                or database_before != tree_identity(database)
                or db_before != digest(compile_database)):
            raise ValueError('compiler input trees or database changed during replay')
    defs = json.loads((layout['outputs'] / 'preproc.ranges.json').read_text(encoding='utf-8'))
    calls = json.loads((layout['outputs'] / 'clang-calls.json').read_text(encoding='utf-8'))
    preproc = json.loads((layout['outputs'] / 'preproc.json').read_text(encoding='utf-8'))
    validate_preprocessor_receipt(
        preproc,
        (layout['logs'] / 'preproc.stderr').read_text(encoding='utf-8'),
        roots,
    )
    classes = {}
    for site in calls['call_sites']:
        classes[site['class']] = classes.get(site['class'], 0) + 1
    if (defs['units'], defs['errored'], defs['skipped'], len(defs['ranges'])) != (10, 0, 0, 23):
        raise ValueError('SDK definition replay changed')
    if classes != {'resolved': 147, 'declaration': 26} or len(preproc['active_lines']) != 10:
        raise ValueError('SDK call/active-line replay changed')
    if any(data['tier'] != 'B' for data in (defs, calls, preproc)):
        raise ValueError('reconstructed compiler context must remain tier B')
    summary = {'schema': 'opencl-c-sdk-pipeline-replay-v1', 'commit': SDK, 'tier': 'B',
               'scope': 'triage only; no extraction grade or runtime configuration union',
               'compile_units': 10, 'errored': 0, 'skipped': 0, 'function_ranges': 23,
               'written_sites': 173, 'classes': classes, 'active_line_files': 10,
               'active_line_ranges': sum(len(rows) for rows in preproc['active_lines'].values()),
               'active_lines': sum(b-a+1 for rows in preproc['active_lines'].values() for a,b in rows),
               'subject_snapshot_unchanged': True, 'database_unchanged': True,
               'minimal_resource_tree_unchanged': True,
               'compiler_and_minimal_resource_pins_checked_before_after_each_consumer': True,
               'snapshot_sha256': source_before, 'resource_tree_sha256': resource_before,
               'raw_output_sha256': {
                   name: digest(layout['outputs'] / name)
                   for name in ('preproc.ranges.json', 'clang-calls.json', 'preproc.json')
               },
               'path_bound_diagnostic_run_identity': {
                   'scope': ('includes absolute checkout, output, compiler, and resource paths; '
                             'not a cross-host reproducibility check'),
                   'database_tree_sha256': database_before,
                   'database_sha256': db_before,
                   'capture_receipt_sha256': digest(database / 'summary.json'),
               }}
    (work / 'summary.json').write_text(
        json.dumps(summary, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    print(json.dumps(summary, sort_keys=True))


if __name__ == '__main__':
    main()
