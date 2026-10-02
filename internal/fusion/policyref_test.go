package fusion

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func polNode(kind core.SourceKind, name, sig string) core.SymbolNode {
	return core.SymbolNode{Service: "s", Name: name, Kind: kind, Signature: sig}
}

func polRef(from, strategy, raw, target string) extractor.DocCrossFileRef {
	return extractor.DocCrossFileRef{From: from, Edge: PolicyRefEdge, Strategy: strategy, Raw: raw, Target: target}
}

// policyTargetFiles are the declared targets the producers below join:
// Kubernetes objects (k8sextract's names, the apiVersion as Signature, their
// metadata.labels beside them), a CloudFormation stack and a Terraform
// module with physical names, and a doc entity declared once and one
// declared twice.
func policyTargetFiles() []DocFile {
	withPkg := func(path string, nodes ...core.SymbolNode) []core.SymbolNode {
		for i := range nodes {
			nodes[i].Package = path
			nodes[i].FilePath = path
		}
		return nodes
	}
	return []DocFile{
		{Path: "deploy/app.yaml", Result: extractor.DocResult{
			Nodes: withPkg("deploy/app",
				polNode(core.KindK8sWorkload, "Deployment/shop/web", "apps/v1"),
				polNode(core.KindK8sWorkload, "Deployment/api", "apps/v1"),
				polNode(core.KindK8sWorkload, "Service/shop/web", "v1"),
				polNode(core.KindK8sWorkload, "Gateway/shop/gw", "gateway.networking.k8s.io/v1"),
				polNode(core.KindK8sWorkload, "Gateway/mesh/ig", "networking.istio.io/v1"),
			),
			Labels: []extractor.DocLabelSet{
				{Entity: "Deployment/shop/web", Scope: "shop", Labels: map[string]string{"app": "web", "tier": "front"}},
				{Entity: "Service/shop/web", Scope: "shop", Labels: map[string]string{"app": "web"}},
			},
		}},
		{Path: "infra/stack.yaml", Result: extractor.DocResult{
			Nodes: withPkg("infra/stack",
				polNode("CFNResource", "Bucket", "AWS::S3::Bucket"),
				polNode("CFNResource", "Table", "AWS::DynamoDB::Table"),
				polNode("CFNParameter", "Env", "String"),
			),
			Labels: []extractor.DocLabelSet{{Entity: "Bucket", Scope: PolicyPhysicalNameScope, Labels: map[string]string{"name": "orders-bucket"}}},
		}},
		{Path: "tf/main.tf", Result: extractor.DocResult{
			Nodes:  withPkg("tf/main", polNode(core.KindTerraformResource, "aws_s3_bucket.logs", `resource "aws_s3_bucket" "logs"`)),
			Labels: []extractor.DocLabelSet{{Entity: "aws_s3_bucket.logs", Scope: PolicyPhysicalNameScope, Labels: map[string]string{"name": "logs-bucket"}}},
		}},
		{Path: "shared/a.yaml", Result: extractor.DocResult{Nodes: withPkg("shared/a", polNode("KongService", "unique-config", ""), polNode("KongService", "twice", ""))}},
		{Path: "shared/b.yaml", Result: extractor.DocResult{Nodes: withPkg("shared/b", polNode("KongService", "twice", ""))}},
	}
}

// TestResolvePolicyRefs (LB-F135): every strategy joins what it names and
// every reason is counted, from producers written in each item's emission
// shape — a Kyverno rule and a Gatekeeper constraint (resource-kind,
// label-selector), an IAM statement and a Custodian policy (aws-arn,
// resource-type, the cloud item), a Rego rule (resource-kind and
// service-name, the native item), an ARM-style variable.
func TestResolvePolicyRefs(t *testing.T) {
	files := policyTargetFiles()
	files = append(files,
		DocFile{Path: "policies/kyverno.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("KyvernoRule", "r", "")},
			CrossFile: []extractor.DocCrossFileRef{
				polRef("r", PolicyResourceKind, "apps/v1/Deployment", "apps/Deployment"),
				polRef("r", PolicyResourceKind, "gateway.networking.k8s.io/v1/Gateway", "gateway.networking.k8s.io/Gateway"),
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyResourceKind, Raw: "Deployment", Target: "Deployment", Scope: "sh*"},
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyResourceKind, Raw: "Deployment", Target: "Deployment", Scope: "team-*"},
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyResourceKind, Raw: "Service", Target: "Service", Labels: map[string]string{"app": "web"}},
				polRef("r", PolicyResourceKind, "widgets", ""),
				polRef("r", PolicyResourceKind, "*", "*"),
				polRef("r", PolicyResourceKind, "CronJob", "CronJob"),
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyLabelSelector, Raw: "tier=front", Target: "tier=front", Labels: map[string]string{"tier": "front"}},
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyLabelSelector, Raw: "", Target: ""},
				{From: "r", Edge: PolicyRefEdge, Strategy: PolicyLabelSelector, Raw: "app={{request.object.app}}", Labels: map[string]string{"app": "{{request.object.app}}"}},
				{From: "missing-entity", Edge: PolicyRefEdge, Strategy: PolicyResourceKind, Raw: "Deployment", Target: "Deployment"},
				{From: "r", Edge: "exception_of", Strategy: "service-name", Raw: "twice"},
			},
		}},
		DocFile{Path: "gatekeeper/constraint.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("GatekeeperConstraint", "K8sRequiredLabels/c", "")},
			CrossFile: []extractor.DocCrossFileRef{
				{From: "K8sRequiredLabels/c", Edge: PolicyRefEdge, Strategy: PolicyLabelSelector, Raw: "app=web", Target: "app=web", Scope: "shop", Labels: map[string]string{"app": "web"}},
			},
		}},
		DocFile{Path: "iam/policy.json", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("IAMStatement", "AllowS3", "")},
			CrossFile: []extractor.DocCrossFileRef{
				polRef("AllowS3", PolicyAWSARN, "arn:aws:s3:::orders-bucket/*", "arn:aws:s3:::orders-bucket/*"),
				polRef("AllowS3", PolicyAWSARN, "arn:aws:s3:::logs-bucket", ""),
				polRef("AllowS3", PolicyAWSARN, "arn:aws:s3:::other-bucket", "arn:aws:s3:::other-bucket"),
				polRef("AllowS3", PolicyAWSARN, "arn:aws:s3:::*", "arn:aws:s3:::*"),
				polRef("AllowS3", PolicyAWSARN, "arn:aws:ec2:us-east-1:111122223333:instance/i-0", "arn:aws:ec2:us-east-1:111122223333:instance/i-0"),
				polRef("AllowS3", PolicyAWSARN, "arn:aws:s3:::${BucketName}", "arn:aws:s3:::${BucketName}"),
				polRef("AllowS3", PolicyResourceType, "AWS::DynamoDB::Table", "AWS::DynamoDB::Table"),
			},
		}},
		DocFile{Path: "custodian/policies.yml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("CustodianPolicy", "s3-encryption", "")},
			CrossFile: []extractor.DocCrossFileRef{
				polRef("s3-encryption", PolicyResourceType, "aws.s3", "aws_s3_bucket"),
				polRef("s3-encryption", PolicyResourceType, "aws.sqs", "AWS::SQS::Queue"),
				polRef("s3-encryption", PolicyResourceType, "aws.glue-catalog", ""),
				polRef("s3-encryption", PolicyResourceType, "AWS::S3::*", "AWS::S3::*"),
			},
		}},
		DocFile{Path: "azure/policy.json", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("AzurePolicyDefinition", "deny-public", "")},
			CrossFile: []extractor.DocCrossFileRef{
				polRef("deny-public", PolicyResourceType, "[parameters('resourceType')]", "[parameters('resourceType')]"),
			},
		}},
		DocFile{Path: "rego/deny.rego", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{polNode("RegoRule", "deny", "")},
			CrossFile: []extractor.DocCrossFileRef{
				polRef("deny", PolicyResourceKind, "Deployment", "Deployment"),
				polRef("deny", PolicyServiceName, "unique-config", ""),
				polRef("deny", PolicyServiceName, "twice", "twice"),
				polRef("deny", PolicyServiceName, "nowhere", "nowhere"),
			},
		}},
	)
	res := ResolvePolicyRefs(files)
	var got []string
	for _, r := range res.Refs {
		got = append(got, fmt.Sprintf("%s|%s -%s-> %s|%s", r.FromFile, r.From, r.Strategy, r.ToFile, r.To))
	}
	want := []string{
		"custodian/policies.yml|s3-encryption -resource-type-> tf/main.tf|aws_s3_bucket.logs",
		"gatekeeper/constraint.yaml|K8sRequiredLabels/c -label-selector-> deploy/app.yaml|Deployment/shop/web",
		"gatekeeper/constraint.yaml|K8sRequiredLabels/c -label-selector-> deploy/app.yaml|Service/shop/web",
		"iam/policy.json|AllowS3 -aws-arn-> infra/stack.yaml|Bucket",
		"iam/policy.json|AllowS3 -aws-arn-> tf/main.tf|aws_s3_bucket.logs",
		"iam/policy.json|AllowS3 -resource-type-> infra/stack.yaml|Table",
		"policies/kyverno.yaml|r -resource-kind-> deploy/app.yaml|Deployment/api",
		"policies/kyverno.yaml|r -resource-kind-> deploy/app.yaml|Deployment/shop/web",
		"policies/kyverno.yaml|r -resource-kind-> deploy/app.yaml|Deployment/shop/web",
		"policies/kyverno.yaml|r -resource-kind-> deploy/app.yaml|Gateway/shop/gw",
		"policies/kyverno.yaml|r -resource-kind-> deploy/app.yaml|Service/shop/web",
		"policies/kyverno.yaml|r -label-selector-> deploy/app.yaml|Deployment/shop/web",
		"rego/deny.rego|deny -resource-kind-> deploy/app.yaml|Deployment/api",
		"rego/deny.rego|deny -resource-kind-> deploy/app.yaml|Deployment/shop/web",
		"rego/deny.rego|deny -service-name-> shared/a.yaml|unique-config",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("refs\n got: %q\nwant: %q", got, want)
	}
	var gotU []string
	for _, u := range res.Unresolved {
		gotU = append(gotU, u.From+" "+u.Raw+" "+u.Reason)
	}
	wantU := []string{
		"AllowS3 arn:aws:ec2:us-east-1:111122223333:instance/i-0 unmapped",
		"AllowS3 arn:aws:s3:::${BucketName} variable",
		"AllowS3 arn:aws:s3:::* wildcard",
		"AllowS3 arn:aws:s3:::other-bucket external",
		"deny nowhere undeclared",
		"deny twice ambiguous",
		"deny-public [parameters('resourceType')] variable",
		"r  wildcard",
		"r * wildcard",
		"r CronJob undeclared",
		"r Deployment undeclared",
		"r app={{request.object.app}} variable",
		"r widgets unmapped",
		"s3-encryption AWS::S3::* wildcard",
		"s3-encryption aws.glue-catalog unmapped",
		"s3-encryption aws.sqs undeclared",
	}
	sort.Strings(gotU)
	if !reflect.DeepEqual(gotU, wantU) {
		t.Errorf("unresolved\n got: %q\nwant: %q", gotU, wantU)
	}
	by := res.UnresolvedByReason()
	for _, reason := range []string{PolicyWildcard, PolicyVariable, PolicyExternal, PolicyUndeclared, PolicyUnmapped, PolicyAmbiguous} {
		if by[reason] == 0 {
			t.Errorf("reason %s never counted (%v)", reason, by)
		}
	}
	// r reaches Deployment/shop/web three ways; the store keeps one edge.
	if n := len(PolicyRefEdges(res.Refs)); n != len(want)-2 {
		t.Errorf("PolicyRefEdges = %d edges, want %d (one per symbol pair)", n, len(want)-2)
	}
}

// TestResolvePolicyRefs_Nearest: a reference joins the targets nearest its
// policy — its own file, else its directory subtree, else the repository —
// drops a join to itself, and refuses more than policyFanOutCap targets
// (fan-out).
func TestResolvePolicyRefs_Nearest(t *testing.T) {
	pods := DocFile{Path: "fixtures/pods.yaml"}
	for i := 0; i <= policyFanOutCap; i++ {
		pods.Result.Nodes = append(pods.Result.Nodes, polNode(core.KindK8sWorkload, fmt.Sprintf("Pod/p%03d", i), "v1"))
	}
	files := []DocFile{
		pods,
		{Path: "team/policy.yaml", Result: extractor.DocResult{
			Nodes:     []core.SymbolNode{polNode("KyvernoRule", "r", "")},
			CrossFile: []extractor.DocCrossFileRef{polRef("r", PolicyResourceKind, "Deployment", "Deployment"), polRef("r", PolicyResourceKind, "Pod", "Pod")},
		}},
		{Path: "team/.kyverno-test/resource.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{polNode(core.KindK8sWorkload, "Deployment/near", "apps/v1")}}},
		{Path: "other/far.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{polNode(core.KindK8sWorkload, "Deployment/far", "apps/v1")}}},
		{Path: "teams/sibling.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{polNode(core.KindK8sWorkload, "Deployment/sibling", "apps/v1")}}},
		{Path: "infra/stack.yaml", Result: extractor.DocResult{
			Nodes: []core.SymbolNode{
				{Service: "s", Package: "infra/stack", Name: "Bucket", Kind: "CFNResource", Signature: "AWS::S3::Bucket"},
				{Service: "s", Package: "infra/stack", Name: "Other", Kind: "CFNResource", Signature: "AWS::S3::Bucket"},
			},
			CrossFile: []extractor.DocCrossFileRef{polRef("Bucket", PolicyResourceType, "AWS::S3::Bucket", "AWS::S3::Bucket")},
		}},
		{Path: "infra2/stack.yaml", Result: extractor.DocResult{Nodes: []core.SymbolNode{{Service: "s", Package: "infra2/stack", Name: "Bucket", Kind: "CFNResource", Signature: "AWS::S3::Bucket"}}}},
	}
	res := ResolvePolicyRefs(files)
	var got []string
	for _, r := range res.Refs {
		got = append(got, r.From+"->"+r.ToFile+"|"+r.To)
	}
	want := []string{"Bucket->infra/stack.yaml|Other", "r->team/.kyverno-test/resource.yaml|Deployment/near"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("refs %q, want %q", got, want)
	}
	if res.SelfDropped != 1 {
		t.Errorf("SelfDropped = %d, want 1", res.SelfDropped)
	}
	if by := res.UnresolvedByReason(); by[PolicyFanOut] != 1 || len(by) != 1 {
		t.Errorf("unresolved %v, want one fan-out", by)
	}
}

// TestPolicyTargetFacts: the facts the policy pass reads beside a target's
// nodes — a Kubernetes object's own metadata.labels, a CloudFormation
// resource's literal physical name, a Terraform resource's literal one
// (an interpolation is none).
func TestPolicyTargetFacts(t *testing.T) {
	k8s := k8sObjectLabels([]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  namespace: shop\n  labels:\n    app: web\nspec:\n  template:\n    metadata:\n      labels:\n        pod: only\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: plain\n"))
	if len(k8s) != 1 || k8s[0].Entity != "Deployment/shop/web" || k8s[0].Scope != "shop" || !reflect.DeepEqual(k8s[0].Labels, map[string]string{"app": "web"}) {
		t.Errorf("k8sObjectLabels = %+v", k8s)
	}
	cfn := cfnPhysicalNames([]byte("Resources:\n  Bucket:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: orders-bucket\n  Named:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: !Sub '${Env}-bucket'\n  Queue:\n    Type: AWS::SQS::Queue\n"))
	if len(cfn) != 1 || cfn[0].Entity != "Bucket" || cfn[0].Labels["name"] != "orders-bucket" || cfn[0].Scope != PolicyPhysicalNameScope {
		t.Errorf("cfnPhysicalNames = %+v", cfn)
	}
	tf := tfPhysicalNames("main.tf", []byte("resource \"aws_s3_bucket\" \"logs\" {\n  bucket = \"logs-bucket\"\n}\nresource \"aws_s3_bucket\" \"env\" {\n  bucket = \"${var.env}-bucket\"\n}\nresource \"aws_vpc\" \"main\" {\n  cidr_block = \"10.0.0.0/16\"\n}\n"))
	if len(tf) != 1 || tf[0].Entity != "aws_s3_bucket.logs" || tf[0].Labels["name"] != "logs-bucket" {
		t.Errorf("tfPhysicalNames = %+v", tf)
	}
}
