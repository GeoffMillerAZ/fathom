#!/usr/bin/env python3
"""Measure recall on executed native pairs; never grade unexecuted predictions."""
import argparse
import json
import os
from pathlib import Path
import subprocess

from fetch import SOURCES, ready


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--indexbench', required=True, type=Path)
    ap.add_argument('--traces', required=True, type=Path)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    for name in ['guzzle', 'monolog']:
        ready()
        out = args.traces/name
        reading = out/'diagnostic-reading.json'
        with (out/'score.log').open('w') as log:
            subprocess.run([str(args.indexbench.resolve()), 'lang-oracle', '-dialect', 'php',
                            '-subject', str((args.tools/name).resolve()), '-subject-name', SOURCES[name][0],
                            '-subject-sha', SOURCES[name][1], '-tool-oracle', str((out/'oracle.json').resolve()),
                            '-out', str(reading.resolve())], check=True,
                           stdout=log, stderr=subprocess.STDOUT)
        data = json.loads(reading.read_text())
        summary = json.loads((out/'summary.json').read_text())
        summary['executed_call_truth'] = data['counts']['call_truth']
        summary['executed_call_truth_found'] = data['counts']['call_truth_found']
        summary['executed_call_recall'] = data['metrics']['call_recall']
        summary['precision'] = 'unmeasured: the diagnostic harness also counts unexecuted predictions'
        (out/'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')
        print(name, summary['executed_call_truth_found'], '/', summary['executed_call_truth'], flush=True)


if __name__ == '__main__':
    main()
