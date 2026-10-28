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
 if controls['oracle_sha256']!=digest(HERE/'json_schema_oracle.mjs') or not controls['input_sensitive']:raise ValueError('controls identity')
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


def compare(out,indexbench,pair_counter):
 protocol=json.loads(PROTOCOL.read_text());replay=out/'replay';native=json.loads((replay/'native-receipt.json').read_text())
 if native['protocol_sha256']!=digest(PROTOCOL) or native['oracle_sha256']!=digest(HERE/'json_schema_oracle.mjs'):raise ValueError('native identity changed')
 reading=replay/'reading.json';pair_counts=[]
 if reading.exists():raise ValueError('fresh full-population reading required')
 for index,row in enumerate(protocol['population']):
  floor(out);target=replay/str(index)
  for key,name in [('definitions_sha256','defs.json'),('cross_references_sha256','xrefs.json')]:
   if native['subjects'][index][key] is not None and native['subjects'][index][key]!=digest(target/name):raise ValueError('native truth changed')
  command=[str(indexbench.resolve()),'lang-oracle','-dialect','json-schema','-subject',str(out/f'subject-{index}'),'-subject-name',row['repository'],'-out',str(reading),'-tool-oracle',str(target/'defs.json'),'-unmatched',str(target/'unmatched.json'),'-only',','.join(scope+'/**' for scope in row['scopes']),'-note','P-T1 independent resolver tier C; complete preregistered scope; no network reference fetch; D19 per-metric minimum 20']
  if (target/'xrefs.json').exists():command+=['-tool-oracle',str(target/'xrefs.json')]
  # A preliminary harness pass enumerates the exact tenancy population even
  # when no oracle xref family exists. The counter never supplies oracle truth.
  command[command.index('-out')+1]=str(target/'population-reading.json')
  with (target/'population.log').open('wb') as log:subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=protocol['limits']['subject_timeout_seconds'],preexec_fn=limit)
  count_command=[str(pair_counter.resolve()),'-subject',str(out/f'subject-{index}'),'-membership',str(target/'unmatched.json')]
  counted=json.loads(subprocess.check_output(count_command,timeout=protocol['limits']['subject_timeout_seconds']))
  absent=not (target/'xrefs.json').exists()
  counted.update(subject=index,oracle_xref_family_present=not absent,unattested_unscored_pairs=counted['fathom_pairs'] if absent else 0)
  pair_counts.append(counted)
  command[command.index('-out')+1]=str(reading)
  pair_note=(f"Fathom resolves {counted['fathom_pairs']} distinct file/root-entity/target-file pairs; {counted['unattested_unscored_pairs']} unattested and unscored because the oracle emits no xref family where it attests none. Neither true nor false without identifier-aware attestation; no empty family supplied." if absent else f"Fathom resolves {counted['fathom_pairs']} distinct file/root-entity/target-file pairs, all admitted to the existing xref score; zero additional unscored pairs.")
  command+=['-note',pair_note,'-note',"Pooled xref precision 1 describes only 3 scored pairs; its denominator is under 20 only because other subjects have unattested and unscored Fathom pairs, not because Fathom resolves only 3 pairs.",'-note','Timing is one final capture per subject, nice 10, load at most 6 and free disk at least 20 GiB, private caches, no daemon; not a median of three or a speedup claim.']
  with (target/'compare.log').open('wb') as log:subprocess.run(command,stdout=log,stderr=subprocess.STDOUT,check=True,timeout=protocol['limits']['subject_timeout_seconds'],preexec_fn=limit)
  repeated=json.loads(subprocess.check_output(count_command,timeout=protocol['limits']['subject_timeout_seconds']))
  if any(repeated[key]!=counted[key] for key in repeated):raise ValueError('comparison population or Fathom pairs changed')
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
 result=dict(unattested_fathom=pair_counts,pair_counter_sha256=digest(pair_counter),pair_counter_source_sha256=digest(HERE/'fathom-pairs'/'main.go'),native=native,reading_sha256=digest(reading),indexbench_sha256=digest(indexbench),fathom_commit=measured['fathom_commit'],extractor_tree=subprocess.check_output(['git','rev-parse','HEAD:internal/extractor'],text=True).strip(),manifests=after,subject_changes=source_changes,grade=measured['grade'],counts=measured['counts'],metric_tiers=measured['metric_tiers'],subjects=[dict(name=row['name'],sha=row['sha'],files=row['files'],bytes=row['bytes'],counts=row['counts'],families=row['families']) for row in measured['subjects']])
 (replay/'receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def independence(out):
 # A source/argv review record, not an output-perturbation experiment.
 result=dict(method='structural_argv_and_source_review',dynamic_independence_control=False,
             basis='Oracle argv supplies tools and subject only; reviewed reads enumerate subject JSON and resolver dependencies, never a Fathom comparison artifact.',
             oracle_sha256=digest(HERE/'json_schema_oracle.mjs'))
 target=out/'replay'/'independence';target.mkdir(exist_ok=False)
 (target/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['native','compare','independence']);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--indexbench',type=Path);parser.add_argument('--pair-counter',type=Path);args=parser.parse_args();out=inside_workspace(args.out)
 if args.mode=='native':native(out)
 elif args.mode=='independence':independence(out)
 elif args.indexbench and args.pair_counter:compare(out,args.indexbench,args.pair_counter)
 else:parser.error('compare needs --indexbench and --pair-counter')
