#!/usr/bin/env python3
"""Measure all pinned Sentinel policy inputs with the bench-only open parser."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

from fetch import SUBJECTS


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--subjects', required=True, type=Path)
    ap.add_argument('--probe', required=True, type=Path)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    args.out.mkdir(parents=True, exist_ok=False)
    for name, (repo, sha) in SUBJECTS.items():
        if os.getloadavg()[0] > 6:
            raise ValueError('wait for load <=6, then use a fresh output directory')
        run = subprocess.run([str(args.probe.resolve()), '-subject', str(args.subjects/name),
                              '-identity', repo+'@'+sha], check=True, capture_output=True, text=True)
        data = json.loads(run.stdout)
        data['probe_sha256'] = hashlib.sha256(args.probe.read_bytes()).hexdigest()
        (args.out/(name+'.json')).write_text(json.dumps(data, indent=2, sort_keys=True)+'\n')
        counts = data['groups']['all']
        print(name, counts['files_in_error'], '/', counts['files'], flush=True)


if __name__ == '__main__':
    main()
