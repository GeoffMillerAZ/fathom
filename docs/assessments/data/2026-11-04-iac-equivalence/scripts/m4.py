import sys,statistics; sys.path.insert(0,".")
from load import *
pairs=json.load(open("./measure/pairs.json"))
def nm(s): return s.replace("_","").lower()
def tfattrs(sch):
    b=sch["block"]; out={}
    for k,v in b.get("attributes",{}).items():
        out[nm(k)]=("opt" if (v.get("optional") or v.get("required")) else "comp", v.get("type"), k)
    for k,v in b.get("block_types",{}).items(): out[nm(k)]=("opt","block:"+v.get("nesting_mode",""),k)
    return out
def cfn_t(p,d=None):
    if "$ref" in p and d is not None and p["$ref"].startswith("#/definitions/"):
        q=d.get("definitions",{}).get(p["$ref"].split("/")[-1],{})
        return cfn_t(q) if "$ref" not in q else "object"
    t=p.get("type")
    if t is None:
        if "$ref" in p: return "object"
        if "oneOf" in p or "anyOf" in p: return "union"
        return "?"
    if isinstance(t,list): return "union"
    return t
def tf_t(x):
    if isinstance(x,str): return {"string":"string","number":"number","bool":"boolean"}.get(x,x)
    if isinstance(x,list):
        return {"list":"array","set":"array","map":"object","object":"object"}.get(x[0],x[0])
    return "?"
def classify(c,t):
    if isinstance(t,str) and t.startswith("block:"): return "array" if t[6:] in("list","set") else "object"
    return tf_t(t)
def run(plist,prov,label):
    shares=[];shares_cfg=[];zero=0;mm=collections.Counter();tot=0;rows=[]
    for tfn,cn in plist:
        d=cfn[cn]; ro={p.split("/")[-1] for p in d.get("readOnlyProperties",[])}
        w=[k for k in d["properties"] if k not in ro]
        if not w: continue
        a=tfattrs(prov[tfn]); hit=0;hitc=0
        for k in w:
            x=a.get(nm(k))
            if x:
                hit+=1
                if x[0]=="opt": hitc+=1
                ct=cfn_t(d["properties"][k],d); tt=classify(ct,x[1])
                tot+=1
                if ct in("string","number","boolean","integer","array","object"):
                    ok = (ct==tt) or (ct=="integer" and tt=="number")
                    mm["match" if ok else "mismatch"]+=1
        shares.append(hit/len(w)); shares_cfg.append(hitc/len(w)); rows.append((tfn,cn,len(w),hit))
    q=statistics.quantiles(shares,n=4)
    print(label,"pairs",len(shares),"median %.3f q1 %.3f q3 %.3f mean %.3f"%(statistics.median(shares),q[0],q[2],statistics.mean(shares)),"| configurable-only median %.3f"%statistics.median(shares_cfg),"| zero-share pairs",sum(1 for s in shares if s==0), "| pairs>=0.9", sum(1 for s in shares if s>=0.9),"| <0.5",sum(1 for s in shares if s<0.5))
    print(" type compare (cheap, coarse)",dict(mm))
    return rows
r=run([tuple(p) for p in pairs["aws"]],aws,"aws")
r2=run([tuple(p) for p in pairs["cc"]],awscc,"awscc")
# examples of low share aws
r.sort(key=lambda x:x[3]/x[2])
print("lowest",[(a,c,w,h) for a,c,w,h in r[:8]])
# top-level only caveat: nested blocks not compared
# ec2 and s3 breakdown
for svc in ["S3","EC2","IAM","Lambda","RDS"]:
    rs=[h/w for a,c,w,h in r if c.split("::")[1]==svc]
    if rs: print(svc,len(rs),"median %.2f"%statistics.median(rs))
