#!/usr/bin/env python3
"""Real Xdebug fixture, extractor-output perturbation and native-input change."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

from fetch import ready
from trace import write_collection


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', type=Path, required=True)
    ap.add_argument('--indexbench', type=Path, required=True)
    ap.add_argument('--out', type=Path, required=True)
    args = ap.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    subject = out/'subject'
    subject.mkdir()
    here = Path(__file__).resolve().parent
    fixture = subject/'native.php'
    original = (here/'fixtures/native.php').read_text()
    fixture.write_text(original)

    def collect(name, callee):
        ready()
        target = out/name
        subprocess.run([sys.executable, '-B', str(here/'trace.py'), '--tools', str(args.tools.resolve()),
                        '--subject', str(subject), '--out', str(target), '--fixture',
                        '--fixture-callee', callee], check=True)
        oracle = json.loads((target/'oracle.json').read_text())
        summary = json.loads((target/'summary.json').read_text())
        assert summary['fixture_assertions_passed']
        return oracle, summary, target

    baseline, summary, raw = collect('baseline', 'strlen')
    ready()
    extracted = subprocess.check_output([str(args.indexbench.resolve()), 'lang-probe', '-dialect', 'php', str(fixture)])
    artifact = out/'extractor-output.txt'
    artifact.write_bytes(extracted)
    original_extract = hashlib.sha256(extracted).hexdigest()
    artifact.write_text('ALL EXTRACTOR CALLS REMOVED; fake_caller -> fake_target\n')
    changed_extract = hashlib.sha256(artifact.read_bytes()).hexdigest()
    assert original_extract != changed_extract
    # The collector receives the same fixed native events/scope. No extractor
    # artifact is an oracle input, including the real output just perturbed.
    fixed, _ = write_collection(subject, raw, json.loads((raw/'syntax.json').read_text()), summary['php'], 0)
    assert digest(baseline) == digest(fixed)

    assert original.count('return strlen($value);') == 1
    fixture.write_text(original.replace('return strlen($value);', "return strspn($value, 'x');"))
    changed, _, _ = collect('changed-input', 'strspn')
    assert digest(baseline) != digest(changed)
    old_pairs = {(x['from'], x['to']) for x in baseline['calls']}
    new_pairs = {(x['from'], x['to']) for x in changed['calls']}
    assert old_pairs-new_pairs == {('target', 'strlen')}
    assert new_pairs-old_pairs == {('target', 'strspn')}
    receipt = dict(native_fixture='PASS: 2 of 4 literal sites; alias/dynamic gaps retained',
                   php=summary['php'], xdebug=summary['xdebug'],
                   oracle_code_sha256={p: hashlib.sha256((here/p).read_bytes()).hexdigest()
                       for p in ['trace.py', 'literal_sites.php', 'reflect_shutdown.php', 'prove_native.py']},
                   extractor_before_sha256=original_extract, extractor_after_sha256=changed_extract,
                   raw_truth_before_sha256=digest(baseline), raw_truth_after_extractor_perturbation_sha256=digest(fixed),
                   raw_truth_after_native_input_change_sha256=digest(changed),
                   input_change='native function call strlen to strspn, original scope unchanged',
                   changed_native_pairs=1, original_literal_sites=summary['literal_sites'])
    (out/'proof.json').write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')
    print(json.dumps(receipt, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
