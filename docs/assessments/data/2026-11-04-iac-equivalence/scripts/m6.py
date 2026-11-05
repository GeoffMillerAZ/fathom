import sys,re; sys.path.insert(0,".")
exec(open("./m4.py").read().split("def run")[0])
def snake(s): return re.sub(r'(?<=[a-z0-9])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])','_',s).lower()
tfnames=set(aws)
found={}
for tfn,cn in pairs["aws"]:
    d=cfn[cn]; ro={p.split("/")[-1] for p in d.get("readOnlyProperties",[])}
    a=tfattrs(aws[tfn])
    for k in d["properties"]:
        if k in ro or nm(k) in a: continue
        sn=snake(k)
        cands=[t for t in tfnames if t!=tfn and (t.startswith(tfn+"_"+sn) or t==tfn+"_"+sn or t.replace("_","")==(tfn+sn).replace("_",""))]
        if cands: found.setdefault(cn,[]).append((k,sorted(cands)[:2]))
print("pairs with >=1 detected companion",len(found),"of",len(pairs["aws"]))
for svc in ("S3","EC2"):
    f={c:v for c,v in found.items() if c.split("::")[1]==svc}
    n=len([1 for t,c in pairs["aws"] if c.split("::")[1]==svc])
    print(svc,"matched pairs",n,"with detected splits",len(f))
    for c,v in list(f.items())[:12]: print(" ",c,v[:3])
print("total (CFN type, property) split edges", sum(len(v) for v in found.values()))
