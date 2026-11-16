"""Candidate-side exact intersections with an independently projected owner model."""
import argparse,collections,hashlib,json,sys
from pathlib import Path
from owner_compare import project,match,output_unit,read_json,ComparisonRefused,refuse
parser=argparse.ArgumentParser(description='Partial owner comparison; never a native support grade')
parser.add_argument('--candidate',type=Path,required=True)
parser.add_argument('--inputs',type=Path,required=True)
parser.add_argument('--raw',type=Path,required=True)
parser.add_argument('--out',type=Path,required=True)
args=parser.parse_args()
def validate_candidate(result, data):
 symbols, references = result['symbols'], result['references']
 if not isinstance(symbols,list) or not isinstance(references,list) or len(symbols)>100000 or len(references)>100000:
  raise ComparisonRefused('candidate record cap or type')
 lines=len(data.splitlines())
 keys={};ranges={}
 for symbol in symbols:
  if not isinstance(symbol,dict) or any(not isinstance(symbol[k],str) or not symbol[k] for k in ('key','name','kind')):
   raise ComparisonRefused('candidate symbol fields')
  if any(type(symbol[k]) is not int for k in ('line','end_line')) or not 1<=symbol['line']<=symbol['end_line']<=lines:
   raise ComparisonRefused('candidate symbol range')
  if symbol['key'] in keys:raise ComparisonRefused('duplicate candidate owner')
  keys[symbol['key']]=symbol['name'];ranges[symbol['key']]=(symbol['line'],symbol['end_line'])
 for ref in references:
  if not isinstance(ref,dict) or any(not isinstance(ref[k],str) or not ref[k] for k in ('from','to')) or ref['from'] not in keys:
   raise ComparisonRefused('undefined candidate owner')
  if type(ref['line']) is not int or not ranges[ref['from']][0]<=ref['line']<=ranges[ref['from']][1]:
   raise ComparisonRefused('candidate reference range')
 return keys

try:
 with output_unit(args.out):
  inputs=args.inputs
  raw=read_json(args.raw)
  project(raw) # Strict native receipt, including refusals and caps.
  candidate_document,candidate_sha=read_json(args.candidate,with_digest=True)
  candidate=candidate_document['files']
  if not isinstance(candidate,list) or len(candidate)!=len(raw['files']):raise ValueError('candidate input population')
  files=[]
  for row in candidate:
   relative=Path(row['path'])
   if relative.is_absolute() or '..' in relative.parts or '\\' in row['path']:raise ValueError('candidate path')
   file=inputs/relative
   if file.is_symlink() or not file.resolve().is_relative_to(inputs.resolve()):raise ValueError('candidate path escape')
   if file.stat().st_size>1024**2:raise ValueError('candidate source cap')
   files.append(file)
  if len(set(files))!=len(files):raise ValueError('duplicate candidate file')
  truth=collections.defaultdict(list)
  for row in raw['files']:truth[(row['subject'],row['sha256'])].append(row)
  subjects={}
  for file,row in zip(files,candidate):
   if row['path']!=file.relative_to(inputs).as_posix() or row['error']:raise ValueError('candidate read failure')
   result=row['result']
   if not isinstance(result,dict) or type(result['incomplete']) is not bool or not isinstance(result['skipped'],str):raise ComparisonRefused('candidate result fields')
   if result['dialect']!='jenkinsfile' or result['skipped'] or result['incomplete']:raise ValueError('candidate incomplete or wrong dialect')
   with file.open('rb') as stream:data=stream.read(1024**2+1)
   if len(data)>1024**2:raise ComparisonRefused('candidate source cap')
   subject=file.relative_to(inputs).parts[0];sha=hashlib.sha256(data).hexdigest()
   native=truth[(subject,sha)].pop()
   p=project(dict(raw,files=[native]));keys=validate_candidate(result,data)
   definitions=[(s['name'],{'JenkinsPipeline':'job','JenkinsStage':'stage','JenkinsAgent':'resource','JenkinsStages':'section','JenkinsSteps':'step','JenkinsPost':'section','JenkinsEnvironment':'variable_decl','JenkinsParameters':'section','JenkinsOptions':'section','JenkinsFunction':'function','JenkinsLibrary':'import'}.get(s['kind'],s['kind']),s['line'],s['end_line']) for s in result['symbols']]
   pairs=[(keys[r['from']],r['to']) for r in result['references']]
   group=subjects.setdefault(subject,dict(files=0,definitions=collections.Counter(),pairs=collections.Counter()))
   group['files']+=1
   for family,a,b in [('definitions',p['definitions'],definitions),('pairs',p['pairs'],pairs)]:group[family].update(match(a,b))
  if not all(not rows for rows in truth.values()):raise ValueError('unaccounted native inputs')
  for group in subjects.values():
   for family in ('definitions','pairs'):
    group[family]=dict(group[family]);group[family]['precision_lower_bound']=group[family]['matched']/group[family]['candidate'] if group[family]['candidate'] else None
  receipt=dict(subjects=subjects,full_population_native_metrics=False,tier_granted=False,reading_changed=False,
   projection_sha256=hashlib.sha256(Path(__file__).with_name('owner_compare.py').read_bytes()).hexdigest(),
   candidate_raw_sha256=candidate_sha)
  args.out.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
except ComparisonRefused as error:
 sys.exit(refuse(error))
