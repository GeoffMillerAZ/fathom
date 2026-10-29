"""Exercise native context and root-order mutations; always restore source."""
import argparse
import json
from pathlib import Path
import subprocess
from prepare import HERE,ROOT,digest,private


def run(out):
 out.mkdir(parents=True,exist_ok=False);source=HERE.parent/'main.go';original=source.read_bytes();text=original.decode();results=[]
 cases=[('compiler_context','TestReferenceDestinationKeepsCompilerContexts','func referenceDestination(',lambda body:body.replace('if contexts != nil {','if false {',1)),('root_precedence','TestDescriptorFileUsesCompilerRootPrecedence','func descriptorFile(',lambda body:body.replace('\n\tfor _, root := range roots {','\n\troots = append([]string(nil), roots...)\n\tsort.SliceStable(roots, func(i,j int) bool { return len(roots[i]) > len(roots[j]) })\n\tfor _, root := range roots {',1))]
 for name,test,anchor,mutate in cases:
  start=text.index(anchor);end=text.index('\nfunc ',start+len(anchor));body=text[start:end];changed=mutate(body)
  if changed==body:raise ValueError('mutation anchor did not change')
  command=['bash',str(ROOT/'scripts'/'fathom-go.sh'),'test','-race','.','-run','^'+test+'$','-count=1']
  try:
   source.write_text(text[:start]+changed+text[end:])
   red=subprocess.run(command,cwd=HERE.parent,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=120)
   (out/(name+'-red.log')).write_bytes(red.stdout)
  finally:source.write_bytes(original)
  green=subprocess.run(command,cwd=HERE.parent,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=120)
  (out/(name+'-green.log')).write_bytes(green.stdout)
  if red.returncode==0 or b'--- FAIL: '+test.encode() not in red.stdout or green.returncode!=0:raise ValueError('context mutant failed to prove RED then GREEN')
  results.append(dict(mutant=name,test=test,restored_green=True))
 if source.read_bytes()!=original:raise ValueError('source not restored')
 result=dict(oracle_source_sha256=digest(source),mutations=results)
 (out/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();run(private(args.out))
