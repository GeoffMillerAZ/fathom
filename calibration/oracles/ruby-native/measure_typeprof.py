#!/usr/bin/env python3
"""Full original-population TypeProf measurement; unresolved pairs remain misses."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys

from bootstrap import SUBJECTS, environment, ready


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', type=Path, required=True)
    ap.add_argument('--indexbench', type=Path, required=True)
    ap.add_argument('--out', type=Path, required=True)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise SystemExit('run with nice -n 10')
    base, out = args.tools.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    bootstrap = json.loads((base/'bootstrap.json').read_text())
    ruby = base/bootstrap['ruby']['binary']
    env = environment(base, ruby)
    pyts = base/'pyts/bin/python'
    if not (base/'pyts-install.json').exists():
        ready()
        pipenv = dict(os.environ, PIP_CONFIG_FILE='/dev/null', PIP_CACHE_DIR=str(base/'pip-cache'),
                      PIP_DISABLE_PIP_VERSION_CHECK='1')
        # Resolve a launcher symlink so venv's home points at the actual
        # interpreter/stdlib, not the directory containing a PATH shim.
        subprocess.run([str(Path(sys.executable).resolve()), '-m', 'venv', '--clear', str(base/'pyts')],
                       env=pipenv, check=True)
        ready()
        subprocess.run([str(pyts), '-m', 'pip', 'install', '--report', str(base/'pyts-install.json'),
                        'tree-sitter==0.25.2', 'tree-sitter-ruby==0.23.1'], env=pipenv, check=True)
    design=(repo/'docs/design/language-breadth.md').read_text()
    start=design.index('import hashlib, json, os, pathlib, stat, subprocess',design.index('### 13.3.1'))
    end=design.index('\nPY\n```',start)
    assert (here/'tracked_set.py').read_text()==design[start:end]+'\n','manifest differs from exact recipe'
    reading = json.loads((repo/'docs/bench/readings/lang/ruby.json').read_text())
    public = {'ruling': 'OL-R10', 'canonical_reading_changed': False, 'subjects': {},
              'tools': bootstrap['loaded_gems'], 'ruby': bootstrap['ruby'],
              'benchmark_product_revision':subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip(),
              'oracle_code_sha256':{p:digest(here/p) for p in ['typeprof_export.rb','measure_typeprof.py','tracked_set.py']},
              'scope_code_sha256':{p:digest(repo/'calibration/oracles/ts-tags'/p) for p in ['tags_oracle.py','ruby-tags.scm','ruby-locals.scm']},
              'native_truth': 'TypeProf native method/constant binding answers; never syntax pairs filtered by answer presence'}
    for name, (repository, pin) in SUBJECTS.items():
        root = base/name
        actual = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
        assert actual == pin
        canonical = next(s for s in reading['subjects'] if s['sha'] == pin)
        item = out/name
        item.mkdir()
        def manifest(suffix):
            prefix = item/('tracked-'+suffix)
            menv = dict(env, SUBJECT=str(root), SUBPATH='.', RECEIPT=str(prefix))
            subprocess.run([sys.executable, '-B', str(here/'tracked_set.py')], env=menv, check=True)
            return json.loads(prefix.with_suffix('.json').read_text()), prefix.with_suffix('.untracked').read_bytes()

        ready()
        before, _ = manifest('before')
        ready()
        raw = item/'native-answers.json'
        with raw.open('w') as target, (item/'typeprof.log').open('w') as log:
            process = subprocess.Popen([str(ruby), str(here/'typeprof_export.rb'), str(root)], env=env,
                                       stdout=target, stderr=log, start_new_session=True)
            try:
                status = process.wait(timeout=1200)
            except subprocess.TimeoutExpired:
                status = 'timeout'
            finally:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
        if status != 0:
            (item/'refusal.json').write_text(json.dumps({'whole_unit_refused': True, 'driver_exit': status})+'\n')
            raise RuntimeError('released TypeProf failed on '+name+'; inspect private log, no A oracle published')
        native = json.loads(raw.read_text())
        oracle = {k: native[k] for k in ['tier', 'tool', 'version', 'calls']}
        oracle['disclose'] = ['OL-R10: full source population analyzed; unresolved pairs count as misses in separate full-population audit, not dropped. This diagnostic recall alone is resolved-native-pair recall, never whole-population inference recall.']
        path = item/'oracle.json'
        path.write_text(json.dumps(oracle, indent=2, sort_keys=True)+'\n')
        ready()
        syntax = item/'syntax-population.json'
        with syntax.open('w') as target:
            subprocess.run([str(pyts), str(repo/'calibration/oracles/ts-tags/tags_oracle.py'), '--lang', 'ruby',
                '--query', str(repo/'calibration/oracles/ts-tags/ruby-tags.scm'),
                '--locals', str(repo/'calibration/oracles/ts-tags/ruby-locals.scm'),
                '--ext', '.rb', '--ext', '.rake', '--ext', '.ru', '--basename', 'Rakefile',
                '--shebang', 'ruby', '--shebang', 'jruby', str(root)], stdout=target, check=True)
        written = {(x['path'], x['from'], x['to']) for x in json.loads(syntax.read_text())['calls']}
        resolved = {(x['path'], x['from'], x['to']) for x in oracle['calls']}
        overlap = written & resolved
        ready()
        measured = item/'diagnostic-reading.json'
        with (item/'score.log').open('w') as log:
            subprocess.run([str(args.indexbench.resolve()), 'lang-oracle', '-dialect', 'ruby', '-subject', str(root),
                '-subject-name', repository, '-subject-sha', pin, '-tool-oracle', str(path),
                '-out', str(measured), '-unmatched', str(item/'misses.json')], check=True, stdout=log, stderr=log)
        diagnostic = json.loads(measured.read_text())
        fresh = diagnostic['subjects'][0]
        assert native['receipt']['files'] == fresh['files'], 'reconcile current full source discovery before scoring'
        assert native['receipt']['bytes'] == fresh['bytes'], 'reconcile current complete source bytes before scoring'
        after, untracked = manifest('after')
        assert before == after and untracked == b'', 'oracle changed tracked or generated subject inputs'
        public['subjects'][name] = dict(revision=pin, native=native['receipt'], written_pairs=len(written),
            tracked_before=before, tracked_after=after, generated_untracked_or_ignored_paths=0,
            current_files=fresh['files'], current_bytes=fresh['bytes'],
            historical_reading_files=canonical['files'], historical_reading_bytes=canonical['bytes'],
            historical_written_pairs=canonical['counts']['call_truth'],
            attested_written_pairs=len(overlap), full_unattested_pairs=len(written-overlap),
            full_unattested_fraction=len(written-overlap)/len(written),
            full_matched_fraction_with_unresolved_as_misses=len(overlap)/len(written),
            native_pairs=len(resolved), resolved_native_pair_counts={k:diagnostic['counts'][k] for k in
                ['call_truth','call_truth_found','call_fathom','call_fathom_in_truth']},
            resolved_native_pair_precision=diagnostic['metrics'].get('call_precision'),
            resolved_native_pair_recall=diagnostic['metrics'].get('call_recall'),
            native_oracle_sha256=digest(path), syntax_population_sha256=digest(syntax))
        (out/'summary.json').write_text(json.dumps(public, indent=2, sort_keys=True)+'\n')
        print(name, public['subjects'][name], flush=True)


if __name__ == '__main__':
    main()
