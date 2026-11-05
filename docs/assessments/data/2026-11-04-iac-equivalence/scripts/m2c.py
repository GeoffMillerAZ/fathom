import sys; sys.path.insert(0,".")
from load import *
d=json.load(open("./measure/m2.json"))
s=d["aws_miss_sample"]; t2={a for a,b in d["tier2"]}
lab={}
N="none";P="split";A="alias"
for n in """ses_domain_identity_verification guardduty_organization_admin_account devicefarm_upload iam_service_specific_credential acmpca_policy opensearch_package_association rds_certificate resiliencehubv2_assertion appfabric_app_authorization_connection lex_slot_type s3control_directory_bucket_access_point_scope osis_resource_policy load_balancer_backend_server_policy directory_service_shared_directory_accepter cloudwatch_contributor_managed_insight_rule ebs_volume_copy ec2_default_credit_specification cloudsearch_domain sns_platform_application neptune_cluster_snapshot observabilityadmin_telemetry_evaluation_for_organization waf_rule_group""".split(): lab["aws_"+n]=N
for n in """route53_resolver_firewall_rule elasticache_user_group_association api_gateway_integration_response transfer_web_app_customization lightsail_bucket_access_key vpc_endpoint_route_table_association vpc_endpoint_subnet_association transfer_ssh_key sqs_queue_redrive_policy eks_access_policy_association""".split(): lab["aws_"+n]=P
for n in """db_proxy_target rds_cluster_parameter_group route_table_association vpc_security_group_ingress_rule sagemaker_endpoint_configuration bedrockagent_data_source iam_group_membership codedeploy_app schemas_registry_policy vpc_route_server_endpoint s3control_storage_lens_configuration db_subnet_group s3control_bucket_policy pinpointsmsvoicev2_configuration_set""".split(): lab["aws_"+n]=A
for n in ["aws_vpn_gateway","aws_nat_gateway","aws_verifiedaccess_trust_provider"]: lab[n]="svcdrop"
miss=[n for n in s if n not in lab]; print("unlabeled",miss)
c=collections.Counter(lab.get(n,"?") for n in s); print(c)
print("svcdrop in t2", [(n, n in t2) for n in s if lab.get(n)=="svcdrop"])
print("tier2 total",len(t2),"of",d["aws_miss_n"])
json.dump({n:lab.get(n) for n in s},open("./measure/m2labels.json","w"),indent=1)
