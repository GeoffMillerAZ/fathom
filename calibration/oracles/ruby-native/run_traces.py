#!/usr/bin/env python3
"""Original Ruby test drivers, native executed-call evidence, full site coverage."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import signal
import socket
import subprocess
import sys

from bootstrap import environment, ready
from trace_decode import decode


def validate_driver(status, log):
    if status == 'timeout' or status < 0:
        raise ValueError('whole-unit refusal: native driver timed out or was signalled')
    if 'trace_hook.rb:' in log:
        raise ValueError('whole-unit refusal: native instrumentation traceback')


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS,0)<10:
        raise SystemExit('run with nice -n 10')
    base,out=args.tools.resolve(),args.out.resolve()
    out.mkdir(parents=True,exist_ok=False)
    here=Path(__file__).resolve().parent
    state=json.loads((base/'bootstrap.json').read_text())
    installed=json.loads((base/'bundles.json').read_text())
    ruby=base/state['ruby']['binary']
    root_env=environment(base,ruby)
    public={'ruling':'OL-R10','native_scope':'executed population only; no recall on unexecuted code','subjects':{}}
    jobs={'sinatra':[('sinatra','sinatra','test'),('sinatra-contrib','sinatra/sinatra-contrib','spec'),
                     ('rack-protection','sinatra/rack-protection','spec')],
          'jekyll':[('jekyll','jekyll','test')]}
    for subject,drivers in jobs.items():
        ready()
        target=out/subject
        target.mkdir()
        root=base/subject
        def manifest(suffix):
            prefix=target/('tracked-'+suffix)
            env=dict(root_env,SUBJECT=str(root),SUBPATH='.',RECEIPT=str(prefix))
            subprocess.run([sys.executable,'-B',str(here/'tracked_set.py')],env=env,check=True)
            return json.loads(prefix.with_suffix('.json').read_text())
        before=manifest('before')
        syntax=json.loads(subprocess.check_output([str(base/'pyts/bin/python'),'-B',str(here/'syntax_sites.py'),str(root)]))
        (target/'syntax.json').write_text(json.dumps(syntax,sort_keys=True)+'\n')
        runs=[]
        for name,relative,task in drivers:
            if installed[name]['exit']!=0:
                raise RuntimeError('original declared bundle unavailable: '+name)
            ready()
            driver=target/name
            driver.mkdir()
            cwd=base/relative
            requested_task=task
            occupied=[]
            if name=='sinatra':
                # Original integration helpers use PID-derived ports in this
                # range. Never send test requests to an existing listener.
                for port in range(5000,5205):
                    with socket.socket() as sock:
                        sock.settimeout(.02)
                        if sock.connect_ex(('127.0.0.1',port))==0:occupied.append(port)
                if occupied:
                    task='test:core'  # The subject's own declared alternative.
            env=dict(root_env,BUNDLE_GEMFILE=str(cwd/'Gemfile'),BUNDLE_PATH=str(base/'bundles'/name),
                     BUNDLE_APP_CONFIG=str(base/'bundle-config'/name),RUBY_ORACLE_SUBJECT=str(root),
                     RUBY_ORACLE_TRACE_DIR=str(driver),RUBYOPT='-r'+str(here/'trace_hook.rb'))
            command=[str(ruby),str(ruby.parent/'bundle'),'exec','rake',task]
            def limits():
                resource.setrlimit(resource.RLIMIT_FSIZE,(128<<20,128<<20))
            with (driver/'driver.log').open('w') as log:
                proc=subprocess.Popen(command,cwd=cwd,env=env,stdout=log,stderr=log,start_new_session=True,preexec_fn=limits)
                try:
                    status=proc.wait(timeout=600)
                except subprocess.TimeoutExpired:
                    status='timeout'
                finally:
                    try:os.killpg(proc.pid,signal.SIGKILL)
                    except ProcessLookupError:pass
                    proc.wait()
            try:
                validate_driver(status,(driver/'driver.log').read_text(errors='replace'))
            except ValueError:
                (target/'refusal.json').write_text(json.dumps({'driver':name,'status':status,'whole_unit_refused':True})+'\n')
                raise
            runs.append({'driver':name,'requested_task':requested_task,'task':task,'exit':status,
                         'occupied_integration_port_count':len(occupied),
                         'raw_files':len(list(driver.glob('native-*.json')))})
            print('finished original driver',name,status,flush=True)
        after=manifest('after')
        if before!=after:
            raise RuntimeError('original tracked source changed during runtime; no oracle emitted')
        raw=sorted(target.glob('*/native-*.json'))
        oracle,summary=decode(raw,syntax)
        (target/'oracle.json').write_text(json.dumps(oracle,indent=2,sort_keys=True)+'\n')
        public['subjects'][subject]=dict(summary,drivers=runs,tracked_before=before,tracked_after=after,
            original_files=syntax['files'],raw_native_files=len(raw),
            native_oracle_sha256=hashlib.sha256((target/'oracle.json').read_bytes()).hexdigest(),
            syntax_population_sha256=hashlib.sha256((target/'syntax.json').read_bytes()).hexdigest(),
            trace_hook_sha256=hashlib.sha256((here/'trace_hook.rb').read_bytes()).hexdigest(),
            precision='unmeasured outside executed population')
        (out/'summary.json').write_text(json.dumps(public,indent=2,sort_keys=True)+'\n')
        print(subject,public['subjects'][subject],flush=True)


if __name__=='__main__':main()
