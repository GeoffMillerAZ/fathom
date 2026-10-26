#!/usr/bin/env python3
"""Run focused oracle mutants, always restore source, then prove green."""
import os
from pathlib import Path
import subprocess
import sys


def main():
    here = Path(__file__).resolve().parent
    repo = here.parents[2]
    env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1')
    mutants = [
        ('trace.py', 'caller = next(iter(callers))', "caller = syntax['sites'][eligible[0]]['from']",
         'test_native_trace.NativeAnswerTests.test_native_caller_is_not_the_syntax_walk_pair'),
        ('trace.py', "if target.startswith('__') or (relative, lineno) in blocked:", "if target.startswith('__'):",
         'test_native_trace.NativeAnswerTests.test_dynamic_call_on_same_line_cannot_attest_unexecuted_literal'),
        ('trace.py', 'traces = validate_collection(output, status, limits)', "traces = sorted(output.glob('trace.*.xt'))",
         'test_trace_caps.TraceCapsTests.test_total_trace_byte_cap_plus_one'),
        ('audit_scope.py', "outside = {p for p in pairs if p[0] not in docs}", 'outside = set()',
         'test_scope.CompletePopulationTests.test_omitted_documents_are_unattested_even_when_local_blind_share_is_small'),
    ]
    for filename, old, new, test in mutants:
        path = here/filename
        original = path.read_text()
        assert original.count(old) == 1, filename+' mutant anchor changed'
        try:
            path.write_text(original.replace(old, new))
            result = subprocess.run([sys.executable, '-B', '-m', 'unittest', test], cwd=here,
                                    env=env, capture_output=True, text=True)
            print('Mutation:', filename, old, '=>', new, flush=True)
            print(result.stdout+result.stderr, flush=True)
            assert result.returncode != 0 and 'FAIL:' in result.stderr, 'mutant survived or did not reach assertion'
        finally:
            path.write_text(original)
        subprocess.run([sys.executable, '-B', '-m', 'unittest', test], cwd=here, env=env, check=True)
    subprocess.run([sys.executable, '-B', '-m', 'unittest', 'discover', '-s', str(here), '-p', 'test_*.py'],
                   cwd=repo, env=env, check=True)


if __name__ == '__main__':
    main()
