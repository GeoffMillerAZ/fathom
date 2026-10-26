#!/usr/bin/env python3
"""Freeze the original subjects around a fresh complete syntax-population audit."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

from fetch import SOURCES, ready


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', type=Path, required=True)
    ap.add_argument('--out', type=Path, required=True)
    args = ap.parse_args()
    base, out = args.tools.resolve(), args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    design = (repo/'docs/design/language-breadth.md').read_text()
    start = design.index('import hashlib, json, os, pathlib, stat, subprocess', design.index('### 13.3.1'))
    end = design.index('\nPY\n```', start)
    assert (here/'tracked_set.py').read_text() == design[start:end]+'\n', 'manifest differs from the exact recipe'
    reading = json.loads((repo/'docs/bench/readings/lang/php.json').read_text())
    public = {'scope': 'fresh syntax-population audit at original pins; not retrospective manifests of discarded native suites',
              'canonical_reading_changed': False, 'subjects': {}}
    for name, expected_sites in [('guzzle', 3189), ('monolog', 5968)]:
        root = base/name
        pin = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
        assert pin == SOURCES[name][1]
        canonical = next(s for s in reading['subjects'] if s['sha'] == pin)

        def manifest(suffix):
            prefix = out/(name+'-'+suffix)
            env = dict(os.environ, SUBJECT=str(root), SUBPATH='.', RECEIPT=str(prefix))
            subprocess.run([sys.executable, '-B', str(here/'tracked_set.py')], env=env, check=True)
            return json.loads(prefix.with_suffix('.json').read_text()), prefix.with_suffix('.untracked')

        ready()
        before, _ = manifest('before')
        php = base/'runtime/php'
        autoload = base/'callsite-fork/vendor/autoload.php'
        ready()
        sites = json.loads(subprocess.check_output([str(php), '-n', str(here/'literal_sites.php'), str(autoload), str(root)]))
        ready()
        dump = json.loads(subprocess.check_output([str(php), '-n', str(repo/'calibration/oracles/php-parser/PhpParserDump.php'),
                                                 str(autoload), str(root)]))
        after, untracked = manifest('after')
        assert before == after, 'tracked subject changed during oracle audit'
        # Fresh subject trees contain no installed dependencies. Inspect every
        # added ignored/untracked path; this replay expects none at all.
        assert untracked.read_bytes() == b'', 'new untracked/ignored input requires explicit review'
        pairs = {(x['path'], x['from'], x['to']) for x in dump['calls']}
        assert len(pairs) == canonical['counts']['call_truth']
        assert sites['files'] == canonical['files'] and len(sites['sites']) == expected_sites
        paths = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).split(b'\0')
        php_paths = [root/os.fsdecode(p) for p in paths if p.endswith(b'.php')]
        php_bytes = sum(p.stat().st_size for p in php_paths if p.is_file() and not p.is_symlink())
        assert len(php_paths) == canonical['files'] and php_bytes == canonical['bytes']
        public['subjects'][name] = dict(revision=pin, subpath='.', before=before, after=after,
            reading_files=canonical['files'], reading_bytes=canonical['bytes'], syntax_files=sites['files'],
            original_php_bytes=php_bytes, literal_sites=len(sites['sites']), unique_written_pairs=len(pairs),
            generated_untracked_or_ignored_paths=0,
            exclusions='non-PHP tracked files reconcile the manifest difference; .git, dot directories, vendor and node_modules skipped by syntax; no symlinks followed; submodule contents not covered',
            receipt_prefix='workspace/artifacts/lang-php/scope/'+name+'-{before,after}')
    (out/'scope-proof.json').write_text(json.dumps(public, indent=2, sort_keys=True)+'\n')
    print(json.dumps(public, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
