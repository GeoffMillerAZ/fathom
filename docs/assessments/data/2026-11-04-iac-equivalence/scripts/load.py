import json,glob,os,re,sys,collections
base="./measure"
cfn={}
for f in glob.glob(base+"/cfn/x/*.json"):
    d=json.load(open(f)); cfn[d["typeName"]]=d
tf=json.load(open(base+"/tf/schema.json"))["provider_schemas"]
aws=tf["registry.opentofu.org/hashicorp/aws"]["resource_schemas"]
awscc=tf["registry.opentofu.org/hashicorp/awscc"]["resource_schemas"]
