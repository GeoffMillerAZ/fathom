"""Full preregistered native-first D19 replay; all raw artifacts remain private."""
import argparse
import hashlib
import json
from pathlib import Path
import resource
import subprocess
from prepare_d19 import HERE,ROOT,PROTOCOL,check_inputs,floor,inside_workspace,manifests


def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def bounded_json(path,cap):
 with path.open('rb') as source:raw=source.read(cap+1)
 if len(raw)>cap:raise ValueError('oracle artifact cap')
 return json.loads(raw)


def limit():
 cap=json.loads(PROTOCOL.read_text())['limits']['oracle_artifact_bytes']
 resource.setrlimit(resource.RLIMIT_FSIZE,(cap,cap))


def native(out):
 protocol=json.loads(PROTOCOL.read_text());replay=out/'replay';replay.mkdir(exist_ok=False)
 controls=json.loads((out/'controls'/'summary.json').read_text())
 if controls['oracle_sha256']!=digest(HERE/'json_schema_oracle.mjs') or not controls['truth_independent'] or not controls['input_sensitive']:raise ValueError('controls identity')
 receipts=[]
 for index,row in enumerate(protocol['population']):
  floor(out);subject=out/f'subject-{index}';counts=check_inputs(subject,row['scopes'],protocol['limits'])
  target=replay/str(index);target.mkdir()
  command=['node','--max-old-space-size=1024',str(HERE/'json_schema_oracle.mjs'),'-tools',str(out/'tools'),'-subject',str(subject),'-out',str(target/'defs.json'),'-xref-out',str(target/'xrefs.json'),'-summary-out',str(target/'stats.json'),'-files-out',str(target/'files')]
  with (target/'native.log').open('wb') as log:subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=protocol['limits']['subject_timeout_seconds'],preexec_fn=limit)
  defs=bounded_json(target/'defs.json',protocol['limits']['oracle_artifact_bytes'])
  xref=bounded_json(target/'xrefs.json',protocol['limits']['oracle_artifact_bytes']) if (target/'xrefs.json').exists() else None
  stats=bounded_json(target/'stats.json',protocol['limits']['oracle_artifact_bytes'])['stats']
  if defs['tier']!='C' or defs['version']!=protocol['oracle']['version'] or (xref and (xref['tier']!='C' or xref['version']!=protocol['oracle']['version'])):raise ValueError('oracle authority or version')
  receipts.append(dict(subject=index,repository=row['repository'],sha=row['sha'],inputs=counts,stats=stats,definitions_sha256=digest(target/'defs.json'),cross_references_sha256=digest(target/'xrefs.json') if xref else None))
 receipt=dict(protocol_sha256=digest(PROTOCOL),oracle_sha256=digest(HERE/'json_schema_oracle.mjs'),controls_sha256=digest(out/'controls'/'summary.json'),subjects=receipts)
 (replay/'native-receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
 print(json.dumps(receipt,sort_keys=True))


def compare(out,indexbench):
 protocol=json.loads(PROTOCOL.read_text());replay=out/'replay';native=json.loads((replay/'native-receipt.json').read_text())
 if native['protocol_sha256']!=digest(PROTOCOL) or native['oracle_sha256']!=digest(HERE/'json_schema_oracle.mjs'):raise ValueError('native identity changed')
 reading=replay/'reading.json'
 if reading.exists():raise ValueError('fresh full-population reading required')
 for index,row in enumerate(protocol['population']):
  floor(out);target=replay/str(index)
  for key,name in [('definitions_sha256','defs.json'),('cross_references_sha256','xrefs.json')]:
   if native['subjects'][index][key] is not None and native['subjects'][index][key]!=digest(target/name):raise ValueError('native truth changed')
  command=[str(indexbench.resolve()),'lang-oracle','-dialect','json-schema','-subject',str(out/f'subject-{index}'),'-subject-name',row['repository'],'-out',str(reading),'-tool-oracle',str(target/'defs.json'),'-unmatched',str(target/'unmatched.json'),'-only',','.join(scope+'/**' for scope in row['scopes']),'-note','P-T1 independent resolver tier C; complete preregistered scope; no network reference fetch; D19 per-metric minimum 20']
  if (target/'xrefs.json').exists():command+=['-tool-oracle',str(target/'xrefs.json')]
  with (target/'compare.log').open('wb') as log:subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=protocol['limits']['subject_timeout_seconds'],preexec_fn=limit)
 after=manifests(out,'after')
 for row in after:
  before=json.loads((out/(str(row['subject'])+'-'+str(row['scope'])+'-before.json')).read_text())
  if before!=row['summary']:raise ValueError('tracked population changed')
 source_changes=[]
 for index,_ in enumerate(protocol['population']):
  status=subprocess.check_output(['git','-C',str(out/f'subject-{index}'),'status','--porcelain','--untracked-files=all','--ignored'],text=True,timeout=30)
  if status:raise ValueError('subject acquired modified, untracked or ignored input')
  source_changes.append(dict(subject=index,modified_untracked_ignored=0))
 measured=bounded_json(reading,protocol['limits']['oracle_artifact_bytes'])
 if len(measured['subjects'])!=len(protocol['population']):raise ValueError('population dropped')
 result=dict(native=native,reading_sha256=digest(reading),indexbench_sha256=digest(indexbench),fathom_commit=measured['fathom_commit'],extractor_tree=subprocess.check_output(['git','rev-parse','HEAD:internal/extractor'],text=True).strip(),manifests=after,subject_changes=source_changes,grade=measured['grade'],counts=measured['counts'],metric_tiers=measured['metric_tiers'],subjects=[dict(name=row['name'],sha=row['sha'],files=row['files'],bytes=row['bytes'],counts=row['counts'],families=row['families']) for row in measured['subjects']])
 (replay/'receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def independence(out):
 protocol=json.loads(PROTOCOL.read_text());replay=out/'replay';target=replay/'independence';target.mkdir(exist_ok=False)
 reading=replay/'reading.json';original=reading.read_bytes()
 # Perturb the actual comparison artifact, not a substitute oracle input.
 try:
  reading.write_text('{"nodes":[],"edges":[],"authored_mutant":true}\n')
  floor(out)
  command=['node','--max-old-space-size=1024',str(HERE/'json_schema_oracle.mjs'),'-tools',str(out/'tools'),'-subject',str(out/'subject-1'),'-out',str(target/'defs.json'),'-xref-out',str(target/'xrefs.json')]
  with (target/'native.log').open('wb') as log:subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=protocol['limits']['subject_timeout_seconds'],preexec_fn=limit)
  same=all(digest(target/name)==digest(replay/'1'/name) for name in ['defs.json','xrefs.json'])
  if not same:raise ValueError('extractor output changed native truth')
 finally:reading.write_bytes(original)
 result=dict(subject=1,actual_comparison_artifact_perturbed=True,native_definitions_unchanged=True,native_cross_references_unchanged=True,reading_restored=True)
 (target/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['native','compare','independence']);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--indexbench',type=Path);args=parser.parse_args();out=inside_workspace(args.out)
 if args.mode=='native':native(out)
 elif args.mode=='independence':independence(out)
 elif args.indexbench:compare(out,args.indexbench)
 else:parser.error('compare needs --indexbench')
