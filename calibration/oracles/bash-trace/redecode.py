#!/usr/bin/env python3
"""Verify historical capture bytes, then decode native pairs without syntax selection."""
import argparse,copy,hashlib,importlib.util,json,subprocess
from pathlib import Path
from call_sites import collect
from collect import merge_function_phase
from function_decode import decode

HERE=Path(__file__).resolve().parent


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def historical_hash(revision,name):
    assert len(revision)==40 and all(c in '0123456789abcdef' for c in revision)
    assert Path(name).name==name
    return hashlib.sha256(subprocess.check_output(['git','show',revision+':calibration/oracles/bash-trace/'+name])).hexdigest()


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['native','subject','shfmt','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--capture-revision',required=True)
    a=p.parse_args();native=a.native.resolve();root=a.subject.resolve();out=a.out.resolve()
    original=json.loads((native/'summary.json').read_text())
    for name,digest in original['instrument_sha256'].items():assert historical_hash(a.capture_revision,name)==digest,'capture instrument drift: '+name
    for phase in original['runs']:
        for name,digest in phase['raw_sha256'].items():assert sha(native/phase['phase']/name)==digest,'whole-unit refusal: captured raw bytes changed'
    for name,digest in original['source_sha256'].items():assert sha(root/name)==digest,'whole-unit refusal: original source drift'
    spec=importlib.util.spec_from_file_location('bash_syntax',HERE.parent/'bash-source/bash_source_dump.py')
    syntax=importlib.util.module_from_spec(spec);spec.loader.exec_module(syntax)
    population=collect(root,sorted(original['source_sha256']),str(a.shfmt.resolve()),syntax)
    population_hash=hashlib.sha256(json.dumps(population,sort_keys=True).encode()).hexdigest()
    assert population_hash==original['function_population_sha256'],'whole-unit refusal: coverage population changed'
    pairs,hits,without_syntax=set(),set(),set();accepted_unknown=discarded_unknown=0
    result=copy.deepcopy(original)
    for phase in result['runs']:
        trace=native/phase['phase']/'functions.trace'
        native_pairs,native_hits,unknown=decode(trace,native/'child',population)
        independent,_,_=decode(trace,native/'child',{'sites':[]})
        assert native_pairs==independent,'whole-unit refusal: syntax selected native truth'
        accepted_unknown+=merge_function_phase(pairs,hits,native_pairs,native_hits,unknown,
            changed=phase['changed_source'],timed=phase['timeout'],exit_code=phase['exit'])
        rejected=bool(phase['changed_source'] or phase['timeout'] or phase['exit']<0)
        if rejected:discarded_unknown+=unknown
        else:without_syntax.update(independent)
        phase.update(native_function_pairs=len(native_pairs),native_function_sites=len(native_hits),function_site_unpaired_rows=unknown,events_accepted=not rejected)
    assert hits and len(hits)==original['function_attested_sites']
    assert pairs==without_syntax
    result.update(function_pairs=[dict(path=f,**{'from':c,'to':t}) for f,c,t in sorted(pairs)],
        function_unknown_rows=accepted_unknown,discarded_function_site_unpaired_rows=discarded_unknown,
        legacy_C_selected_pair_count=len(original['function_pairs']),capture_code_revision=a.capture_revision,
        capture_summary_sha256=sha(native/'summary.json'),raw_evidence_root=str(native),
        decoder_code_sha256={name:sha(HERE/name) for name in ['redecode.py','function_decode.py','collect.py']},
        native_pair_policy='All absolute in-root native frame identities enter truth before syntax coverage; original eligible caller-file scope is disclosed separately by replay.',
        syntax_selection_independence=dict(native_pairs=len(pairs),sha256=hashlib.sha256(json.dumps(sorted(pairs)).encode()).hexdigest(),empty_syntax_same_pairs=True))
    out.mkdir(parents=True,exist_ok=False)
    (out/'summary.json').write_text(json.dumps(result,indent=2,sort_keys=True)+'\n')
    print(json.dumps(dict(native_pairs=len(pairs),coverage_hits=len(hits),certified_site_unpaired_rows=accepted_unknown,discarded_site_unpaired_rows=discarded_unknown)))


if __name__=='__main__':main()
