import os
import concurrent.futures, subprocess, pathlib, json, urllib.request, time
ROOT=pathlib.Path.cwd(); JOB=pathlib.Path(os.environ['BENCH_JOB'])
subjects={'Go':['go-chi/chi','pressly/goose','spf13/cobra','stretchr/testify','urfave/cli'],'Python':['pallets/click','marshmallow-code/marshmallow','python-attrs/attrs','Textualize/rich','encode/starlette'],'TypeScript':['immerjs/immer','typestack/class-transformer','typestack/class-validator','microsoft/tsyringe','reduxjs/redux']}
def prep(item):
 lang,repo=item; name=repo.replace('/','--'); dest=pathlib.Path(os.environ['BENCH_SUBJECTS'])/name
 with open(JOB/'logs'/f'{name}-clone.log','w') as log:
  p=subprocess.run(['git','clone','https://github.com/'+repo+'.git',str(dest)],stdout=log,stderr=log)
 if p.returncode: return [repo,'clone_failed']
 def git(*args):return subprocess.check_output(['git','-C',str(dest),*args],text=True).strip()
 sha=git('rev-parse','HEAD'); ref=git('symbolic-ref','--short','HEAD'); subprocess.run(['git','-C',str(dest),'checkout','--detach',sha],capture_output=True)
 try:
  with urllib.request.urlopen(urllib.request.Request('https://api.github.com/repos/'+repo,headers={'User-Agent':'bench-prep-measurement'})) as r: meta=json.load(r)
 except Exception as e: meta={'error':str(e)}
 out={'name':repo,'id':name,'language':lang,'sha':sha,'ref':ref,'metadata':{k:meta.get(k) for k in ['created_at','stargazers_count','fork','archived','default_branch','html_url']}}
 (JOB/'receipts'/f'{name}.json').write_text(json.dumps(out,indent=2)+'\n')
 return [repo,sha,meta.get('stargazers_count')]
with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
 for result in pool.map(prep,[(l,r) for l,rs in subjects.items() for r in rs]): print(json.dumps(result),flush=True)
