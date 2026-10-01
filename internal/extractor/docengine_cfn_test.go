package extractor

// docengine_cfn_test.go — the doc engine's extension for CloudFormation
// (LB-Q-cloudformation, design §4): the opt-in `yaml_tags: "cloudformation"`
// decode (a YAML short-form intrinsic read as its long form), the `..key`
// (recursive descent) and `[N]` (one item) selector steps, a reference's
// `declared_only` and a working `detect.content_sniff`. Every other dialect
// decodes as before: the option is off unless a spec names it.

import (
	"reflect"
	"strings"
	"testing"
)

// cfnEngineSpec reads Parameters, Resources and Conditions and records, per
// resource, what each reference selector reaches (unfiltered unless the
// test turns declared_only on).
const cfnEngineSpec = `{
  "decode": "yaml", "yaml_tags": "cloudformation",
  "detect": { "require_keys": ["Resources"] },
  "entities": [
    { "select": "$.Parameters.*", "where": { "is_map": true }, "kind": "variable_decl", "source_kind": "CFNParameter", "name": "$key" },
    { "select": "$.Resources.*", "where": { "is_map": true }, "kind": "resource", "source_kind": "CFNResource", "name": "$key", "signature": ".Type" },
    { "select": "$.Conditions.*", "kind": "rule", "source_kind": "CFNCondition", "name": "$key" }
  ],
  "references": [
    { "from": "resource", "select": "$..Ref", "value": "$", "to_kind": "resource", "edge": "ref" },
    { "from": "resource", "select": "$..Fn::GetAtt[0]", "value": "$", "to_kind": "resource", "edge": "getatt" },
    { "from": "resource", "select": "$..Fn::If[0]", "value": "$", "to_kind": "rule", "edge": "condition" },
    { "from": "resource", "select": "$..Fn::Sub", "value": "$", "to_kind": "resource", "edge": "sub" },
    { "from": "resource", "select": "$..Fn::Rain::Embed", "value": "$", "to_kind": "resource", "edge": "custom" },
    { "from": "resource", "select": "$.DependsOn[*]", "value": "$", "to_kind": "resource", "edge": "depends_on" }
  ]
}`

const cfnEngineTemplate = `AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
Conditions:
  IsProd: !Equals [!Ref Env, prod]
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: !Sub "${Env}-bucket"
      Tags:
        - Key: env
          Value: !Ref Env
  Role:
    Type: AWS::IAM::Role
    DependsOn: Bucket
    Properties:
      Arn: !GetAtt Bucket.Arn
      Other: !GetAtt [Bucket, DomainName]
      Long:
        Fn::GetAtt: Queue.QueueName
      Joined: !Join ["", [!Ref AWS::Region, !Ref Bucket]]
      Picked: !If [IsProd, !Ref Bucket, !Ref AWS::NoValue]
      Custom: !Rain::Embed file.txt
  Queue:
    Type: AWS::SQS::Queue
    DependsOn: [Bucket, Role]
`

func cfnCalls(t *testing.T, spec, src string) map[string][]string {
	t.Helper()
	x := mustDoc(t, spec)
	res, err := x.ExtractDoc("svc", "template.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, n := range res.Nodes {
		out[string(n.Kind)+":"+n.Name] = n.Calls
	}
	return out
}

// TestDocYAMLTagsCloudFormation: under the scheme, `!Ref X` and
// `!Condition X` read as {Ref: X} and {Condition: X}, every other short form
// `!Name v` as {Fn::Name: v} (cfn-lint's rule, a tag it does not know
// included), and a GetAtt written `A.B`, short form or long, as the list
// [A, B]; `..key` reaches each at any depth, through maps and lists, and
// `[0]` takes a list's first item.
func TestDocYAMLTagsCloudFormation(t *testing.T) {
	got := cfnCalls(t, cfnEngineSpec, cfnEngineTemplate)
	want := map[string][]string{
		"CFNParameter:Env":    nil,
		"CFNCondition:IsProd": nil,
		"CFNResource:Bucket":  {"${Env}-bucket", "Env"},
		"CFNResource:Role":    {"AWS::NoValue", "AWS::Region", "Bucket", "IsProd", "Queue", "file.txt"},
		"CFNResource:Queue":   {"Bucket", "Role"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls\n got: %v\nwant: %v", got, want)
	}
}

// TestDocYAMLTagsOffByDefault: without yaml_tags a local tag is dropped, as
// it always was (the node reads as its untagged value), so GitLab's
// `!reference [a, b]` stays a two-item list and `!Ref X` the scalar X.
func TestDocYAMLTagsOffByDefault(t *testing.T) {
	spec := strings.Replace(cfnEngineSpec, `"yaml_tags": "cloudformation",`, "", 1)
	got := cfnCalls(t, spec, cfnEngineTemplate)
	if calls := got["CFNResource:Role"]; len(calls) != 1 || calls[0] != "Bucket" {
		t.Errorf("Role calls without the scheme = %v; only DependsOn (no tag involved) should read", calls)
	}
	x := mustDoc(t, `{"decode":"yaml","detect":{},"entities":[
	  {"select":"$.*","where":{"is_map":true},"kind":"job","source_kind":"GitLabCIJob","name":"$key"}],
	  "references":[{"from":"job","select":"$.script[*][*]","value":"$","to_kind":"job","edge":"refs"}]}`)
	res, err := x.ExtractDoc("svc", ".gitlab-ci.yml", []byte("build:\n  script:\n    - !reference [.setup, script]\n    - make\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 1 || !reflect.DeepEqual(res.Nodes[0].Calls, []string{".setup", "make", "script"}) {
		t.Errorf("!reference untouched: got %+v", brief(res.Nodes))
	}
}

// TestDocYAMLTagsKeepLines: a normalised intrinsic sits on the line its tag
// was written on, so an entity's end line and a reference's line are the
// file's.
func TestDocYAMLTagsKeepLines(t *testing.T) {
	x := mustDoc(t, cfnEngineSpec)
	res, err := x.ExtractDoc("svc", "template.yaml", []byte(cfnEngineTemplate))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CFNParameter Env@3-4",
		"CFNCondition IsProd@6-6",
		"CFNResource Bucket@8-14 sig=AWS::S3::Bucket calls=${Env}-bucket,Env",
		"CFNResource Role@15-25 sig=AWS::IAM::Role calls=AWS::NoValue,AWS::Region,Bucket,IsProd,Queue,file.txt",
		"CFNResource Queue@26-28 sig=AWS::SQS::Queue calls=Bucket,Role",
	}
	if got := brief(res.Nodes); !reflect.DeepEqual(got, want) {
		t.Errorf("nodes\n got: %q\nwant: %q", got, want)
	}
}

// TestDocDeclaredOnly: a reference with declared_only records a name only
// when an entity of its to_kind (kind or source_kind) of that name is
// declared in the same document: a pseudo parameter (AWS::Region) and a
// name the template never declares are dropped.
func TestDocDeclaredOnly(t *testing.T) {
	spec := `{
  "decode": "yaml", "yaml_tags": "cloudformation",
  "detect": {},
  "entities": [
    { "select": "$.Parameters.*", "where": { "is_map": true }, "kind": "variable_decl", "source_kind": "CFNParameter", "name": "$key" },
    { "select": "$.Resources.*", "where": { "is_map": true }, "kind": "resource", "source_kind": "CFNResource", "name": "$key" }
  ],
  "references": [
    { "from": "resource", "select": "$..Ref", "value": "$", "to_kind": "CFNParameter", "edge": "ref", "declared_only": true },
    { "from": "resource", "select": "$..Ref", "value": "$", "to_kind": "resource", "edge": "ref", "declared_only": true },
    { "from": "resource", "select": "$..Fn::GetAtt[0]", "value": "$", "to_kind": "resource", "edge": "getatt", "declared_only": true }
  ]
}`
	got := cfnCalls(t, spec, cfnEngineTemplate+"  Stray:\n    Type: AWS::SNS::Topic\n    Properties:\n      Name: !Ref Nowhere\n      Arn: !GetAtt Ghost.Arn\n")
	want := map[string][]string{
		"CFNParameter:Env":   nil,
		"CFNResource:Bucket": {"Env"},
		"CFNResource:Role":   {"Bucket", "Queue"},
		"CFNResource:Queue":  nil,
		"CFNResource:Stray":  nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls\n got: %v\nwant: %v", got, want)
	}
}

// TestDocDescendAndIndexSelectors: `..key` returns every value of key under
// the node, in document order (a match's own subtree after it), with the
// dotted $path of the keys walked (a list item adds none); `[N]` is a list's
// Nth item and nothing else (out of range, a scalar or a map yield nothing).
func TestDocDescendAndIndexSelectors(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","detect":{},"entities":[
	  {"select":"$..id","where":{"is_scalar":true},"kind":"key","source_kind":"Id","name":"$","signature":"$path"},
	  {"select":"$.list[1]","kind":"key","source_kind":"Second","name":"$"},
	  {"select":"$.list[9]","kind":"key","source_kind":"Missing","name":"$"},
	  {"select":"$.scalar[0]","kind":"key","source_kind":"NotAList","name":"$"},
	  {"select":"$.map[0]","kind":"key","source_kind":"NotAList","name":"$"}]}`)
	src := "id: top\nmap:\n  id: m\n  deeper:\n    - id: in-list\n      nested:\n        id: nested\nlist: [a, b, c]\nscalar: s\n"
	res, err := x.ExtractDoc("svc", "x.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Id top@1-1 sig=id",
		"Id m@3-3 sig=map.id",
		"Id in-list@5-5 sig=map.deeper.id",
		"Id nested@7-7 sig=map.deeper.nested.id",
		"Second b@8-8",
	}
	if got := brief(res.Nodes); !reflect.DeepEqual(got, want) {
		t.Errorf("nodes\n got: %q\nwant: %q", got, want)
	}
}

// TestDocContentSniff: detect.content_sniff compiles with the spec and is
// what DetectsContent answers over the whole file; a spec without one
// detects nothing, and a sniff that is not a regular expression refuses the
// spec.
func TestDocContentSniff(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","detect":{"content_sniff":"(?m)^AWSTemplateFormatVersion\\s*:"},
	  "entities":[{"select":"$.Resources.*","kind":"resource","source_kind":"CFNResource","name":"$key"}]}`)
	if !x.DetectsContent([]byte("Description: x\nAWSTemplateFormatVersion: 2010-09-09\n")) {
		t.Error("a template the sniff matches is not detected")
	}
	if x.DetectsContent([]byte("apiVersion: v1\nkind: Pod\n")) {
		t.Error("a Kubernetes manifest is detected")
	}
	if mustDoc(t, `{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"package","source_kind":"X","name":"$file"}]}`).DetectsContent([]byte("AWSTemplateFormatVersion: x")) {
		t.Error("a spec without a sniff detects content")
	}
	s, err := ParseDocSpec([]byte(`{"decode":"yaml","detect":{"content_sniff":"(unclosed"},"entities":[{"select":"$","kind":"package","source_kind":"X","name":"$file"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewDocExtractor(s); err == nil || !strings.Contains(err.Error(), "content_sniff") {
		t.Errorf("a bad sniff: err = %v, want a content_sniff error", err)
	}
}

// TestDocCFNSpecRefusals: the new fields are checked like the old ones.
func TestDocCFNSpecRefusals(t *testing.T) {
	ent := `"entities":[{"select":"$","kind":"package","source_kind":"X","name":"$file"}]`
	for name, spec := range map[string]string{
		"unknown scheme":     `{"decode":"yaml","yaml_tags":"k8s","detect":{},` + ent + `}`,
		"tags on json":       `{"decode":"json","yaml_tags":"cloudformation","detect":{},` + ent + `}`,
		"descend no key":     `{"decode":"yaml","detect":{},"entities":[{"select":"$..","kind":"package","source_kind":"X","name":"$file"}]}`,
		"descend wildcard":   `{"decode":"yaml","detect":{},"entities":[{"select":"$..*","kind":"package","source_kind":"X","name":"$file"}]}`,
		"index not a number": `{"decode":"yaml","detect":{},"entities":[{"select":"$.a[x]","kind":"package","source_kind":"X","name":"$file"}]}`,
		"negative index":     `{"decode":"yaml","detect":{},"entities":[{"select":"$.a[-1]","kind":"package","source_kind":"X","name":"$file"}]}`,
	} {
		s, err := ParseDocSpec([]byte(spec))
		if err != nil {
			continue // refused at parse: also a refusal
		}
		if _, err := NewDocExtractor(s); err == nil {
			t.Errorf("%s: spec accepted", name)
		}
	}
}
