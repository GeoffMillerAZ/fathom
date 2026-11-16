"""Candidate-side exact intersections with an independently projected owner model."""
import argparse,collections,hashlib,json
from pathlib import Path
from owner_compare import project,match
parser=argparse.ArgumentParser(description='Partial owner comparison; never a native support grade')
parser.add_argument('--candidate',type=Path,required=True)
parser.add_argument('--inputs',type=Path,required=True)
parser.add_argument('--raw',type=Path,required=True)
parser.add_argument('--out',type=Path,required=True)
args=parser.parse_args()
inputs=args.inputs
if args.raw.stat().st_size>32*1024**2 or args.candidate.stat().st_size>32*1024**2:raise ValueError('JSON parse cap')
raw=json.loads(args.raw.read_text())
project(raw) # Strict native receipt, including refusals and caps.
candidate=json.loads(args.candidate.read_text())['files']
if len(candidate)!=len(raw['files']):raise ValueError('candidate input population')
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
 if result['dialect']!='jenkinsfile' or result['skipped'] or result['incomplete']:raise ValueError('candidate incomplete or wrong dialect')
 subject=file.relative_to(inputs).parts[0];sha=hashlib.sha256(file.read_bytes()).hexdigest()
 native=truth[(subject,sha)].pop()
 p=project(dict(raw,files=[native]));keys={s['key']:s['name'] for s in result['symbols']}
 definitions=[(s['name'],{'JenkinsPipeline':'job','JenkinsStage':'stage','JenkinsAgent':'resource','JenkinsStages':'section','JenkinsSteps':'step','JenkinsPost':'section','JenkinsEnvironment':'variable_decl','JenkinsParameters':'section','JenkinsOptions':'section','JenkinsFunction':'function','JenkinsLibrary':'import'}.get(s['kind'],s['kind']),s['line'],s['end_line']) for s in result['symbols']]
 pairs=[(keys.get(r['from'],r['from']),r['to']) for r in result['references']]
 group=subjects.setdefault(subject,dict(files=0,definitions=collections.Counter(),pairs=collections.Counter()))
 group['files']+=1
 for family,a,b in [('definitions',p['definitions'],definitions),('pairs',p['pairs'],pairs)]:group[family].update(match(a,b))
if not all(not rows for rows in truth.values()):raise ValueError('unaccounted native inputs')
for group in subjects.values():
 for family in ('definitions','pairs'):
  group[family]=dict(group[family]);group[family]['precision_lower_bound']=group[family]['matched']/group[family]['candidate'] if group[family]['candidate'] else None
receipt=dict(subjects=subjects,full_population_native_metrics=False,tier_granted=False,reading_changed=False,
 projection_sha256=hashlib.sha256(Path(__file__).with_name('owner_compare.py').read_bytes()).hexdigest(),
 candidate_raw_sha256=hashlib.sha256(args.candidate.read_bytes()).hexdigest())
args.out.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps(receipt))
