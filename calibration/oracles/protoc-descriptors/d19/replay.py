"""Native-first full-population Protobuf replay; raw identities stay private."""
import argparse
import json
import os
from pathlib import Path
import re
import resource
import signal
import subprocess
import tempfile
from prepare import HERE,ROOT,PROTOCOL,digest,floor,manifests,private,verify_sdk


def bounded_json(path,cap):
 with path.open('rb') as source:raw=source.read(cap+1)
 if len(raw)>cap:raise ValueError('artifact cap')
 return json.loads(raw)


def native_value(path,cap):
 value=bounded_json(path,cap)
 if value['tool']!='protoc' or value['version']!='34.1' or value['tier']!='A':raise ValueError('native authority/version')
 return value


def run(command,target,timeout,cap):
 def limits():resource.setrlimit(resource.RLIMIT_FSIZE,(cap,cap))
 with target.with_suffix('.json').open('xb') as output,target.with_suffix('.stderr').open('xb') as error:
  process=subprocess.Popen(command,stdout=output,stderr=error,start_new_session=True,preexec_fn=limits)
  try:
   code=process.wait(timeout=timeout)
  except subprocess.TimeoutExpired:
   try:os.killpg(process.pid,signal.SIGKILL)
   except ProcessLookupError:pass
   process.wait()
   raise
  if code:raise ValueError('subprocess refused')


def oracle_command(out,subject):
 return [str(out/'bin'/'protoc-oracle'),'-subject',str(subject),'-protoc',str(out/'protoc'/'bin'/'protoc')]


def controls(out):
 target=out/'controls';target.mkdir(exist_ok=False);cap=64*1024**2
 with tempfile.TemporaryDirectory() as directory:
  root=Path(directory)
  for name in ['First','Second','Zero']:(root/(name.lower()+'.proto')).write_text('syntax="proto3";package authored;message '+name+'{}\n')
  def input(name):
   (root/'entry.proto').write_text('syntax="proto3";package authored;import "'+name.lower()+'.proto";message Entry{'+name+' one=1;'+name+' two=2;}\n')
  def measure(number):
   path=target/f'control-{number}';run(oracle_command(out,root),path,30,cap);value=native_value(path.with_suffix('.json'),cap)
   if len(value['definitions'])!=4 or len(value['cross_references'])!=2 or len({(r['path'],r['from'],r['to_path']) for r in value['cross_references']})!=1:raise ValueError('authored native population/unit')
   return value
  input('First');before=measure(0)
  (target/'authored-fathom-output.json').write_text('{"nodes":[],"edges":[]}')
  same=measure(1)
  if before!=same:raise ValueError('oracle depends on extractor output')
  input('Second');changed=measure(2)
  if before['cross_references']==changed['cross_references'] or any(r['to_path']!='second.proto' for r in changed['cross_references']):raise ValueError('oracle input insensitive')
  # Driver admission must refuse a lower-authority output as a whole.
  mutant=dict(before,tier='C');path=target/'authority-mutant.json';path.write_text(json.dumps(mutant))
  try:native_value(path,cap)
  except ValueError:pass
  else:raise ValueError('authority mutant survived')
  if native_value(target/'control-0.json',cap)!=before:raise ValueError('restored native admission failed')
 result=dict(native_input_sensitive=True,native_independent=True,zero_reference_file_retained=True,repeated_sites_preserve_distinct_file_pair_unit=True,authority_mutant_refused=True,restored_green=True,oracle_source_sha256=digest(HERE.parent/'main.go'),compiler_sha256=digest(out/'protoc'/'bin'/'protoc'))
 (target/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def native(out):
 protocol=json.loads(PROTOCOL.read_text());cap=protocol['limits']['oracle_artifact_bytes'];target=out/'replay';target.mkdir(exist_ok=False)
 prepared=json.loads((out/'prepare-receipt.json').read_text());control=json.loads((out/'controls'/'summary.json').read_text())
 verify_sdk(out/'compiler.zip',out/'protoc')
 if prepared['protocol_sha256']!=digest(PROTOCOL) or control['oracle_source_sha256']!=digest(HERE.parent/'main.go') or control['compiler_sha256']!=digest(out/'protoc'/'bin'/'protoc'):raise ValueError('preparation/control identity')
 rows=[]
 for index,row in enumerate(protocol['population']):
  floor(out);subject=out/f'subject-{index}'/row['subpath'];path=target/str(index)
  run(oracle_command(out,subject),path,protocol['limits']['subject_timeout_seconds'],cap)
  value=native_value(path.with_suffix('.json'),cap)
  stats=re.findall(r'protoc: (\d+) files, (\d+) compiled, (\d+) rejected;',path.with_suffix('.stderr').read_text())
  if len(stats)!=1 or int(stats[0][0])!=prepared['subjects'][index]['proto_files']:raise ValueError('compiler population count')
  rows.append(dict(subject=index,files=int(stats[0][0]),compiled=int(stats[0][1]),rejected=int(stats[0][2]),definitions=len(value['definitions']),reference_sites=len(value['references']),import_sites=len(value['imports']),cross_reference_sites=len(value['cross_references']),distinct_cross_file_pairs=len({(r['path'],r['from'],r['to_path']) for r in value['cross_references']}),oracle_sha256=digest(path.with_suffix('.json'))))
 result=dict(protocol_sha256=digest(PROTOCOL),compiler_sha256=prepared['compiler_sha256'],oracle_source_sha256=digest(HERE.parent/'main.go'),oracle_binary_sha256=digest(out/'bin'/'protoc-oracle'),subjects=rows)
 (target/'native-receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def compare(out):
 protocol=json.loads(PROTOCOL.read_text());cap=protocol['limits']['oracle_artifact_bytes'];target=out/'replay';native=json.loads((target/'native-receipt.json').read_text());reading=target/'reading.json'
 if reading.exists() or native['protocol_sha256']!=digest(PROTOCOL):raise ValueError('fresh full replay/identity required')
 for index,row in enumerate(protocol['population']):
  floor(out);truth=target/(str(index)+'.json')
  if digest(truth)!=native['subjects'][index]['oracle_sha256']:raise ValueError('native truth changed')
  command=[str(out/'bin'/'indexbench'),'lang-oracle','-dialect','protobuf','-subject',str(out/f'subject-{index}'/row['subpath']),'-subject-name',row['repository'],'-subject-sha',row['sha'],'-tool-oracle',str(truth),'-out',str(reading),'-unmatched',str(target/f'unmatched-{index}.json'),'-note','D19 full preregistered replay; owner protoc34.1 A; no dispatch/signature/ENGINE changes']
  if row['only']:command+=['-only',row['only']]
  run(command,target/f'compare-{index}',protocol['limits']['subject_timeout_seconds'],cap)
 after=manifests(out,'after');before=json.loads((out/'prepare-receipt.json').read_text())['manifests']
 if after!=before:raise ValueError('tracked input changed')
 for index,_ in enumerate(protocol['population']):
  status=subprocess.check_output(['git','-C',str(out/f'subject-{index}'),'status','--porcelain','--untracked-files=all','--ignored'],text=True,timeout=30)
  if status:raise ValueError('subject modified/untracked/ignored input')
 value=bounded_json(reading,cap)
 if len(value['subjects'])!=3:raise ValueError('population dropped')
 result=dict(native=native,manifests=after,modified_untracked_ignored=0,indexbench_sha256=digest(out/'bin'/'indexbench'),reading_sha256=digest(reading),grade=value['grade'],counts=value['counts'],metric_tiers=value['metric_tiers'],subjects=[dict(name=r['name'],sha=r['sha'],files=r['files'],bytes=r['bytes'],counts=r['counts'],families=r['families']) for r in value['subjects']])
 (target/'receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def reconcile(out):
 protocol=json.loads(PROTOCOL.read_text());target=out/'replay';reading=json.loads((target/'reading.json').read_text());rows=[]
 for index,row in enumerate(protocol['population']):
  subject=out/f'subject-{index}'/row['subpath'];native=json.loads((target/f'{index}.json').read_text());unmatched=json.loads((target/f'unmatched-{index}.json').read_text());measured=set(unmatched['tenancy_files']);all_proto={p.relative_to(subject).as_posix() for p in subject.rglob('*.proto') if p.is_file() and not p.is_symlink()}
  found=next(r for r in reading['subjects'] if r['name']==row['repository'])
  eligible={(r['path'],r['from'],r['to_path']) for r in native['cross_references'] if r['path'] in measured and r['to_path'] in measured}
  if len(eligible)!=found['counts'].get('xref_truth',0) or len(measured)!=found['files'] or sum((subject/p).stat().st_size for p in measured)!=found['bytes']:raise ValueError('native/discovery reconciliation')
  excluded=all_proto-measured
  errors=re.findall(r'^protoc rejected ([^:]+):', (target/f'{index}.stderr').read_text(),re.M)
  extra=unmatched.get('dialect_only') or []
  rows.append(dict(subject=index,tracked_proto=len(all_proto),measured_proto=len(measured),measured_bytes=found['bytes'],excluded_proto=len(excluded),excluded_testdata=sum('testdata' in Path(p).parts for p in excluded),native_pairs=len({(r['path'],r['from'],r['to_path']) for r in native['cross_references']}),eligible_native_pairs=len(eligible),excluded_native_pairs=len({(r['path'],r['from'],r['to_path']) for r in native['cross_references']})-len(eligible),compiler_refusals=len(errors),extra_definitions=len(extra),extras_in_compiler_refused_files=sum(r['Path'] in errors for r in extra)))
 (target/'reconciliation.json').write_text(json.dumps(rows,indent=2)+'\n');print(json.dumps(rows,sort_keys=True))


def independence(out):
 target=out/'replay';path=target/'reading.json';original=path.read_bytes();cap=json.loads(PROTOCOL.read_text())['limits']['oracle_artifact_bytes']
 floor(out)
 try:
  path.write_text('{"nodes":[],"edges":[],"authored_mutant":true}\n')
  run(oracle_command(out,out/'subject-2'/'src'),target/'independence',300,cap)
  if digest(target/'independence.json')!=digest(target/'2.json'):raise ValueError('actual extractor output changed raw native truth')
 finally:path.write_bytes(original)
 result=dict(subject=2,actual_comparison_artifact_perturbed=True,native_raw_truth_unchanged=True,reading_restored=True)
 (target/'independence-receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def coexistence(out):
 # Diagnostic only: keep the grading population unchanged, and ask the owner
 # compiler whether every individually accepted new input coexists in one unit.
 # A successful joint compile rejects the global duplicate-type-name concern.
 target=out/'replay';subject=out/'subject-2'/'src'
 refused=set(re.findall(r'^protoc rejected ([^:]+):', (target/'2.stderr').read_text(),re.M))
 files=sorted(p.relative_to(subject).as_posix() for p in subject.rglob('*.proto') if p.is_file() and not p.is_symlink())
 accepted=[p for p in files if p not in refused]
 stats=json.loads((target/'native-receipt.json').read_text())['subjects'][2]
 if len(accepted)!=stats['compiled'] or len(refused)!=stats['rejected']:raise ValueError('joint diagnostic population')
 floor(out)
 command=[str(out/'protoc'/'bin'/'protoc'),'-I',str(subject),'--include_source_info','--include_imports','--descriptor_set_out='+str(target/'coexistence.pb'),*[str(subject/p) for p in accepted]]
 run(command,target/'coexistence',300,64*1024**2)
 result=dict(subject=2,individually_compiled=len(accepted),native_refused=len(refused),joint_compile_succeeded=True,grading_population_unchanged=True)
 (target/'coexistence-receipt.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


def context_control(out):
 target=out/('context-controls-'+digest(HERE.parent/'main.go')[:12]);target.mkdir(exist_ok=False)
 with tempfile.TemporaryDirectory() as directory:
  root=Path(directory)
  for name in ['a','b']:(root/(name+'.proto')).write_text('syntax="proto3";package shared;message Dup{}\n')
  for name,dependency in [('one','a'),('two','b')]:
   (root/(name+'.proto')).write_text('syntax="proto3";package '+name+';import "'+dependency+'.proto";message Caller{shared.Dup value=1;}\n')
  run(oracle_command(out,root),target/'native',30,64*1024**2)
  value=native_value(target/'native.json',64*1024**2)
  actual={r['path']:r['to_path'] for r in value['cross_references']}
  if actual!={'one.proto':'a.proto','two.proto':'b.proto'}:raise ValueError('independent compiler-unit bindings conflated: '+json.dumps(actual,sort_keys=True))
 result=dict(independent_same_name_compiler_contexts=True,native_pairs=2,oracle_source_sha256=digest(HERE.parent/'main.go'))
 (target/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,sort_keys=True))


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('mode',choices=['controls','native','compare','reconcile','independence','coexistence','context-control']);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=private(args.out)
 {'controls':controls,'native':native,'compare':compare,'reconcile':reconcile,'independence':independence,'coexistence':coexistence,'context-control':context_control}[args.mode](out)
