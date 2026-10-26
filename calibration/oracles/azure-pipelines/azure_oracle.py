"""Tier C: independent PyYAML syntax walk; no Azure evaluation is claimed."""
import argparse,json,re
from pathlib import Path
import yaml
from yaml.nodes import MappingNode,SequenceNode,ScalarNode
SKIP={'.git','testdata','vendor','node_modules','.idea','.vscode','generated'}
SIGNAL=re.compile(r'^\s*-\s*(?:stage|job|deployment)\s*:',re.M)
def fields(n):
 return {k.value:v for k,v in n.value if isinstance(k,ScalarNode)} if isinstance(n,MappingNode) else {}
def text(n):return n.value if isinstance(n,ScalarNode) and n.tag=='tag:yaml.org,2002:str' else ''
def definitions(root):
 out=[];seen=set()
 def walk(n):
  if id(n) in seen:return
  seen.add(id(n))
  if isinstance(n,MappingNode):
   for k,v in n.value:
    if k.value in ('stages','jobs') and isinstance(v,SequenceNode):
     for item in v.value:
      d=fields(item)
      for key,kind in [('stage','stage'),('job','job'),('deployment','deployment')]:
       if key in d and text(d[key]):out.append((text(d[key]),kind,item.start_mark.line+1,d))
    walk(v)
  elif isinstance(n,SequenceNode):
   for v in n.value:walk(v)
 walk(root);return out
def measure(subject):
 defs=[];refs=[];rejected=0
 for p in sorted(Path(subject).rglob('*')):
  if not p.is_file() or p.suffix not in ('.yml','.yaml') or any(x in SKIP for x in p.relative_to(subject).parts):continue
  src=p.read_text();rel=p.relative_to(subject).as_posix()
  if not (p.name.startswith('azure-pipelines') or SIGNAL.search(src)):continue
  try:root=yaml.compose(src,Loader=yaml.SafeLoader)
  except yaml.YAMLError:rejected+=1;continue
  if not isinstance(root,MappingNode):continue
  fs=fields(root)
  if 'apiVersion' in fs and 'kind' in fs or '.github/workflows/' in rel:continue
  ds=definitions(root)
  defs.append({'path':rel,'name':p.name,'line':root.start_mark.line+1,'kind':'pipeline'})
  names={'stage':{n for n,k,_,_ in ds if k=='stage'},'job':{n for n,k,_,_ in ds if k in ('job','deployment')}}
  for name,kind,line,d in ds:
   defs.append({'path':rel,'name':name,'line':line,'kind':kind})
   deps=d.get('dependsOn');values=deps.value if isinstance(deps,SequenceNode) else [deps]
   for target in sorted({text(v) for v in values} & names['stage' if kind=='stage' else 'job']):refs.append({'path':rel,'from':name,'to':target})
 return {'tool':'azure-pipelines independent PyYAML walk','version':yaml.__version__,'tier':'C','definitions':defs,'references':refs},rejected
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('-subject',required=True);p.add_argument('-out',required=True);a=p.parse_args()
 data,rejected=measure(Path(a.subject));Path(a.out).write_text(json.dumps(data,indent=2)+'\n');print('definitions',len(data['definitions']),'references',len(data['references']),'rejected',rejected)
