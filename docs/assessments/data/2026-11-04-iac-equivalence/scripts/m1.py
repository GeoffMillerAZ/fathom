import sys; sys.path.insert(0,".")
from load import *
print("cfn types",len(cfn))
pre=collections.Counter(t.split("::")[0] for t in cfn); print(pre)
print("keys present counts")
kc=collections.Counter(k for d in cfn.values() for k in d); print(kc.most_common(40))
