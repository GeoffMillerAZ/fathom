"""Check actual owner-model facts from the committed authored controls."""
import argparse
import hashlib
import json
from pathlib import Path
from native_model_probe import summarize

def validate(raw):
    summary=summarize(raw)
    if [summary[k] for k in ('inputs','parsed','refused','no_pipeline')]!=[4,2,1,1]:raise ValueError('authored control states')
    parsed=[row for row in raw['files'] if row['status']=='parsed']
    names=[];normalized=[];hashes=[]
    for row in parsed:
        stages=[node for node in row['nodes'] if node['type']=='ModelASTStage']
        if len(stages)!=1:raise ValueError('authored stage population')
        names.append(stages[0]['name'])
        hashes.append(hashlib.sha256(json.dumps(row['nodes'],sort_keys=True).encode()).hexdigest())
        nodes=[dict(node,name='authored-stage') if node['type']=='ModelASTStage' else node for node in row['nodes']]
        normalized.append(nodes)
    if set(names)!={'Compile','Verify'} or hashes[0]==hashes[1] or normalized[0]!=normalized[1]:
        raise ValueError('known input fact did not change native truth alone')
    return dict(inputs=4,parsed=2,refused=1,no_pipeline=1,stage_truth_changed=True,unrelated_truth_equal=True,
                extractor_input_used=False,calibration_measured=False,tier_granted=False)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('raw',type=Path);args=parser.parse_args()
    print(json.dumps(validate(json.loads(args.raw.read_text())),sort_keys=True))
