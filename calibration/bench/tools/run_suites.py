import os
import pathlib, subprocess, json, time, sys, os
ROOT=pathlib.Path.cwd(); JOB=pathlib.Path(os.environ['BENCH_JOB'])
IMAGES={'Go':'golang:1.27-bookworm','Python':'python:3.12-bookworm','TypeScript':'node:22-bookworm'}
def run(d,stage,command,offline=False,limit=900):
 name=d['id']; work=JOB/'builds'/name; work.mkdir(exist_ok=True); (work/'tmp').mkdir(exist_ok=True)
 argv=['docker','run','--rm','--network','none' if offline else 'bridge','--cpus','4','-v',str(pathlib.Path(os.environ['BENCH_SUBJECTS'])/name)+':/subject','-v',str(work)+':/work','-w','/subject','-e','TMPDIR=/work/tmp','-e','GIT_CONFIG_COUNT=1','-e','GIT_CONFIG_KEY_0=safe.directory','-e','GIT_CONFIG_VALUE_0=/subject']
 if d['language']=='Go':
  cache=JOB/'builds/go-cache'; cache.mkdir(exist_ok=True)
  argv+=['-v',str(cache)+':/cache','-e','GOMODCACHE=/cache/mod','-e','GOCACHE=/cache/build','-e','GOTOOLCHAIN=local','-e','GOMAXPROCS=4','-e','GOFLAGS=-mod=readonly']
  if offline:argv+=['-e','GOPROXY=off']
 if offline: argv+=['-e','PIP_NO_INDEX=1','-e','npm_config_offline=true','-e','HTTP_PROXY=','-e','HTTPS_PROXY=','-e','ALL_PROXY=']
 argv += [IMAGES[d['language']],'sh','-c',command]
 start=time.monotonic()
 with open(JOB/'logs'/f'{name}-{stage}.log','w') as log:
  try:p=subprocess.run(argv,stdout=log,stderr=log,timeout=limit); code=p.returncode
  except subprocess.TimeoutExpired:code=124
 result={'command':command,'seconds':round(time.monotonic()-start,3),'exit_code':code,'network':'none' if offline else 'bridge','log':f'{name}-{stage}.log'}
 (JOB/'receipts'/f'{name}-{stage}.json').write_text(json.dumps(result,indent=2)+'\n')
 print(name,stage,result['seconds'],code,flush=True); return result

def suite(d):
 lang=d['language']; name=d['id']
 if lang=='Go': setup='go version && go mod download'; test='go test -count=1 -timeout=14m ./...'
 elif lang=='TypeScript': setup='node --version && npm --version && npm ci'+(' --legacy-peer-deps --ignore-scripts' if name=='typestack--class-transformer' else ''); test='npm run test:ci -- --coverageDirectory=/work/coverage'
 else:
  extras={
   'pallets--click':'pytest',
   'marshmallow-code--marshmallow':'pytest simplejson',
   'python-attrs--attrs':'pytest hypothesis pympler cloudpickle pytest-xdist',
   'Textualize--rich':'pytest==7.4.4 pytest-cov==3.0.0 attrs==21.4.0 pygments==2.19.2 markdown-it-py==3.0.0',
   'encode--starlette':'pytest pytest-codspeed anyio[trio] httpx httpx2[zstd] blockbuster opentelemetry-sdk importlib-metadata python-multipart jinja2 itsdangerous pyyaml typing_extensions',
  }[name]
  setup=f'python --version && python -m venv /work/venv && /work/venv/bin/pip install -e . {extras} && /work/venv/bin/pip freeze > /work/resolved-requirements.txt'
  test='/work/venv/bin/python -m pytest -q -o cache_dir=/work/pytest-cache --basetemp=/work/pytest-temp'
  if name=='pallets--click':
   setup='apt-get update && (cd /work && apt-get download less) && mkdir -p /work/os-tools && dpkg-deb --fsys-tarfile /work/less_*.deb | tar --no-same-owner -x -C /work/os-tools && /work/os-tools/usr/bin/less --version && '+setup
   test='PATH=/work/os-tools/usr/bin:$PATH '+test
 d['suite']={'image':IMAGES[lang],'platform':'linux/arm64','cpus':4,'setup':run(d,'setup',setup,limit=1200),'test':test,'warmup':None,'runs':[]}
 if d['suite']['setup']['exit_code']==0:
  # The only warm-up, also with networking denied. Never count a cached Go test result.
  d['suite']['warmup']=run(d,'warmup',test,offline=True)
  for i in range(1,4): d['suite']['runs'].append(run(d,f'run{i}',test,offline=True))
 (JOB/'receipts'/f'{name}-suite.json').write_text(json.dumps(d['suite'],indent=2)+'\n')
if __name__=='__main__':
 for name in sys.argv[1:]:
  f=JOB/'receipts'/f'{name}.json'; suite(json.loads(f.read_text()))
