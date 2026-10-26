#!/usr/bin/env python3
"""Xdebug's actual calls, attributed by PHP Reflection; syntax is denominator only."""
import argparse
from collections import Counter, defaultdict
import hashlib
import json
import os
from pathlib import Path
import resource
import signal
import subprocess

from fetch import ready


def bare(name):
    return name.replace('->', '::').split('::')[-1].split('\\')[-1]


def decode(root, output, syntax, php_version='8.5.8'):
    root = root.resolve()
    ranges = defaultdict(dict)
    for path in output.glob('reflection-*.json'):
        for function in json.loads(path.read_text())['functions']:
            key = (function['start'], function['end'], bare(function['name']))
            ranges[function['file']][key] = function
    locations = defaultdict(list)
    blocked = {(site['path'], line) for site in syntax.get('nonliteral_call_lines', [])
               for line in range(site['line'], site['end']+1)}
    for number, site in enumerate(syntax['sites']):
        for line in range(site['line'], site.get('end', site['line'])+1):
            locations[(site['path'], line, site['written_name'].lower())].append(number)
    pairs, attested = set(), set()
    counts = Counter()
    for path in sorted(output.glob('trace.*.xt')):
        stack = {}
        with path.open(errors='replace') as source:
            for line in source:
                fields = line.rstrip('\n').split('\t')
                if len(fields) >= 3 and fields[0].isdigit() and fields[2] == '1':
                    depth = int(fields[0])
                    stack = {d: fn for d, fn in stack.items() if d < depth}
                    continue
                if len(fields) < 10 or fields[2] != '0' or not fields[0].isdigit():
                    continue
                depth = int(fields[0])
                parent = stack.get(depth-1)
                stack = {d: fn for d, fn in stack.items() if d < depth}
                stack[depth] = fields[5]
                counts['native_entries'] += 1
                filename, target = fields[8], bare(fields[5])
                try:
                    relative = Path(filename).relative_to(root).as_posix()
                    lineno = int(fields[9])
                except (ValueError, TypeError):
                    continue
                counts['native_entries_under_subject_root_including_vendor'] += 1
                if target.startswith('__') or (relative, lineno) in blocked:
                    counts['ambiguous_nonliteral_or_magic_source'] += 1
                    continue
                enclosing = [v for v in ranges.get(filename, {}).values() if v['start'] <= lineno <= v['end']]
                if not enclosing:
                    counts['no_named_native_caller'] += 1
                    continue
                width = min(v['end']-v['start'] for v in enclosing)
                callers = {bare(v['name']) for v in enclosing if v['end']-v['start'] == width}
                if len(callers) != 1:
                    counts['ambiguous_native_caller'] += 1
                    continue
                eligible = locations.get((relative, lineno, target.lower()), [])
                if not eligible:
                    counts['unpaired_native_entries'] += 1
                    continue
                # Both endpoints below are native runtime answers. Syntax only
                # establishes literal-call scope and a conservative coverage key.
                caller = next(iter(callers))
                if parent is None or (bare(parent).lower() != caller.lower()
                                      and '{closure:'+filename+':' not in parent):
                    counts['unpaired_native_parent'] += 1
                    continue
                pairs.add((relative, caller, target))
                if len(eligible) == 1:
                    attested.add(eligible[0])
                else:
                    counts['ambiguous_same_line_entries'] += 1
    oracle = {'tier': 'A', 'tool': 'Xdebug actual PHP calls + native PHP Reflection', 'version': '3.5.3 / PHP '+php_version,
              'calls': [{'path': p, 'from': f, 'to': t} for p, f, t in sorted(pairs)],
              'disclose': ['OL-R9: executed literal-call population only; unexecuted/ambiguous syntax sites are not native recall truth']}
    return oracle, {'files': syntax['files'], 'literal_sites': len(syntax['sites']), 'attested_sites': len(attested),
                    'coverage': len(attested)/len(syntax['sites']) if syntax['sites'] else None,
                    'unattested_sites': len(syntax['sites'])-len(attested), 'native_pairs': len(pairs),
                    'native_counts': dict(sorted(counts.items())),
                    'pair_source': 'Xdebug target + smallest enclosing named PHP Reflection range; never syntax pairs filtered by answers',
                    'site_population_source': 'php-parser syntax, denominator/scope only',
                    'same_line_duplicate_policy': 'unattested when multiple literal sites share file/line/name',
                    'nonliteral_policy': 'unattested when a nonliteral call shares the source line; magic dispatch is not attributed from line-only traces',
                    'population_sha256': hashlib.sha256(json.dumps(syntax, sort_keys=True).encode()).hexdigest()}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--subject', required=True, type=Path)
    ap.add_argument('--out', required=True, type=Path)
    ap.add_argument('--fixture', action='store_true')
    ap.add_argument('--max-trace-mib', type=int, default=2048, help='bounded private trace file size')
    args = ap.parse_args()
    if not 1 <= args.max_trace_mib <= 2048:
        raise ValueError('trace cap must be between 1 and 2048 MiB')
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    base, subject, out = args.tools.resolve(), args.subject.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    here = Path(__file__).resolve().parent
    php = base/'runtime/php'
    php_version = subprocess.check_output([str(php), '-n', '-r', 'echo PHP_VERSION;'], text=True)
    ready()
    with (out/'syntax.json').open('w') as target:
        subprocess.run([str(php), '-n', str(here/'literal_sites.php'), str(base/'callsite-fork/vendor/autoload.php'),
                        str(subject)], check=True, stdout=target)
    env = {k: v for k, v in os.environ.items() if k in {'PATH', 'HOME', 'LANG', 'TMPDIR', 'XDG_CACHE_HOME', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME'}}
    env.update(PHP_ORACLE_SUBJECT=str(subject), PHP_ORACLE_META_DIR=str(out), XDEBUG_MODE='trace', PHP_INI_SCAN_DIR='')
    env['PATH'] = str(php.parent)+os.pathsep+env.get('PATH', '')
    extension = base/'build-sources/xdebug-3.5.3/modules/xdebug.so'
    cmd = [str(php), '-n', '-d', 'zend_extension='+str(extension), '-d', 'memory_limit=512M',
           '-d', 'xdebug.mode=trace', '-d', 'xdebug.start_with_request=yes', '-d', 'xdebug.trace_format=1',
           '-d', 'xdebug.collect_params=0', '-d', 'xdebug.collect_return=0', '-d', 'xdebug.use_compression=0',
           '-d', 'xdebug.output_dir='+str(out), '-d', 'xdebug.trace_output_name=trace.%p',
           '-d', 'auto_prepend_file='+str(here/'reflect_shutdown.php')]
    if args.fixture:
        cmd += [str(subject/'native.php')]
    else:
        cmd += ['vendor/bin/phpunit', '--no-coverage', '--colors=never', '--random-order-seed=1']
    ready()
    def limits():
        cap = args.max_trace_mib << 20
        resource.setrlimit(resource.RLIMIT_FSIZE, (cap, cap))
    with (out/'phpunit.log').open('w') as log:
        process = subprocess.Popen(cmd, cwd=subject, env=env, stdout=log, stderr=subprocess.STDOUT,
                                   start_new_session=True, preexec_fn=limits)
        try:
            status = process.wait(timeout=300)
        except subprocess.TimeoutExpired:
            status = 'timeout'
        finally:
            # Only the new process group created for this driver, including its
            # own subject test server; never a pre-existing shared process.
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
    syntax = json.loads((out/'syntax.json').read_text())
    oracle, summary = decode(subject, out, syntax, php_version)
    traces = list(out.glob('trace.*.xt'))
    def complete(path):
        with path.open('rb') as source:
            source.seek(max(0, path.stat().st_size-200))
            return b'TRACE END' in source.read()
    summary.update(driver='original vendor/bin/phpunit' if not args.fixture else 'original contract fixture',
                   driver_exit=status, php=php_version, xdebug='3.5.3', trace_format=1,
                   trace_cap_mib=args.max_trace_mib,
                   trace_bytes=sum(p.stat().st_size for p in traces),
                   trace_complete=bool(traces) and all(complete(p) for p in traces))
    if args.fixture:
        actual = {(p['path'], p['from'], p['to']) for p in oracle['calls']}
        expected = {('native.php', 'driver', 'target'), ('native.php', 'target', 'strlen')}
        if actual != expected or summary['literal_sites'] != 4 or summary['attested_sites'] != 2 or status != 0:
            raise AssertionError('native fixture must leave alias and same-line dynamic sites unattested')
        summary['fixture_assertions_passed'] = True
    (out/'oracle.json').write_text(json.dumps(oracle, indent=2, sort_keys=True)+'\n')
    (out/'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
