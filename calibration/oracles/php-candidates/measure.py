#!/usr/bin/env python3
"""Measure the pinned PHP SCIP candidates on the complete original subjects."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

from fetch import SOURCES, ready


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--indexbench', required=True, type=Path)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    base, out = args.tools.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    php = base/'runtime/php'
    indexbench = args.indexbench.resolve()
    env = os.environ.copy()
    env.update(COMPOSER_HOME=str(base/'composer-home'), COMPOSER_CACHE_DIR=str(base/'composer-cache'),
               PHP_INI_SCAN_DIR='', PATH=str(php.parent)+os.pathsep+env['PATH'])

    command_number = 0

    def run(cmd, cwd=None, stdout=None):
        nonlocal command_number
        ready()
        command_number += 1
        with (out/f'command-{command_number}.log').open('w') as log:
            subprocess.run([str(x) for x in cmd], cwd=cwd, env=env, check=True,
                           stdout=stdout if stdout is not None else log, stderr=log)

    summary = {'ruling': 'OL-R9', 'canonical_reading_changed': False, 'candidates': {}}
    for candidate, tier in [('mago-indexer', 'unadmitted released candidate; even A exceeds the full-scope blind cap'),
                            ('callsite-fork', 'C'), ('php84-fork', 'C')]:
        results = {}
        for name, expected in [('guzzle', 74), ('monolog', 217)]:
            subject = base/name
            sha = subprocess.check_output(['git', '-C', str(subject), 'rev-parse', 'HEAD'], text=True).strip()
            if sha != SOURCES[name][1]:
                raise ValueError('wrong immutable subject revision')
            files = subprocess.check_output(['git', '-C', str(subject), 'ls-files', '-z']).split(b'\0')
            php_files = [p for p in files if p.endswith(b'.php')]
            if len(php_files) != expected:
                raise ValueError('incomplete original PHP population')
            subprocess.run(['git', '-C', str(subject), 'diff', '--exit-code', 'HEAD', '--', '*.php'], check=True)
            prefix = out/(candidate+'-'+name)
            scip = prefix.with_suffix('.scip')
            if candidate == 'mago-indexer':
                run([base/'mago-scip-php', subject, '-o', scip])
            elif candidate == 'callsite-fork':
                raw = prefix.with_suffix('.native.json')
                run([php, '-n', base/candidate/'bin/scip-php', '-d', subject, '-o', raw,
                     '--experimental', '--memory-limit=1G'])
                run([php, '-n', '-d', 'memory_limit=512M', here/'unified_to_scip.php',
                     base/candidate/'vendor/autoload.php', raw, scip])
            else:
                run([php, '-n', base/candidate/'bin/scip-php', '--memory-limit=1G'], cwd=subject)
                (subject/'index.scip').rename(scip)
            definitions = prefix.with_suffix('.defs.json')
            with definitions.open('w') as target:
                run([php, '-n', repo/'calibration/oracles/php-parser/PhpParserDump.php',
                     base/'callsite-fork/vendor/autoload.php', subject, 'definitions-only'], stdout=target)
            reading = prefix.with_suffix('.reading.json')
            run([indexbench, 'lang-oracle', '-dialect', 'php', '-subject', subject,
                 '-subject-name', SOURCES[name][0], '-subject-sha', sha, '-scip', scip,
                 '-scip-tier', 'C',
                 '-tool-oracle', definitions, '-unmatched', prefix.with_suffix('.misses.json'), '-out', reading])
            data = json.loads(reading.read_text())
            counts = data['counts']
            results[name] = {'subject_revision': sha, 'original_php_files': len(php_files),
                             'counts': counts, 'metrics': data['metrics'],
                             'scip_sha256': hashlib.sha256(scip.read_bytes()).hexdigest(),
                             'diagnostic_reading_sha256': hashlib.sha256(reading.read_bytes()).hexdigest(),
                             'notes': data.get('notes', [])}
            print(candidate, name, 'precision', data['metrics'].get('call_precision'),
                  'blind', counts.get('calls_oracle_blind', 0), flush=True)
        summary['candidates'][candidate] = {'revision': SOURCES[candidate][1], 'tier_under_OL_R9': tier,
                                           'subjects': results}
        (out/'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')


if __name__ == '__main__':
    main()
