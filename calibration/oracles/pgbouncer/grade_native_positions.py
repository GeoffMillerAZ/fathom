"""Replay unchanged PgBouncer L1/L2 with native token positions and native pairs."""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path
HERE = Path(__file__).resolve().parent


def run(command):
    subprocess.run(['df', '-h', str(Path.home())], check=True)
    if shutil.disk_usage(Path.home()).free < 20 * 1024**3:
        raise ValueError('20 GiB disk floor')
    subprocess.run([str(value) for value in command], check=True)


def tracked_receipt(subject, subpath, receipt):
    # Run the exact normative recipe, not a second discovery implementation.
    design = (HERE.parents[2] / 'docs/design/language-breadth.md').read_text()
    marker = "SUBJECT=workspace/calibration/<subject> SUBPATH=. RECEIPT=workspace/artifacts/lang-<id>/<subject>-before python3 - <<'PY'\n"
    body = design.split(marker, 1)[1].split('\nPY\n', 1)[0]
    env = dict(os.environ, SUBJECT=str(subject), SUBPATH=subpath, RECEIPT=str(receipt))
    subprocess.run([sys.executable, '-B', '-c', body], env=env, check=True)
    summary = json.loads(Path(str(receipt) + '.json').read_text())
    untracked = Path(str(receipt) + '.untracked').read_bytes()
    summary.update(untracked_entries=len([p for p in untracked.split(b'\0') if p]),
                   untracked_sha256=hashlib.sha256(untracked).hexdigest(),
                   normative_recipe_sha256=hashlib.sha256(body.encode()).hexdigest())
    return summary


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for name in ['subject', 'pgbouncer', 'indexbench', 'out']:
        parser.add_argument('-' + name, type=Path, required=True)
    args = parser.parse_args()
    args.subject = args.subject.resolve()
    args.out = args.out.resolve()
    if args.out.exists():
        raise ValueError('fresh output directory required')
    args.out.mkdir(parents=True)
    before = tracked_receipt(args.subject, '.', args.out / 'tracked-before')
    native = args.out / 'positions'
    pairs = args.out / 'pairs'
    run([sys.executable, '-B', HERE / 'native_positions.py', '-subject', args.subject,
         '-pgbouncer', args.pgbouncer.resolve(), '-work', args.out / 'position-work', '-out', native])
    run([sys.executable, '-B', HERE / 'native_oracle.py', '-subject', args.subject,
         '-pgbouncer', args.pgbouncer.resolve(), '-build-receipt', HERE / 'native-position-build-r1.json',
         '-work', args.out / 'pair-work', '-out-dir', pairs])
    reading = args.out / 'reading.json'
    run([args.indexbench.resolve(), 'lang-oracle', '-dialect', 'pgbouncer',
         '-subject', args.subject, '-subject-name', 'pgbouncer/pgbouncer',
         '-tool-oracle', native / 'defs.json', '-tool-oracle', pairs / 'refs.json', '-out', reading,
         '-note', 'Native PgBouncer 1.26.1 cfparser enumerates every explicit token before comparison; each token line is attested by syntax ablation. Five existing logfile values point to private scratch; graded keys and lines are unchanged. Native SHOW DATABASES enumerates all sources for auth_dbname bindings. Full six-file canonical population; no backend SQL, Docker or L3.'])
    run([args.indexbench.resolve(), 'lang-retime', '-reading', reading, '-subject',
         'pgbouncer/pgbouncer=' + str(args.subject), '-runs', '3', '-write'])
    after = tracked_receipt(args.subject, '.', args.out / 'tracked-after')
    if before != after or after['untracked_entries']:
        raise ValueError('tracked/untracked input identity changed')
    result = json.loads(reading.read_text())
    population = json.loads((native / 'summary.json').read_text())
    if result['counts']['oracle_defs'] != population['definitions']:
        raise ValueError('native population excluded by harness gates')
    summary = dict(subject=result['subjects'][0]['name'], sha=result['subjects'][0]['sha'],
                   counts=result['counts'], grade=result['grade'], metrics=result['metrics'], metric_tiers=result['metric_tiers'],
                   subjects=[{key: subject[key] for key in ['name','sha','files','bytes','counts','families']}
                             for subject in result['subjects']],
                   native_positions=population, tracked_before=before, tracked_after=after,
                   submodule_contents='No gitlinks in tracked approximation; no submodule contents graded.',
                   reading_sha256=hashlib.sha256(reading.read_bytes()).hexdigest())
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary, sort_keys=True))
