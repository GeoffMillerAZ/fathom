#!/usr/bin/env python3
"""Remove measured load-only wrapper output before product discovery scoring."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    base=args.tools.resolve()
    root=base/'pester'
    assert (base/'runtime-preparation.json').exists(),'only this recipe-owned build may be removed'
    assert subprocess.run(['git','-C',str(root),'diff','--quiet']).returncode==0
    build=root/'bin'
    paths=[p for p in build.rglob('*') if p.is_file() and not p.is_symlink()]
    eligible=[p for p in paths if p.suffix.lower() in ['.ps1','.psm1','.psd1']]
    receipt={'removed_recipe_owned_build':'pester/bin','removed_files':len(paths),
             'removed_bytes':sum(p.stat().st_size for p in paths),
             'load_only_powershell_files':len(eligible),'load_only_powershell_bytes':sum(p.stat().st_size for p in eligible),
             'original_tracked_source_changed':False,
             'scope':'the three generated wrapper inputs were load state only, outside the frozen original tracked population; removed before product discovery scoring'}
    shutil.rmtree(build)
    assert subprocess.run(['git','-C',str(root),'diff','--quiet']).returncode==0
    args.out.parent.mkdir(parents=True,exist_ok=True)
    with args.out.open('x') as output:output.write(json.dumps(receipt,indent=2,sort_keys=True)+'\n')
    print(json.dumps(receipt,indent=2,sort_keys=True))


if __name__=='__main__':main()
