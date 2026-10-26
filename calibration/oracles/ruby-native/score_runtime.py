#!/usr/bin/env python3
"""Score only native executed Ruby pairs; disclose full original-site coverage."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

from bootstrap import SUBJECTS, ready


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--runtime',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    out=args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    data=json.loads((args.runtime/'summary.json').read_text())
    repo=Path(__file__).resolve().parents[3]
    data['benchmark_product_revision']=subprocess.check_output(['git','-C',str(repo),'rev-parse','HEAD'],text=True).strip()
    for name,(repository,pin) in SUBJECTS.items():
        ready()
        reading=out/(name+'-diagnostic.json')
        oracle=args.runtime/name/'oracle.json'
        with (out/(name+'-score.log')).open('w') as log:
            subprocess.run([str(args.indexbench.resolve()),'lang-oracle','-dialect','ruby',
                            '-subject',str((args.tools/name).resolve()),'-subject-name',repository,'-subject-sha',pin,
                            '-tool-oracle',str(oracle.resolve()),'-out',str(reading)],stdout=log,stderr=log,check=True)
        measured=json.loads(reading.read_text())
        counts=measured['counts']
        row=data['subjects'][name]
        assert counts['call_truth']==row['native_pairs']
        row.update(executed_call_truth=counts['call_truth'],executed_call_truth_found=counts['call_truth_found'],
                   executed_call_recall=measured['metrics']['call_recall'],
                   precision='unmeasured: diagnostic predictions include unexecuted calls; no whole-population runtime precision grade',
                   scoring_oracle_sha256=hashlib.sha256(oracle.read_bytes()).hexdigest())
    (out/'summary.json').write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
    print(json.dumps(data,indent=2,sort_keys=True))


if __name__=='__main__':main()
