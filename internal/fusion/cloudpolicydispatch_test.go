package fusion

// cloudpolicydispatch_test.go — wave pc, LB-F138: ingest's claim of AWS IAM
// policy documents, Azure Policy definitions and initiatives, and Cloud
// Custodian policy files, by content (the dialects' detect.content_sniff plus
// a decode check), and the negatives: every file an earlier signal claims
// keeps its extractor and tool kind.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	cpIAMJSON = "{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\"Sid\": \"Read\", \"Effect\": \"Allow\", \"Action\": [\"s3:GetObject\"], \"Resource\": \"arn:aws:s3:::b/*\"},\n    {\"Effect\": \"Deny\", \"Action\": \"s3:*\", \"Resource\": \"*\"}\n  ]\n}\n"
	cpIAMYAML = "Version: 2012-10-17\nStatement:\n  - Effect: Allow\n    Action: sqs:SendMessage\n    Resource: arn:aws:sqs:us-east-1:111122223333:q\n"

	cpAzureDef  = "{\n  \"properties\": {\n    \"displayName\": \"d\",\n    \"parameters\": {\"effect\": {\"type\": \"String\"}},\n    \"policyRule\": {\n      \"if\": {\"field\": \"type\", \"equals\": \"Microsoft.Storage/storageAccounts\"},\n      \"then\": {\"effect\": \"[parameters('effect')]\"}\n    }\n  }\n}\n"
	cpAzureSet  = "{\"properties\": {\"policyDefinitions\": [{\"policyDefinitionReferenceId\": \"a\", \"policyDefinitionId\": \"/providers/Microsoft.Authorization/policyDefinitions/x\"}]}}\n"
	cpAzureBare = "{\n  \"if\": {\"field\": \"type\", \"equals\": \"Microsoft.Web/sites\"},\n  \"then\": {\"effect\": \"deny\"}\n}\n"

	cpCustodian = "policies:\n  - name: stale\n    resource: aws.ec2\n    filters:\n      - type: instance-age\n        days: 30\n    actions:\n      - stop\n"
)

// cloudPolicyCases are the files the three claims read, with the tool kind
// each dispatches to.
var cloudPolicyCases = []struct {
	rel, src string
	tool     ToolKind
}{
	{"iam/policy.json", cpIAMJSON, ToolAWSIAMPolicy},
	{"iam/trust.yaml", cpIAMYAML, ToolAWSIAMPolicy},
	{"iam/role.yml", cpIAMYAML, ToolAWSIAMPolicy},
	{"azure/definition.json", cpAzureDef, ToolAzurePolicy},
	{"azure/initiative.json", cpAzureSet, ToolAzurePolicy},
	{"azure/azurepolicy.rules.json", cpAzureBare, ToolAzurePolicy},
	{"custodian/policy.yml", cpCustodian, ToolCloudCustodian},
	{"custodian/policy.yaml", cpCustodian, ToolCloudCustodian},
}

// TestCloudPolicyDispatchClaimsByContent: each format is dispatched to its
// dialect, by content, with its tool kind (ingest's resolveExtractor, the
// path ExtractForBench shares).
func TestCloudPolicyDispatchClaimsByContent(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	idx := buildSiblingIndex(nil)
	for _, c := range cloudPolicyCases {
		e, tool, ok, unclassified := p.resolveExtractor(c.rel, []byte(c.src), idx)
		if !ok || unclassified {
			t.Errorf("%s: ok=%v unclassified=%v; want a dialect extractor", c.rel, ok, unclassified)
			continue
		}
		if tool != c.tool {
			t.Errorf("%s: tool kind %q, want %q", c.rel, tool, c.tool)
		}
		if _, isDoc := e.(extractor.DocResulter); !isDoc {
			t.Errorf("%s: dispatches to %T, want the doc-engine dialect", c.rel, e)
		}
	}
}

// TestCloudPolicyDispatchLeavesOthersUnchanged: a file a basename or an
// earlier signal claims keeps its verdict, whatever else its content says:
// package.json, a CloudFormation template (inline IAM and all), an OpenAPI
// document, a Kubernetes manifest and a ConfigMap that carries a policy as
// text, an Envoy bootstrap, a Compose file, a GitHub Actions workflow, a
// GitLab CI pipeline, and a generic JSON or YAML file that only names a key.
func TestCloudPolicyDispatchLeavesOthersUnchanged(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	idx := buildSiblingIndex(nil)
	cfnInline := "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  P:\n    Type: AWS::IAM::ManagedPolicy\n    Properties:\n      PolicyDocument:\n        Version: 2012-10-17\n        Statement:\n          - Effect: Allow\n            Action: s3:Get*\n            Resource: '*'\n"
	cfnJSON := "{\n  \"AWSTemplateFormatVersion\": \"2010-09-09\",\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [],\n  \"Resources\": {\"P\": {\"Type\": \"AWS::IAM::ManagedPolicy\"}}\n}\n"
	openapi := "{\"openapi\": \"3.0.0\", \"info\": {\"x-policyRule\": 1}, \"paths\": {}, \"Version\": \"2012-10-17\", \"Statement\": []}\n"
	configMap := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: p\ndata:\n  policy.json: |\n    {\"Version\": \"2012-10-17\", \"Statement\": []}\n  rules.yaml: |\n    policies:\n      - name: a\n        resource: ec2\n"
	k8sCustodian := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: p\npolicies:\n  - name: a\n    resource: ec2\n"
	for _, c := range []struct {
		rel, src string
		want     func() (extractor.Extractor, ToolKind)
	}{
		{"package.json", "{\"name\": \"x\", \"Version\": \"2012-10-17\", \"Statement\": [], \"policyRule\": {}}\n", func() (extractor.Extractor, ToolKind) { return p.exts["package.json"], "" }},
		{"infra/stack.yaml", cfnInline, func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolCloudFormation], ToolCloudFormation }},
		{"infra/stack.json", cfnJSON, func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolCloudFormation], ToolCloudFormation }},
		{"api/openapi.json", openapi, func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolOpenAPI], ToolOpenAPI }},
		{"k8s/cm.yaml", configMap, func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolK8sManifest], ToolK8sManifest }},
		{"k8s/cm2.yaml", k8sCustodian, func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolK8sManifest], ToolK8sManifest }},
		{"stack/app.yml", "services:\n  web:\n    image: nginx\npolicies:\n  - name: a\n    resource: b\n", func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolCompose], ToolCompose }},
		{".github/workflows/ci.yml", "on: push\njobs:\n  b:\n    runs-on: x\npolicies:\n  - name: a\n    resource: b\n", func() (extractor.Extractor, ToolKind) { return p.exts[".yml"], ToolGHA }},
		{".gitlab-ci.yml", "build:\n  script: [make]\npolicies:\n  - name: a\n    resource: b\n", func() (extractor.Extractor, ToolKind) { return p.toolExts[ToolGitLabCI], ToolGitLabCI }},
		{"data/generic.json", "{\"Version\": \"2012-10-17\", \"Statement\": \"not a policy\"}\n", func() (extractor.Extractor, ToolKind) { return p.exts[".json"], "" }},
		{"data/generic2.json", "{\"if\": 1, \"then\": 2, \"policyRule\": \"a string\"}\n", func() (extractor.Extractor, ToolKind) { return p.exts[".json"], "" }},
		{"data/plain.json", "{\"a\": 1}\n", func() (extractor.Extractor, ToolKind) { return p.exts[".json"], "" }},
	} {
		want, wantTool := c.want()
		e, tool, ok, unclassified := p.resolveExtractor(c.rel, []byte(c.src), idx)
		if !ok || unclassified {
			t.Errorf("%s: ok=%v unclassified=%v; want an extractor", c.rel, ok, unclassified)
			continue
		}
		if e != want || tool != wantTool {
			t.Errorf("%s: dispatches to %T (%q), want %T (%q)", c.rel, e, tool, want, wantTool)
		}
	}
	for rel, src := range map[string]string{
		"conf/app.yaml":      "name: x\nsettings:\n  a: 1\n",
		"conf/policies.yaml": "policies:\n  - name: retention\n    days: 30\n",
		"conf/version.yaml":  "Version: 2012-10-17\n",
	} {
		if _, tool, ok, unclassified := p.resolveExtractor(rel, []byte(src), idx); ok || !unclassified || tool != ToolUnclassified {
			t.Errorf("%s: ok=%v unclassified=%v tool=%q; want unclassified as before", rel, ok, unclassified, tool)
		}
	}
	// Envoy keeps its gateway claim (a basename AND its sniff), even with a policies list in it.
	envoy := gwEnvoyYAML + "policies:\n  - name: a\n    resource: b\n"
	if e, tool, ok, _ := p.resolveExtractor("envoy.yaml", []byte(envoy), idx); !ok || tool != ToolEnvoy {
		t.Errorf("envoy.yaml: ok=%v tool=%q (%T), want the envoy dialect", ok, tool, e)
	}
}

// TestCloudPolicyDialectNames: CloudPolicyDialect names the dialect ingest
// reads a file with by content, "" for every other file, with
// Pipeline.ContentDialect's contract (the harness measures on it).
func TestCloudPolicyDialectNames(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	want := map[string]string{
		"iam/policy.json": "aws-iam-policy", "iam/trust.yaml": "aws-iam-policy", "azure/definition.json": "azure-policy",
		"azure/initiative.json": "azure-policy", "azure/azurepolicy.rules.json": "azure-policy", "custodian/policy.yml": "cloud-custodian",
	}
	for _, c := range cloudPolicyCases {
		if w, ok := want[c.rel]; ok {
			if got := p.CloudPolicyDialect(c.rel, []byte(c.src), nil); got != w {
				t.Errorf("%s: CloudPolicyDialect = %q, want %q", c.rel, got, w)
			}
		}
	}
	for rel, src := range map[string]string{"data/plain.json": "{\"a\": 1}\n", "api/openapi.yaml": "openapi: 3.0.0\npaths: {}\n", "x.txt": cpIAMJSON} {
		if got := p.CloudPolicyDialect(rel, []byte(src), nil); got != "" {
			t.Errorf("%s: CloudPolicyDialect = %q, want none", rel, got)
		}
	}
}

func cloudPolicyRepoCorpus() map[string]string {
	return map[string]string{
		"iam/policy.json":              cpIAMJSON,
		"iam/trust.yaml":               cpIAMYAML,
		"azure/definition.json":        cpAzureDef,
		"azure/initiative.json":        cpAzureSet,
		"azure/azurepolicy.rules.json": cpAzureBare,
		"custodian/policy.yml":         cpCustodian,
		"conf/settings.json":           "{\"Version\": 1}\n",
		"k8s/deploy.yaml":              "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
	}
}

// TestIngest_CloudPolicyRepo (the product-reach proof): an ingest reads each
// format through its dialect (statements with their references on Calls, a
// definition's parameter, a policy's filter and action types), and leaves a
// Kubernetes manifest and other JSON to the extractors they had.
func TestIngest_CloudPolicyRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cloudPolicyRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cp", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cp", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	sawK8s, sawJSONKey := false, false
	for _, o := range obs {
		switch o.SourceKind {
		case "IAMPolicy", "IAMStatement", "AzurePolicyDefinition", "AzurePolicyInitiative", "AzurePolicyParameter", "AzurePolicyEffect", "AzurePolicyMember", "CustodianPolicy":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o.Calls
		case core.KindK8sWorkload:
			sawK8s = o.SourceRef == "k8s/deploy.yaml"
		}
		if o.SourceRef == "conf/settings.json" {
			if o.SourceKind != core.KindJSONKey {
				t.Errorf("conf/settings.json read as %s; generic JSON keeps the JSON extractor", o.SourceKind)
			}
			sawJSONKey = true
		}
	}
	want := map[string][]string{
		"iam/policy.json|IAMPolicy|policy.json":                                     nil,
		"iam/policy.json|IAMStatement|Read":                                         {"arn:aws:s3:::b/*", "s3:GetObject"},
		"iam/policy.json|IAMStatement|1":                                            {"*", "s3:*"},
		"iam/trust.yaml|IAMPolicy|trust.yaml":                                       nil,
		"iam/trust.yaml|IAMStatement|0":                                             {"arn:aws:sqs:us-east-1:111122223333:q", "sqs:SendMessage"},
		"azure/definition.json|AzurePolicyDefinition|definition.json":               {"effect"},
		"azure/definition.json|AzurePolicyParameter|effect":                         nil,
		"azure/definition.json|AzurePolicyEffect|[parameters('effect')]":            nil,
		"azure/initiative.json|AzurePolicyInitiative|initiative.json":               nil,
		"azure/initiative.json|AzurePolicyMember|a":                                 nil,
		"azure/azurepolicy.rules.json|AzurePolicyDefinition|azurepolicy.rules.json": nil,
		"azure/azurepolicy.rules.json|AzurePolicyEffect|deny":                       nil,
		"custodian/policy.yml|CustodianPolicy|stale":                                {"instance-age", "stop"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observations\n got: %v\nwant: %v", got, want)
	}
	if !sawK8s {
		t.Error("the Kubernetes manifest was not read by the Kubernetes extractor")
	}
	if !sawJSONKey {
		t.Error("conf/settings.json yielded nothing; generic JSON keeps the JSON extractor")
	}
}

// TestExtractForBench_CloudPolicy: the language-breadth harness's extraction
// (ExtractForBench) dispatches each format as ingest does.
func TestExtractForBench_CloudPolicy(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for _, c := range cloudPolicyCases {
		ex, err := p.ExtractForBench("cp", nil, c.rel, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if ex.Skipped != "" || ex.ToolKind != c.tool || len(ex.Nodes) == 0 {
			t.Errorf("%s: skipped=%q tool=%q nodes=%d, want tool %q with nodes", c.rel, ex.Skipped, ex.ToolKind, len(ex.Nodes), c.tool)
		}
	}
}

// TestReingestFile_CloudPolicy: the one-file refresh makes ingest's decision
// for each format (the hook in ReingestFile).
func TestReingestFile_CloudPolicy(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cloudPolicyRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cp", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{
		"iam/policy.json":       "{\"Version\": \"2012-10-17\", \"Statement\": [{\"Sid\": \"Only\", \"Effect\": \"Allow\", \"Action\": \"sns:Publish\", \"Resource\": \"*\"}]}\n",
		"custodian/policy.yml":  "policies:\n  - name: renamed\n    resource: s3\n",
		"azure/definition.json": strings.Replace(cpAzureDef, "Microsoft.Storage/storageAccounts", "Microsoft.Sql/servers", 1),
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "cp", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err := st.QueryObservations(ctx, "cp", "")
	if err != nil {
		t.Fatal(err)
	}
	res := map[string][]string{}
	for _, o := range obs {
		switch o.SourceKind {
		case "IAMStatement", "CustodianPolicy", "AzurePolicyDefinition":
			res[o.SourceRef] = append(res[o.SourceRef], o.Symbol)
		}
	}
	for rel := range res {
		sort.Strings(res[rel])
	}
	want := map[string][]string{
		"iam/policy.json":              {"Only"},
		"iam/trust.yaml":               {"0"},
		"custodian/policy.yml":         {"renamed"},
		"azure/definition.json":        {"definition.json"},
		"azure/azurepolicy.rules.json": {"azurepolicy.rules.json"},
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("after ReingestFile\n got: %v\nwant: %v", res, want)
	}
}
