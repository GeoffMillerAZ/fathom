package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	cfnYAMLTemplate = "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Bucket:\n    Type: AWS::S3::Bucket\n"
	cfnSAMTemplate  = "Transform: AWS::Serverless-2016-10-31\nResources:\n  Fn:\n    Type: AWS::Serverless::Function\n"
	cfnJSONTemplate = "{\n  \"AWSTemplateFormatVersion\": \"2010-09-09\",\n  \"Resources\": {\n    \"Bucket\": { \"Type\": \"AWS::S3::Bucket\" }\n  }\n}\n"
)

// TestCloudFormationRoutingLeavesOthersUnchanged (LB-Q-cloudformation): a
// template is routed by its content — YAML through classifyYAML's last
// signal, JSON through resolveExtractor's sniff and decode — and every other
// file keeps the extractor and tool kind it had: a Kubernetes manifest and a
// CRD, a compose file by content and by name, a GitHub Actions workflow, a
// GitLab CI pipeline by name and by path, an OpenAPI document, unclassified
// YAML, generic JSON, a JSON document that only mentions a template's key,
// and package.json (its basename wins even over a template's content).
func TestCloudFormationRoutingLeavesOthersUnchanged(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	idx := buildSiblingIndex(nil)
	cfn := p.toolExts[ToolCloudFormation]
	if cfn == nil {
		t.Fatal("no cloudformation extractor registered")
	}
	if _, ok := cfn.(extractor.DocResulter); !ok {
		t.Fatalf("cloudformation dispatches to %T, want the doc-engine dialect", cfn)
	}
	for _, c := range []struct {
		rel, src string
		want     extractor.Extractor
		tool     ToolKind
	}{
		{"infra/stack.yaml", cfnYAMLTemplate, cfn, ToolCloudFormation},
		{"infra/stack.yml", cfnSAMTemplate, cfn, ToolCloudFormation},
		{"infra/stack.json", cfnJSONTemplate, cfn, ToolCloudFormation},
		{"k8s/deploy.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n", p.toolExts[ToolK8sManifest], ToolK8sManifest},
		{"k8s/crd.yaml", "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: x\n", p.toolExts[ToolK8sCRD], ToolK8sCRD},
		{"stack/app.yml", "services:\n  web:\n    image: nginx\n", p.toolExts[ToolCompose], ToolCompose},
		{"docker-compose.yml", "services:\n  web:\n    image: nginx\n", p.exts["docker-compose.yml"], ToolCompose},
		{".github/workflows/ci.yml", "on: push\njobs:\n  b:\n    runs-on: x\n", p.toolExts[ToolGHA], ToolGHA},
		{".gitlab-ci.yml", "build:\n  script: [make]\n", p.toolExts[ToolGitLabCI], ToolGitLabCI},
		{".gitlab/ci/deploy.yml", "deploy:\n  script: [make]\n", p.toolExts[ToolGitLabCI], ToolGitLabCI},
		{"api/openapi.yaml", "openapi: 3.0.0\npaths: {}\n", p.toolExts[ToolOpenAPI], ToolOpenAPI},
		{"data/data.json", "{\"a\": 1}\n", p.exts[".json"], ""},
		{"schema/cfn-schema.json", "{\"properties\": {\"AWSTemplateFormatVersion\": {\"type\": \"string\"}}}\n", p.exts[".json"], ""},
		{"package.json", "{\"name\": \"x\", \"AWSTemplateFormatVersion\": \"2010-09-09\", \"Resources\": {}}\n", p.exts["package.json"], ""},
	} {
		e, tool, ok, unclassified := p.resolveExtractor(c.rel, []byte(c.src), idx)
		if !ok || unclassified {
			t.Errorf("%s: ok=%v unclassified=%v; want an extractor", c.rel, ok, unclassified)
			continue
		}
		if e != c.want || tool != c.tool {
			t.Errorf("%s: dispatches to %T (%q), want %T (%q)", c.rel, e, tool, c.want, c.tool)
		}
	}
	if _, tool, ok, unclassified := p.resolveExtractor("conf/app.yaml", []byte("name: x\nsettings:\n  a: 1\n"), idx); ok || !unclassified || tool != ToolUnclassified {
		t.Errorf("unclassified YAML: ok=%v unclassified=%v tool=%q; want unclassified as before", ok, unclassified, tool)
	}
}

// TestResolveDocXrefs_CloudFormation pins LB-Q-cloudformation's L3 shape: a
// nested stack's TemplateURL and a serverless application's Location that
// name a template of the repository by a relative path resolve, entity to
// entity (the Stack resource to the child template's root entity); an S3
// URL, one built with !Sub, a SAR application and a TemplateURL property on
// a resource that is not a Stack resolve to nothing.
func TestResolveDocXrefs_CloudFormation(t *testing.T) {
	files := docFiles(t, "cloudformation", map[string]string{
		"infra/main.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  Network:\n    Type: AWS::CloudFormation::Stack\n    Properties:\n      TemplateURL: ./network/vpc.yaml\n" +
			"  Data:\n    Type: AWS::CloudFormation::Stack\n    Properties:\n      TemplateURL: ../shared/data.json\n" +
			"  Hosted:\n    Type: AWS::CloudFormation::Stack\n    Properties:\n      TemplateURL: https://s3.amazonaws.com/b/t.yaml\n" +
			"  Built:\n    Type: AWS::CloudFormation::Stack\n    Properties:\n      TemplateURL: !Sub 'https://${Bucket}.s3.amazonaws.com/t.yaml'\n" +
			"  App:\n    Type: AWS::Serverless::Application\n    Properties:\n      Location: ./apps/app.yaml\n" +
			"  Sar:\n    Type: AWS::Serverless::Application\n    Properties:\n      Location:\n        ApplicationId: arn:aws:serverlessrepo:x\n        SemanticVersion: 1.0.0\n" +
			"  NotAStack:\n    Type: AWS::S3::Bucket\n    Properties:\n      TemplateURL: ./network/vpc.yaml\n",
		"infra/network/vpc.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Vpc:\n    Type: AWS::EC2::VPC\n",
		"shared/data.json":       "{\"AWSTemplateFormatVersion\": \"2010-09-09\", \"Resources\": {\"Table\": {\"Type\": \"AWS::DynamoDB::Table\"}}}\n",
		"infra/apps/app.yaml":    "Transform: AWS::Serverless-2016-10-31\nResources:\n  Fn:\n    Type: AWS::Serverless::Function\n",
	})
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"infra/main.yaml:App -nested_application-> infra/apps/app.yaml:app.yaml",
		"infra/main.yaml:Data -nested_stack-> shared/data.json:data.json",
		"infra/main.yaml:Network -nested_stack-> infra/network/vpc.yaml:vpc.yaml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func cfnRepoCorpus() map[string]string {
	return map[string]string{
		"infra/main.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nParameters:\n  Env:\n    Type: String\nConditions:\n  IsProd: !Equals [!Ref Env, prod]\n" +
			"Resources:\n  Network:\n    Type: AWS::CloudFormation::Stack\n    Condition: IsProd\n    Properties:\n      TemplateURL: ./network/vpc.yaml\n" +
			"  Bucket:\n    Type: AWS::S3::Bucket\n    DependsOn: Network\n    Properties:\n      BucketName: !Sub '${Env}-b'\n      Tags:\n        - Key: vpc\n          Value: !GetAtt Network.Outputs.VpcId\n" +
			"Outputs:\n  BucketName:\n    Value: !Ref Bucket\n",
		"infra/network/vpc.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Vpc:\n    Type: AWS::EC2::VPC\nOutputs:\n  VpcId:\n    Value: !Ref Vpc\n",
		"infra/legacy.json":      "{\n  \"AWSTemplateFormatVersion\": \"2010-09-09\",\n  \"Resources\": {\n    \"Queue\": { \"Type\": \"AWS::SQS::Queue\" },\n    \"Topic\": { \"Type\": \"AWS::SNS::Topic\", \"DependsOn\": [\"Queue\"] }\n  }\n}\n",
		"k8s/deploy.yaml":        "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
		"conf/settings.json":     "{\"Resources\": 1}\n",
	}
}

// TestIngest_CloudFormationRepo (the product-reach proof): an ingest reads
// every YAML and JSON template through the cloudformation dialect — the
// template, its parameters, conditions, resources and outputs, with the
// references a resource or output makes to the template's own entities on
// Calls — stores the nested stack whose TemplateURL names a template of the
// repository as a file-tier edge to that template, and leaves a Kubernetes
// manifest and other JSON to the extractors they had.
func TestIngest_CloudFormationRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cfnRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "cf", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cf", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	sawK8s, sawJSONKey := false, false
	for _, o := range obs {
		switch o.SourceKind {
		case "CFNTemplate", "CFNParameter", "CFNCondition", "CFNResource", "CFNOutput", "CFNMapping":
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
		"infra/main.yaml|CFNTemplate|main.yaml":       nil,
		"infra/main.yaml|CFNParameter|Env":            nil,
		"infra/main.yaml|CFNCondition|IsProd":         nil,
		"infra/main.yaml|CFNResource|Network":         {"IsProd"},
		"infra/main.yaml|CFNResource|Bucket":          {"Network"}, // the Sub is not a claimed reference
		"infra/main.yaml|CFNOutput|BucketName":        {"Bucket"},
		"infra/network/vpc.yaml|CFNTemplate|vpc.yaml": nil,
		"infra/network/vpc.yaml|CFNResource|Vpc":      nil,
		"infra/network/vpc.yaml|CFNOutput|VpcId":      {"Vpc"},
		"infra/legacy.json|CFNTemplate|legacy.json":   nil,
		"infra/legacy.json|CFNResource|Queue":         nil,
		"infra/legacy.json|CFNResource|Topic":         {"Queue"},
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
	all, _ := st.TypedEdges(ctx, "cf")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	if want := []string{"infra/main.yaml->cf::infra/network/vpc.vpc.yaml"}; !reflect.DeepEqual(edges, want) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, want)
	}
	if rep.DocXrefEdges != 1 {
		t.Errorf("DocXrefEdges = %d, want 1", rep.DocXrefEdges)
	}
}

// TestReingestFile_CloudFormationRepo: the one-file refresh reads a YAML and
// a JSON template through the dialect, as an ingest does (their content, not
// their extension, decides).
func TestReingestFile_CloudFormationRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cfnRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cf", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{
		"infra/network/vpc.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Vpc:\n    Type: AWS::EC2::VPC\n  Subnet:\n    Type: AWS::EC2::Subnet\n    Properties:\n      VpcId: !Ref Vpc\n",
		"infra/legacy.json":      "{\"AWSTemplateFormatVersion\": \"2010-09-09\", \"Resources\": {\"Queue\": {\"Type\": \"AWS::SQS::Queue\"}, \"Dlq\": {\"Type\": \"AWS::SQS::Queue\"}}}\n",
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "cf", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err := st.QueryObservations(ctx, "cf", "")
	if err != nil {
		t.Fatal(err)
	}
	res := map[string][]string{}
	for _, o := range obs {
		if o.SourceKind == "CFNResource" {
			res[o.SourceRef] = append(res[o.SourceRef], o.Symbol)
		}
	}
	for rel := range res {
		sort.Strings(res[rel])
	}
	want := map[string][]string{
		"infra/main.yaml":        {"Bucket", "Network"},
		"infra/network/vpc.yaml": {"Subnet", "Vpc"},
		"infra/legacy.json":      {"Dlq", "Queue"},
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("resources after ReingestFile\n got: %v\nwant: %v", res, want)
	}
}
