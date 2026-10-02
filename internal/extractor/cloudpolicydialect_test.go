package extractor

// cloudpolicydialect_test.go — goldens for the three cloud-policy doc-engine
// dialects (wave pc: aws-iam-policy, azure-policy, cloud-custodian; no
// incumbent, design §13.2 steps 10/12/14): each fixture's entities, the
// references a statement, definition or policy makes, and the policy_ref
// cross-file references it emits (LB-F139: edge policy_ref, strategy aws-arn
// or resource-type, Raw as written; fusion's policy-reference resolver says
// what each names). A fixture is read as if it sat in policies/<name>.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var cloudPolicyDialects = []string{"aws-iam-policy", "azure-policy", "cloud-custodian"}

func cloudPolicyFixtures(t *testing.T, id string) []string {
	t.Helper()
	dir := filepath.Join("dialects", id, "testdata")
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

func cloudPolicyActual(t *testing.T, id, fx string) cfnGolden {
	t.Helper()
	x, err := NewDocDialectExtractor(id)
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
	}
	rel := "policies/" + filepath.Base(fx)
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
	return got
}

// TestCloudPolicyDialectGoldens: every fixture of the three dialects against
// its hand-written golden (the golden's cross_file is the policy_ref
// emission, LB-F139).
func TestCloudPolicyDialectGoldens(t *testing.T) {
	for _, id := range cloudPolicyDialects {
		for _, fx := range cloudPolicyFixtures(t, id) {
			t.Run(id+"/"+filepath.Base(fx), func(t *testing.T) {
				got := cloudPolicyActual(t, id, fx)
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
}

func cloudPolicySniff(t *testing.T, id, src string) bool {
	t.Helper()
	x, err := NewDocDialectExtractor(id)
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
	}
	return x.DetectsContent([]byte(src))
}

// TestCloudPolicyDetectsByContent: none of the three claims a file by name;
// each dialect's detect.content_sniff says what it reads. A Kubernetes,
// CloudFormation, OpenAPI or generic document, and a file that only mentions
// a key, is not one of them (fusion's cloudPolicyDispatch adds the decode
// check and the earlier-signal exclusions: cloudpolicydispatch_test.go).
func TestCloudPolicyDetectsByContent(t *testing.T) {
	for _, c := range []struct {
		id, name, src string
		want          bool
	}{
		{"aws-iam-policy", "json 2012", "{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": []\n}", true},
		{"aws-iam-policy", "json 2008, statement first", "{\"Statement\": [], \"Version\" : \"2008-10-17\"}", true},
		{"aws-iam-policy", "yaml", "Version: 2012-10-17\nStatement:\n  - Effect: Allow\n", true},
		{"aws-iam-policy", "yaml quoted", "Statement: []\nVersion: '2008-10-17'\n", true},
		{"aws-iam-policy", "other version", "{\"Version\": \"1.0\", \"Statement\": []}", false},
		{"aws-iam-policy", "no statement", "{\"Version\": \"2012-10-17\"}", false},
		{"aws-iam-policy", "nested yaml version", "Resources:\n  P:\n    Properties:\n      PolicyDocument:\n        Version: 2012-10-17\n        Statement: []\n", false},
		{"azure-policy", "arm definition", "{\"properties\": {\"policyRule\": {}}}", true},
		{"azure-policy", "initiative", "{\"properties\": {\"policyDefinitions\": []}}", true},
		{"azure-policy", "bare rule", "{\"if\": {\"field\": \"type\"},\n \"then\": {\"effect\": \"deny\"}}", true},
		{"azure-policy", "if/then without an effect", "{\"if\": 1, \"then\": 2}", false},
		{"azure-policy", "generic json", "{\"name\": \"x\"}", false},
		{"cloud-custodian", "policies", "policies:\n  - name: a\n    resource: aws.s3\n", true},
		{"cloud-custodian", "flat list", "policies:\n- name: a\n  resource: ec2\n", true},
		{"cloud-custodian", "policies without a resource", "policies:\n  - name: a\n    days: 3\n", false},
		{"cloud-custodian", "nested policies key", "spec:\n  policies:\n    - name: a\n      resource: b\n", false},
		{"cloud-custodian", "kubernetes", "apiVersion: v1\nkind: ConfigMap\n", false},
	} {
		if got := cloudPolicySniff(t, c.id, c.src); got != c.want {
			t.Errorf("%s / %s: DetectsContent = %v, want %v", c.id, c.name, got, c.want)
		}
	}
}

// TestCustodianTypeMapIsTheCommittedRegistryMap: the doc spec's target_map is
// exactly calibration/oracles/custodian/cfn_types.json, the map custodian_oracle.py
// -type-map-out generates from c7n's resource registry (tier A, cfn_type),
// so a Target comes from that registry and nowhere else (LB-F140).
func TestCustodianTypeMapIsTheCommittedRegistryMap(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "calibration", "oracles", "custodian", "cfn_types.json"))
	if err != nil {
		t.Fatal(err)
	}
	var committed map[string]string
	if err := json.Unmarshal(raw, &committed); err != nil {
		t.Fatal(err)
	}
	spec, err := os.ReadFile(filepath.Join("dialects", "cloud-custodian", "doc.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseDocSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range s.CrossFile {
		if c.Resolve != "resource-type" {
			continue
		}
		found = true
		if !reflect.DeepEqual(c.TargetMap, committed) {
			t.Errorf("doc.json target_map (%d) differs from cfn_types.json (%d): rerun custodian_oracle.py -type-map-out ... -doc-json ...", len(c.TargetMap), len(committed))
		}
	}
	if !found || len(committed) < 100 {
		t.Errorf("resource-type rule found %v, committed map %d entries", found, len(committed))
	}
	for _, k := range []string{"aws.s3", "s3", "aws.ec2", "ec2"} {
		if committed[k] == "" {
			t.Errorf("the registry map has no %q", k)
		}
	}
	// The emission: a mapped resource gets the registry's CloudFormation type, a
	// resource c7n's registry has none for (another cloud's, an unknown one) gets "".
	x, err := NewDocDialectExtractor("cloud-custodian")
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.ExtractDoc("svc", "p.yaml", []byte("policies:\n  - {name: a, resource: aws.s3}\n  - {name: b, resource: ec2}\n  - {name: c, resource: azure.vm}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.Raw+"="+c.Target)
	}
	if want := []string{"aws.s3=" + committed["aws.s3"], "ec2=" + committed["ec2"], "azure.vm="}; !reflect.DeepEqual(got, want) {
		t.Errorf("resource-type targets = %q, want %q", got, want)
	}
}
