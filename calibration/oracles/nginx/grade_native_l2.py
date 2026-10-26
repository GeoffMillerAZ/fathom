"""Replay the original nginx scope with native L2 truth; never regrade L3.

Full inventories/readings stay in scratch; publication is aggregate evidence.
"""
import argparse
import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path
from resource_guard import require_free_space

HERE = Path(__file__).resolve().parent


def run(command):
    subprocess.run(['df', '-h', str(Path.home())], check=True)
    require_free_space('measurement step')
    subprocess.run([str(x) for x in command], check=True)


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


def original_scope(prior, name):
    prefix = name + ': restricted to '
    notes = [note for note in prior['notes'] if note.startswith(prefix)]
    if not notes:
        raise ValueError('no original scope receipt; do not invent a new exclusion')
    # Retain every previously disclosed input. Conflicting historical receipts
    # must never silently select the narrower last note to improve a reading.
    return sorted({file for note in notes for file in note[len(prefix):].split(' (-only;', 1)[0].split(', ')})


def prepare(repo, dest):
    if not dest.exists():
        require_free_space('calibration subject fetch')
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
    import yaml
    parser = argparse.ArgumentParser()
    parser.add_argument('-indexbench', type=Path, required=True)
    parser.add_argument('-python', type=Path, required=True)
    parser.add_argument('-nginx', type=Path, required=True)
    parser.add_argument('-build-receipt', type=Path, required=True)
    parser.add_argument('-subjects', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('-native-population', action='store_true', help='require native observer identity and authored controls')
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    reading = args.out / 'reading.json'
    if reading.exists():
        raise SystemExit('fresh output required')
    prior = json.loads(Path('docs/bench/readings/lang/nginx.json').read_text())
    repos = yaml.safe_load(Path('calibration/corpus.d/nginx.yaml').read_text())['repos']
    retime = [args.indexbench.resolve(), 'lang-retime', '-reading', reading, '-runs', '3', '-write']
    receipts = []
    manifests = []
    for subject in prior['subjects']:
        repo = next(repo for repo in repos if repo['name'].split('/')[-1].lower() == subject['name'])
        if repo['sha'] != subject['sha']:
            raise SystemExit('canonical pin differs from shard')
        name = subject['name']
        checkout = (args.subjects / name).resolve()
        prepare(repo, checkout)
        if repo.get('sparse'):
            # The normative tracked-file recipe must see real tracked files,
            # not sparse index entries without worktree contents. Restoring
            # these inputs does not widen the measured -only scope below.
            require_free_space('complete tracked-set checkout')
            run(['git', '-C', checkout, 'sparse-checkout', 'disable'])
        subpath = repo.get('subpath', '.')
        before = tracked_receipt(checkout, subpath, args.out / 'manifests' / (name + '-before'))
        scope = original_scope(prior, name)
        files = args.out / (name + '-files.txt')
        files.write_text(''.join(f + '\n' for f in sorted(set(scope))))
        output = args.out / (name + '-native.json')
        runner = 'native_population.py' if args.native_population else 'native_upstream.py'
        run([args.python, HERE / runner, '-subject', checkout, '-subject-sha', repo['sha'],
             '-files', files, '-nginx', args.nginx, '-build-receipt', args.build_receipt,
             '-work', args.out / (name + '-work'), '-out', output])
        receipts.append(json.loads(output.read_text()))
        run([args.indexbench.resolve(), 'lang-oracle', '-dialect', 'nginx', '-subject', checkout,
             '-subject-name', name, '-subject-sha', repo['sha'],
             '-tool-oracle', output.with_name(output.stem + '-defs.json'),
             '-tool-oracle', output.with_name(output.stem + '-refs.json'), '-only', ','.join(scope),
             '-out', reading, '-note', ('Native nginx1.31.6 parser enumerates the whole local upstream population before lexical owner classification; every local pair receives target-only ablation. Reference family A; definitions C. No L3 measurement.' if args.native_population else 'Native nginx 1.31.6 target-only ablation validates independently selected upstream pairs, not native recall completeness. Reference family and definition placement remain C.') + ' No server, backend request or Docker. Scope is the union of all historical disclosures; raw file inventory stays scratch.'])
        after = tracked_receipt(checkout, subpath, args.out / 'manifests' / (name + '-after'))
        if before != after or before['untracked_entries']:
            raise SystemExit('tracked inputs or generated untracked inputs changed: ' + name)
        manifests.append(dict(subject=name, subpath=subpath, before=before, after=after))
        retime += ['-subject', name + '=' + str(checkout)]
    run(retime)
    result = json.loads(reading.read_text())
    # Native evidence cannot silently change the measured denominator.
    fixed = ('oracle_defs', 'fathom_defs', 'matched')
    if not args.native_population:
        fixed += ('ref_truth', 'ref_fathom', 'ref_truth_found', 'ref_fathom_in_truth')
    for key in fixed:
        if result['counts'].get(key) != prior['counts'].get(key):
            raise SystemExit('original scoped count changed: ' + key)
    summary = {key: result[key] for key in ('dialect', 'fathom_commit', 'counts', 'metrics', 'metric_tiers', 'grade', 'tenancy')}
    summary.update(kind='L1/L2 diagnostic only; canonical L3 reading untouched',
                   native_subjects=receipts, reading_sha256=hashlib.sha256(reading.read_bytes()).hexdigest(),
                   tracked_set_receipts=manifests,
                   per_subject=[{key: s[key] for key in ('name', 'sha', 'files', 'bytes', 'counts', 'families')}
                                for s in result['subjects']],
                   before_L2=prior['grade']['L2'], after_L2=result['grade']['L2'])
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
