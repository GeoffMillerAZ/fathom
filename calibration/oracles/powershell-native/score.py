#!/usr/bin/env python3
"""Resolved-binder diagnostics and executed-only native pair recall."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

from fetch import SUBJECTS, ready
from trace_decode import decode


def scope_digest(population):
    return hashlib.sha256(json.dumps([{k:s[k] for k in ['path','line','column','from','written','kind']}
                                    for s in population['sites']],sort_keys=True).encode()).hexdigest()


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--binder',type=Path,required=True)
    ap.add_argument('--runtime-pester',type=Path,required=True)
    ap.add_argument('--runtime-psa',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    args=ap.parse_args()
    out=args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    binder=json.loads((args.binder/'summary.json').read_text())
    data={'binder':binder,'runtime':{'ruling':'OL-R11','subjects':{}},
          'benchmark_product_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()}
    for name,(repository,pin,subpath) in SUBJECTS.items():
        population=json.loads((args.binder/name/'native-answers.json').read_text())
        runtime=args.runtime_pester if name=='pester' else args.runtime_psa/'psa'
        oracle,summary=decode(runtime/'native.json',population)
        saved=json.loads((runtime/'oracle.json').read_text())
        assert oracle==saved,'final native truth changed under re-decode'
        summary['literal_population_sha256']=scope_digest(population)
        assert summary['literal_sites']==binder['subjects'][name]['literal_sites']
        for mode,path in [('binder',args.binder/name/'oracle.json'),('runtime',runtime/'oracle.json')]:
            ready()
            reading=out/(name+'-'+mode+'-diagnostic.json')
            with (out/(name+'-'+mode+'.log')).open('w') as log:
                subprocess.run([str(args.indexbench.resolve()),'lang-oracle','-dialect','powershell',
                    '-subject',str((args.tools/name/subpath).resolve()),'-subject-name',repository,'-subject-sha',pin,
                    '-tool-oracle',str(path.resolve()),'-out',str(reading)],stdout=log,stderr=log,check=True)
            measured=json.loads(reading.read_text())
            if mode=='binder':
                row=binder['subjects'][name]
                row['resolved_pair_diagnostic_counts']={k:measured['counts'][k] for k in ['call_truth','call_truth_found','call_fathom','call_fathom_in_truth']}
                row['resolved_pair_diagnostic_precision']=measured['metrics']['call_precision']
                row['resolved_pair_diagnostic_recall']=measured['metrics']['call_recall']
                row['product_discovered_files']=measured['subjects'][0]['files']
                row['product_discovered_bytes']=measured['subjects'][0]['bytes']
                assert row['product_discovered_files']==row['historical_reading_files']
                assert row['product_discovered_bytes']==row['historical_reading_bytes']
                row['full_population_policy']='all unresolved original sites retained as misses; resolved-only pair diagnostics do not establish whole-population recall'
                row['literal_population_sha256']=scope_digest(population)
            else:
                summary['harness_scoped_pair_truth']=measured['counts']['call_truth']
                summary['executed_pair_truth']=summary['native_pairs']
                summary['executed_pair_truth_found']=measured['counts']['call_truth_found']
                summary['executed_pair_recall']=measured['counts']['call_truth_found']/summary['native_pairs']
                summary['runtime_precision']='unmeasured: predictions include unexecuted calls'
                summary['native_pairs_outside_product_scope']=summary['native_pairs']-measured['counts']['call_truth']
        data['runtime']['subjects'][name]=summary
    (out/'summary.json').write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
    print(json.dumps(data,indent=2,sort_keys=True))


if __name__=='__main__':main()
