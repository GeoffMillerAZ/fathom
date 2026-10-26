"""Bench-only native Conftest config-loader membership, independent TOML positions."""
import argparse,json,os,subprocess,tempfile
from pathlib import Path
import tomli
SKIP={'.git','testdata','vendor','node_modules','generated','.idea','.vscode'}
def positions(src):
 """Validate independently; complete TOML chunks locate top-level keys/values."""
 tomli.loads(src)
 result={};chunk='';start=1;in_table=False
 for line_no,line in enumerate(src.splitlines(keepends=True),1):
  if not chunk:start=line_no
  chunk+=line
  try:part=tomli.loads(chunk)
  except tomli.TOMLDecodeError:continue
  meaningful=chunk.lstrip()
  table=meaningful.startswith('[')
  if table:in_table=True
  if table or not in_table:
   for key in part:result.setdefault(key,(start,line_no))
  chunk=''
 return result

def native_keys(binary,config,work):
 with tempfile.TemporaryDirectory(dir=work) as d:
  out=Path(d)/'keys.json'
  env=dict(os.environ,FATHOM_ORACLE_CONFIG=str(Path(config).resolve()),FATHOM_ORACLE_KEYS=str(out))
  subprocess.run([str(Path(binary).resolve()),'-test.run=^TestFathomOracleConfigKeys$'],env=env,check=True,timeout=30,stdout=subprocess.DEVNULL)
  keys=json.loads(out.read_text())
  if not isinstance(keys,list) or any(not isinstance(k,str) for k in keys):raise ValueError('invalid native key dump')
  return keys

def measure(subject,binary,work):
 defs=[];rejected=0
 for file in sorted(Path(subject).rglob('conftest.toml')):
  if any(p in SKIP for p in file.relative_to(subject).parts):continue
  src=file.read_text()
  try:loc=positions(src)
  except tomli.TOMLDecodeError:rejected+=1;continue
  keys=native_keys(binary,file,work)
  for key in keys:
   if key not in loc:raise ValueError('native key unlocated: '+key)
   lo,hi=loc[key]
   defs.append({'path':file.relative_to(subject).as_posix(),'name':key,'kind':'key','line':lo,'end':hi})
 return {'tool':'Conftest native commands.readInConfig + viper.InConfig (test-only probe)','version':'744c5637a97d60d9cbc06ff41881749c2dc766b9; tomli 2.2.1 positions','tier':'A','definitions':defs},rejected
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('-subject',required=True);p.add_argument('-native',required=True);p.add_argument('-work',required=True);p.add_argument('-out',required=True);a=p.parse_args()
 data,bad=measure(Path(a.subject),a.native,a.work);Path(a.out).write_text(json.dumps(data,indent=2)+'\n');print('native definitions',len(data['definitions']),'TOML rejected',bad)
