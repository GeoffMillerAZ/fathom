#!/usr/bin/env python3
"""Use matching released DLLs with Pester's declared non-inline source build."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess

from fetch import environment, ready


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    args=ap.parse_args()
    base=args.tools.resolve()
    ready()
    source=base/'modules/Pester/6.2.0'
    destination=base/'pester/bin'
    if destination.exists():raise ValueError('runtime build output must be fresh')
    shutil.copytree(source,destination)
    env=environment(base)
    env['PESTER_BUILD_INLINE']='0'
    with (base/'pester-source-build.log').open('w') as log:
        result=subprocess.run([str(base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive',
                               '-File',str(base/'pester/build.ps1'),'-Inline:$false'],
                              cwd=base/'pester',env=env,stdout=log,stderr=log,timeout=120)
    if result.returncode:raise RuntimeError('declared source wrapper build failed; inspect private log')
    assert subprocess.run(['git','diff','--quiet'],cwd=base/'pester').returncode==0,'original source changed'
    receipt={'recipe':'original build.ps1 -Inline:$false, existing bin seeded with matching released Pester 6.2.0 DLLs/format data; no C# build or source edits',
             'released_dll_sha256':hashlib.sha256((source/'bin/net8.0/Pester.dll').read_bytes()).hexdigest(),
             'generated_module_sha256':hashlib.sha256((destination/'Pester.psm1').read_bytes()).hexdigest(),
             'original_tracked_source_changed':False}
    (base/'runtime-preparation.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
