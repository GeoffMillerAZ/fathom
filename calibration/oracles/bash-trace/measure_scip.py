#!/usr/bin/env python3
"""Measure the unmodified released shell producer on both complete Git scopes."""
import argparse,hashlib,importlib.util,json,os
from pathlib import Path
import resource,subprocess,sys
from build_bash import ready
from fetch import SUBJECTS
from call_sites import collect
from scope_limits import validate
from scip_audit import audit,diagnostic_tier


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['tools','indexbench','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();base=a.tools.resolve();out=a.out.resolve();out.mkdir(parents=True,exist_ok=False)
    here=Path(__file__).resolve().parent;repo=here.parents[2]
    spec=importlib.util.spec_from_file_location('bash_syntax',here.parent/'bash-source/bash_source_dump.py')
    syntax=importlib.util.module_from_spec(spec);spec.loader.exec_module(syntax)
    summary={'candidate':'scip-shell','version':'0.1.0','released_revision':'bd95dfa882c2529fa83a29aba403a3caa5b4b267',
        'subjects':{},'canonical_reading_changed':False,'scoring_tier':'C product-range-assisted pair diagnostic; independent native occurrence audit does not use product output',
        'code_sha256':{n:hashlib.sha256((here/n).read_bytes()).hexdigest() for n in ['measure_scip.py','scip_audit.py','call_sites.py','scope_limits.py','syntax_scope.py','tracked_set.py']},
        'benchmark_product_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()}
    for name,(repository,pin) in SUBJECTS.items():
        ready();root=base/name;target=out/name;target.mkdir()
        assert subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()==pin
        def manifest(suffix):
            prefix=target/('tracked-'+suffix)
            subprocess.run([sys.executable,'-B',str(here/'tracked_set.py')],env=dict(os.environ,SUBJECT=str(root),SUBPATH='.',RECEIPT=str(prefix)),check=True)
            return json.loads(prefix.with_suffix('.json').read_text())
        before=manifest('before')
        files=sorted(os.fsdecode(p) for p in subprocess.check_output(['git','-C',str(root),'ls-files','-z']).split(b'\0') if p and (root/os.fsdecode(p)).is_file() and not(root/os.fsdecode(p)).is_symlink() and syntax.wanted(str(root),os.fsdecode(p),{'.sh','.bash'},['bash','sh']))
        validate(root,files);population=collect(root,files,str(base/'shfmt'),syntax);validate(root,files,call_sites=population['sites'])
        native=target/'index.scip'
        def caps():resource.setrlimit(resource.RLIMIT_FSIZE,(32<<20,32<<20))
        with (target/'producer.log').open('w') as log:
            subprocess.run([str(base/'scip-target/release/scip-shell'),'--project-root',str(root),'-o',str(native),*[str(root/f) for f in files]],
                cwd=root,env=dict(os.environ,PATH='/usr/bin:/bin:/usr/sbin:/sbin'),stdout=log,stderr=log,preexec_fn=caps,timeout=180,check=True)
        audited=audit(native,files,population)
        ready()
        with (target/'score.log').open('w') as log:
            subprocess.run([str(a.indexbench.resolve()),'lang-oracle','-dialect','bash','-subject',str(root),'-subject-name',repository,
                '-subject-sha',pin,'-scip',str(native),'-scip-tier',diagnostic_tier(audited),'-scip-defs','-out',str(target/'diagnostic.json'),
                '-unmatched',str(target/'unmatched.json')],stdout=log,stderr=log,check=True)
        measured=json.loads((target/'diagnostic.json').read_text());after=manifest('after');assert before==after
        original_bytes=sum((root/f).stat().st_size for f in files)
        assert measured['subjects'][0]['files']==len(files),'whole-unit refusal: product file scope differs'
        assert measured['subjects'][0]['bytes']==original_bytes,'whole-unit refusal: product byte scope differs'
        summary['subjects'][name]=dict(revision=pin,original_files=len(files),original_bytes=original_bytes,
            product_discovered_files=measured['subjects'][0]['files'],product_discovered_bytes=measured['subjects'][0]['bytes'],
            tracked_before=before,tracked_after=after,audit=audited,counts=measured['counts'],metrics=measured['metrics'],
            native_scip_sha256=hashlib.sha256(native.read_bytes()).hexdigest(),population_sha256=hashlib.sha256(json.dumps(population,sort_keys=True).encode()).hexdigest())
    (out/'summary.json').write_text(json.dumps(summary,indent=2,sort_keys=True)+'\n')
    print(json.dumps(summary,indent=2,sort_keys=True))


if __name__=='__main__':main()
