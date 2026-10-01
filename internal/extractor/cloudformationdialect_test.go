package extractor

// cloudformationdialect_test.go — goldens for the cloudformation doc-engine
// dialect (LB-Q-cloudformation, no incumbent, design §13.2 steps 10/12/14):
// a template's Parameters, Mappings, Conditions, Resources and Outputs, the
// Ref, GetAtt, DependsOn and Condition references a resource or output makes
// to the template's own entities, and a nested stack's or serverless
// application's local template as a cross-file reference. The same golden
// shape as the taskfile dialect's. A fixture is read as if it sat in
// infra/<name>, so a relative TemplateURL resolves against a directory.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type cfnGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type cfnGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type cfnGolden struct {
	Symbols   []cfnGoldenSymbol `json:"symbols"`
	CrossFile []cfnGoldenCross  `json:"cross_file"`
}

func loadCloudFormationExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("cloudformation")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(cloudformation): %v", err)
	}
	return x
}

func cfnFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "cloudformation", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

// TestCloudFormationDialectGoldens: every fixture's entities, references and
// cross-file references against a hand-written golden.
func TestCloudFormationDialectGoldens(t *testing.T) {
	x := loadCloudFormationExtractor(t)
	for _, fx := range cfnFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			rel := "infra/" + filepath.Base(fx)
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", rel, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := cfnGolden{Symbols: []cfnGoldenSymbol{}, CrossFile: []cfnGoldenCross{}}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != rel {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, cfnGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, cfnGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want cfnGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []cfnGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []cfnGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// TestCloudFormationDetectsByContent: the dialect claims no file by name;
// its detect.content_sniff is what says a .yaml, .yml or .json is a
// template: AWSTemplateFormatVersion, an AWS:: Transform, or a top-level
// Resources map whose Types are Namespace::Service names. Kubernetes,
// Compose, GitHub Actions, GitLab CI, OpenAPI, a Serverless Framework config
// (its Resources nested under `resources:`) and generic JSON are not.
func TestCloudFormationDetectsByContent(t *testing.T) {
	x := loadCloudFormationExtractor(t)
	for name, c := range map[string]struct {
		src  string
		want bool
	}{
		"yaml version":         {"AWSTemplateFormatVersion: '2010-09-09'\nResources: {}\n", true},
		"yaml after a comment": {"# a template\nDescription: x\nAWSTemplateFormatVersion: 2010-09-09\n", true},
		"sam transform":        {"Transform: AWS::Serverless-2016-10-31\nResources:\n  F:\n    Type: AWS::Serverless::Function\n", true},
		"transform list":       {"Transform:\n  - AWS::LanguageExtensions\n  - AWS::Serverless-2016-10-31\nResources: {}\n", true},
		"resources only":       {"Description: d\nResources:\n  B:\n    Type: AWS::S3::Bucket\n", true},
		"third-party type":     {"Resources:\n  C:\n    Type: MongoDB::Atlas::Cluster\n", true},
		"json template":        {"{\n  \"AWSTemplateFormatVersion\": \"2010-09-09\",\n  \"Resources\": {}\n}\n", true},
		"json resources only":  {"{\n\t\"Resources\": {\n\t\t\"B\": { \"Type\": \"AWS::S3::Bucket\" }\n\t}\n}\n", true},
		"minified json":        {"{\"Description\":\"x\",\"AWSTemplateFormatVersion\":\"2010-09-09\",\"Resources\":{}}", true},
		"kubernetes":           {"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\ndata:\n  Type: AWS::S3::Bucket\n", false},
		"compose":              {"services:\n  web:\n    image: nginx\n", false},
		"github actions":       {"on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: aws-actions/configure-aws-credentials@v4\n", false},
		"gitlab ci":            {"stages: [build]\nbuild:\n  script: [make]\n", false},
		"openapi":              {"openapi: 3.0.0\npaths: {}\n", false},
		"serverless framework": {"service: api\nprovider:\n  name: aws\nresources:\n  Resources:\n    Bucket:\n      Type: AWS::S3::Bucket\n", false},
		"generic json":         {"{\"name\": \"x\", \"Type\": \"AWS::S3::Bucket\"}\n", false},
	} {
		if got := x.DetectsContent([]byte(c.src)); got != c.want {
			t.Errorf("%s: DetectsContent = %v, want %v", name, got, c.want)
		}
	}
}
