#!/usr/bin/env python3
"""Mutate native pairing, population/refusal and status guards; restore GREEN."""
import argparse,json,os
from pathlib import Path
import subprocess,sys


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);p.add_argument('--tools',type=Path,required=True);a=p.parse_args()
    here=Path(__file__).resolve().parent;out=a.out.resolve();out.mkdir(parents=True,exist_ok=False)
    env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1')
    mutants=[
        ('function_decode.py','pairs.add((relative,caller,callee))',"pairs.add((relative,caller,site['written_name']))",'test_function_decode.NativeFunctionTests.test_actual_native_alias_target_is_truth'),
        ('function_decode.py',"or site['from']!=caller",'or False','test_function_decode.NativeFunctionTests.test_wrong_native_caller_cannot_use_parser_guess'),
        ('function_decode.py','if len(candidates)!=1:','if not candidates:','test_function_decode.NativeFunctionTests.test_multiple_commands_on_one_line_stay_unattested'),
        ('function_decode.py','if path.stat().st_size>max_bytes:','if False:','test_function_decode.NativeFunctionTests.test_byte_cap_plus_one'),
        ('trace_driver.py','if content.count(b\'\\x00\') > max_fields:','if False:','test_trace.NativeTrace.test_each_native_cap_refuses_limit_plus_one'),
        ('trace_driver.py','if not cwd:\n                return None',"if not cwd:\n                cwd = str(root)",'test_trace.NativeTrace.test_missing_native_cwd_is_unknown_not_process_cwd'),
        ('trace_hook.sh','  return 0\n}', '  return "$__oracle_status"\n}','test_trace.NativeTrace.test_debug_observer_preserves_errexit_conditionals_and_status'),
        ('function_hook.sh','  return 0\n}', '  return "$__oracle_return_status"\n}','test_trace.NativeTrace.test_return_observer_does_not_trigger_subject_err_trap'),
        ('scope_limits.py','if not files or len(files)>limits.files:','if not files:','test_scope_limits.ScopeRefusalTests.test_each_source_cap_accepts_n_then_refuses_n_plus_one'),
        ('scip_audit.py',"elif symbol.startswith('scip-shell system '):","elif symbol.startswith(('scip-shell system ','local ')):",'test_scip_audit.ScipAuditTests.test_variable_is_not_a_bound_command'),
        ('scip_audit.py',"unattested_command_sites=len(population['sites'])-len(hits)",'unattested_command_sites=0','test_scip_audit.ScipAuditTests.test_complete_population_keeps_unindexed_file_sites'),
        ('scip_audit.py','if len(docs)>max_documents:','if False:','test_scip_audit.ScipAuditTests.test_caps_accept_n_then_refuse_n_plus_one'),
        ('call_sites.py','if len(sites)>=max_sites:','if False:','test_syntax_scope.OriginalScopeTests.test_call_site_cap_stops_before_later_parser_failure'),
        ('syntax_scope.py','if output.tell() > max_bytes or error.tell() > max_bytes:','if False:','test_syntax_scope.OriginalScopeTests.test_parser_byte_cap_accepts_n_then_refuses_n_plus_one'),
        ('syntax_scope.py','if depth > max_depth:','if False:','test_syntax_scope.OriginalScopeTests.test_parser_nesting_cap_accepts_n_then_refuses_n_plus_one'),
        ('syntax_scope.py','if len(sites) >= max_sites:','if False:','test_syntax_scope.OriginalScopeTests.test_source_site_cap_accepts_n_then_refuses_n_plus_one'),
        ('syntax_scope.py','external += 1','external += 1\n            continue','test_syntax_scope.OriginalScopeTests.test_external_absolute_source_stays_in_full_population'),
        ('control.py',"s == 'ok' and not re.search(r'#\\s*skip\\b', n, re.I)","s == 'ok'",'test_control.DriverOutcomeTests.test_skip_is_not_a_pass_and_failure_is_retained'),
        ('control.py'," or [int(n) for _, n, _ in rows] != list(range(1, int(plans[0]) + 1))",'', 'test_control.DriverOutcomeTests.test_truncated_or_repeated_results_cannot_certify_driver_completion'),
        ('replay.py',"'tier':'A' if mode=='calls' else 'C'", "'tier':'A' if mode=='calls' else 'A'",'test_replay.DiagnosticTierTests.test_perfect_observed_coverage_cannot_promote_independent_source_population'),
        ('runtime_copy.py','symlinks=True','symlinks=False','test_runtime_copy.RuntimeCopyTests.test_symlink_identity_and_copy_isolation'),
        ('runtime_copy.py',"shutil.ignore_patterns('node_modules')","shutil.ignore_patterns('node_modules','.git')",'test_runtime_copy.RuntimeCopyTests.test_git_metadata_is_copied_without_shared_worktree_reference'),
        ('scip_audit.py',"return 'C' if audited['function_definitions_without_native_enclosing_range'] else 'A'","return 'A'",'test_scip_audit.ScipAuditTests.test_missing_native_caller_ranges_are_disclosed'),
        ('scope_limits.py','if len(destination)>max_items:','if False:','test_scope_limits.NativeUnionTests.test_repeated_observations_do_not_expand_but_cap_plus_one_refuses'),
        ('runtime_copy.py',"result[key]=str(scratch)","result[key]=inherited.get(key,str(scratch))",'test_runtime_copy.RuntimeCopyTests.test_driver_scratch_is_inside_allowed_cache'),
        ('runtime_copy.py',"HOME=str(home)","HOME=inherited.get('HOME',str(home))",'test_runtime_copy.RuntimeCopyTests.test_driver_scratch_is_inside_allowed_cache'),
        ('measure_lookup.py',"(kind=='function' and name in functions) or (kind=='builtin' and name in builtins)","name in native_names",'test_lookup.LookupTests.test_available_requires_native_inventory_and_matching_kind'),
        ('measure_lookup.py','if len(fields)>max_records*3+1:','if False:','test_lookup.LookupTests.test_lookup_caps_accept_n_refuse_n_plus_one'),
    ]
    receipt=[]
    for filename,old,new,test in mutants:
        path=here/filename;original=path.read_text();assert original.count(old)==1,(filename,test)
        command=[sys.executable,'-B','-m','unittest',test]
        try:
            path.write_text(original.replace(old,new))
            result=subprocess.run(command,cwd=here,env=env,capture_output=True,text=True)
            assert result.returncode and 'FAIL:' in result.stderr,'mutant survived or failed before assertion: '+test+'\n'+result.stderr
            failure=next(l for l in result.stderr.splitlines() if l.startswith('FAIL:'))
            print(filename,test,failure,flush=True)
        finally:path.write_text(original)
        subprocess.run(command,cwd=here,env=env,check=True)
        receipt.append(dict(file=filename,mutant=new,cwd='calibration/oracles/bash-trace',command='python3 -B -m unittest '+test,FAIL=failure,restored='GREEN'))
    path=here/'collect.py';original=path.read_text()
    old='if sites and not n.functions_only else None'
    assert original.count(old)==1
    common=[sys.executable,'-B',str(here/'prove_mode.py'),'--tools',str(a.tools.resolve())]
    try:
        path.write_text(original.replace(old,'if sites else None'))
        result=subprocess.run(common+['--out',str(out/'source-mode-mutant')],env=env,cwd=here,capture_output=True,text=True)
        assert result.returncode and 'FAIL:' in result.stderr and '0.0 is not None' in result.stderr,result.stderr
        failure=next(l for l in result.stderr.splitlines() if l.startswith('FAIL:'))
    finally:path.write_text(original)
    subprocess.run(common+['--out',str(out/'source-mode-restored')],env=env,cwd=here,check=True)
    receipt.append(dict(file='collect.py',mutant='grade disabled source probe',cwd='calibration/oracles/bash-trace',command='nice -n 10 python3.13 -B prove_mode.py --tools ../../../workspace/tools/bash-native --out ../../../workspace/artifacts/lang-bash/mode-proof',FAIL=failure,restored='GREEN'))
    subprocess.run([sys.executable,'-B','-m','unittest','discover','-s',str(here),'-p','test_*.py'],env=env,check=True)
    (out/'mutations.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
