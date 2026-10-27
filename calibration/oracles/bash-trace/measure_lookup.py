#!/usr/bin/env python3
"""Full original-site availability audit of Bash's declared-session namespace."""
import argparse,collections,hashlib,importlib.util,json,os,resource,subprocess,sys
from pathlib import Path
from build_bash import ready
from call_sites import collect
from fetch import SUBJECTS
from runtime_copy import copy_subject,runtime_environment
from scope_limits import validate


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def decode(path,max_bytes=32<<20,max_records=1_000_000,max_field_bytes=1<<20):
    if not (1<=max_bytes<=32<<20 and 1<=max_records<=1_000_000 and 1<=max_field_bytes<=1<<20):raise ValueError('whole-unit refusal: invalid lookup caps')
    if path.stat().st_size>max_bytes:raise ValueError('whole-unit refusal: lookup byte cap')
    fields=path.read_bytes().split(b'\0')
    if fields[-1]!=b'' or (len(fields)-1)%3:raise ValueError('whole-unit refusal: truncated lookup record')
    if len(fields)>max_records*3+1:raise ValueError('whole-unit refusal: lookup row cap')
    if any(len(v)>max_field_bytes for v in fields):raise ValueError('whole-unit refusal: lookup field cap')
    functions={};builtins=set();queries={}
    for i in range(0,len(fields)-1,3):
        kind,name,value=(v.decode() for v in fields[i:i+3])
        if kind=='F':
            if name in functions:raise ValueError('whole-unit refusal: duplicate native function')
            functions[name]=value
        elif kind=='B':
            if name in builtins:raise ValueError('whole-unit refusal: duplicate native builtin')
            builtins.add(name)
        elif kind=='Q':
            if name in queries:raise ValueError('whole-unit refusal: duplicate lookup query')
            queries[name]=value
        else:raise ValueError('whole-unit refusal: unknown lookup record')
    native_names={name:name for name in [*functions,*builtins]}
    answers={name:native_names[name] for name,kind in queries.items() if (kind=='function' and name in functions) or (kind=='builtin' and name in builtins)}
    return answers,queries,functions,builtins


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['tools','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();ready()
    if os.getpriority(os.PRIO_PROCESS,0)<10:raise SystemExit('nice10 required')
    here=Path(__file__).resolve().parent;base=a.tools.resolve();out=a.out.resolve();out.mkdir(parents=True,exist_ok=False)
    spec=importlib.util.spec_from_file_location('bash_syntax',here.parent/'bash-source/bash_source_dump.py');syntax=importlib.util.module_from_spec(spec);spec.loader.exec_module(syntax)
    version=subprocess.check_output([str(base/'bash-5.3/bash'),'--version'],text=True).splitlines()[0]
    assert '5.3.20' in version
    summary=dict(tool='GNU Bash type / compgen / declare namespace audit',version=version,tier='C',bash_binary_sha256=sha(base/'bash-5.3/bash'),
        policy='Independent syntax selects source sites; native namespace answers establish availability in the recorded union, not context-sensitive native reference/caller truth. No whole-source grade or runtime coverage inferred.',subjects={},
        code_sha256={n:sha(here/n) for n in ['measure_lookup.py','lookup.sh','call_sites.py','syntax_scope.py','scope_limits.py','runtime_copy.py']})
    for name,(_,pin) in SUBJECTS.items():
        ready();root=base/name;target=out/name;target.mkdir()
        assert subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD'],text=True).strip()==pin
        def manifest(suffix):
            prefix=target/('tracked-'+suffix)
            subprocess.run([sys.executable,'-B',str(here/'tracked_set.py')],env=dict(os.environ,SUBJECT=str(root),SUBPATH='.',RECEIPT=str(prefix)),check=True)
            return json.loads(prefix.with_suffix('.json').read_text())
        before=manifest('before')
        tracked=[os.fsdecode(v) for v in subprocess.check_output(['git','-C',str(root),'ls-files','-z']).split(b'\0') if v]
        files=sorted(f for f in tracked if (root/f).is_file() and not(root/f).is_symlink() and syntax.wanted(str(root),f,{'.sh','.bash'},['bash','sh']))
        validate(root,files);population=collect(root,files,str(base/'shfmt'),syntax);validate(root,files,call_sites=population['sites'])
        child=target/'child';copy_subject(root,child)
        if name=='nvm':loads=['nvm.sh'];contract='original documented nvm.sh --no-use loader'
        else:
            package=json.loads((root/'package.json').read_text());declared=package['files']
            loads=sorted(f for f in tracked if f.endswith('.bash') and any(f.startswith(d.rstrip('/')+'/') for d in declared) and not(root/f).is_symlink())
            contract='session-wide union of tracked Bash libraries from package files manifest; no test body loaded'
        (target/'loads').write_bytes(b''.join(f.encode()+b'\0' for f in loads))
        unique=sorted({s['written_name'] for s in population['sites'] if s['literal']})
        (target/'queries').write_bytes(b''.join(n.encode()+b'\0' for n in unique))
        raw=target/'native.raw';raw.write_bytes(b'')
        profile=target/'writes.sb';profile.write_text('(version 1)(allow default)(deny file-write*)(allow file-write* (subpath '+json.dumps(str(target))+') (literal "/dev/null"))')
        env=runtime_environment(target/'environment',os.environ)
        env.pop('BASH_ENV',None);env.update(PATH=str(base/'bash-5.3')+':/usr/bin:/bin:/usr/sbin:/sbin',LC_ALL='C')
        def caps():resource.setrlimit(resource.RLIMIT_FSIZE,(64<<20,64<<20))
        with (target/'lookup.log').open('w') as log:
            subprocess.run(['/usr/bin/sandbox-exec','-f',str(profile),str(base/'bash-5.3/bash'),'--noprofile','--norc',str(here/'lookup.sh'),str(child),str(target/'loads'),str(target/'queries'),str(raw)],cwd=child,env=env,stdout=log,stderr=log,timeout=60,preexec_fn=caps,check=True)
        answers,kinds,functions,builtins=decode(raw)
        assert set(kinds)==set(unique)
        misses=collections.Counter('dynamic' if not s['literal'] else kinds[s['written_name']] for s in population['sites'] if s['written_name'] not in answers)
        hits=sum(s['written_name'] in answers for s in population['sites'])
        assert hits+sum(misses.values())==len(population['sites'])
        assert all(sha(root/f)==sha(child/f) for f in files)
        after=manifest('after');assert before==after
        summary['subjects'][name]=dict(revision=pin,original_files=len(files),original_bytes=sum((root/f).stat().st_size for f in files),
            sites=len(population['sites']),native_available_sites=hits,unavailable_sites=sum(misses.values()),unavailable_by_kind=dict(misses),
            native_functions=len(functions),native_builtins=len(builtins),load_contract=contract,loaded_files=len(loads),
            loaded_code_sha256=hashlib.sha256('\n'.join(sorted(sha(root/f) for f in loads)).encode()).hexdigest(),
            native_raw_sha256=sha(raw),tracked_before=before,tracked_after=after,
            population_sha256=hashlib.sha256(json.dumps(population,sort_keys=True).encode()).hexdigest())
        import shutil
        shutil.rmtree(child)
    (out/'summary.json').write_text(json.dumps(summary,indent=2,sort_keys=True)+'\n')
    print(json.dumps(summary,indent=2,sort_keys=True))


if __name__=='__main__':main()
