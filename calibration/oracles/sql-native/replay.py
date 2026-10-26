#!/usr/bin/env python3
"""Rebuild native FK truth and take the median of three complete SQL measurements."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--sqitch', required=True)
    ap.add_argument('--pagila', required=True)
    ap.add_argument('--tools', required=True)
    ap.add_argument('--ctags', required=True)
    ap.add_argument('--indexbench', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=False)
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    subjects = [
        ('sqitch', 'sqitchers/sqitch', args.sqitch, 'ce0550c45ade3527d0c7f981c934e1ea4bdd6c19',
         'lib/App/Sqitch/Engine/sqlite.sql,lib/App/Sqitch/Engine/Upgrade/sqlite-*.sql,t/lib/upgradable_registries/sqlite.sql,t/engine/**/*.sql,t/sql/**/*.sql'),
        ('pagila', 'devrimgunduz/pagila', args.pagila, '9baf49c4149e43229f6021e6218d6b2ac8ef4f34',
         'pagila-schema.sql,pagila-schema-jsonb.sql,pagila-temporal-setup.sql'),
    ]

    def run(argv, **kwargs):
        if os.getloadavg()[0] > 6:
            raise ValueError('load above 6; retry in a fresh output directory')
        return subprocess.run([str(a) for a in argv], check=True, **kwargs)

    for item, _name, subject, _sha, scope in subjects:
        run([sys.executable, here / 'sql_native.py', '--subject', subject, '--item', item,
             '--pglite-root', args.tools, '--out', out / (item + '.json')])
        with (out / (item + '.ranges.json')).open('w') as target:
            files = sorted({p.relative_to(subject).as_posix() for pattern in scope.split(',')
                            for p in Path(subject).glob(pattern) if p.is_file()})
            if not files:
                raise ValueError('no scoped range inputs for ' + item)
            run([sys.executable, repo / 'calibration/oracles/parser-ranges/pglast_ranges.py', subject, *files], stdout=target)
    runs = []
    for number in range(1, 4):
        reading = out / f'run-{number}.json'
        for item, name, subject, sha, scope in subjects:
            run([Path(args.indexbench).resolve(), 'lang-oracle', '-dialect', 'sql', '-subject', subject,
                 '-subject-sha', sha, '-subject-name', name, '-only', scope,
                 '-ctags', Path(args.ctags).resolve(), '-tool-oracle', out / (item + '.json'),
                 '-note', 'Three serialized complete measurements at niceness 10, each started below load 6; median pooled CPU throughput retained; scope unchanged',
                 '-unmatched', out / f'{item}-misses-{number}.json', '-out', reading])
        runs.append(json.loads(reading.read_text()))
    for r in runs[1:]:
        for key in ['counts', 'metrics', 'metric_tiers', 'grade', 'level_met']:
            if r[key] != runs[0][key]:
                raise ValueError('non-deterministic measurement: ' + key)
    median = sorted(range(3), key=lambda n: runs[n]['tenancy']['throughput_mb_s'])[1]
    shutil.copyfile(out / f'run-{median + 1}.json', out / 'sql.json')
    summary = {
        'runs_mb_per_cpu_second': [r['tenancy']['throughput_mb_s'] for r in runs],
        'selected_run': median + 1, 'counts_equal': True, 'metrics_equal': True,
        'counts': runs[median]['counts'], 'metrics': runs[median]['metrics'], 'grade': runs[median]['grade'],
        'oracle_code_sha256': {p.name: hashlib.sha256(p.read_bytes()).hexdigest()
                               for p in sorted(here.glob('*')) if p.suffix in ('.py', '.mjs')},
    }
    (out / 'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True) + '\n')
    print(json.dumps(summary, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
