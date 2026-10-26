#!/usr/bin/env python3
"""Run original declared PowerShell test tasks under native function tracing."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import signal
import subprocess
import sys

from fetch import SUBJECTS, environment, ready
from trace_decode import decode


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--binder',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    ap.add_argument('--only',choices=list(SUBJECTS),action='append')
    ap.add_argument('--pester-main',action='store_true',help='run the original declared -SkipPTests phase as a supplement, without changing the coverage denominator')
    args=ap.parse_args()
    base,out=args.tools.resolve(),args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    here=Path(__file__).resolve().parent
    public={'ruling':'OL-R11','scope':'executed native function population only; all unobserved original sites retained in coverage','subjects':{}}
    for name,(_,pin,subpath) in SUBJECTS.items():
        if args.only and name not in args.only:continue
        ready()
        target=out/name
        target.mkdir()
        subject=base/name/subpath
        binder=args.binder.resolve()/name
        population=json.loads((binder/'native-answers.json').read_text())
        raw=target/'native.json'
        env=environment(base)
        env['PATH']=str(base/'runtime')+os.pathsep+env['PATH']
        # Native runtime files and all temporary test output stay in roots.
        def manifest(suffix):
            prefix=target/('tracked-'+suffix)
            subprocess.run([sys.executable,'-B',str(here/'tracked_set.py')],env=dict(env,SUBJECT=str(base/name),SUBPATH=subpath,RECEIPT=str(prefix)),check=True)
            return json.loads(prefix.with_suffix('.json').read_text())
        before=manifest('before')
        command=[str(base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive','-File',str(here/'TraceFunctions.ps1'),
                 '-Subject',str(subject),'-FilesJson',str(binder/'files.json'),'-Out',str(raw),'-Driver',name,
                 '-PesterManifest',str(base/('modules/Pester/5.7.1/Pester.psd1' if name=='psa' else 'modules/Pester/6.2.0/Pester.psd1')),
                 '-AnalyzerManifest',str(base/'modules/PSScriptAnalyzer/1.25.0/PSScriptAnalyzer.psd1')]
        if args.pester_main and name=='pester':command+=['-SkipPTests']
        def cap():resource.setrlimit(resource.RLIMIT_FSIZE,(128<<20,128<<20))
        with (target/'driver.log').open('w') as log:
            process=subprocess.Popen(command,cwd=base/name,env=env,stdout=log,stderr=log,start_new_session=True,preexec_fn=cap)
            try:status=process.wait(timeout=1200)
            except subprocess.TimeoutExpired:status='timeout'
            finally:
                try:os.killpg(process.pid,signal.SIGKILL)
                except ProcessLookupError:pass
                process.wait()
        if status=='timeout' or status<0 or not raw.exists():
            (target/'refusal.json').write_text(json.dumps({'whole_unit_refused':True,'status':status,'native_receipt_present':raw.exists()})+'\n')
            raise RuntimeError('whole-unit refusal: timeout, signal or missing native receipt')
        after=manifest('after')
        if before!=after:raise RuntimeError('whole-unit refusal: tracked source changed during runtime')
        oracle,summary=decode(raw,population)
        (target/'oracle.json').write_text(json.dumps(oracle,indent=2,sort_keys=True)+'\n')
        public['subjects'][name]=dict(summary,declared_skip_ptests_phase=args.pester_main and name=='pester',tracked_before=before,tracked_after=after,driver_exit=status,revision=pin,subpath=subpath,
            oracle_sha256=hashlib.sha256((target/'oracle.json').read_bytes()).hexdigest(),
            trace_code_sha256=hashlib.sha256((here/'TraceFunctions.ps1').read_bytes()).hexdigest(),
            population_sha256=hashlib.sha256((binder/'native-answers.json').read_bytes()).hexdigest())
        (out/'summary.json').write_text(json.dumps(public,indent=2,sort_keys=True)+'\n')
        print(name,public['subjects'][name],flush=True)


if __name__=='__main__':main()
