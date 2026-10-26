"""Reproduce actual-output independence and native population mutation controls."""
import argparse
import hashlib
import io
import json
import os
import subprocess
import unittest
from pathlib import Path
from unittest.mock import patch
import native_positions


def truth(source, binary, directory):
    definitions, _ = native_positions.measure(source, 'authored.ini', binary, directory)
    return hashlib.sha256(json.dumps(definitions, sort_keys=True).encode()).hexdigest()


def focused(name='test_native_positions.PositionReaderTest.test_native_population_cannot_be_selected_by_locator'):
    stream = io.StringIO()
    tests = unittest.defaultTestLoader.loadTestsFromName(name)
    result = unittest.TextTestRunner(stream=stream, verbosity=2).run(tests)
    return result, stream.getvalue()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-pgbouncer', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('-build-receipt', type=Path, default=native_positions.HERE / 'native-position-build-r1.json')
    args = parser.parse_args()
    binary, out = args.pgbouncer.resolve(), args.out.resolve()
    native_positions.verify(binary, json.loads(args.build_receipt.read_text()))
    os.environ['PGB_POSITION_BIN'] = str(binary)
    out.mkdir(parents=True, exist_ok=False)
    source = '[pgbouncer]\nmax_client_conn=101\n'
    file = out / 'authored.ini'
    file.write_text(source)
    nodes = out / 'fathom-nodes.json'
    subprocess.run(['go', 'run', '-race', './calibration/oracles/pgbouncer/extractor-output',
                    '-source', str(file), '-out', str(nodes)], check=True)
    count = len(json.loads(nodes.read_text()))
    if not count:
        raise ValueError('actual fathom output is empty')
    before = truth(source, binary, out / 'before')
    nodes.write_text('[]\n')
    after = truth(source, binary, out / 'after')
    changed = truth(source.replace('max_client_conn', 'default_pool_size'), binary, out / 'changed')
    if before != after or before == changed:
        raise ValueError('oracle independence/sensitivity control failed')
    original = native_positions.reconcile
    def selected(events, sites):
        kept = [event for event in events if any(all(site.get(k) == v for k, v in event.items()) for site in sites)]
        return original(kept, sites)
    with patch.object(native_positions, 'reconcile', selected):
        red, red_log = focused()
    green, green_log = focused()
    (out / 'mutation.log').write_text(red_log + '\nRESTORED\n' + green_log)
    if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful():
        raise ValueError('mutation did not fail and restore green')
    os.environ['PGB_POSITION_BIN'] = str(binary)
    line_test = 'test_native_positions.NativePositionTest.test_shifted_or_reversed_locator_lines_are_refused'
    with patch.object(native_positions, 'require_prefix', lambda *args: None):
        line_red, line_red_log = focused(line_test)
    line_green, line_green_log = focused(line_test)
    (out / 'line-mutation.log').write_text('MUTANT: disable native callback prefix requirements\n' + line_red_log + '\nRESTORED\n' + line_green_log)
    if line_red.wasSuccessful() or not line_red.failures or line_red.errors or not line_green.wasSuccessful() or line_green.skipped:
        raise ValueError('line-prefix mutation did not FAIL and restore GREEN')
    original_candidates = native_positions.candidates
    def shifted(value):
        return [dict(row, line=row['line'] + 1) for row in original_candidates(value)]
    authored = '# prefix\n[pgbouncer]\n\n# pad\nmax_client_conn=101\n\n# tail\n\n'
    refused = None
    with patch.object(native_positions, 'candidates', shifted):
        try:
            native_positions.measure(authored, 'authored.ini', binary, out / 'line-shift')
        except ValueError as error:
            refused = str(error)
    if not refused or 'prefix' not in refused:
        raise ValueError('locator line +1 mutant was not refused by native prefixes')
    receipt = dict(fathom_nodes_before=count, fathom_nodes_after=0, race_enabled=True,
                   truth_before_sha256=before, truth_after_output_removal_sha256=after,
                   truth_after_input_change_sha256=changed,
                   mutation_file='native_positions.py', mutant='filter native callbacks by independent mutation sites',
                   failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),
                   restored_green=True,
                   locator_line_shift=dict(offset=1, native_refused=True, reason=refused),
                   line_bracket_mutation=dict(mutant='disable native callback prefix requirements',
                                             failure_line=next(line for line in line_red_log.splitlines() if line.startswith('FAIL:')),
                                             restored_green=True))
    (out / 'summary.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt, sort_keys=True))
