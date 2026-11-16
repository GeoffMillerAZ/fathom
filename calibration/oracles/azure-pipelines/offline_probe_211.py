"""Validate authored offline evaluator controls; no support metric or resolver."""
import argparse
import hashlib
import json
from pathlib import Path


def validate(raw):
    if set(raw) != {'classification','support_claim','native_owner_attested','cases'}:
        raise ValueError('unexpected receipt fields')
    if raw['classification'] != 'feasibility-only' or raw['support_claim'] is not False or raw['native_owner_attested'] is not False:
        raise ValueError('unattested controls cannot claim support')
    expected = {
        'template': ('evaluated',['Build','Publish'],['Compile','Release'],['Publish:Build']),
        'changed': ('evaluated',['Changed','Publish'],['Compile','Release'],['Publish:Changed']),
        'malformed': ('refused',[],[],[]), 'missing': ('refused',[],[],[]),
    }
    rows=raw['cases']
    if not isinstance(rows,list) or len(rows)!=4:
        raise ValueError('whole control population required')
    seen=set()
    for row in rows:
        if set(row)!={'case','status','stages','jobs','dependencies'}:
            raise ValueError('unexpected case fields')
        name=row['case']
        if name in seen or name not in expected:
            raise ValueError('duplicate or unknown case')
        seen.add(name)
        actual=(row['status'],row['stages'],row['jobs'],row['dependencies'])
        if actual!=expected[name]:
            raise ValueError('failed authored control: '+name)
    return dict(inputs=4,evaluated=2,refused=2,unexpected_failures=0,
                classification='feasibility-only',support_claim=False,native_owner_attested=False,
                canonical_files_measured=0,reading_changed=False)


if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--raw',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    if args.raw.stat().st_size>1024**2:
        raise ValueError('receipt parse cap')
    data=args.raw.read_bytes()
    summary=validate(json.loads(data))
    summary['raw_sha256']=hashlib.sha256(data).hexdigest()
    args.out.write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary))
