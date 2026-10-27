#!/usr/bin/env python3
"""Export counts/hashes only, or verify the committed supplementary Bash receipt."""
import argparse,hashlib,json,re
from pathlib import Path
from redecode import historical_bytes,historical_hash

HERE=Path(__file__).resolve().parent
REPO=HERE.parents[2]
DEFAULT=REPO/'docs/assessments/data/bash-oracle-2026-10-26/evidence.json'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def check(data,verify_code=True):
    assert data['canonical_reading_changed'] is False
    assert data['canonical_reading_sha256']==sha(REPO/'docs/bench/readings/lang/bash.json')
    runtime,scip,lookup=(data[k] for k in ['runtime','scip','lookup'])
    revision=(data['followup']['historical_replay_revision'] if 'followup' in data else
              runtime.get('code_revision') or data.get('historical_capture_revision'))
    if revision is not None or 'followup' in data:
        old_reading=historical_bytes(revision,'docs/bench/readings/lang/bash.json')
        historical_reading_sha256=hashlib.sha256(old_reading).hexdigest()
        historical_receipt=json.loads(historical_bytes(revision,'docs/assessments/data/bash-oracle-2026-10-26/evidence.json'))
        if (data['canonical_reading_sha256']!=historical_reading_sha256 or
                data['canonical_reading_sha256']!=historical_receipt['canonical_reading_sha256']):
            assert 'followup' in data,'followup required when reading differs from historical bytes'
    assert lookup['tier']=='C' and scip['scoring_tier'].startswith('C ')
    assert len(data['mutations'])==(31 if 'historical_capture_revision' in data else 32)
    assert all(r['FAIL'].startswith('FAIL:') and r['restored']=='GREEN' for r in data['mutations'])
    proof=data['independence']
    for prefix in ['source','calls','scip']:
        assert proof[prefix+'_before']==proof[prefix+'_after_extractor_change']
        assert proof[prefix+'_before']!=proof[prefix+'_after_native_change']
    for prefix in ['source','calls']:
        assert proof[prefix+'_raw_before_sha256']==proof[prefix+'_raw_after_extractor_change_sha256']
    for name in ['nvm','bats']:
        r,s,l=(x['subjects'][name] for x in [runtime,scip,lookup])
        assert r['revision']==s['revision']==l['revision']
        assert r['original_files']==s['original_files']==l['original_files']
        assert r['original_bytes']==s['original_bytes']==l['original_bytes']
        assert r['command_sites']==s['audit']['original_command_sites']==l['sites']
        assert r['command_sites']==r['literal_command_sites']+r['dynamic_command_sites']
        assert l['native_available_sites']+l['unavailable_sites']==l['sites']
        assert s['audit']['native_command_sites']+s['audit']['unattested_command_sites']==l['sites']
        assert s['audit']['function_definitions_without_native_enclosing_range']>0
        assert 0<=r['executed_pairs_found']<=r['scored_executed_pairs']==r.get('eligible_native_pairs',r['scored_executed_pairs'])<=r['executed_native_pairs']
        assert r['native_pairs_outside_product_scope']==r['executed_native_pairs']-r['scored_executed_pairs']
        assert r['source_coverage'] is None and r['observed_source_diagnostic_counts'] is None
        assert r['source_changed_children']==0
        witness=data['witness_diagnostic_C'][name]
        assert witness['accepted_pair_identities']==r['executed_native_pairs']
        assert witness['raw_identities_with_original_definition']+witness['accepted_identities_without_original_definition_witness']==r['executed_native_pairs']
        assert witness['unmatched_executed_pairs']==r['scored_executed_pairs']-r['executed_pairs_found']
        assert witness['unmatched_pairs_whose_target_occurs_in_simple_literal_ERR_registration']<=witness['unmatched_executed_pairs']
        for unit in [r,s,l]:assert unit['tracked_before']==unit['tracked_after']==r['tracked_before']
    control=data['control']
    for key in ['control','observed']:
        row=control[key];assert row['tests']==row['passed']+row['skipped']+row['failed']
    assert control['only_observed_failed_tests']+control['shared_failed_tests']==control['observed']['failed']
    assert control['only_control_failed_tests']+control['shared_failed_tests']==control['control']['failed']
    if 'followup' in data:
        followup=data['followup'];row=runtime['subjects']['nvm']
        assert followup['excluded_executed_pairs']==row['native_pairs_outside_original_caller_scope']==32
        assert followup['all_in_root_recall_bound']==row['executed_pairs_found']/row['executed_native_pairs']
        assert runtime['code_revision']==followup['historical_replay_revision']
        assert all(r['decoder_code_revision']==followup['historical_replay_revision'] for r in runtime['subjects'].values())
        assert historical_receipt['canonical_reading_sha256']==followup['historical_canonical_reading_sha256'],'historical receipt reading hash does not match followup'
        assert historical_reading_sha256==followup['historical_canonical_reading_sha256']
        before,after=json.loads(old_reading),json.loads((REPO/'docs/bench/readings/lang/bash.json').read_text())
        before.pop('notes');after.pop('notes')
        assert before==after,'canonical reading changed beyond inherited notes'
    def visit(value,revision=None):
        if isinstance(value,dict):
            revision=value.get('code_revision',revision)
            for k,v in value.items():
                if verify_code and k in ['code_sha256','instrument_sha256','decoder_code_sha256','current_code_sha256']:
                    code_revision=value.get('capture_code_revision',revision) if k=='instrument_sha256' else revision
                    if k=='decoder_code_sha256':code_revision=value.get('decoder_code_revision')
                    if k=='current_code_sha256':code_revision=None
                    for filename,digest in v.items():
                        path=HERE/filename
                        assert path.parent==HERE and path.is_file()
                        actual=historical_hash(code_revision,filename) if code_revision else sha(path)
                        assert actual==digest,filename+' code drift'
                else:visit(v,revision)
        elif isinstance(value,list):
            for item in value:visit(item,revision)
    visit(data,data.get('historical_capture_revision'))
    return True



def witness_diagnostic(capture,tools,name,folder,runtime_folder="runtime-final"):
    from function_decode import decode as validate_raw
    native=capture/folder
    row=json.loads((native/'summary.json').read_text())
    keys={(x['path'],x['from'],x['to']) for x in row['function_pairs']}
    inside,outside=set(),set()
    original=set(row['source_sha256'])
    raw_root=Path(row.get('raw_evidence_root',native)).resolve()
    child=(raw_root/'child').resolve()
    for phase in row['runs']:
        if phase['timeout'] or phase['exit']<0 or phase['changed_source']:continue
        trace=raw_root/phase['phase']/'functions.trace'
        assert sha(trace)==phase['raw_sha256']['functions.trace']
        validate_raw(trace,child,{'sites':[]})  # fixed bounded native decoder
        fields=trace.read_bytes().split(b'\0')
        for i in range(0,len(fields)-1,6):
            _,file,line,caller,callee,target=(v.decode('utf-8','surrogateescape') for v in fields[i:i+6])
            try:relative=Path(file).relative_to(child).as_posix()
            except ValueError:continue
            key=relative,caller,callee
            if key not in keys:continue
            try:target_relative=Path(target).relative_to(child).as_posix()
            except ValueError:target_relative=None
            (inside if target_relative in original else outside).add(key)
    handlers=set()
    for relative,digest in row['source_sha256'].items():
        source=tools/name/relative
        assert sha(source)==digest
        handlers.update(m[1] for m in re.findall(r"\btrap\s+(['\"])([A-Za-z_][A-Za-z0-9_:.+-]*)\1\s+ERR\b",source.read_text()))
    unmatched=json.loads((capture/runtime_folder/(name+'-calls-unmatched.json')).read_text())['calls']['oracle_only']
    return dict(tier='C source-contract diagnostic only; no native truth or score filtering',
        accepted_pair_identities=len(keys),raw_identities_with_original_definition=len(inside),
        raw_identities_with_outside_original_definition=len(outside),raw_identities_with_both=len(inside&outside),
        accepted_identities_without_original_definition_witness=len(keys-inside),unmatched_executed_pairs=len(unmatched),
        unmatched_pairs_whose_target_occurs_in_simple_literal_ERR_registration=sum(x['callee'] in handlers for x in unmatched),
        unmatched_target_identity_sha256=hashlib.sha256('\n'.join(sorted({x['callee'] for x in unmatched})).encode()).hexdigest(),
        policy='Definition-witness counts concern raw rows sharing accepted pair identities, not a complete event-crossing census. Literal ERR registration matches explain a different runtime population; every unmatched pair stays scored.')


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--check',type=Path)
    p.add_argument('--capture',type=Path,help='private artifact root from README commands')
    p.add_argument('--tools',type=Path)
    p.add_argument('--out',type=Path,default=DEFAULT)
    p.add_argument('--historical-base',help='committed capture revision for an offline review refresh')
    p.add_argument('--followup-base',help='retain reviewed measurements at this revision; refresh disclosure/check hashes only')
    p.add_argument('--runtime-folder',default='runtime-final')
    p.add_argument('--mutations-folder',default='mutations-final')
    p.add_argument('--nvm-folder',default='nvm-functions')
    p.add_argument('--bats-folder',default='bats-final-v4')
    a=p.parse_args()
    if a.followup_base:
        data=json.loads(historical_bytes(a.followup_base,'docs/assessments/data/bash-oracle-2026-10-26/evidence.json'))
        data['runtime']['code_revision']=a.followup_base
        for row in data['runtime']['subjects'].values():row['decoder_code_revision']=a.followup_base
        data['current_code_sha256']={p.name:sha(p) for p in HERE.iterdir() if p.is_file()}
        nvm=data['runtime']['subjects']['nvm']
        data['followup']=dict(historical_replay_revision=a.followup_base,excluded_executed_pairs=32,
            excluded_executed_files=17,dotted_shebang_files=30,
            historical_canonical_reading_sha256=data['canonical_reading_sha256'],
            all_in_root_recall_bound=nvm['executed_pairs_found']/nvm['executed_native_pairs'],
            file_counts_basis='17 executed files and 30 dotted-shebang files are operator-attested from raw capture/source inventory at '+a.followup_base+'; not recomputed by --check.',
            scope_basis='Binding round-2 review: excluded pairs are executed dotted-basename shebang scripts skipped by the shared extension-first discovery rule; counts, not file lists.',
            replay_precondition='Raw captures must stay at their recorded absolute path. A relocated archive is refused with a coverage/path diagnostic; no frame rebasing is implemented.',
            policy='Disclosure/diagnostic follow-up only. Native executions, offline re-score and 31 mutants remain at historical_replay_revision; current authored regression/check code is separately hashed. No new subject execution or grade.')
        data['canonical_reading_change_basis']='canonical_reading_changed means edited by this lane, not byte equality with historical measurements. Inherited notes-only changes are checked against the historical receipt and every non-notes reading field.'
        data['canonical_reading_sha256']=sha(REPO/'docs/bench/readings/lang/bash.json')
        check(data)
        a.out.parent.mkdir(parents=True,exist_ok=True)
        a.out.write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
        print('PASS: retained historical measurements and refreshed follow-up disclosure/check hashes')
        return
    if a.capture:
        assert a.tools,'--tools required for capture'
        def read(folder,filename='summary.json'):return json.loads((a.capture/folder/filename).read_text())
        if a.historical_base:
            data=json.loads(historical_bytes(a.historical_base,'docs/assessments/data/bash-oracle-2026-10-26/evidence.json'))
            data['historical_capture_revision']=a.historical_base
            data['historical_mutations']=data['mutations']
            data['mutations']=read(a.mutations_folder,'mutations.json')
            data['runtime']=read(a.runtime_folder)
            data['runtime']['code_revision']=None
            data['current_code_sha256']={p.name:sha(p) for p in HERE.iterdir() if p.is_file()}
            data['witness_diagnostic_C']={name:witness_diagnostic(a.capture.resolve(),a.tools.resolve(),name,folder,a.runtime_folder) for name,folder in [('nvm',a.nvm_folder),('bats',a.bats_folder)]}
            data['policy']='Offline review correction: native frames enumerate all in-root pairs; A recall uses only unchanged eligible caller-file membership, and syntax gates coverage only. Rejected phases contribute no accepted counts. Original native execution/proof hashes are pinned to historical_capture_revision; current decoder and checks are separately hashed.'
            check(data)
            a.out.parent.mkdir(parents=True,exist_ok=True)
            a.out.write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
            print('PASS: exported corrected native population and historical/current provenance')
            return
        # These producers already export counts, hashes and software contracts only.
        # Never include native traces, source manifests/pairs, SCIP docs or driver logs.
        data=dict(canonical_reading_changed=False,canonical_reading_sha256=sha(REPO/'docs/bench/readings/lang/bash.json'),
            runtime=read('runtime-final'),scip=read('scip-final-c'),lookup=read('lookup-final-v3'),
            control=read('bats-control-v4'),contracts=read('contracts'),
            independence=read('proof-final','proof.json'),mutations=read('mutations-final','mutations.json'),
            policy='Supplementary only: A executed-pair recall, C product-assisted SCIP pair diagnostics and C namespace availability; no canonical promotion, raw subject content or whole-source grade.')
        phases=[]
        for i in range(9):
            log=(a.capture/'nvm-functions/phases'/str(i)/'child.log').read_text(errors='replace')
            passed=re.findall(r'(?m)^([0-9]+) tests? passed\.',log)
            failed=re.findall(r'([0-9]+) tests? failed\.',log)
            # The timeout's incomplete outcomes are never certified or salvaged.
            assert i==0 or (len(passed)==1 and len(failed)==1)
            phases.append(dict(phase=i,timeout=i==0,passed=None if i==0 else int(passed[0]),failed=None if i==0 else int(failed[0])))
        data['nvm_driver_outcomes']=phases
        data['witness_diagnostic_C']={name:witness_diagnostic(a.capture.resolve(),a.tools.resolve(),name,folder) for name,folder in [('nvm','nvm-functions'),('bats','bats-final-v4')]}
        data['tool_receipts']={n:json.loads((a.tools/n).read_text()) for n in ['bash-build.json','scip-build.json','fetch.json']}
        check(data)
        a.out.parent.mkdir(parents=True,exist_ok=True)
        a.out.write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
        print('PASS: exported sanitized supplementary receipt')
    else:
        check(json.loads((a.check or DEFAULT).read_text()))
        print('PASS: scope, denominators, tiers, independence, mutations and committed code hashes; follow-up file counts are operator-attested, not recomputed')


if __name__=='__main__':main()
