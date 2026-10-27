#!/usr/bin/env python3
"""Kill native scope/tier/pairing/cap mutants and always restore GREEN."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    ap.add_argument('--followups-only',action='store_true',help='run only the deferred-minor native regressions')
    args=ap.parse_args()
    here=Path(__file__).resolve().parent
    out=args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1')
    receipt=[]
    decoder=[
        ('pairs.add((path,bare(caller),bare(target)))', "pairs.add((path,site['from'],bare(target)))",'test_native_names_do_not_come_from_parser_pair'),
        ('pairs.add((path,bare(caller),bare(target)))', 'pairs.add((path,bare(caller),bare(invoked)))','test_native_alias_target_is_not_written_alias'),
        ("or bare(caller).casefold()!=bare(site['from']).casefold()",'or False','test_implicit_callback_is_unattested'),
        ('if Path(path).stat().st_size>limits.raw_bytes:', 'if False:','test_bytes_cap_plus_one'),
    ]
    owners=[
        ('ResolveCalls.ps1',"tier = 'A'","tier = 'C'",'tier'),
        ('ResolveCalls.ps1',"$allowed = $isRuntimeIntrinsic -or ($source -and", "$allowed = $false -or ($source -and",'reject-initial-runtime-intrinsic'),
        ('ResolveCalls.ps1',"elseif ($found.Count) { 'found_not_admitted' }", "elseif ($found.Count) { 'missing' }",'hide-found-rejected-command'),
        ('ResolveCalls.ps1',"($command.Module.ModuleBase -eq $PSHOME -or $command.Module.ModuleBase.StartsWith($PSHOME + '/', [StringComparison]::Ordinal))", "$command.Module.ModuleBase.StartsWith($PSHOME + '/', [StringComparison]::Ordinal)",'reject-baseline-root-cmdlets'),
        ('ResolveCalls.ps1',"$PSModuleAutoLoadingPreference = 'None'\n$before = @(Get-OracleModules)","$PSModuleAutoLoadingPreference = 'All'\n$before = @(Get-OracleModules)",'autoload'),
        ('ResolveCalls.ps1',"@('Microsoft.PowerShell.Archive','Microsoft.PowerShell.Host','Microsoft.PowerShell.Management','Microsoft.PowerShell.Security','Microsoft.PowerShell.ThreadJob','Microsoft.PowerShell.Utility')", "@('Microsoft.PowerShell.Archive','Microsoft.PowerShell.Host','Microsoft.PowerShell.Management','Microsoft.PowerShell.ThreadJob','Microsoft.PowerShell.Utility')",'explicit-runtime-baseline'),
        ('ResolveCalls.ps1',"$scope = if ($moduleSources.Contains([IO.Path]::GetFullPath((Join-Path $root $relative)))) { 'module' } else { 'global' }","$scope = 'module'",'privacy'),
        ('ResolveCalls.ps1',"if ($sites.Count -gt $siteCap) { throw 'whole-unit refusal: site count cap' }",'# mutated cap bypass','site-cap'),
        ('TraceFunctions.ps1',"Set-PSBreakpoint -Command '*' -Action $action","Set-PSBreakpoint -Script @($files | Where-Object { [IO.Path]::GetExtension($_) -in '.ps1','.psm1' }) -Command '*' -Action $action",'script-qualified-breakpoint'),
    ]
    def run_mutant(filename,old,new,name,command,restored):
        path=here/filename
        original=path.read_text()
        assert original.count(old)==1,'mutant anchor changed: '+name
        try:
            path.write_text(original.replace(old,new))
            result=subprocess.run(command,env=env,cwd=here,text=True,capture_output=True)
            print('Mutation:',filename,name,flush=True)
            print(result.stdout+result.stderr,flush=True)
            assert result.returncode!=0 and 'FAIL:' in result.stderr,'mutant survived or failed before an assertion'
            fail=next(line for line in result.stderr.splitlines() if line.startswith('FAIL:'))
        finally:path.write_text(original)
        subprocess.run(restored,env=env,cwd=here,check=True)
        receipt.append({'file':filename,'mutant':name,'FAIL':fail,'restored':'GREEN'})
    followups=[
        ('ResolveCalls.ps1', '-or $parent.CommandElements[0] -is [Management.Automation.Language.ParenExpressionAst]', '-or $false', 'omit-parenthesized-module-scope'),
        ('ResolveCalls.ps1', '$definitionAst = [Management.Automation.Language.Parser]::ParseFile($declaration.target, [ref]$null, [ref]$null)', '. $declaration.target; $definitionAst = [Management.Automation.Language.Parser]::ParseFile($declaration.target, [ref]$null, [ref]$null)', 'execute-transitive-definition-preamble'),
        ('ResolveCalls.ps1', 'if ($originIds.Count -and -not $localOrigins.Count -and -not $ownedModuleDefinition)', 'if ($false)', 'omit-test-helper-crossings'),
        ('DiscoverHarness.ps1', '} while ($changed)', '} while ($false)', 'omit-transitive-declaring-closure'),
        ('ResolveCalls.ps1', 'if ($shadowedHelperIds.Count)', 'if ($false)', 'omit-shadowed-declared-origin-count'),
        ('ResolveCalls.ps1', "@('test.ps1','tst/Pester.BeforeContainer.ps1'", "@('tst/Pester.BeforeContainer.ps1'", 'omit-declared-suite-runner-global'),
    ]
    common=[sys.executable,'-B',str(here/'prove_followups.py'),'--tools',str(args.tools.resolve())]
    for filename,old,new,name in followups:
        run_mutant(filename,old,new,name,common+['--out',str(out/(name+'-mutant'))],common+['--out',str(out/(name+'-restored'))])
    if args.followups_only:
        (out/'mutations.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')
        return
    for old,new,test in decoder:
        command=[sys.executable,'-B','-m','unittest','test_trace_decode.NativeFunctionTests.'+test]
        run_mutant('trace_decode.py',old,new,test,command,command)
    command=[sys.executable,'-B','-m','unittest','test_combine_runtime.PhaseCombinationTests.test_both_original_phases_retained']
    run_mutant('combine_runtime.py',"for item in original for row in item['calls']","for item in original[:1] for row in item['calls']",'drop-second-phase',command,command)
    command=[sys.executable,'-B','-m','unittest','test_driver_outcome.DriverOutcomeTests.test_zero_exit_does_not_hide_stopped_default_phase']
    run_mutant('driver_outcome.py',"status='stopped-after-p-test-failures'","status='tests-passed'",'stopped-phase-is-success',command,command)
    command=[sys.executable,'-B','-m','unittest','test_analyze_misses.MissAttributionTests.test_generated_mock_requires_actual_written_capture']
    run_mutant('analyze_misses.py',"if MOCK.fullmatch(pair[2]) and captured:","if MOCK.fullmatch(pair[2]):",'mock-without-written-capture',command,command)
    for filename,old,new,name in owners:
        common=[sys.executable,'-B',str(here/'prove_native.py'),'--tools',str(args.tools.resolve()),'--indexbench',str(args.indexbench.resolve())]
        run_mutant(filename,old,new,name,common+['--out',str(out/(name+'-mutant'))],common+['--out',str(out/(name+'-restored'))])
    common=[sys.executable,'-B',str(here/'prove_inventory.py'),'--tools',str(args.tools.resolve())]
    inventory_mutants=[
        ('DiscoverHarness.ps1', '@($declarations.Values | Sort-Object { $_.kind + ":" + $_.target })', '@($declarations.Values | Where-Object { $_.kind -ne "module" } | Sort-Object { $_.kind + ":" + $_.target })', 'drop-parser-enumerated-module'),
        ('ResolveCalls.ps1', "$parent.InvocationOperator -in @('Ampersand','Dot')", "$parent.InvocationOperator -eq 'Unknown'", 'omit-module-variable-private-ancestor'),
        ('ResolveCalls.ps1', "$matches = @($excludedNames[$name] | Where-Object { $_.target -eq $queryPath -or $_.declaring_files.Contains($queryPath) })", "$matches = @($excludedNames[$name])", 'leak-refused-helper-class-across-files'),
    ]
    inventory_mutants.append(('DiscoverHarness.ps1', "} else { $null }\n                    $declarations[$identity]", "} else { 'fabricated-code-hash' }\n                    $declarations[$identity]", 'invent-code-hash-for-absent-target'))
    for filename,old,new,name in inventory_mutants:
        run_mutant(filename,old,new,name,common+['--out',str(out/(name+'-mutant'))],common+['--out',str(out/(name+'-restored'))])
    common=[sys.executable,'-B',str(here/'prove_harness.py'),'--tools',str(args.tools.resolve()),'--indexbench',str(args.indexbench.resolve())]
    run_mutant('measure_binder.py',"'-Harness',name", "'-Harness','none'",'omit-declared-harness',
               common+['--out',str(out/'harness-mutant')],common+['--out',str(out/'harness-restored')])
    subprocess.run([sys.executable,'-B','-m','unittest','discover','-s',str(here),'-p','test_*.py'],env=env,check=True)
    (out/'mutations.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
