"""Verify round-two decisions against the reading, pinned receipts and raw archive."""
import sys
sys.dont_write_bytecode = True
from collections import Counter
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tarfile
from recount import product_path as base_product_path
from verify_apps import check_privacy



def product_path(path):
    parts = PurePosixPath(path).parts
    support = re.compile(r'^(?:test(?:[-_].+|lib|utils?|helpers?|support|suite)|.+[-_]tests?|playground|rfc|drafts)$', re.I)
    return (base_product_path(path) and parts[0] != 'features'
            and not any(support.fullmatch(part) for part in parts[:-1]))


def comparable_go_names(names):
    result = set()
    for package, test in names:
        # This pinned test embeds testing.TempDir's random directory number.
        # Preserve every other path component and require no identity merges.
        if package == 'github.com/goreleaser/goreleaser/v2/cmd' and test.startswith('TestCheckPathLiteralExecutable/'):
            test = re.sub(r'(/work/tmp/TestCheckPathLiteralExecutable)[0-9]+(?=/)', r'\1<TMP>', test)
        result.add((package, test))
    assert len(result) == len(names), 'normalization merged distinct cases'
    return result


def go_events(raw):
    result = []
    for line in raw.decode().splitlines():
        try:
            value = json.loads(line)
        except ValueError:
            continue
        if isinstance(value, dict) and 'Action' in value:
            result.append(value)
    return result


def go_leaf_names(events, comparable=False):
    names = {(e['Package'], e['Test']) for e in events if e.get('Test') and e['Action'] == 'run'}
    parents = {(p, t[:i]) for p, t in names for i, char in enumerate(t) if char == '/'}
    leaves = names - parents
    # A parent skipped before spawning children in another driver invocation is
    # itself a terminal leaf; do not lose it because a different invocation ran children.
    leaves |= {(e['Package'], e['Test']) for e in events if e.get('Test') and e['Action'] == 'skip'}
    return comparable_go_names(leaves) if comparable else leaves


def skipped_go_tests(events):
    skipped = {(e['Package'], e['Test']) for e in events if e.get('Test') and e['Action'] == 'skip'}
    result = {key: [] for key in sorted(skipped)}
    for event in events:
        key = (event.get('Package'), event.get('Test'))
        if key in result and event.get('Output'):
            result[key].append(event['Output'].strip())
    return result


def skipped_vitest_tests(report):
    result = {}
    for file in report['testResults']:
        occurrences = Counter()
        for test in file['assertionResults']:
            occurrences[test['fullName']] += 1
            if test['status'] in {'pending', 'skipped', 'todo', 'disabled'}:
                key = (file['name'], test['fullName'], occurrences[test['fullName']])
                result[key] = [test['status'], *test.get('failureMessages', [])]
    return result


def skipped_pytest_tests(raw):
    import xml.etree.ElementTree as ET
    result = {}
    for test in ET.fromstring(raw).iter('testcase'):
        skip = test.find('skipped')
        if skip is not None:
            result[(test.get('classname'), test.get('name'))] = [skip.get('message', ''), (skip.text or '').strip()]
    return result


def skipped_tests(language, raw):
    if language == 'Go':
        return skipped_go_tests(go_events(raw))
    if language == 'Python':
        return skipped_pytest_tests(raw)
    return skipped_vitest_tests(json.loads(raw))


def markdown_rows(reading):
    # Escape table delimiters without discarding literal backticks in test names.
    return [[c.strip().replace('\\|', '|') for c in re.split(r'(?<!\\)\|', line[1:-1])]
            for line in reading.splitlines() if line.startswith('|') and line.endswith('|')]


def check_skips(subject, contents):
    observed = {}
    for stage, artifacts in subject['skip_artifacts'].items():
        found = {}
        for artifact in artifacts:
            raw = contents[artifact]
            skips = skipped_tests(subject['language'], raw)
            for key, lines in skips.items():
                found.setdefault(key, []).extend(lines)
        observed[stage] = found
    assert set(observed) == {'warmup', 'run1', 'run2', 'run3'}
    expected = {tuple(row['identity']): row for row in subject['skipped_leaves']}
    assert len(expected) == len(subject['skipped_leaves'])
    assert set(expected) == set().union(*(set(v) for v in observed.values()))
    for key, row in expected.items():
        assert row['classification'] in {'environment', 'platform'}
        assert row['test'] == '::'.join(map(str, key))
        # Platform relief must have an actual OS/architecture guard, never merely
        # a tool which happens to be unavailable on this architecture.
        if row['classification'] == 'platform':
            assert row.get('source_artifact') and row.get('source_quote'), 'platform skip lacks source guard'
            assert re.search(r'runtime\.GO(?:OS|ARCH)|sys\.platform|os\.name', row['source_quote'])
            assert row['source_quote'] in contents[row['source_artifact']].decode()
        evidence = {e['stage']: e for e in row['evidence']}
        assert set(evidence) == {stage for stage, tests in observed.items() if key in tests}
        for stage, item in evidence.items():
            assert item['artifact'] in subject['skip_artifacts'][stage]
            assert item['quote'] in observed[stage][key]
        assert row['quote'] == row['evidence'][0]['quote']
    assert subject['skip_phase_counts'] == {stage: {kind: sum(key in tests and row['classification'] == kind for key, row in expected.items()) for kind in ['environment', 'platform']} for stage, tests in observed.items()}
    assert subject['env_skips'] == sum(r['classification'] == 'environment' for r in expected.values())
    assert subject['platform_skips'] == sum(r['classification'] == 'platform' for r in expected.values())
    excluded = {x['test']: x for x in subject['excluded']}
    for row in expected.values():
        if row['classification'] == 'environment':
            assert row['test'] in excluded
            assert excluded[row['test']]['log'] == row['quote']
            assert excluded[row['test']]['reason'] == row['reason']


def check_exclusions(subject, contents, static):
    excluded = {x['test']: x for x in subject['excluded']}
    assert len(excluded) == len(subject['excluded'])
    env = {r['test'] for r in subject['skipped_leaves'] if r['classification'] == 'environment'}
    failures = set(subject['failure_exclusions'])
    assert set(excluded) == env | failures
    assert all('setup' in x['reason'].lower() for x in excluded.values())
    if failures:
        baseline = json.loads(contents[subject['baseline_suite_artifact']])
        stages = {'warmup': baseline['warmup'], **{'run'+str(i+1): r for i, r in enumerate(baseline['runs'])}}
        baseline_names = None
        for stage, run in stages.items():
            raw = contents['logs/' + run['log']]
            assert hashlib.sha256(raw).hexdigest() == run['log_sha256']
            assert run['network'] == 'none' and run['exit_code'] not in {0, 124}
            events = go_events(raw)
            leaves = go_leaf_names(events)
            failed = {(e['Package'], e['Test']) for e in events if e.get('Test') and e['Action'] == 'fail'} & leaves
            assert {t for p, t in failed} == failures | set(subject.get('setup_repaired_tests', []))
            names = comparable_go_names(leaves)
            if baseline_names is None:
                baseline_names = names
            assert baseline_names == names
            for package, test in failed:
                if test not in failures:
                    continue
                evidence = [e for e in subject['exclusion_evidence'][test] if e['stage'] == stage]
                assert len(evidence) == 1 and evidence[0]['log'] == 'logs/' + run['log']
                output = ''.join(e.get('Output', '') for e in events if e.get('Package') == package and e.get('Test') == test)
                assert evidence[0]['quote'] in output
                assert excluded[test]['log'] == subject['exclusion_evidence'][test][0]['quote']
        for run in [subject['suite']['warmup'], *subject['suite']['runs']]:
            events = go_events(contents['logs/' + run['log']])
            ran = comparable_go_names(go_leaf_names(events))
            assert not {t for p, t in ran} & failures
            passed = {e['Test'] for e in events if e.get('Test') and e['Action'] == 'pass'}
            assert set(subject.get('setup_repaired_tests', [])) <= passed
    fixes = [h for h in static['history'] if h['fix_message'] and h['primary_source']]
    files = set(subject['d28_defining_files'])
    dropped = {h['sha'] for h in fixes if set(h['tests'] + h['primary_source']) & files}
    assert dropped == set(subject['d28_dropped_fix_commits'])
    assert subject['eligible_fix_co_changes'] == len(fixes) - len(dropped) >= 30


def leaf_tests(language, contents, artifacts):
    names = set()
    if language == 'Go':
        for artifact in artifacts:
            names |= go_leaf_names(go_events(contents[artifact]), comparable=True)
        return len(names)
    if language == 'TypeScript':
        for artifact in artifacts:
            report = json.loads(contents[artifact])
            assert report['numTotalTests'] == sum(len(r['assertionResults']) for r in report['testResults'])
            for result in report['testResults']:
                occurrences = Counter()
                for test in result['assertionResults']:
                    # Parameterized cases can share a full name. Count each leaf,
                    # while deduplicating the same file/case across builder runs.
                    occurrences[test['fullName']] += 1
                    names.add((result['name'], test['fullName'], occurrences[test['fullName']]))
        return len(names)
    import xml.etree.ElementTree as ET
    for artifact in artifacts:
        root = ET.fromstring(contents[artifact])
        for case in root.iter('testcase'):
            names.add((case.get('classname'), case.get('name')))
    return len(names)


def verify(path=Path('docs/bench/readings/bench-prep/apps-r2.json')):
    data = json.loads(path.read_text())
    subjects = data['subjects']
    assert len(subjects) == len({s['id'] for s in subjects}) == 18
    assert Counter(s['language'] for s in subjects if not s['rescreen']) == {'Go': 5, 'Python': 4, 'TypeScript': 3}
    assert {s['id'] for s in subjects if s['rescreen']} == {
        'derailed--k9s', 'goreleaser--goreleaser', 'pdm-project--pdm',
        'python-poetry--poetry', 'verdaccio--verdaccio', 'slidevjs--slidev'}
    archive_path = path.parent / data['archive']['path']
    assert archive_path.stat().st_size <= 6_000_000, 'round-two archive exceeds 6 MB'
    assert hashlib.sha256(archive_path.read_bytes()).hexdigest() == data['archive']['sha256']
    contents = {}
    with tarfile.open(archive_path) as archive:
        members = archive.getmembers()
        assert len(members) == len(data['archive']['members'])
        assert {m.name for m in members} == set(data['archive']['members'])
        for member in members:
            name = PurePosixPath(member.name)
            assert member.isfile() and not name.is_absolute() and '..' not in name.parts
            assert member.uid == member.gid == 0 and not member.uname and not member.gname
            raw = archive.extractfile(member).read()
            assert hashlib.sha256(raw).hexdigest() == data['archive']['members'][member.name]
            check_privacy(raw, member.name)
            contents[member.name] = raw
    for instrument, digest in data['instruments'].items():
        assert hashlib.sha256(contents['instruments/' + instrument]).hexdigest() == digest
    for instrument, digest in data['post_measurement_instruments'].items():
        assert hashlib.sha256(Path(instrument).read_bytes()).hexdigest() == digest
        assert hashlib.sha256(contents['instruments/bx85-' + Path(instrument).name]).hexdigest() == digest
    reading = path.with_suffix('.md').read_text()
    assert f"SHA-256 `{data['archive']['sha256']}`" in reading
    rows = markdown_rows(reading)
    expected_logs = {k: v for k, v in data['archive']['members'].items() if k.startswith('logs/')}
    assert {r[0]: r[1] for r in rows if len(r) == 2 and r[0].startswith('logs/')} == expected_logs
    for language, carried in [('Go', 0), ('Python', 1), ('TypeScript', 1)]:
        selected = [s for s in subjects if s['language'] == language]
        kept = sum(s['status'] == 'kept' for s in selected)
        dev = sum(s['status'] == 'kept' and bool(s['prior_reads']) for s in selected)
        keepers = [s for s in selected if s['status'] == 'kept']
        totals = {'kept_subjects': kept, 'excluded_leaf_tests': sum(len(s['excluded']) for s in keepers),
                  'suite_leaf_tests': sum(s['suite_tests'] for s in keepers),
                  'mean_share_percent': round(sum(100*len(s['excluded'])/s['suite_tests'] for s in keepers)/kept, 4) if kept else 0}
        assert data['exclusion_totals'][language] == totals
        assert [r for r in rows if len(r) == 5 and r[0] == language] == [[language, str(kept), str(totals['excluded_leaf_tests']), str(totals['suite_leaf_tests']), f"{totals['mean_share_percent']:.4f}%"]]
        summary = [r for r in rows if len(r) == 7 and r[0] == language and r[1].isdigit()]
        assert summary == [[language, str(kept), str(len(selected)-kept), str(dev), str(carried+kept), str(max(0, 6-carried-kept)), '3 / 3']]
    for subject in subjects:
        sid = subject['id']
        assert 'env_skips' in subject and 'platform_skips' in subject
        if 'skip_artifacts' not in subject:
            assert subject['env_skips'] is subject['platform_skips'] is None
            assert subject['skip_accounting'], 'unmeasured skip accounting needs a reason'
        assert re.fullmatch('[0-9a-f]{40}', subject['sha'])
        assert subject['full_history'] and subject['replayed'] == 0
        assert subject['c5_window'] == ['2024-10-25T00:00:00Z', '2026-10-26T00:00:00Z']
        assert len(subject['product_directory_witness']) == subject['product_directories'] >= 10
        static = json.loads(contents[subject['static_artifact']])
        for key in ['sha', 'primary_lines', 'primary_fix_co_changes']:
            assert static[key] == subject[key], (sid, key)
        screen_rows = [r for r in rows if len(r) == 7 and r[1] == subject['name'] + ' / ' + subject['sha']]
        assert screen_rows == [[subject['language'], subject['name'] + ' / ' + subject['sha'],
                                f"{subject['primary_lines']:,} / {subject['band']}", str(subject['product_directories']),
                                str(subject['primary_fix_co_changes']), str(subject['c6_qualifying_symbols']) if subject['c6_qualifying_symbols'] is not None else 'unrun',
                                'kept for preparation' if subject['status'] == 'kept' else '; '.join(subject['refusals'])]]
        if subject['c6_artifact']:
            c6 = json.loads(contents[subject['c6_artifact']])
            assert sum(r['qualifies'] for r in c6) == subject['c6_qualifying_symbols']
            for row in c6:
                sites = row['sites']
                paths = [s.rsplit(':', 2)[0] for s in sites]
                assert len(sites) == len(set(sites)) == row['calls']
                assert len({str(PurePosixPath(p).parent) for p in paths}) == len(row['units'])
                assert all(product_path(p) for p in paths)
                assert row['defining_unit'] not in row['units']
                assert row['qualifies'] == (len(sites) >= 5 and len(row['units']) >= 3)
        if 'skip_artifacts' in subject:
            check_skips(subject, contents)
            assert leaf_tests(subject['language'], contents, subject['suite_count_artifacts']) + len(subject['failure_exclusions']) == subject['suite_tests']
        skip_rows = [r for r in rows if len(r) == 6 and r[0] == subject['name']]
        def count(value):
            return 'unmeasured' if value is None else str(value)
        assert skip_rows == [[subject['name'], count(subject['env_skips']), count(subject['platform_skips']), str(len(subject['excluded'])), count(subject['suite_tests']), subject['status']]]
        for exclusion in subject['excluded']:
            label = subject['name'] + ' / ' + exclusion['test']
            quote_rows = [r for r in rows if len(r) == 3 and r[0] == label]
            assert quote_rows == [[label, exclusion['reason'], exclusion['log']]], ('exclusion quote', label)
        if subject.get('final_preparation_artifact'):
            final = subject['final_preparation']
            assert final == json.loads(contents[subject['final_preparation_artifact']])
            stage = final['setup']
            assert final['uid'] == stage['uid'] == 1000 and final['runs'] == []
            assert stage['exit_code'] != 0 and 'warmup' not in final
            raw = contents['logs/' + stage['log']]
            assert hashlib.sha256(raw).hexdigest() == stage['log_sha256']
            assert re.findall(rb'^BENCH_UID=(.*)$', raw, re.M) == [b'1000']
            assert 'go mod download all' in stage['command']
            assert all(pin['sha256'].encode() in raw for pin in json.loads(contents['instruments/bx85-apps-r2-tools.json']) if pin['kind'] == 'prefetch')
        suite = subject['suite']
        def phase(value):
            return 'unrun' if not value else f"{value['seconds']} / {value['exit_code']}"
        phase_rows = [r for r in rows if len(r) == 4 and r[0] == subject['name'] and not r[1].isdigit()]
        assert phase_rows == [[subject['name'], phase(suite.get('setup') if suite else None),
                               phase(suite.get('warmup') if suite else None),
                               ', '.join(phase(r) for r in suite['runs']) if suite and suite['runs'] else 'unrun']]
        if suite is not None:
            assert suite == json.loads(contents[subject.get('suite_artifact', 'receipts/' + sid + '-suite.json')])
            assert suite['sha'] == subject['sha'] and suite['cpus'] == 4
            assert suite['uname_m'] == 'aarch64'
            if subject['status'] == 'kept' or subject.get('remeasured_uid'):
                assert suite['uid'] == 1000
            for stage in [suite['setup']] + [suite[k] for k in ['network_probe', 'warmup'] if k in suite] + suite['runs']:
                assert hashlib.sha256(contents['logs/' + stage['log']]).hexdigest() == stage['log_sha256']
                assert stage['seconds'] > 0 and stage['started_at'].endswith('Z')
                assert stage['uname_m'] == 'aarch64'
                assert b'BENCH_UNAME_M=aarch64\n' in contents['logs/' + stage['log']]
                if subject['status'] == 'kept' or subject.get('remeasured_uid'):
                    assert stage['uid'] == 1000
                    assert re.findall(rb'^BENCH_UID=(.*)$', contents['logs/' + stage['log']], re.M) == [b'1000']
            assert all(r['network'] == 'none' for r in suite['runs'])
        if subject['status'] == 'refused':
            assert subject['refusals'] and subject['manifest'] is None
            assert not Path('calibration/bench.d/' + sid + '.yaml').exists()
            continue
        assert not subject['refusals']
        assert subject['metadata']['archived'] is False
        assert subject['primary_fix_co_changes'] >= 30 and 5000 <= subject['primary_lines'] <= 150000
        assert subject['c6_qualifying_symbols'] >= 3
        assert suite['setup']['exit_code'] == suite['warmup']['exit_code'] == 0
        assert len(suite['runs']) == 3 and all(r['exit_code'] == 0 and r['seconds'] < 900 for r in suite['runs'])
        assert suite['network_probe']['exit_code'] != 0
        assert subject['suite_tests'] > 0
        assert 'skip_artifacts' in subject and subject['remeasured_uid']
        assert leaf_tests(subject['language'], contents, subject['suite_count_artifacts']) + len(subject['failure_exclusions']) == subject['suite_tests']
        assert len(subject['excluded']) <= subject['suite_tests']*2//100
        if subject['excluded']:
            check_exclusions(subject, contents, static)
        assert subject['lock_sha256'] == subject['staged_lock_sha256']
        manifest = Path(subject['manifest'])
        assert hashlib.sha256(manifest.read_bytes()).hexdigest() == subject['manifest_sha256']
        instance = json.loads(manifest.read_text())
        assert instance['sha'] == subject['sha']
        assert instance['bench']['suite_tests'] == subject['suite_tests']
        assert instance['bench'].get('excluded', []) == subject['excluded']
        if subject['prior_reads']:
            assert instance['bench']['split'] == 'dev'
        exclusions_row = [r for r in rows if len(r) == 4 and r[0] == subject['name'] and r[1].isdigit()]
        assert exclusions_row == [[subject['name'], str(len(subject['excluded'])), str(subject['suite_tests']), f"{100*len(subject['excluded'])/subject['suite_tests']:.2f}%"]]
    images = json.loads(contents['receipts/images.json'])
    assert images['pulled'] == images['removed'] == []
    print('PASS: 18 decisions, 12 new candidates, six rescreens, reading tables, native suite receipts, C6 witnesses, manifests and every archive byte')


if __name__ == '__main__':
    verify(Path(sys.argv[1]) if len(sys.argv) > 1 else Path('docs/bench/readings/bench-prep/apps-r2.json'))
