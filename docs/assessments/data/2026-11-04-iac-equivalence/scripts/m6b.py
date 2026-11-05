import sys,re; sys.path.insert(0,".")
exec(open("./m6.py").read().split("tfnames=set")[0])
matched={a:c for a,c in pairs["aws"]}
unm=[n for n in aws if n not in matched]
comp=collections.defaultdict(list)
for n in unm:
    best=None
    for m in matched:
        if n.startswith(m+"_") and (best is None or len(m)>len(best)): best=m
    if best: comp[best].append(n)
print("unmatched TF with a matched-parent prefix:",sum(len(v) for v in comp.values()),"of",len(unm),"under",len(comp),"parents")
def show(prefixes):
    for p,v in sorted(comp.items()):
        if any(p.startswith(x) for x in prefixes):
            cn=matched[p]; props=[k for k in cfn[cn]["properties"]]
            hits=[]
            for n in v:
                suf=n[len(p)+1:]; toks=set(suf.split("_"))
                for k in props:
                    ks=set(snake(k).split("_"))
                    if toks<=ks or (suf.replace("_","") in k.lower()): hits.append((n,k))
            print(p,"->",cn,"| companions",len(v),"| companion-with-CFN-inline-prop",len(hits),hits[:6])
show(["aws_s3_","aws_ec2_","aws_vpc","aws_instance","aws_security_group","aws_network_","aws_subnet","aws_route","aws_launch","aws_lb","aws_ebs","aws_eip","aws_internet","aws_nat","aws_default"])
allh=0;allp=0
for p,v in comp.items():
    cn=matched[p]; props=list(cfn[cn]["properties"])
    for n in v:
        allp+=1
        suf=n[len(p)+1:]; toks=set(suf.split("_"))
        if any(toks<=set(snake(k).split("_")) or suf.replace("_","") in k.lower() for k in props): allh+=1
print("all services: companion candidates",allp,"with matching inline CFN prop",allh)
