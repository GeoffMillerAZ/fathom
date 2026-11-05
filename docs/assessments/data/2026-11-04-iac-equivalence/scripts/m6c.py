import sys,re; sys.path.insert(0,".")
exec(open("./m6b.py").read().split("print(\"unmatched TF")[0])
for t,ks in [("AWS::EC2::VPCEndpoint",["RouteTableIds","SubnetIds","SecurityGroupIds"]),("AWS::SQS::Queue",["RedrivePolicy","RedriveAllowPolicy"]),("AWS::EKS::AccessEntry",["AccessPolicies"]),("AWS::Transfer::User",["SshPublicKeys"]),("AWS::EC2::TransitGateway",["DefaultRouteTableAssociation","DefaultRouteTablePropagation"]),("AWS::EC2::NatGateway",["SecondaryAllocationIds"]),("AWS::S3::Bucket",["WebsiteConfiguration","LifecycleConfiguration","VersioningConfiguration","NotificationConfiguration"]),("AWS::EC2::SecurityGroup",["SecurityGroupIngress","SecurityGroupEgress"])]:
    print(t,{k:(k in cfn[t]["properties"]) for k in ks})
for n in ["aws_vpc_endpoint_route_table_association","aws_vpc_endpoint_subnet_association","aws_sqs_queue_redrive_policy","aws_eks_access_policy_association","aws_transfer_ssh_key","aws_ec2_transit_gateway_default_route_table_association","aws_nat_gateway_eip_association","aws_s3_bucket_website_configuration","aws_security_group_rule"]: print(n, n in aws)
matched={a:c for a,c in pairs["aws"]}
parents=set()
for p,v in comp.items():
    cn=matched[p]; props=list(cfn[cn]["properties"])
    for n in v:
        suf=n[len(p)+1:]; toks=set(suf.split("_"))
        if any(toks<=set(snake(k).split("_")) or suf.replace("_","") in k.lower() for k in props): parents.add(cn)
print("distinct CFN parent types in mechanical hits",len(parents), sorted(parents)[:30])
