"""Measure Claude Code subagent spawns, hook outcomes and token use from local transcripts.

Reads ~/.claude/projects/**/*.jsonl read-only and writes aggregate numbers (no prompt text) as JSON.
Usage: python3 -I claude_history.py [OUT.json]   (default: ./claude-numbers.json)
Produced the Claude Code figures in README.md on 2026-10-19.
"""
import os,json,re,statistics as st,collections as C,sys
R=os.path.expanduser('~/.claude/projects'); OUT=sys.argv[1] if len(sys.argv)>1 else 'claude-numbers.json'
CUT='2026-08-20'
def med(x): return st.median(x) if x else None
def p90(x):
    x=sorted(x); return x[int(0.9*(len(x)-1))] if x else None
def txt(c):
    if isinstance(c,str): return c
    if isinstance(c,list): return ' '.join((b if isinstance(b,str) else b.get('text','')) for b in c if isinstance(b,(dict,str)))
    return ''
A=dict(bench=C.defaultdict(lambda:C.defaultdict(int)))
agents=[];denies=[];nudge=C.Counter();critwarn=C.Counter();cave=C.Counter();usage={}
hookvol=C.defaultdict(lambda:C.defaultdict(int)); sess_hook=C.defaultdict(lambda:C.defaultdict(int))
errs=C.Counter(); sessions=set(); firstnudge=None; sess_in={}
files=0
for root,d,fs in os.walk(R):
    bench='scenariobench' in root or '-private-tmp-' in root
    for f in fs:
        if not f.endswith('.jsonl'):continue
        p=os.path.join(root,f); files+=1
        side='/subagents/' in p
        sid=None; order=[]; pend={}
        nturn=0
        try: fh=open(p,errors='replace')
        except: continue
        for l in fh:
            if '"type"' not in l: continue
            try:r=json.loads(l)
            except: continue
            sid=r.get('sessionId') or sid
            ts=(r.get('timestamp') or '')[:10]; mo=ts[:7]
            t=r.get('type'); m=r.get('message') or {}
            if t=='assistant':
                nturn+=1
                u=m.get('usage'); mid=m.get('id')
                if u and mid and m.get('model') and m['model']!='<synthetic>':
                    usage[(mid,p if not mid else mid)]=(m['model'],side or r.get('isSidechain',False),ts,bench,u.get('input_tokens',0) or 0,u.get('cache_creation_input_tokens',0) or 0,u.get('cache_read_input_tokens',0) or 0,u.get('output_tokens',0) or 0,sid)
                if isinstance(m.get('content'),list):
                    for b in m['content']:
                        if b.get('type')=='tool_use':
                            n=b.get('name'); i=b.get('input') or {}
                            if n in('Agent','Task'):
                                pr=(i.get('prompt') or '')
                                rec=dict(id=b['id'],f=p,sid=sid,ts=(r.get('timestamp') or ''),mo=mo,model=(i.get('model') or 'unset').lower(),st=i.get('subagent_type') or 'unset',cave='/caveman' in pr or '/caveman' in (i.get('description') or ''),turn=nturn,bench=bench,side=side,err=None,len=None,critic=bool(re.search(r'\[critic\]|critic|adversarial|verif|review|audit|judge|synthesi|strateg',pr+(i.get('description') or ''),re.I)) and '[worker]' not in pr)
                                agents.append(rec); pend[b['id']]=rec; order.append(rec)
                            elif n=='Skill':
                                s=(i.get('skill') or '')
                                if 'cave' in s: cave[('skill:'+s,mo)]+=1
            elif t=='user' and not isinstance(m.get('content'),list) and isinstance(m.get('content'),str) and '<task-notification>' in m['content']:
                c=m['content']; mm=re.search(r'<tool-use-id>(.*?)</tool-use-id>',c); rr=re.search(r'<result>(.*?)</result>',c,re.S)
                if mm and rr and mm.group(1) in pend: pend[mm.group(1)]['len']=len(rr.group(1))
            elif t=='user' and isinstance(m.get('content'),list):
                tur=r.get('toolUseResult')
                if isinstance(tur,dict) and tur.get('resolvedModel'):
                    for b in m['content']:
                        if b.get('type')=='tool_result' and b.get('tool_use_id') in pend: pend[b['tool_use_id']]['resolved']=tur['resolvedModel']
                for b in m['content']:
                    if b.get('type')=='text' and '<task-notification>' in b.get('text',''):
                        c=b['text']; mm=re.search(r'<tool-use-id>(.*?)</tool-use-id>',c); rr=re.search(r'<result>(.*?)</result>',c,re.S)
                        if mm and rr and mm.group(1) in pend: pend[mm.group(1)]['len']=len(rr.group(1))
                for b in m['content']:
                    if b.get('type')=='tool_result':
                        tx=txt(b.get('content'))
                        rec=pend.get(b.get('tool_use_id'))
                        if rec:
                            if b.get('is_error') and any(k in tx for k in('No model set on this Agent','requested Opus','Unknown model string')):
                                rec['err']=('hookerr_prefixed' if 'hook error' in tx[:40] else 'plain')+':'+('nomodel' if 'No model' in tx else 'opus' if 'Opus' in tx else 'unknown'); denies.append(rec)
                            elif not b.get('is_error') and 'Async agent launched' not in tx: rec['len']=len(tx)
                        if re.search(r'hook (error|failed)|timed out|Hook .* (failed|timed)',tx[:300],re.I) and 'PreToolUse' in tx[:300]: errs['toolresult_hookerr']+=1
            elif t=='attachment':
                a=r.get('attachment') or {}; at=a.get('type')
                if at=='hook_success':
                    out=a.get('stdout') or ''
                    hn=a.get('hookName','')
                    if hn.startswith('PreToolUse:Agent'):
                        if 'Token-efficiency reminder' in out:
                            nudge[mo]+=1; ts_full=r.get('timestamp'); 
                            if firstnudge is None or ts_full<firstnudge: firstnudge=ts_full
                            rec=pend.get(a.get('toolUseID'))
                            if rec: rec['nudged']=True
                        if 'Critic-class spawn carries /caveman' in out: critwarn[mo]+=1
                    if a.get('exitCode') not in (0,None): errs['hook_exit_'+str(a.get('exitCode'))+':'+hn]+=1
                    if (a.get('durationMs') or 0)>10000: errs['hook_slow>10s:'+hn]+=1
                elif at in('hook_additional_context',):
                    c=a.get('content'); s=txt(c) if isinstance(c,list) else str(c)
                    if 'code-intelligence is live' in s or 'fathom prime' in s[:300]: k='fathom_session_primer'
                    elif re.search(r'obligation|Governing rule',s[:400],re.I): k='fathom_edit_obligations'
                    elif 'platform' in s[:200].lower() and 'brief' in s[:300].lower(): k='platform_session_brief'
                    else: k='other_hook_ctx:'+s[:40].replace('\n',' ')
                    hookvol[k]['n']+=1; hookvol[k]['chars']+=len(s)
                    sess_hook[sid][k.split(':')[0]]+=len(s); sess_hook[sid]['_all']+=len(s)
                elif at in('hook_non_blocking_error','hook_blocking_error','hook_error_during_execution','hook_timeout','hook_cancelled'):
                    errs[at]+=1
            elif t=='system' and r.get('subtype')=='stop_hook_summary' and r.get('hookErrors'): errs['stop_hook_errors']+=1
        fh.close()
        # retries
        for i,rec in enumerate(order):
            if rec in denies and rec.get('retry') is None:
                nxt=next((o2 for o2 in order[i+1:] if o2['turn']>rec['turn']),None)
                rec['retry']=None if not nxt else (nxt['model'] in('sonnet','haiku') or 'sonnet' in nxt['model'] or 'haiku' in nxt['model'], nxt['turn']-rec['turn'])
out={}
def split(rows): return rows
for label,flt in(('all',lambda r:True),('nonbench',lambda r:not r['bench'])):
    rs=[r for r in agents if flt(r)]
    o={}
    o['n']=len(rs); o['by_month']=dict(sorted(C.Counter(r['mo'] for r in rs).items()))
    o['model']={m:C.Counter(r['model'] if r['model'] in('unset','sonnet','opus','haiku','fable') else ('other:'+r['model'] if r['model']!='unset' else 'unset') for r in rs if r['mo']==m) for m in sorted({r['mo'] for r in rs})}
    o['resolved_model']=dict(C.Counter(re.sub(r'\[.*','',r.get('resolved','?')) for r in rs))
    o['model_total']=dict(C.Counter(re.sub(r'.*(sonnet|opus|haiku|fable).*',r'\1',r['model']) for r in rs))
    o['subagent_type_top15']=C.Counter(r['st'] for r in rs).most_common(15)
    ds=[r for r in denies if flt(r)]; o['denies']=len(ds); o['denies_by_month']=dict(sorted(C.Counter(r['mo'] for r in ds).items()))
    o['denies_by_kind']=dict(C.Counter(r['err'] for r in ds))
    rt=[r['retry'] for r in ds if r.get('retry')]; o['deny_next_call_retry_with_model_frac']=(sum(1 for x in rt if x[0])/len(rt)) if rt else None; o['deny_with_next_call']=len(rt)
    o['deny_median_turns_to_retry']=med([x[1] for x in rt if x[0]])
    # caveman before/after
    fn=firstnudge or ''
    for nm,sel in(('pre',lambda r:r['ts']<fn),('post',lambda r:r['ts']>=fn)):
        w=[r for r in rs if r['critic'] is False and r['model']!='opus' and r['ts'] and sel(r)]
        o['worker_caveman_'+nm]=dict(n=len(w),frac=(sum(r['cave'] for r in w)/len(w)) if w else None)
    # lens
    for nm,sel in(('cave',True),('nocave',False)):
        L=[r['len'] for r in rs if r['len'] is not None and r['cave']==sel]
        o['result_chars_'+nm]=dict(n=len(L),median=med(L),p90=p90(L))
        L=[r['len'] for r in rs if r['len'] is not None and r['cave']==sel and not r['critic']]
        o['result_chars_worker_'+nm]=dict(n=len(L),median=med(L),p90=p90(L))
    o['cavecrew_agents_by_month']=dict(sorted(C.Counter(r['mo'] for r in rs if r['st'].startswith('cavecrew')).items()))
    out[label]=o
out['date_range']=[min(v[2] for v in usage.values()),max(v[2] for v in usage.values())]
out['first_nudge_ts']=firstnudge; out['nudge_by_month']=dict(sorted(nudge.items())); out['critic_caveman_warn_by_month']=dict(sorted(critwarn.items()))
out['caveman_skill_by_month']={f'{k[0]}|{k[1]}':v for k,v in cave.items()}
# usage
def ub(flt):
    g=C.defaultdict(lambda:[0,0,0,0,0])
    for (mid,_),(mo,sd,ts,b,i,cc,cr,o,s) in usage.items():
        if not flt(ts,b): continue
        for key in((mo,'sub' if sd else 'main'),(mo,'ALL'),('ALL','sub' if sd else 'main')):
            x=g[key]; x[0]+=i;x[1]+=cc;x[2]+=cr;x[3]+=o;x[4]+=1
    return g
for label,flt in(('usage_all',lambda ts,b:True),('usage_last60d',lambda ts,b:ts>=CUT),('usage_nonbench',lambda ts,b:not b),('usage_nonbench_last60d',lambda ts,b:(not b) and ts>=CUT)):
    g=ub(flt); tot=g[('ALL','main')],g[('ALL','sub')]
    T=[sum(x) for x in zip(*[t[:4] for t in tot])]; TO=tot[0][3]+tot[1][3]; TT=sum(tot[0][:4])+sum(tot[1][:4])
    out[label]={f'{k[0]}|{k[1]}':dict(msgs=v[4],in_=v[0],cache_create=v[1],cache_read=v[2],out=v[3],out_share=v[3]/TO if TO else 0,total_share=sum(v[:4])/TT if TT else 0) for k,v in sorted(g.items(),key=lambda kv:-sum(kv[1][:4])) if sum(v[:4])>0.002*TT}
# hook volume
out['hook_ctx_by_kind']={k:dict(v) for k,v in hookvol.items() if v['n']>20 or k.startswith('fathom') or k.startswith('platform')}
tot_in={}
for (mid,_),(mo,sd,ts,b,i,cc,cr,o,s) in usage.items():
    tot_in[s]=tot_in.get(s,0)+i+cc+cr
sess=[(s,v) for s,v in sess_hook.items() if s]
for kk in('fathom_session_primer','fathom_edit_obligations','platform_session_brief','_all'):
    xs=[v.get(kk,0) for s,v in sess]; out['hook_chars_per_session_'+kk]=dict(sessions=len(xs),median=med(xs),p90=p90(xs),mean=sum(xs)/len(xs) if xs else 0)
ch=sum(v['_all'] for s,v in sess); out['hook_chars_total']=ch; out['hook_tokens_est_total']=ch/4; out['total_input_tokens_in_sessions_with_hooks']=sum(tot_in.get(s,0) for s,_ in sess)
out['errors']=dict(errs); out['files']=files
json.dump(out,open(OUT,'w'),indent=1,default=str)
print('done',files,len(agents),len(denies),len(usage))
