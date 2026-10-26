#!/usr/bin/env python3
"""Complete original-population clean-session binder measurement under OL-R11."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

from fetch import SUBJECTS, environment, ready


def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS,0)<10:raise SystemExit('run with nice -n 10')
    base,out=args.tools.resolve(),args.out.resolve()
    here=Path(__file__).resolve().parent
    repo=here.parents[2]
    out.mkdir(parents=True,exist_ok=False)
    public={'ruling':'OL-R11','subjects':{},'canonical_reading_changed':False,
            'benchmark_product_revision':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),
            'oracle_code_sha256':{p:digest(here/p) for p in ['ResolveCalls.ps1','DiscoverHarness.ps1','measure_binder.py','tracked_set.py']}}
    reading=json.loads((repo/'docs/bench/readings/lang/powershell.json').read_text())
    for name,(repository,pin,subpath) in SUBJECTS.items():
        ready()
        root=base/name
        subject=root/subpath
        target=out/name
        target.mkdir()
        env=environment(base)
        def manifest(suffix):
            prefix=target/('tracked-'+suffix)
            subprocess.run([sys.executable,'-B',str(here/'tracked_set.py')],
                env=dict(env,SUBJECT=str(root),SUBPATH=subpath,RECEIPT=str(prefix)),check=True)
            return json.loads(prefix.with_suffix('.json').read_text()),prefix.with_suffix('.untracked').read_bytes()
        before,initial_untracked=manifest('before')
        tracked=subprocess.check_output(['git','-C',str(root),'ls-files','-z','--',subpath]).split(b'\0')
        paths=[]
        binaries=0
        for raw in tracked:
            if not raw:continue
            p=root/os.fsdecode(raw)
            if p.suffix.lower() not in ['.ps1','.psm1','.psd1']:continue
            if not p.is_file() or p.is_symlink():raise ValueError('whole-unit refusal: missing original source or symlink')
            relative=p.relative_to(subject).as_posix()
            paths.append(relative)
            if b'\0' in p.read_bytes()[:8000]:binaries+=1
        files=target/'files.json'
        files.write_text(json.dumps(sorted(paths))+'\n')
        repository_files=target/'repository-files.json'
        repository_files.write_text(json.dumps([os.fsdecode(p) for p in subprocess.check_output(['git','-C',str(root),'ls-files','-z']).split(b'\0') if p and Path(os.fsdecode(p)).suffix.lower() in ['.ps1','.psm1','.psd1']])+'\n')
        module='Pester' if name=='pester' else 'PSScriptAnalyzer'
        version='6.2.0' if name=='pester' else '1.25.0'
        manifest_path=base/'pester/bin/Pester.psd1' if name=='pester' else base/'modules'/module/version/(module+'.psd1')
        raw=target/'native-answers.json'
        ready()
        with (target/'binder.log').open('w') as log:
            result=subprocess.run([str(base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive',
                '-File',str(here/'ResolveCalls.ps1'),'-Subject',str(subject),'-FilesJson',str(files),
                '-Manifest',str(manifest_path),'-Harness',name,'-Repository',str(root),'-TrackedJson',str(repository_files),
                '-TestDependencyManifest',str(base/'modules/Pester/5.7.1/Pester.psd1'),'-Out',str(raw)],env=env,cwd=target,stdout=log,stderr=log,timeout=600)
        if result.returncode!=0:raise RuntimeError('native binder refused; inspect private log for '+name)
        answers=json.loads(raw.read_text())
        oracle={key:answers[key] for key in ['tool','version','tier','calls']}
        oracle['disclose']=['OL-R11: clean pinned-runtime session plus subject own released manifest. Unresolvable original members and undeclared state remain misses in the full-population audit.']
        oracle_path=target/'oracle.json'
        oracle_path.write_text(json.dumps(oracle,indent=2,sort_keys=True)+'\n')
        written={(s['path'],s['from'],s['written']) for s in answers['sites']}
        resolved={(s['path'],s['from'],s['to']) for s in answers['calls']}
        # Actual aliases/case canonicalization can differ from the written
        # name. Keep those disagreements distinct from native resolution.
        site_population_sha256=hashlib.sha256(json.dumps([{k:s[k] for k in ['path','line','column','from','written','kind']} for s in answers['sites']],sort_keys=True).encode()).hexdigest()
        matched_sites=sum(s['resolved'] is not None for s in answers['sites'])
        after,untracked=manifest('after')
        assert before==after and untracked==initial_untracked, 'original subject changed or new generated input requires review'
        canonical=next(s for s in reading['subjects'] if s['sha']==pin)
        public['subjects'][name]=dict(revision=pin,subpath=subpath,native=answers['receipt'],tracked_before=before,tracked_after=after,
            files=answers['receipt']['files'],original_binary_classified_files=binaries,
            preexisting_untracked_or_ignored_paths=len(initial_untracked.split(b'\0'))-1 if initial_untracked else 0,
            new_generated_paths=0,original_scope='frozen original Git-tracked eligible source; generated non-inline wrapper is load-state only, outside measurement membership',
            historical_reading_files=canonical['files'],historical_reading_bytes=canonical['bytes'],
            literal_sites=len(answers['sites']),resolved_sites=matched_sites,
            unresolved_sites=len(answers['sites'])-matched_sites,resolved_site_fraction=matched_sites/len(answers['sites']),
            unique_written_pairs=len(written),native_pairs=len(resolved),
            written_name_pairs_without_equal_native_target=len(written-resolved),
            native_oracle_sha256=digest(oracle_path),population_sha256=digest(raw),literal_population_sha256=site_population_sha256,
            population='all original tracked PowerShell source including UTF-16; no original file discarded')
        (out/'summary.json').write_text(json.dumps(public,indent=2,sort_keys=True)+'\n')
        print(name,json.dumps(public['subjects'][name]),flush=True)


if __name__=='__main__':main()
