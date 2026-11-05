import sys; sys.path.insert(0,".")
exec(open("./m4.py").read().split("def run")[0])
c=collections.Counter()
for tfn,cn in pairs["aws"]:
    d=cfn[cn]; ro={p.split("/")[-1] for p in d.get("readOnlyProperties",[])}
    a=tfattrs(aws[tfn])
    for k,p in d["properties"].items():
        if k in ro: continue
        x=a.get(nm(k))
        if not x: continue
        ct=cfn_t(p,d); tt=classify(ct,x[1])
        if ct in("string","number","boolean","integer","array","object") and not (ct==tt or (ct=="integer" and tt=="number")): c[(ct,("block-"+x[1][6:]) if isinstance(x[1],str) else tt,k=="Tags")]+=1
print(c.most_common(12))
