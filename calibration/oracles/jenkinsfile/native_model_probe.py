"""Bounded owner-model diagnostic, not a reading or a tier upgrade."""
import argparse
import collections
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess

HERE=Path(__file__).resolve().parent

def summarize(raw):
    if set(raw)!={'core_version','tool_declarations','files'} or not isinstance(raw['core_version'],str):
        raise ValueError('native header')
    if type(raw['tool_declarations']) is not int or not 0<=raw['tool_declarations']<=64:raise ValueError('tool count cap')
    files=raw['files']
    if not isinstance(files,list) or not 1<=len(files)<=128:raise ValueError('input count cap')
    states=collections.Counter();classes=collections.Counter();positioned=named=nodes=0
    per={}
    for row in files:
        if set(row)!={'subject','sha256','status','nodes'} or not re.fullmatch('[0-9a-f]{64}',row['sha256']):raise ValueError('native file fields')
        if row['status'] not in ('parsed','refused','no_pipeline'):raise ValueError('native state')
        if row['subject'] not in ('examples','library','authored'):raise ValueError('native subject alias')
        group=per.setdefault(row['subject'],collections.Counter())
        group['inputs']+=1;group[row['status']]+=1
        states[row['status']]+=1
        if not isinstance(row['nodes'],list) or len(row['nodes'])>100000:raise ValueError('node count cap')
        if row['status']!='parsed' and row['nodes']:raise ValueError('partial refused model')
        for index,node in enumerate(row['nodes']):
            if set(node)!={'type','name','line','end','parent'}:raise ValueError('native node fields')
            if not re.fullmatch(r'ModelAST[A-Za-z]+(?:\$[A-Za-z]+)?',node['type']):raise ValueError('native class')
            if node['name'] is not None and not isinstance(node['name'],str):raise ValueError('native name')
            if any(type(node[k]) is not int for k in ('line','end','parent')):raise ValueError('native position type')
            if not -1<=node['parent']<index:raise ValueError('native parent')
            if not ((node['line']==-1 and node['end']==-1) or 1<=node['line']<=node['end']):raise ValueError('native range')
            positioned+=node['line']>0;named+=node['name'] is not None;nodes+=1;classes[node['type']]+=1
            group['model_nodes']+=1;group['positioned_nodes']+=node['line']>0;group['named_nodes']+=node['name'] is not None
    return dict(core_version=raw['core_version'],tool_declarations=raw['tool_declarations'],tools_installed=False,inputs=len(files),parsed=states['parsed'],refused=states['refused'],
                no_pipeline=states['no_pipeline'],model_nodes=nodes,positioned_nodes=positioned,named_nodes=named,
                model_classes=dict(sorted(classes.items())),subjects=[dict(subject=name,**{k:per[name][k] for k in ('inputs','parsed','refused','no_pipeline','model_nodes','positioned_nodes','named_nodes')}) for name in sorted(per)],
                input_sha256=hashlib.sha256(json.dumps(sorted(row['sha256'] for row in files),separators=(',',':')).encode()).hexdigest(),
                calibration_measured=False,tier_granted=False,pipeline_executed=False)

def verify_bundle(java,war,plugins,bundle):
    def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
    if digest(java)!=bundle['java_binary_sha256']:raise ValueError('JDK identity')
    core=next(row for row in bundle['sdk_downloads'] if row['name']=='jenkins.war')
    if digest(war)!=core['sha256']:raise ValueError('core identity')
    expected={row['id']+'.jpi':row['sha256'] for row in bundle['plugins']}
    archives=list(plugins.glob('*.jpi'))+list(plugins.glob('*.hpi'))
    if {path.name for path in archives}!=set(expected):raise ValueError('plugin population identity')
    for path in archives:
        if digest(path)!=expected[path.name]:raise ValueError('plugin content identity')
    return len(archives)


def run(java,war,plugins,inputs,out,tool_declarations=None):
    bundle=json.loads((HERE/'native-bundle-r1.json').read_text())
    plugin_count=verify_bundle(java,war,plugins,bundle)
    out=out.resolve();out.mkdir(parents=True,exist_ok=False)
    home=out/'controller';(home/'init.groovy.d').mkdir(parents=True)
    shutil.copyfile(HERE/'native_model.groovy',home/'init.groovy.d'/'native-model.groovy')
    (home/'plugins').symlink_to(plugins.resolve(),target_is_directory=True)
    raw=out/'raw.json'
    env=dict(PATH='/usr/bin:/bin',LANG='C',JENKINS_HOME=str(home),FATHOM_NATIVE_INPUTS=str(inputs.resolve()),FATHOM_NATIVE_RAW=str(raw))
    if tool_declarations is not None:
        declarations=json.loads(tool_declarations.read_text())
        if not isinstance(declarations,list) or len(declarations)>64:raise ValueError('tool count cap')
        for declaration in declarations:
            if set(declaration)!={'type','name'} or declaration['type'] not in ('jdk','maven') or not isinstance(declaration['name'],str) or not 1<=len(declaration['name'])<=256:raise ValueError('tool declaration')
        env['FATHOM_TOOL_DECLARATIONS']=str(tool_declarations.resolve())
    command=[str(java.resolve()),'-Xmx768m','-Djava.awt.headless=true','-Djenkins.install.runSetupWizard=false',
             '-Dhudson.model.UpdateCenter.never=true','-Dhudson.model.DownloadService.noSignatureCheck=false',
             '-jar',str(war.resolve()),'--httpPort=-1','--httpsPort=-1','--webroot='+str(out/'webroot')]
    with (out/'controller.log').open('wb') as log:
        process=subprocess.Popen(command,env=env,cwd=out,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
        try:code=process.wait(timeout=180)
        finally:
            try:os.killpg(process.pid,signal.SIGKILL)
            except ProcessLookupError:pass
            process.wait()
    if (out/'pipeline-side-effect').exists():raise ValueError('subject execution detected')
    if code or not raw.is_file() or raw.stat().st_size>32*1024*1024:raise ValueError('native controller refused')
    result=summarize(json.loads(raw.read_text()))
    result['instrument_sha256']=hashlib.sha256((HERE/'native_model.groovy').read_bytes()).hexdigest()
    result['core_sha256']=hashlib.sha256(war.read_bytes()).hexdigest()
    result['plugin_count']=plugin_count
    result['bundle_sha256']=hashlib.sha256((HERE/'native-bundle-r1.json').read_bytes()).hexdigest()
    (out/'receipt.json').write_text(json.dumps(result,indent=2)+'\n')
    return result

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    for name in ('java','war','plugins','inputs','out'):parser.add_argument('--'+name,type=Path,required=True)
    parser.add_argument('--tool-declarations',type=Path)
    args=parser.parse_args()
    print(json.dumps(run(args.java,args.war,args.plugins,args.inputs,args.out,args.tool_declarations),sort_keys=True))
