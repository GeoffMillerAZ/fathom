"""Full original Kong input replay; L1 positions C, native written fields A.

Canonical L3 and its reading remain untouched. Public receipts use cohort
ordinals, counts and hashes; original subject names and inventories stay private.
"""
import argparse
from collections import Counter, defaultdict
import hashlib
import json
import os
import platform
import shutil
import subprocess
from pathlib import Path
from probe_native_controls import native_query, sha

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
KINDS = dict(service='service', route='route', upstream='resource', target='element', plugin='policy')


def run(command, **kwargs):
    if shutil.disk_usage(Path.home()).free < 20 * 1024**3:
        raise ValueError('free disk below 20 GiB before measurement step')
    return subprocess.run([str(part) for part in command], check=True, **kwargs)


def tracked_receipt(subject, receipt):
    design = (ROOT / 'docs/design/language-breadth.md').read_text()
    marker = "SUBJECT=workspace/calibration/<subject> SUBPATH=. RECEIPT=workspace/artifacts/lang-<id>/<subject>-before python3 - <<'PY'\n"
    body = design.split(marker, 1)[1].split('\nPY\n', 1)[0]
    env = dict(os.environ, SUBJECT=str(subject), SUBPATH='.', RECEIPT=str(receipt))
    run(['python3', '-B', '-c', body], env=env)
    summary = json.loads(Path(str(receipt) + '.json').read_text())
    untracked = Path(str(receipt) + '.untracked').read_bytes()
    summary.update(untracked_entries=len([p for p in untracked.split(b'\0') if p]),
                   untracked_sha256=sha(untracked), normative_recipe_sha256=sha(body.encode()))
    return summary


def prepare(repo, dest):
    run(['git', 'init', '-q', dest])
    run(['git', '-C', dest, 'remote', 'add', 'origin', repo['url']])
    run(['git', '-C', dest, 'config', 'remote.origin.promisor', 'true'])
    run(['git', '-C', dest, 'config', 'remote.origin.partialclonefilter', 'blob:none'])
    run(['git', '-C', dest, 'fetch', '--depth=1', '--filter=blob:none', 'origin', repo['sha']])
    run(['git', '-C', dest, 'checkout', '-q', '--detach', 'FETCH_HEAD'])
    if subprocess.check_output(['git', '-C', str(dest), 'rev-parse', 'HEAD'], text=True).strip() != repo['sha']:
        raise ValueError('source pin mismatch')


def original_scope(prior, name):
    prefix = name + ': restricted to '
    notes = [note for note in prior['notes'] if note.startswith(prefix)]
    if not notes:
        raise ValueError('no original scope; refuse to invent one')
    return sorted({p for note in notes for p in note[len(prefix):].split(' (-only;', 1)[0].split(', ')})


def shadow_copy(source, dest):
    dest.mkdir()
    raw = subprocess.check_output(['git', '-C', str(source), 'ls-files', '-z'])
    rows = []
    for entry in raw.split(b'\0'):
        if not entry:
            continue
        rel = entry.decode()
        if Path(rel).suffix not in ('.yml', '.yaml'):
            continue
        src = source / rel
        if src.is_symlink() or not src.is_file():
            raise ValueError('nonregular YAML in original shadow population')
        mapped = Path(*('td' if part == 'testdata' else part for part in Path(rel).parts))
        dst = dest / mapped
        if dst.exists():
            raise ValueError('shadow-copy path collision')
        dst.parent.mkdir(parents=True, exist_ok=True)
        dst.write_bytes(src.read_bytes())
        rows.append(dict(source=rel, mapped=mapped.as_posix(), sha256=sha(dst.read_bytes()), bytes=dst.stat().st_size))
    return dict(yaml_files=len(rows), bytes=sum(row['bytes'] for row in rows),
                mapping_sha256=sha(json.dumps(rows, sort_keys=True).encode()),
                mapping='YAML-only; every testdata path component becomes td; content unchanged')


def project(rel, native, located):
    if located.get('tier') != 'C':
        raise ValueError('independent positions must stay C')
    positions = defaultdict(list)
    for node in located['entities']:
        positions[(node['kind'], node['name'])].append(node)
    definitions = []
    native_named = [node for node in native['native_entities'] if node.get('name')]
    # Native enumeration chooses the full multiset. The locator only attaches
    # C positions; missing or extra rows cannot silently change recall truth.
    if Counter((node['kind'], node['name']) for node in native_named) != Counter({key: len(rows) for key, rows in positions.items()}):
        raise ValueError('native/independent named population mismatch; whole input refused')
    for node in native_named:
        position = positions[(node['kind'], node['name'])].pop(0)
        definitions.append(dict(path=rel, name=node['name'], kind=KINDS[node['kind']],
                                line=position['line'], end=position['end']))
    references, outcomes = [], Counter()
    for ref in native['native_written_fields']:
        if ref['outcome'] not in ('owner_registry_resolved', 'owner_registry_missing', 'literal_host_without_named_upstream'):
            raise ValueError('unclassified native outcome; whole input refused')
        outcomes[ref['outcome']] += 1
        if ref['outcome'] != 'owner_registry_resolved':
            continue
        if not ref.get('from') or not ref.get('to'):
            outcomes['unnamed_endpoint_unscored'] += 1
            continue
        references.append(dict(path=rel, **{'from': ref['from'], 'to': ref['to']}))
    # The recipe metric is a semantic owner/target pair, not every duplicate
    # occurrence. Preserve native field/site counts separately above.
    refs = {(row['path'], row['from'], row['to']) for row in references}
    return definitions, [dict(path=p, **{'from': f, 'to': t}) for p, f, t in sorted(refs)], dict(outcomes)


def measure(args):
    start = dict(load_1m=os.getloadavg()[0], nice=os.getpriority(os.PRIO_PROCESS, 0),
                 free_gib=shutil.disk_usage(Path.home()).free / 1024**3)
    if start['load_1m'] > 9 or start['nice'] < 10 or start['free_gib'] < 20:
        raise ValueError('T-R10 correctness start requires load<=9, nice>=10, free>=20 GiB')
    import yaml
    protocol = json.loads((HERE / 'native-protocol-r2.json').read_text())
    identity = json.loads(args.build_receipt.read_text())
    control = json.loads(args.controls.read_text())
    projection_control = json.loads(args.projection_controls.read_text())
    if (sha(args.native.read_bytes()) != identity['binary_sha256'] or
            sha((HERE / 'native/main.go').read_bytes()) != identity['probe_source_sha256'] or
            control['native_binary_sha256'] != identity['binary_sha256'] or control['race_tests'] != 12 or
            len(control['mutations']) != 2 or not all(row['restored_green'] for row in control['mutations']) or
            len(control['cli_cross_checks']) != 3 or control['native_http_requests'] or
            not control['official_cli_network_denied'] or projection_control.get('tests') != 4 or
            not projection_control.get('restored_green') or projection_control.get('skips') != 0):
        raise ValueError('fresh native identity and complete authored controls required')
    args.out.mkdir(parents=True, exist_ok=False)
    args.subjects.mkdir(parents=True, exist_ok=False)
    prior = json.loads((ROOT / 'docs/bench/readings/lang/kong.json').read_text())
    repos = yaml.safe_load((ROOT / 'calibration/corpus.d/kong.yaml').read_text())['repos']
    scopes = [original_scope(prior, subject['name']) for subject in prior['subjects']]
    if len(scopes) != protocol['population']['cohorts'] or sum(map(len, scopes)) != protocol['population']['native_inputs']:
        raise ValueError('original scope differs from preregistration')
    all_rows, refused, mappings, comparisons = [], [], [], []
    reading = args.out / 'reading.json'
    for index, (subject, scope) in enumerate(zip(prior['subjects'], scopes)):
        ordinal = index + 1
        source = args.subjects / ('cohort-' + str(ordinal))
        if subject['sha'].startswith('f21aff'):
            name = subject['name'].split(' (testdata copied as td)')[0]
            repo = next(repo for repo in repos if repo['name'] == name)
            copied = True
        else:
            repo = next(repo for repo in repos if repo['sha'] == subject['sha'])
            copied = False
        prepare(repo, source)
        folder = args.out / ('cohort-' + str(ordinal)); folder.mkdir()
        before = tracked_receipt(source, folder / 'before')
        target = source
        mapping = None
        if copied:
            target = args.subjects / ('shadow-' + str(ordinal))
            mapping = shadow_copy(source, target)
        definitions, references, outcomes = [], [], Counter()
        raw_entities = raw_fields = unnamed = 0
        input_hashes = []
        for input_index, rel in enumerate(scope):
            path = target / rel
            if path.is_symlink() or not path.is_file():
                raise ValueError('original scoped input absent or nonregular')
            data = path.read_bytes()
            input_hashes.append(dict(path=rel, sha256=sha(data)))
            work = folder / ('input-' + str(input_index))
            try:
                native = native_query(args.native, data.decode('utf8'), work)
                raw_entities += len(native['native_entities']);raw_fields += len(native['native_written_fields'])
                unnamed += sum(not node.get('name') for node in native['native_entities'])
                located_raw = subprocess.check_output([str(args.locator), '-locate-source', str(path)], timeout=30)
                located = json.loads(located_raw)
                defs, refs, classified = project(rel, native, located)
                definitions.extend(defs);references.extend(refs);outcomes.update(classified)
            except (ValueError, subprocess.SubprocessError, UnicodeError) as error:
                refused.append(dict(cohort=ordinal, input_ordinal=input_index, source_sha256=sha(data),
                                    reason_class=type(error).__name__, reason_sha256=sha(str(error).encode())))
        after = tracked_receipt(source, folder / 'after')
        if before != after or before['untracked_entries']:
            raise ValueError('original tracked set changed')
        all_rows.append(dict(cohort=ordinal, source_sha=repo['sha'], historical_source_sha=subject['sha'],
                             native_inputs=len(scope), native_entities=raw_entities, native_written_fields=raw_fields,
                             native_unnamed_entities=unnamed, named_definitions=len(definitions),
                             semantic_references=len(references), outcomes=dict(outcomes),
                             input_manifest_sha256=sha(json.dumps(input_hashes, sort_keys=True).encode()),
                             original_tracked_before=before, original_tracked_after=after, shadow_copy=mapping))
        for suffix, tier, defs, refs in [('defs', 'C', definitions, []), ('refs', 'A', [], references)]:
            (folder / (suffix + '.json')).write_text(json.dumps(dict(tool='pinned Kong native typed loader/registry; independent C positions',
                    version='go-database-reconciler1.42.4', tier=tier, definitions=defs, references=refs)))
        comparisons.append((ordinal, target, repo['sha'], scope, folder))
        mappings += ['-subject', 'cohort-' + str(ordinal) + '=' + str(target)]
    summary = dict(kind='full original-population Kong L1/L2 diagnostic; canonical reading unchanged',
                   native_binary_sha256=identity['binary_sha256'], cohorts=all_rows, refusals=refused,
                   status='native_family_refused' if refused else 'native_population_complete',
                   canonical_counts_before=prior['counts'], canonical_grades_before=prior['grade'],
                   correctness_start_conditions=start)
    if not refused:
        # Complete every native input first; no partial family is graded.
        for ordinal, target, source_sha, scope, folder in comparisons:
            run([args.indexbench, 'lang-oracle', '-dialect', 'kong', '-subject', target,
                 '-subject-name', 'cohort-' + str(ordinal), '-subject-sha', source_sha,
                 '-tool-oracle', folder / 'defs.json', '-tool-oracle', folder / 'refs.json',
                 '-only', ','.join(scope), '-out', reading,
                 '-note', 'Native typed original population before comparison; C independent source positions, native written fields. L1/L2 diagnostic only; canonical L3 unchanged. Full source/copy identities in separate ordinal receipt.'])
        timing = dict(start_load_1m=os.getloadavg()[0], nice=os.getpriority(os.PRIO_PROCESS, 0),
                      machine_class=platform.system() + '/' + platform.machine(),
                      serial_lane_job=True, shared_host_other_activity='uncontrolled')
        timed = timing['start_load_1m'] <= 6
        if timed:
            run([args.indexbench, 'lang-retime', '-reading', reading, '-runs', '3', '-write', *mappings])
            timing['end_load_1m'] = os.getloadavg()[0]
            timed = timing['end_load_1m'] <= 6
        timing['runs'] = 3 if timed else 0
        timing['status'] = 'measured_median_of_3' if timed else 'unmeasured'
        # Initial correctness-run timings cannot establish a performance grade.
        # Keep their raw reading private, and omit them from public evidence.

        measured = json.loads(reading.read_text())
        summary.update(counts=measured['counts'], metrics=measured['metrics'], metric_tiers=measured['metric_tiers'],
                       grade=measured['grade'] if timed else 'unmeasured: performance gate not established',
                       timing_conditions=timing, tenancy=measured['tenancy'] if timed else dict(faults=measured['tenancy']['faults']),
                       per_cohort=[{key: row[key] for key in ('sha', 'files', 'bytes', 'counts', 'families')} for row in measured['subjects']],
                       scope_exclusion_counts=dict(notes=sum('skipped by ingest' in note or 'tool-oracle entries on files' in note for note in measured['notes'])),
                       timing_notes=[note for note in measured['notes'] if note.startswith('throughput (design 1.9 T1, T2)')] if timed else ['Throughput unmeasured under T-R10; correctness replay only.'])
    summary['oracle_sources'] = [dict(path=name, sha256=sha((HERE / name).read_bytes())) for name in
                                 ('native/main.go', 'instrument_native.py', 'probe_native_controls.py', 'grade_native_l2.py', 'locations.go', 'main.go')]
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    # Private logs/inventories remain, but measured source clones/copies do not.
    for directory in args.subjects.iterdir():
        shutil.rmtree(directory)
    args.subjects.rmdir()
    if refused:
        raise SystemExit('whole native family refused; see counts/hashes in summary, raw details private')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for name in ('native', 'build-receipt', 'controls', 'projection-controls', 'locator', 'indexbench', 'subjects', 'out'):
        parser.add_argument('-' + name, type=Path, required=True)
    args = parser.parse_args()
    for name in ('native', 'build_receipt', 'controls', 'projection_controls', 'locator', 'indexbench', 'subjects', 'out'):
        setattr(args, name, getattr(args, name).resolve())
    try:
        measure(args)
    finally:
        if args.subjects.exists():
            shutil.rmtree(args.subjects)
