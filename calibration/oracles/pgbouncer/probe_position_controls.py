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


def focused():
    stream = io.StringIO()
    tests = unittest.defaultTestLoader.loadTestsFromName('test_native_positions.PositionReaderTest.test_native_population_cannot_be_selected_by_locator')
    result = unittest.TextTestRunner(stream=stream, verbosity=2).run(tests)
    return result, stream.getvalue()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-pgbouncer', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    binary, out = args.pgbouncer.resolve(), args.out.resolve()
    native_positions.verify(binary, json.loads((native_positions.HERE / 'native-position-build-r1.json').read_text()))
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
    receipt = dict(fathom_nodes_before=count, fathom_nodes_after=0, race_enabled=True,
                   truth_before_sha256=before, truth_after_output_removal_sha256=after,
                   truth_after_input_change_sha256=changed,
                   mutation_file='native_positions.py', mutant='filter native callbacks by independent mutation sites',
                   failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),
                   restored_green=True)
    (out / 'summary.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt, sort_keys=True))
