"""Measure Codex models, efforts, spawn_agent arguments, token use and tool-output sinks from local rollouts.

Reads ~/.codex/sessions/**/*.jsonl read-only and writes aggregate numbers (no prompt text) as JSON.
Usage: python3 -I codex_history.py [OUT.json]   (default: ./codex-numbers.json)
Produced the Codex figures in README.md on 2026-10-19.
"""
import sys
import json,glob,collections,re,statistics,os,datetime
D=os.path.expanduser('~/.codex/')
fs=sorted(glob.glob(D+'sessions/**/*.jsonl',recursive=True))
R=collections.defaultdict(lambda:collections.defaultdict(int))
vers=collections.Counter();srcs=collections.Counter()
tc_model=collections.Counter();tc_month=collections.defaultdict(collections.Counter)
sess_model=collections.defaultdict(set)
spawn_keys=collections.Counter();spawn_month=collections.Counter();spawn_set=collections.Counter();spawn_models=collections.Counter();spawn_eff=collections.Counter();spawn_ex=None
tok=collections.defaultdict(lambda:collections.Counter())  # key (model,kind,period)
sizes=collections.defaultdict(list)
hookev=collections.Counter();hookex={}
cutoff=datetime.datetime(2026,8,20)
def kind_of(src):
    return 'sub' if isinstance(src,dict) and 'subagent' in src else 'parent'
for f in fs:
    m=re.search(r'rollout-(\d{4}-\d\d)-(\d\d)',f);mon=m.group(1)
    day=datetime.datetime.strptime(m.group(1)+'-'+m.group(2),'%Y-%m-%d')
    per='60d' if day>=cutoff else 'old'
    kind='parent';model=None;eff=None;seen=set();callname={}
    for l in open(f,errors='ignore'):
        try:r=json.loads(l)
        except:continue
        t=r.get('type');p=r.get('payload',{})
        if t=='session_meta':
            vers[p.get('cli_version')]+=1;kind=kind_of(p.get('source'))
            s=p.get('source');srcs[json.dumps(s)[:60] if isinstance(s,dict) else s]+=1
        elif t=='turn_context':
            model=p.get('model');eff=p.get('effort')
            tc_model[(model,eff)]+=1;tc_month[mon][(model,eff)]+=1;sess_model[f].add(model)
        elif t=='token_usage_record':
            rid=p.get('response_id')
            if rid in seen:continue
            seen.add(rid);u=p['usage']
            for per_ in ('all',per):
                k=(model,kind,per_)
                for a in('input_tokens','cached_input_tokens','output_tokens','reasoning_output_tokens'):tok[k][a]+=u.get(a,0)
                tok[k]['n']+=1
        elif t=='response_item':
            ty=p.get('type')
            if ty in('function_call','custom_tool_call'):
                n=p['name'];callname[p.get('call_id')]=n
                if n=='spawn_agent':
                    spawn_month[mon]+=1
                    try:a=json.loads(p['arguments'])
                    except:a={}
                    for k in a:spawn_keys[k]+=1
                    spawn_models[a.get('model')]+=1;spawn_eff[a.get('reasoning_effort')]+=1
                    if 'model' in a:spawn_set['model']+=1
                    if 'reasoning_effort' in a:spawn_set['effort']+=1
                    spawn_set['n']+=1
                    if not spawn_ex:spawn_ex={k:(v if k in('task_name','fork_turns','model','reasoning_effort') else '<%s>'%type(v).__name__) for k,v in a.items()}
            elif ty in('function_call_output','custom_tool_call_output'):
                o=p.get('output');o=o if isinstance(o,str) else json.dumps(o)
                sizes[callname.get(p.get('call_id'),'?')].append(len(o))
        if 'hook' in l[:400].lower() and t!='response_item':
            hookev[(t,p.get('type') if isinstance(p,dict) else None)]+=1
out={'files':len(fs),'range':[fs[0][-70:-60],fs[-1][-70:-60]],'versions':vers,'sources':srcs,
'turn_ctx_model_effort':{f'{k[0]}|{k[1]}':v for k,v in tc_model.most_common()},
'month_model':{m:{f'{k[0]}|{k[1]}':v for k,v in c.most_common(6)} for m,c in sorted(tc_month.items())},
'spawn_total':sum(spawn_month.values()),'spawn_by_month':dict(sorted(spawn_month.items())),'spawn_keys':spawn_keys,'spawn_set':spawn_set,'spawn_models':spawn_models,'spawn_eff':spawn_eff,'spawn_example':spawn_ex,
'tokens':{f'{k[0]}|{k[1]}|{k[2]}':dict(v) for k,v in tok.items()},
'sizes':{n:{'calls':len(v),'total_chars':sum(v),'median':statistics.median(v),'p90':sorted(v)[int(len(v)*.9)]} for n,v in sizes.items() if len(v)>=20},
'hook_records':{str(k):v for k,v in hookev.items()}}
json.dump(out,open((sys.argv[1] if len(sys.argv)>1 else 'codex-numbers.json'),'w'),indent=1,default=dict)
print(json.dumps(out,default=dict)[:100])
