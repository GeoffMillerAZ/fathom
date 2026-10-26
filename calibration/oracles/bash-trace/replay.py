#!/usr/bin/env python3
"""Score observed native Bash pairs only; retain full coverage and no grade claim."""
import argparse,collections,hashlib,json
from pathlib import Path
import subprocess
from build_bash import ready
from fetch import SUBJECTS
from redecode import historical_hash


def oracle_payload(mode, native, imports):
    if mode not in ['calls','source']:raise ValueError('unknown diagnostic mode')
    payload={'tool':'GNU Bash native function frames' if mode=='calls' else 'GNU Bash opened filenames with independent original syntax scope',
        'version':native['bash_version'],'tier':'A' if mode=='calls' else 'C',
        'disclose':['Native frame identities alone enumerate truth; scoring is restricted only to the unchanged eligible caller files, never parser sites. Executed function targets are native answers; source comparison retains C scope selection and cannot establish a whole-source tier-A grade. Unknown/unexecuted and ambiguous sites stay in coverage; no runtime precision.']}
    payload['calls' if mode=='calls' else 'imports']=native['function_pairs'] if mode=='calls' else imports
    return payload


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['tools','indexbench','nvm-native','bats-native','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=False)
    here=Path(__file__).resolve().parent
    summary={'canonical_reading_changed':False,'subjects':{},'scope':'A executed native pair recall on unchanged eligible caller files; all in-root native identities and outside-caller-scope counts disclosed; syntax selects coverage only; separate C source-pair comparison with independently selected syntax scope; no whole-written L2/L3 grade',
        'code_sha256':{'replay.py':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()},
        'benchmark_product_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()}
    for name,(repository,pin) in SUBJECTS.items():
        native=getattr(a,name+'_native');r=json.loads((native/'summary.json').read_text());root=(a.tools/name).resolve()
        assert subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()==pin
        assert r['complete'] and not r['unparsed'],'whole-unit refusal: incomplete independent scope'
        for path,h in r['source_sha256'].items():assert hashlib.sha256((root/path).read_bytes()).hexdigest()==h
        for path,h in r['instrument_sha256'].items():
            actual=historical_hash(r['capture_code_revision'],path) if 'capture_code_revision' in r else hashlib.sha256((here/path).read_bytes()).hexdigest()
            assert actual==h
        for path,h in r.get('decoder_code_sha256',{}).items():assert hashlib.sha256((here/path).read_bytes()).hexdigest()==h
        raw_root=Path(r.get('raw_evidence_root',native))
        for phase in r['runs']:
            for filename,digest in phase['raw_sha256'].items():
                assert hashlib.sha256((raw_root/phase['phase']/filename).read_bytes()).hexdigest()==digest,'whole-unit refusal: native phase raw evidence changed'
        literals={(s['path'],s['line']):s['value'] for s in r['literal_sites']}
        imports=[dict(row,module=literals[(row['path'],row['line'])]) for row in r['imports']]
        original_bytes=sum((root/f).stat().st_size for f in r['source_sha256'])
        measured={}
        for mode in ['calls','source']:
            if mode=='source' and not imports:continue
            oracle=out/(name+'-'+mode+'-oracle.json')
            payload=oracle_payload(mode,r,imports)
            oracle.write_text(json.dumps(payload,sort_keys=True)+'\n')
            ready()
            reading=out/(name+'-'+mode+'-diagnostic.json')
            with (out/(name+'-'+mode+'-score.log')).open('w') as log:
                subprocess.run([str(a.indexbench.resolve()),'lang-oracle','-dialect','bash','-subject',str(root),'-subject-name',repository,'-subject-sha',pin,
                    '-tool-oracle',str(oracle),'-out',str(reading),'-unmatched',str(out/(name+'-'+mode+'-unmatched.json'))],stdout=log,stderr=log,check=True)
            measured[mode]=json.loads(reading.read_text())
            assert measured[mode]['subjects'][0]['files']==r['subject_files'],'whole-unit refusal: product file scope differs'
            assert measured[mode]['subjects'][0]['bytes']==original_bytes,'whole-unit refusal: product byte scope differs'
        truth=measured['calls']['counts']['call_truth'];found=measured['calls']['counts']['call_truth_found']
        eligible=sum(x['path'] in r['source_sha256'] for x in r['function_pairs'])
        assert found<=truth==eligible<=len(r['function_pairs']),'whole-unit refusal: scorer dropped an eligible native pair'
        summary['subjects'][name]=dict(revision=pin,original_files=r['subject_files'],original_bytes=original_bytes,
            product_discovered_files=measured['calls']['subjects'][0]['files'],product_discovered_bytes=measured['calls']['subjects'][0]['bytes'],
            tracked_before=r['tracked_before'],tracked_after=r['tracked_after'],instrument_sha256=r['instrument_sha256'],
            system_bash_binary_sha256=r['system_bash_binary_sha256'],system_bash_version=r['system_bash_version'],
            native_versions=r['native_versions'],bash_version=r['bash_version'],bash_binary_sha256=r['bash_binary_sha256'],shfmt_version=r['shfmt_version'],
            function_population_sha256=r['function_population_sha256'],source_population_sha256=r['source_population_sha256'],
            command_sites=r['function_all_sites'],literal_command_sites=r['function_literal_sites'],dynamic_command_sites=r['function_dynamic_sites'],
            function_attested_sites=r['function_attested_sites'],function_site_coverage=r['function_attested_sites']/r['function_all_sites'],
            function_unpaired_rows=r['function_unknown_rows'],discarded_function_site_unpaired_rows=r.get('discarded_function_site_unpaired_rows',0),
            capture_code_revision=r.get('capture_code_revision'),decoder_code_sha256=r.get('decoder_code_sha256',{}),
            syntax_selection_independence=r.get('syntax_selection_independence'),legacy_C_selected_pair_count=r.get('legacy_C_selected_pair_count'),
            executed_native_pairs=len(r['function_pairs']),eligible_native_pairs=eligible,scored_executed_pairs=truth,
            native_pairs_outside_original_caller_scope=len(r['function_pairs'])-eligible,
            native_pairs_outside_product_scope=len(r['function_pairs'])-truth,
            executed_pairs_found=found,executed_pair_recall=found/eligible if eligible else None,runtime_precision='unmeasured: unexecuted predictions are not judged',
            source_probe_enabled=r['source_probe_enabled'],runtime_environment_policy=r['runtime_environment_policy'],
            source_literal_sites=r['denominator'],source_attested_sites=r['attested_sites'],source_coverage=r['coverage'],source_dynamic_or_no_argument=r['dynamic_or_no_argument'],
            source_literal_absolute_outside_original_scope=r['literal_absolute_outside_original_scope'],
            source_unique_native_edges=len({(x['path'],x['to_path']) for x in r['imports']}),source_unknown_frames=r['unknown_native_frames'],
            source_native_targets_outside_original_bash_scope=sum(x['to_path'] not in r['source_sha256'] for x in r['imports']),
            observed_source_diagnostic_tier='C: independent syntax selects the literal-site population; native opened filenames are executed facts, not whole-source enumeration',
            observed_source_diagnostic_counts={k:v for k,v in measured['source']['counts'].items() if 'import' in k or 'file_edge' in k} if 'source' in measured else None,
            observed_source_diagnostic_metrics={k:v for k,v in measured['source']['metrics'].items() if 'import' in k or 'file_edge' in k} if 'source' in measured else None,
            native_phase_raw_sha256=[x['raw_sha256'] for x in r['runs']],
            original_inputs_attempted=len(r['runs']),timeouts=sum(x['timeout'] for x in r['runs']),
            source_changed_children=sum(bool(x['changed_source']) for x in r['runs']),driver_exit_histogram=dict(collections.Counter(str(x['exit']) for x in r['runs'])),
            policy='Timed/signalled/source-modified child events are rejected; every original site retained; zero source-literal denominator is unmeasured, not perfect coverage or an A-tier promotion')
    (out/'summary.json').write_text(json.dumps(summary,indent=2,sort_keys=True)+'\n')
    print(json.dumps(summary,indent=2,sort_keys=True))


if __name__=='__main__':main()
