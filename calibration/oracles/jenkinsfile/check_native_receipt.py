"""Check committed source pins; optionally replay locally retained raw owner data."""
import argparse
import hashlib
import json
from pathlib import Path
from native_model_probe import summarize
from validate_native_controls import validate

HERE=Path(__file__).resolve().parent

def check(canonical=None,authored=None):
    receipt=json.loads((HERE/'native-measurement-r1.json').read_text())
    for row in receipt['sources']:
        path=HERE/row['path']
        if not path.resolve().is_relative_to(HERE) or hashlib.sha256(path.read_bytes()).hexdigest()!=row['sha256']:
            raise ValueError('committed source pin drift')
    for key,path in [('canonical',canonical),('authored',authored)]:
        if path is None:continue
        if path.stat().st_size>32*1024**2:raise ValueError('raw replay cap')
        data=path.read_bytes()
        if hashlib.sha256(data).hexdigest()!=receipt[key+'_raw_sha256']:raise ValueError('raw replay identity')
        raw=json.loads(data)
        for field,value in summarize(raw).items():
            if receipt[key][field]!=value:raise ValueError('native summary replay drift')
        if key=='authored' and validate(raw)!=receipt['authored_controls']:
            raise ValueError('authored native control replay drift')
    return dict(source_pins=len(receipt['sources']),canonical_replayed=canonical is not None,
                authored_replayed=authored is not None,calibration_measured=False,tier_granted=False)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--canonical-raw',type=Path);parser.add_argument('--authored-raw',type=Path)
    args=parser.parse_args();print(json.dumps(check(args.canonical_raw,args.authored_raw),sort_keys=True))
