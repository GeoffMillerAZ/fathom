#!/usr/bin/env python3
"""Redirect Guzzle's hard-coded server log without changing any graded AST field."""
import argparse
import hashlib
import json
from pathlib import Path
import shlex
import subprocess


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--restore', action='store_true', help='restore only this known helper instrumentation')
    args = ap.parse_args()
    base = args.tools.resolve()
    root = base/'guzzle'
    here = Path(__file__).resolve().parent
    php = base/'runtime/php'
    autoload = base/'callsite-fork/vendor/autoload.php'
    target = root/'tests/Server.php'
    original = subprocess.check_output(['git', 'show', 'HEAD:tests/Server.php'], cwd=root)
    shell_path = shlex.quote(str(root/'test-server.log')).replace('\\', '\\\\').replace("'", "\\'")
    replacement = original.replace(b'>> /tmp/server.log', ('>> '+shell_path).encode())
    if replacement == original or target.read_bytes() not in (original, replacement):
        raise ValueError('unexpected helper content; do not overwrite other edits')
    if args.restore:
        target.write_bytes(original)
        print('restored original helper bytes')
        return

    def scopes():
        return [subprocess.check_output([str(php), '-n', str(script), str(autoload), str(root), *extra])
                for script, extra in [(here/'literal_sites.php', []),
                                      (here.parents[2]/'calibration/oracles/php-parser/PhpParserDump.php', ['definitions-only'])]]

    # A retry restores only this known, owned instrumentation change.
    target.write_bytes(original)
    before = scopes()
    target.write_bytes(replacement)
    after = scopes()
    if before != after:
        target.write_bytes(original)
        raise ValueError('log redirection changed a graded syntax field')
    receipt = {'instrumentation': 'one test-helper string literal redirects a server log inside the subject workspace',
               'original_sha256': hashlib.sha256(original).hexdigest(),
               'instrumented_sha256': hashlib.sha256(replacement).hexdigest(),
               'literal_sites_unchanged': True, 'definitions_names_ranges_signatures_and_call_pairs_unchanged': True,
               'literal_site_population_sha256': hashlib.sha256(before[0]).hexdigest(),
               'syntax_dump_sha256': hashlib.sha256(before[1]).hexdigest()}
    (base/'runtime-instrumentation.json').write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')
    print(json.dumps(receipt, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
