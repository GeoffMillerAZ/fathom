"""Replay the original nginx scope with native L2 truth; never regrade L3.

Full inventories/readings stay in scratch; publication is aggregate evidence.
"""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path
import yaml

HERE = Path(__file__).resolve().parent


def run(command):
    subprocess.run([str(x) for x in command], check=True)


def prepare(repo, dest):
    if not dest.exists():
        dest.mkdir(parents=True)
        run(['git', '-C', dest, 'init', '-q'])
        run(['git', '-C', dest, 'remote', 'add', 'origin', repo['url']])
        run(['git', '-C', dest, 'config', 'remote.origin.promisor', 'true'])
        run(['git', '-C', dest, 'config', 'remote.origin.partialclonefilter', 'blob:none'])
        if repo.get('sparse') or repo.get('subpath'):
            run(['git', '-C', dest, 'sparse-checkout', 'init', '--no-cone'])
            run(['git', '-C', dest, 'sparse-checkout', 'set', '--no-cone', *(repo.get('sparse') or [repo['subpath'] + '/**'])])
        run(['git', '-C', dest, 'fetch', '--depth=1', '--filter=blob:none', 'origin', repo['sha']])
        run(['git', '-C', dest, 'checkout', '--detach', '-q', 'FETCH_HEAD'])
    if subprocess.check_output(['git', '-C', str(dest), 'rev-parse', 'HEAD'], text=True).strip() != repo['sha']:
        raise ValueError('subject pin mismatch')
    if subprocess.check_output(['git', '-C', str(dest), 'status', '--porcelain'], text=True).strip():
        raise ValueError('dirty subject')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-indexbench', type=Path, required=True)
    parser.add_argument('-python', type=Path, required=True)
    parser.add_argument('-nginx', type=Path, required=True)
    parser.add_argument('-build-receipt', type=Path, required=True)
    parser.add_argument('-subjects', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    reading = args.out / 'reading.json'
    if reading.exists():
        raise SystemExit('fresh output required')
    prior = json.loads(Path('docs/bench/readings/lang/nginx.json').read_text())
    repos = yaml.safe_load(Path('calibration/corpus.d/nginx.yaml').read_text())['repos']
    retime = [args.indexbench.resolve(), 'lang-retime', '-reading', reading, '-runs', '3', '-write']
    receipts = []
    for subject in prior['subjects']:
        repo = next(repo for repo in repos if repo['name'].split('/')[-1].lower() == subject['name'])
        if repo['sha'] != subject['sha']:
            raise SystemExit('canonical pin differs from shard')
        name = subject['name']
        checkout = (args.subjects / name).resolve()
        prepare(repo, checkout)
        prefix = name + ': restricted to '
        notes = [note for note in prior['notes'] if note.startswith(prefix)]
        if not notes:
            raise SystemExit('no original scope receipt; do not invent a new exclusion')
        scope = notes[-1][len(prefix):].split(' (-only;', 1)[0].split(', ')
        files = args.out / (name + '-files.txt')
        files.write_text(''.join(f + '\n' for f in sorted(set(scope))))
        output = args.out / (name + '-native.json')
        run([args.python, HERE / 'native_upstream.py', '-subject', checkout, '-subject-sha', repo['sha'],
             '-files', files, '-nginx', args.nginx, '-build-receipt', args.build_receipt,
             '-work', args.out / (name + '-work'), '-out', output])
        receipts.append(json.loads(output.read_text()))
        run([args.indexbench.resolve(), 'lang-oracle', '-dialect', 'nginx', '-subject', checkout,
             '-subject-name', name, '-subject-sha', repo['sha'],
             '-tool-oracle', output.with_name(output.stem + '-defs.json'),
             '-tool-oracle', output.with_name(output.stem + '-refs.json'), '-only', ','.join(scope),
             '-out', reading, '-note', 'Native nginx 1.31.6 target-only upstream ablation over every original canonical L2 pair; independent definition placement C, no L3 truth supplied. Complete one-pair population is small. No server, backend request or Docker. Original scope retained exactly; raw file inventory remains scratch.'])
        retime += ['-subject', name + '=' + str(checkout)]
    run(retime)
    result = json.loads(reading.read_text())
    # Native evidence cannot silently change the measured denominator.
    for key in ('oracle_defs', 'fathom_defs', 'matched', 'ref_truth', 'ref_fathom', 'ref_truth_found', 'ref_fathom_in_truth'):
        if result['counts'].get(key) != prior['counts'].get(key):
            raise SystemExit('original scoped count changed: ' + key)
    summary = {key: result[key] for key in ('dialect', 'fathom_commit', 'counts', 'metrics', 'metric_tiers', 'grade', 'tenancy')}
    summary.update(kind='L1/L2 diagnostic only; canonical L3 reading untouched',
                   native_subjects=receipts, reading_sha256=hashlib.sha256(reading.read_bytes()).hexdigest(),
                   before_L2=prior['grade']['L2'], after_L2=result['grade']['L2'])
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
