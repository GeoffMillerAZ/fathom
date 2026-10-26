import os
import pathlib,json,subprocess,re
j=pathlib.Path(os.environ['BENCH_JOB'])
for f in (j/'receipts').glob('*.json'):
 d=json.loads(f.read_text())
 if not isinstance(d,dict) or 'language' not in d:continue
 hist=json.loads((j/'receipts'/f"{d['id']}-history.json").read_text());examples=[]
 for c in hist:
  if not c['fix_message'] or not c['primary_language_non_tests']:continue
  body=subprocess.check_output(['git','-C',str(pathlib.Path('workspace/bench/subjects')/d['id']),'show','-s','--format=%B',c['sha']],text=True)
  closing=re.findall(r'(?i)(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\s*:?\s*#(\d+)',body)
  refs=re.findall(r'#(\d+)',body)
  if refs:examples.append({'sha':c['sha'],'closing_references':sorted(set(closing)),'references':sorted(set(refs)),'test_paths':c['tests'],'source_paths':c['primary_language_non_tests']})
  if len(examples)>=3:break
 d['issue_link_examples']=examples;f.write_text(json.dumps(d,indent=2)+'\n')
 print(d['name'],[(c['sha'][:8],c['closing_references'],c['references']) for c in examples],flush=True)
