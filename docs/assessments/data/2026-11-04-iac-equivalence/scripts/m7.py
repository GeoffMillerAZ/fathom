import sys; sys.path.insert(0,".")
from load import *
top=0;toptypes=set();alln=0;alltypes=set();tp=0
def walk(o,tn,top_level):
    global alln
    if isinstance(o,dict):
        if "default" in o and not isinstance(o.get("default"),dict) or ("default" in o and isinstance(o["default"],dict) and False):
            pass
        for k,v in o.items():
            walk(v,tn,False)
for tn,d in cfn.items():
    for pk,p in d["properties"].items():
        tp+=1
        if isinstance(p,dict) and "default" in p: top+=1; toptypes.add(tn)
    # all: any dict with a "default" key whose parent is a schema node (approximation: count dict nodes containing key default and a type or $ref sibling)
    def w(o):
        global alln
        if isinstance(o,dict):
            if "default" in o and ("type" in o or "$ref" in o or "enum" in o): alln+=1; alltypes.add(tn)
            for v in o.values(): w(v)
        elif isinstance(o,list):
            for v in o: w(v)
    w(d.get("properties",{})); w(d.get("definitions",{}))
print("types",len(cfn),"total top-level props",tp)
print("top-level props with default",top,"in types",len(toptypes))
print("all-depth nodes with default",alln,"in types",len(alltypes))
print("types with createOnlyProperties",sum(1 for d in cfn.values() if d.get("createOnlyProperties")),"key present",sum(1 for d in cfn.values() if "createOnlyProperties" in d))
print("types with readOnlyProperties",sum(1 for d in cfn.values() if "readOnlyProperties" in d), "writeOnly",sum(1 for d in cfn.values() if "writeOnlyProperties" in d),"conditionalCreateOnly",sum(1 for d in cfn.values() if "conditionalCreateOnlyProperties" in d),"deprecated",sum(1 for d in cfn.values() if "deprecatedProperties" in d))
print("types with no handlers key",sum(1 for d in cfn.values() if "handlers" not in d))
# createOnly count total props
c=sum(len(d.get("createOnlyProperties",[])) for d in cfn.values()); print("total createOnly paths",c, "nested(>2 segments)",sum(1 for d in cfn.values() for p in d.get("createOnlyProperties",[]) if p.count("/")>2))
print("replacementStrategy values",collections.Counter(d.get("replacementStrategy") for d in cfn.values() if "replacementStrategy" in d))
# sample defaults
ex=[(tn,pk,p["default"]) for tn,d in cfn.items() for pk,p in d["properties"].items() if isinstance(p,dict) and "default" in p][:6]; print(ex)
