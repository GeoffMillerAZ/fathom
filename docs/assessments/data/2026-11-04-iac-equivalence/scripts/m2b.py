import sys,difflib; sys.path.insert(0,".")
from load import *
s=json.load(open("./measure/m2.json"))["aws_miss_sample"]
keys={t:"".join(t.split("::")[1:]).lower() for t in cfn}
res={t:"".join(t.split("::")[2:]).lower() for t in cfn}
for n in s:
    k=n[4:].replace("_","")
    c=sorted(keys, key=lambda t: -max(difflib.SequenceMatcher(None,k,keys[t]).ratio(),difflib.SequenceMatcher(None,k,res[t]).ratio()))[:3]
    print(n,"|",", ".join(c))
