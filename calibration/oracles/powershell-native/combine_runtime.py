#!/usr/bin/env python3
"""Union two original declared test phases, preserving full site coverage."""
import argparse
import hashlib
import json
from pathlib import Path

from trace_decode import Limits,decode


def combine(paths,population,limits=Limits()):
    if len(paths)!=2:raise ValueError('whole-unit refusal: exactly two declared phases required')
    if sum(Path(path).stat().st_size for path in paths)>limits.raw_bytes:
        raise ValueError('whole-unit refusal: combined native byte cap')
    for path in paths:decode(path,population,limits)
    original=[json.loads(Path(path).read_text()) for path in paths]
    calls=sorted({tuple(row) for item in original for row in item['calls']})
    if len(calls)>limits.rows:raise ValueError('whole-unit refusal: combined native row cap')
    return {**original[0], 'calls':calls,
            'native_events':sum(item['native_events'] for item in original),
            'original_breakpoints':sum(item['original_breakpoints'] for item in original),
            'retained_breakpoints':sum(item['retained_breakpoints'] for item in original),
            'driver_status':'completed' if all(item['driver_status']=='completed' for item in original) else 'one or more driver exceptions',
            'phase_statuses':[item['driver_status'] for item in original],
            'phase_exception_hashes':[item['exception_sha256'] for item in original]}


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--default',type=Path,required=True)
    ap.add_argument('--remaining',type=Path,required=True)
    ap.add_argument('--population',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    args.out.mkdir(parents=True,exist_ok=False)
    population=json.loads(args.population.read_text())
    combined=combine([source/'native.json' for source in [args.default,args.remaining]],population)
    path=args.out/'native.json'
    path.write_text(json.dumps(combined,sort_keys=True)+'\n')
    oracle,summary=decode(path,population)
    target=args.out/'oracle.json'
    target.write_text(json.dumps(oracle,indent=2,sort_keys=True)+'\n')
    summary.update(native_oracle_sha256=hashlib.sha256(target.read_bytes()).hexdigest(),
                   driver_phases=['original test.ps1 -NoBuild','original test.ps1 -NoBuild -SkipPTests'],
                   all_original_sites_retained=True)
    (args.out/'summary.json').write_text(json.dumps(summary,indent=2,sort_keys=True)+'\n')
    print(json.dumps(summary,indent=2,sort_keys=True))


if __name__=='__main__':main()
