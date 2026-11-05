import sys,re,random,subprocess; sys.path.insert(0,".")
exec(open("./m6.py").read().split("tfnames=set")[0])
SRC="./measure/tpa/internal/service"
def findfile(n):
    r=subprocess.run(["grep","-rlE",r'@(SDK|Framework)Resource\("%s"'%n,SRC,"--include=*.go"],capture_output=True,text=True).stdout.split()
    r=[x for x in r if not x.endswith("_test.go")]
    return r[0] if r else None
pairs_l=[tuple(p) for p in pairs["aws"]]
random.seed(20261104)
random.shuffle(pairs_l)
sample=[]
for tfn,cn in pairs_l:
    f=findfile(tfn)
    if f: sample.append((tfn,cn,f))
    if len(sample)==30: break
def tf_forcenew(src,attr):
    lines=src.split("\n")
    # find lines declaring "attr": {   or "attr": schema.XAttribute{
    pat=re.compile(r'^(\s*)"%s":\s*(?:schema\.\w+|\w+\.\w+Attribute|\w+\.\w+Block|\w+)?\s*\{\s*$'%re.escape(attr)) 
    cands=[(i,len(m.group(1))) for i,l in enumerate(lines) for m in [pat.match(l.replace("\t","\t"))] if m]
    if not cands: return None
    i,ind=min(cands,key=lambda x:(x[1],x[0]))
    # scan to closing brace at same indent
    body=[]
    for j in range(i+1,min(i+400,len(lines))):
        l=lines[j]
        if re.match(r'^\s{0,%d}\},?\s*$'%ind,l) and (len(l)-len(l.lstrip()))==ind: break
        body.append(l)
    b="\n".join(body)
    return bool(re.search(r'ForceNew:\s*true',b) or "RequiresReplace" in b)
tab=collections.Counter(); rows=[]; dis=[]; unres=0; per=[]
for tfn,cn,f in sample:
    src=open(f).read()
    d=cfn[cn]; ro={p.split("/")[-1] for p in d.get("readOnlyProperties",[])}
    co={p[len("/properties/"):] for p in d.get("createOnlyProperties",[]) if p.count("/")==2}
    a=tfattrs(aws[tfn]); ag=0;n=0
    for k in d["properties"]:
        if k in ro or nm(k) not in a: continue
        tfk=a[nm(k)][2]
        fn=tf_forcenew(src,tfk)
        if fn is None: unres+=1; continue
        c=k in co
        tab[(c,fn)]+=1; n+=1
        if c==fn: ag+=1
        else: dis.append((tfn,cn,k,tfk,"CFN createOnly" if c else "CFN updatable","TF ForceNew" if fn else "TF in-place"))
    per.append((tfn,cn,n,ag))
print("sample",[(t,c.split("::",1)[1],f.split("/service/")[1]) for t,c,f in sample])
print("per-resource (tf,cfn,attrs compared,agree)",per)
print("table (cfnCreateOnly,tfForceNew)->n",dict(tab),"unresolved attrs",unres)
tot=sum(tab.values()); print("compared",tot,"agree",tab[(True,True)]+tab[(False,False)],"share %.3f"%((tab[(True,True)]+tab[(False,False)])/tot))
import random as r2
r2.seed(1)
bykind=collections.defaultdict(list)
for x in dis: bykind[(x[4],x[5])].append(x)
for k,v in bykind.items(): print(k,len(v))
for x in dis[:40]: print(x)
json.dump({"sample":[(t,c,f) for t,c,f in sample],"table":{str(k):v for k,v in tab.items()},"disagreements":dis,"unresolved":unres},open("./measure/m5.json","w"),indent=1)
