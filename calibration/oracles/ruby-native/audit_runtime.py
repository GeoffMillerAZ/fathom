#!/usr/bin/env python3
"""Revalidate saved native events and driver completion under current rules."""
import argparse
import hashlib
import json
from pathlib import Path

from run_traces import validate_driver
from trace_decode import decode


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--runtime',type=Path,required=True)
    ap.add_argument('--scored',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    data=json.loads((args.scored/'summary.json').read_text())
    here=Path(__file__).resolve().parent
    for name,row in data['subjects'].items():
        target=args.runtime/name
        for driver in row['drivers']:
            validate_driver(driver['exit'],(target/driver['driver']/'driver.log').read_text(errors='replace'))
        syntax=json.loads((target/'syntax.json').read_text())
        oracle,summary=decode(sorted(target.glob('*/native-*.json')),syntax)
        old=json.loads((target/'oracle.json').read_text())
        assert oracle==old,'native truth changed during revalidation'
        assert all(summary[k]==row[k] for k in summary),'coverage changed during revalidation'
        row['final_driver_and_decoder_revalidation']='PASS: saved raw events re-decoded; all driver logs checked for instrumentation traceback, timeout and signal; identical native truth'
    data['revalidation_code_sha256']={p:hashlib.sha256((here/p).read_bytes()).hexdigest()
                                     for p in ['audit_runtime.py','run_traces.py','trace_decode.py','trace_hook.rb']}
    args.out.parent.mkdir(parents=True,exist_ok=True)
    with args.out.open('x') as output:output.write(json.dumps(data,indent=2,sort_keys=True)+'\n')
    print('PASS: all original native events and driver logs revalidated, truth unchanged')


if __name__=='__main__':main()
