#!/usr/bin/env python3
"""Collect original-file native Bash entry events under a lane-only write sandbox."""
from runtime_copy import copy_subject,runtime_environment
import argparse, collections, hashlib, importlib.util, json, os, pathlib, resource, shutil, signal, subprocess, sys
from trace_driver import decode, events
from call_sites import collect as collect_call_sites
from function_decode import decode as decode_functions
from scope_limits import validate,merge_observations
from syntax_scope import parse, source_sites
from build_bash import ready

def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()

def child_limits():
    # Bound a test that emits unbounded output or downloads a large runtime.
    # The native file limit exceeds the decoder's 32 MiB cap, ensuring a
    # truncated file at the OS limit cannot be accepted as complete evidence.
    resource.setrlimit(resource.RLIMIT_FSIZE, (64 * 1024 * 1024, 64 * 1024 * 1024))

def main():
    a = argparse.ArgumentParser()
    a.add_argument('--subject', required=True)
    a.add_argument('--shfmt', required=True)
    a.add_argument('--out', required=True)
    a.add_argument('--cache', required=True)
    a.add_argument('--repo', required=True)
    a.add_argument('--timeout', type=float, default=3)
    a.add_argument('--bash', default='/bin/bash')
    a.add_argument('--fullpath', action='store_true')
    a.add_argument('--driver',choices=['nvm','bats','urchin'],default='nvm')
    a.add_argument('--urchin')
    a.add_argument('--fixture')
    a.add_argument('--functions-only',action='store_true',help='collect executed functions; retain source population as unmeasured')
    n = a.parse_args()
    subprocess.run(['df', '-h', str(pathlib.Path.home())], check=True)
    if shutil.disk_usage(pathlib.Path.home()).free < 20*(1 << 30):
        raise SystemExit('Requires at least 20 GiB free')
    if os.getloadavg()[0] > 6 or os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise SystemExit('Requires nice10 and start load<=6')
    repo = pathlib.Path(n.repo).resolve()
    root = pathlib.Path(n.subject).resolve()
    out = pathlib.Path(n.out).resolve()
    out.mkdir(parents=True, exist_ok=False)
    cache = pathlib.Path(n.cache).resolve()
    shfmt = str(pathlib.Path(n.shfmt).resolve())
    bash = str(pathlib.Path(n.bash).resolve())
    spec = importlib.util.spec_from_file_location('syntax', repo / 'calibration/oracles/bash-source/bash_source_dump.py')
    syntax = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(syntax)
    tracked = subprocess.check_output(['git','-C',str(root),'ls-files','-z']).split(b'\0')
    files = sorted(os.fsdecode(p) for p in tracked if p and not (root/os.fsdecode(p)).is_symlink() and (root/os.fsdecode(p)).is_file() and syntax.wanted(str(root),os.fsdecode(p),{'.sh','.bash'},['bash','sh']))
    validate(root,files)
    def tracked_manifest(suffix):
        prefix=out/('tracked-'+suffix)
        subprocess.run([sys.executable,'-B',str(pathlib.Path(__file__).with_name('tracked_set.py'))],env=dict(os.environ,SUBJECT=str(root),SUBPATH='.',RECEIPT=str(prefix)),check=True)
        return json.loads(prefix.with_suffix('.json').read_text())
    tracked_before=tracked_manifest('before')
    manifest = {p: sha(root / p) for p in files}
    instrument = {p: sha(pathlib.Path(__file__).with_name(p)) for p in ['collect.py','entry_hook.sh','trace_hook.sh','trace_driver.py','supervise.py','function_hook.sh','function_decode.py','call_sites.py','scope_limits.py','syntax_scope.py','build_bash.py','runtime_copy.py']}
    sites = []
    unparsed = []
    dynamic = 0
    external_absolute_sources = 0
    for rel in files:
        tree = parse(shfmt, (root / rel).read_bytes())
        original, computed, external = source_sites(tree, root, rel, syntax, max_sites=1_000_000-len(sites))
        sites.extend(original)
        dynamic += computed
        external_absolute_sources += external
    if len(sites)>1_000_000:raise ValueError('whole-unit refusal: original source site cap')
    function_population=collect_call_sites(root,files,shfmt,syntax)
    validate(root,files,sites,function_population['sites'])
    function_pairs=set();function_hits=set();function_unknown=0
    hits = set()
    runs = []
    imports = set()
    native_unknown = 0
    native_versions=set()
    # Run original executable test inputs; utility programs are not invented entrypoints.
    selected = [p for p in files if p.startswith('test/') and os.access(root / p, os.X_OK)]
    if n.driver=='bats':selected=['bin/bats']
    if n.driver=='urchin':
        if not n.urchin or not n.fixture:raise ValueError('declared Urchin and pinned gitlink fixture required')
        selected=sorted(p.name for p in (root/'test').iterdir() if p.is_dir())
    for (i, rel) in enumerate(selected):
        if rel in {x['path'] for x in runs}:
            continue
        ready()
        child = out / 'child'
        copy_subject(root, child)
        if n.driver=='urchin':
            destination=child/'test/fixtures/nvmrc'
            if destination.exists():destination.rmdir()
            copy_subject(pathlib.Path(n.fixture).resolve(),destination)
        phase = out if len(selected)==1 else out/'phases'/str(i)
        phase.mkdir(parents=True,exist_ok=True)
        trace = phase / 'native.trace'
        trace.write_bytes(b'')
        function_trace=phase/'functions.trace'
        function_trace.write_bytes(b'')
        log = phase / 'child.log'
        # Native scripts can write only disposable child data and lane caches.
        allow = [str(out), str(cache), '/dev/null']
        profile = '(version 1)(allow default)(deny file-write*)(allow file-write* ' + ' '.join(('(subpath ' + json.dumps(p) + ')' for p in allow)) + ')'
        sb = out / 'writes.sb'
        sb.write_text(profile)
        env = runtime_environment(phase/'environment',os.environ)
        env.update(LC_ALL='C', PATH=str(pathlib.Path(bash).parent)+os.pathsep+env.get('PATH',''), ORACLE_FULLPATH='1' if n.fullpath else '0', BASH_ENV=str(pathlib.Path(__file__).with_name('entry_hook.sh').resolve()), ORACLE_CALL_TRACE_FILE=str(function_trace), ORACLE_TRACE_FILE=str(trace), NVM_DIR=str(child), TEST_SHELL=bash)
        # A zero literal-source population is unmeasured, not 100% covered.
        # Its source metadata snapshots provide no graded evidence; collect
        # native function frames without that unnecessary volume instead.
        env['ORACLE_FUNCTION_ONLY']='1' if n.functions_only or not sites else '0'
        cwd=child if n.driver=='bats' else (child/rel).parent
        arguments=['test'] if n.driver=='bats' else []
        command=[bash,str(child/rel),*arguments]
        if n.driver=='urchin':
            cwd=child
            command=['/usr/bin/make','URCHIN='+str(pathlib.Path(n.urchin).resolve()),'TEST_SUITE='+rel,'test-bash']
            env.update(MAKEFLAGS='-j2',NVM_MAKE_JOBS='2')
        status = phase/'driver-status.json'
        status.unlink(missing_ok=True)
        supervisor = pathlib.Path(__file__).with_name('supervise.py').resolve()
        with log.open('wb') as fh:
            proc = subprocess.Popen([sys.executable,'-B',str(supervisor),'--status',str(status),'--timeout',str(n.timeout),
                '--','/usr/bin/sandbox-exec','-f',str(sb),*command],cwd=cwd,env=env,
                stdout=fh,stderr=fh,start_new_session=True,preexec_fn=child_limits)
            supervisor_exit=proc.wait(timeout=n.timeout+60)
        if supervisor_exit != -signal.SIGKILL or not status.exists():
            raise RuntimeError('whole-unit refusal: native supervisor did not complete owned-group cleanup')
        completion=json.loads(status.read_text())
        timed=completion['timeout']
        code=completion['driver_exit'] if completion['driver_exit'] is not None else -signal.SIGKILL
        changed = [p for (p, d) in manifest.items() if not (child / p).is_file() or sha(child / p) != d]
        (rows, unknown) = decode(trace, child)
        native_versions.update(tuple(fields) for kind,fields in events(trace) if kind=='VERSION')
        native_pairs,native_hits,unpaired_functions=decode_functions(function_trace,child,function_population)
        function_unknown+=unpaired_functions
        native_unknown += unknown
        if not changed and not timed and code >= 0:
            merge_observations(function_pairs,native_pairs);function_hits.update(native_hits)
            for (p, line, target) in rows:
                matching = [s for s in sites if s['path'] == p and s['line'] == line]
                if len(matching) == 1:
                    hits.add((p, line))
                    merge_observations(imports,[(p,line,target)])
        runs.append({'path': rel, 'exit': code, 'timeout': timed, 'changed_source': changed, 'native_function_pairs':len(native_pairs),'native_function_sites':len(native_hits),'native_entries': len(rows), 'unknown_native_frames': unknown,'phase':phase.relative_to(out).as_posix(),'raw_sha256':{p.name:sha(p) for p in [trace,function_trace,log,status]}})
        shutil.rmtree(child)
        if (i + 1) % 25 == 0:
            print(f'{i + 1}/{len(selected)} files; {len(hits)}/{len(sites)} literal sites attested', flush=True)
    tracked_after=tracked_manifest('after')
    if tracked_before!=tracked_after:raise ValueError('whole-unit refusal: original tracked source changed')
    result = {'function_pairs':[{'path':p,'from':f,'to':t} for p,f,t in sorted(function_pairs)],'function_literal_sites':function_population['literal_sites'],'function_all_sites':len(function_population['sites']),'function_dynamic_sites':function_population['dynamic_sites'],'function_attested_sites':len(function_hits),'function_unknown_rows':function_unknown,'tracked_before':tracked_before,'tracked_after':tracked_after,'complete': len(runs) == len(selected), 'selected_native_inputs': len(selected), 'requested_fullpath': n.fullpath, 'bash_binary_sha256': sha(pathlib.Path(bash)), 'bash_version': subprocess.check_output([bash, '--version'], text=True).splitlines()[0], 'shfmt_version': subprocess.check_output([shfmt, '--version'], text=True).strip(), 'subject_files': len(files), 'instrument_sha256': instrument, 'source_sha256': manifest, 'literal_sites': sites, 'dynamic_or_no_argument': dynamic, 'unparsed': unparsed, 'runs': runs, 'attested_sites': len(hits), 'denominator': len(sites), 'coverage': len(hits) / len(sites) if sites and not n.functions_only else None, 'unknown_native_frames': native_unknown, 'imports': [{'path': p, 'line': l, 'to_path': t} for (p, l, t) in sorted(imports)], 'unattested': [s for s in sites if (s['path'], s['line']) not in hits]}
    result.update(native_versions=[{'version':v,'fullpath':flag=='1'} for v,flag in sorted(native_versions)],
        system_bash_binary_sha256=sha(pathlib.Path('/bin/bash')),
        system_bash_version=subprocess.check_output(['/bin/bash','--version'],text=True).splitlines()[0],
        source_probe_enabled=not n.functions_only and bool(sites),
        runtime_environment_policy='fresh per-phase HOME/temp/npm/XDG roots; host shell/NVM/Git/npm overrides removed; no PROFILE override; native system mktemp may remain unavailable under lane-only writes',
        literal_absolute_outside_original_scope=external_absolute_sources,
        function_population_sha256=hashlib.sha256(json.dumps(function_population,sort_keys=True).encode()).hexdigest(),
        source_population_sha256=hashlib.sha256(json.dumps(sites,sort_keys=True).encode()).hexdigest())
    (out / 'summary.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps({k: result[k] for k in ['subject_files', 'attested_sites', 'denominator', 'coverage', 'unparsed']}))
if __name__ == '__main__':
    main()
