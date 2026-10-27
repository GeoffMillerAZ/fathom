#!/usr/bin/env python3
"""Restore PSScriptAnalyzer's own test data outside the graded Tests subtree."""
import argparse
import json
from pathlib import Path
import subprocess
from fetch import ready, SUBJECTS


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    args=ap.parse_args()
    base=args.tools.resolve()
    root=base/'psa'
    assert subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()==SUBJECTS['psa'][1]
    ready()
    subprocess.run(['git','-C',str(root),'sparse-checkout','disable'],check=True)
    assert subprocess.run(['git','-C',str(root),'diff','--quiet']).returncode==0
    (base/'psa-runtime-inputs.json').write_text(json.dumps({'revision':SUBJECTS['psa'][1],
        'graded_scope':'Tests unchanged','runtime_inputs':'complete original pinned repository: rule documentation, compatibility profiles, build metadata and all own test data',
        'source_edits':False},indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
