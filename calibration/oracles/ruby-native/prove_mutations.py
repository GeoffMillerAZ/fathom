#!/usr/bin/env python3
"""Mutate native pairing/scope rules, require focused failures, restore GREEN."""
import os
import argparse
from pathlib import Path
import subprocess
import sys


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path)
    ap.add_argument('--indexbench',type=Path)
    ap.add_argument('--out',type=Path)
    args=ap.parse_args()
    if any([args.tools,args.indexbench,args.out]) and not all([args.tools,args.indexbench,args.out]):ap.error('owner mutations require --tools, --indexbench and --out')
    here=Path(__file__).resolve().parent
    env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1')
    mutants=[
        ("pairs.add((relative,native_caller,actual_target))", "pairs.add((relative,syntax['sites'][eligible[0]]['from'],actual_target))",'test_native_caller_never_copies_syntax_pair'),
        ('if callers != {native_caller}:','if False:','test_implicit_callback_is_unattested'),
        ('or (relative,line) in blocked or', 'or False or','test_reflective_dispatch_cannot_impersonate_literal'),
        ("if invoked_name != actual_target or",'if False or','test_alias_is_unattested'),
        ('if len(data[\'calls\']) > limits.rows or len(data[\'functions\']) > limits.functions:', 'if False:','test_rows_cap_plus_one'),
    ]
    path=here/'trace_decode.py'
    for old,new,name in mutants:
        original=path.read_text()
        assert original.count(old)==1,'mutant anchor changed'
        command=[sys.executable,'-B','-m','unittest','test_trace_decode.NativeTraceTests.'+name]
        try:
            path.write_text(original.replace(old,new))
            result=subprocess.run(command,cwd=here,env=env,text=True,capture_output=True)
            print('Mutation:',path.name,name,flush=True)
            print(result.stdout+result.stderr,flush=True)
            assert result.returncode!=0 and 'FAIL:' in result.stderr,'mutant survived or test did not reach assertion'
        finally:path.write_text(original)
        subprocess.run(command,cwd=here,env=env,check=True)
    if args.tools:
        args.out.mkdir(parents=True,exist_ok=False)
        owner_mutants=[
            ("tier: 'A', calls:","tier: 'C', calls:",'tier'),
            ('answers << mid.to_s', 'answers << "fabricated"','target'),
            ('unresolved << loc if answers.empty?', 'nil # mutated unresolved accounting','unresolved'),
            ("abort 'whole-unit refusal: empty source population' if paths.empty?", "paths = paths.first(1)\nabort 'whole-unit refusal: empty source population' if paths.empty?",'scope'),
        ]
        path=here/'typeprof_export.rb'
        for old,new,name in owner_mutants:
            original=path.read_text()
            assert original.count(old)==1,'owner mutant anchor changed'
            command=[sys.executable,'-B',str(here/'prove_native.py'),'--tools',str(args.tools.resolve()),
                     '--indexbench',str(args.indexbench.resolve())]
            try:
                path.write_text(original.replace(old,new))
                result=subprocess.run(command+['--out',str((args.out/(name+'-mutant')).resolve())],env=env,text=True,capture_output=True)
                print('Mutation:',path.name,name,flush=True)
                print(result.stdout+result.stderr,flush=True)
                assert result.returncode!=0 and 'FAIL:' in result.stderr,'owner mutant survived or did not reach assertion'
            finally:path.write_text(original)
            subprocess.run(command+['--out',str((args.out/(name+'-restored')).resolve())],env=env,check=True)
    subprocess.run([sys.executable,'-B','-m','unittest','discover','-s',str(here),'-p','test_*.py'],env=env,check=True)


if __name__=='__main__':main()
