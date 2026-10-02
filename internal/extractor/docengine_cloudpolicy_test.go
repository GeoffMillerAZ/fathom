package extractor

// docengine_cloudpolicy_test.go — the doc engine's additive extension for
// the cloud-policy dialects (aws-iam-policy, azure-policy, cloud-custodian;
// wave pc, LB-F139): the `$index` value (an unnamed list item's position),
// the `..{key=value}` selector step (every map at any depth whose key holds
// that scalar), a reference's `scan` (a regular expression run over every
// string under the selection), the aws-arn and resource-type cross-file
// strategies (policy_ref, written as found) and a cross-file rule's
// `target_map` (a resource-type's mapped target, "" for a type the map does
// not know). Every other dialect reads as before: each option is off unless
// a spec names it.

import (
	"reflect"
	"strings"
	"testing"
)

const cloudPolicyEngineSpec = `{
  "decode": "yaml",
  "detect": {},
  "entities": [
    { "select": "$", "where": { "is_map": true }, "kind": "policy", "source_kind": "TestDoc", "name": "$file" },
    { "select": "$.Statement[*]", "where": { "is_map": true }, "kind": "rule", "source_kind": "TestStatement", "name": ".Sid | $index" },
    { "select": "$.parameters.*", "where": { "is_map": true }, "kind": "variable_decl", "source_kind": "TestParam", "name": "$key" }
  ],
  "references": [
    { "from": "TestStatement", "select": "$", "value": "$", "to_kind": "variable_decl", "edge": "param", "scan": "parameters\\('([^']+)'\\)", "declared_only": true }
  ],
  "cross_file": [
    { "from": "TestStatement", "select": "$.Resource[*]", "value": "$", "resolve": "aws-arn", "edge": "policy_ref" },
    { "from": "TestStatement", "select": "$..{field=type}.equals", "value": "$", "resolve": "resource-type", "edge": "policy_ref" },
    { "from": "TestStatement", "select": "$..{field=type}.in[*]", "value": "$", "resolve": "resource-type", "edge": "policy_ref" }
  ]
}`

const cloudPolicyEngineDoc = `parameters:
  effect: {type: String}
Statement:
  - Sid: Named
    Resource: arn:aws:s3:::bucket/*
    if:
      allOf:
        - field: type
          equals: Microsoft.Storage/storageAccounts
        - field: name
          equals: not-a-type
        - anyOf:
            - field: type
              in: [Microsoft.Compute/virtualMachines, Microsoft.Network/networkInterfaces]
    then: "[parameters('effect')] and [parameters('undeclared')]"
  - Resource: ["*", "arn:aws:iam::1:role/r"]
  - {Resource: "*"}
`

func TestDocIndexNamesUnnamedItems(t *testing.T) {
	x := mustDoc(t, cloudPolicyEngineSpec)
	res, err := x.ExtractDoc("svc", "p.yaml", []byte(cloudPolicyEngineDoc))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range res.Nodes {
		if n.Kind == "TestStatement" {
			got = append(got, n.Name)
		}
	}
	if want := []string{"Named", "1", "2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("statement names = %v, want %v", got, want)
	}
	// A lone map is its own one item, index 0.
	res, _ = x.ExtractDoc("svc", "p.yaml", []byte("Statement:\n  Resource: a\n"))
	found := false
	for _, n := range res.Nodes {
		found = found || (n.Kind == "TestStatement" && n.Name == "0")
	}
	if !found || len(res.Nodes) != 2 {
		t.Errorf("a single statement map: %+v", res.Nodes)
	}
}

func TestDocScanReadsEveryStringUnderTheSelection(t *testing.T) {
	x := mustDoc(t, cloudPolicyEngineSpec)
	res, err := x.ExtractDoc("svc", "p.yaml", []byte(cloudPolicyEngineDoc))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Nodes {
		if n.Name != "Named" {
			continue
		}
		// declared_only keeps `effect` (declared) and drops `undeclared`.
		if want := []string{"effect"}; !reflect.DeepEqual(n.Calls, want) {
			t.Errorf("Named calls = %v, want %v", n.Calls, want)
		}
	}
}

func TestDocDescendWhereAndPolicyRefStrategies(t *testing.T) {
	x := mustDoc(t, cloudPolicyEngineSpec)
	res, err := x.ExtractDoc("svc", "p.yaml", []byte(cloudPolicyEngineDoc))
	if err != nil {
		t.Fatal(err)
	}
	type k struct{ from, raw, strategy, target string }
	var got []k
	for _, c := range res.CrossFile {
		if c.Edge != "policy_ref" {
			t.Errorf("edge = %q, want policy_ref", c.Edge)
		}
		got = append(got, k{c.From, c.Raw, c.Strategy, c.Target})
	}
	want := []k{
		{"Named", "arn:aws:s3:::bucket/*", "aws-arn", "arn:aws:s3:::bucket/*"},
		{"Named", "Microsoft.Storage/storageAccounts", "resource-type", "Microsoft.Storage/storageAccounts"},
		{"Named", "Microsoft.Compute/virtualMachines", "resource-type", "Microsoft.Compute/virtualMachines"},
		{"Named", "Microsoft.Network/networkInterfaces", "resource-type", "Microsoft.Network/networkInterfaces"},
		{"1", "*", "aws-arn", "*"},
		{"1", "arn:aws:iam::1:role/r", "aws-arn", "arn:aws:iam::1:role/r"},
		{"2", "*", "aws-arn", "*"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("policy_ref\n got: %v\nwant: %v", got, want)
	}
}

func TestDocTargetMap(t *testing.T) {
	spec := `{
  "decode": "yaml", "detect": {},
  "entities": [{ "select": "$.policies[*]", "kind": "rule", "source_kind": "TestPolicy", "name": ".name" }],
  "cross_file": [{ "from": "TestPolicy", "select": "$", "value": ".resource", "resolve": "resource-type", "edge": "policy_ref",
                   "target_map": { "aws.s3": "AWS::S3::Bucket", "s3": "AWS::S3::Bucket" } }]
}`
	x := mustDoc(t, spec)
	res, err := x.ExtractDoc("svc", "c.yaml", []byte("policies:\n  - {name: a, resource: aws.s3}\n  - {name: b, resource: s3}\n  - {name: c, resource: aws.mystery}\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.From+" "+c.Raw+" -> "+c.Target)
	}
	want := []string{"a aws.s3 -> AWS::S3::Bucket", "b s3 -> AWS::S3::Bucket", "c aws.mystery -> "}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("target_map\n got: %q\nwant: %q", got, want)
	}
}

func TestDocEmptyTargetMapMapsNothing(t *testing.T) {
	spec := `{
  "decode": "yaml", "detect": {},
  "entities": [{ "select": "$.policies[*]", "kind": "rule", "source_kind": "TestPolicy", "name": ".name" }],
  "cross_file": [{ "from": "TestPolicy", "select": "$", "value": ".resource", "resolve": "resource-type", "edge": "policy_ref", "target_map": {} }]
}`
	res, err := mustDoc(t, spec).ExtractDoc("svc", "c.yaml", []byte("policies:\n  - {name: a, resource: aws.s3}\n"))
	if err != nil || len(res.CrossFile) != 1 || res.CrossFile[0].Raw != "aws.s3" || res.CrossFile[0].Target != "" {
		t.Errorf("empty target_map: %+v, %v", res.CrossFile, err)
	}
}

func TestDocSpecRejectsBadCloudPolicyOptions(t *testing.T) {
	base := `{"decode":"yaml","detect":{},"entities":[{"select":"$","kind":"rule","source_kind":"X","name":"$file"}],`
	for name, tail := range map[string]string{
		"scan without a group": `"references":[{"from":"X","select":"$","value":"$","to_kind":"rule","edge":"e","scan":"abc"}]}`,
		"scan that is not RE2": `"references":[{"from":"X","select":"$","value":"$","to_kind":"rule","edge":"e","scan":"(("}]}`,
		"descend-where no =":   `"cross_file":[{"select":"$..{field}.x","value":"$","resolve":"resource-type","edge":"e"}]}`,
		"target_map elsewhere": `"cross_file":[{"select":"$","value":"$","resolve":"repo-path","edge":"e","target_map":{"a":"b"}}]}`,
	} {
		s, err := ParseDocSpec([]byte(base + tail))
		if err != nil {
			t.Errorf("%s: parse: %v", name, err)
			continue
		}
		if _, err := NewDocExtractor(s); err == nil {
			t.Errorf("%s: NewDocExtractor accepted it", name)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("%s: empty error", name)
		}
	}
}
