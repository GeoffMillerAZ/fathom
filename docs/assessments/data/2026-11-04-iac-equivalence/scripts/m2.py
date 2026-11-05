import sys,random; sys.path.insert(0,".")
from load import *
def ncfn(t):
    p=t.split("::"); return "".join(x.lower() for x in p[1:])
def ntf(n,pre): return n[len(pre):].replace("_","")
cfnN=collections.defaultdict(list)
for t in cfn: cfnN[ncfn(t)].append(t)
print("cfn norm collisions",[v for v in cfnN.values() if len(v)>1])
awsN=collections.defaultdict(list)
for n in aws: awsN[ntf(n,"aws_")].append(n)
ccN=collections.defaultdict(list)
for n in awscc: ccN[ntf(n,"awscc_")].append(n)
print("tf aws",len(aws),"awscc",len(awscc),"cfn",len(cfn))
m_aws=[(n,cfnN[k][0]) for k,v in awsN.items() for n in v if k in cfnN]
print("aws->cfn exact",len(m_aws), len(m_aws)/len(aws))
print("cfn types hit by aws", len({c for _,c in m_aws}), len({c for _,c in m_aws})/len(cfn))
m_cc=[(n,cfnN[k][0]) for k,v in ccN.items() for n in v if k in cfnN]
print("awscc->cfn exact",len(m_cc), len(m_cc)/len(awscc))
print("cfn hit by awscc",len({c for _,c in m_cc}),len({c for _,c in m_cc})/len(cfn))
cc_miss=[n for n in awscc if ntf(n,"awscc_") not in cfnN]; print("awscc misses",cc_miss[:20])
cfn_not_cc=[t for t in cfn if ncfn(t) not in ccN]; print("cfn without awscc",len(cfn_not_cc), cfn_not_cc[:8])
# tier 2: drop ec2 on cfn side etc: resource-only match for unmatched
aws_miss=sorted(n for n in aws if ntf(n,"aws_") not in cfnN)
print("aws misses",len(aws_miss))
# tier2: tf name equals cfn resource-only name (unique) or cfn service+resource with service in {ec2}
resN=collections.defaultdict(list)
for t in cfn: resN["".join(t.split("::")[2:]).lower()].append(t)
t2=[]
for n in aws_miss:
    k=ntf(n,"aws_")
    if len(resN.get(k,[]))==1: t2.append((n,resN[k][0]))
print("tier2 resource-only unique",len(t2), t2[:15])
# tier3: TF name startswith cfn service alias? skip
random.seed(20261104)
s=random.sample(aws_miss,50)
json.dump({"aws_miss_sample":s,"aws_miss_n":len(aws_miss),"tier2":t2},open("./measure/m2.json","w"),indent=1)
for n in s: print(n)
json.dump({"aws":[list(x) for x in m_aws],"cc":[list(x) for x in m_cc]},open("./measure/pairs.json","w"))
