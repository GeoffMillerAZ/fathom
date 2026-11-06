package iacfacts

import (
	"reflect"
	"testing"
)

const pulumiYAML = `name: demo
runtime: yaml
configuration:
  env:
    type: String
    default: dev
  region:
    type: String
variables:
  prefix: ${env}-app
  loop: ${loop}
  flag: true
resources:
  logs:
    type: aws:s3/bucket:Bucket
    properties:
      bucket: ${prefix}-logs
      acl: private
      region: ${region}
      versioning:
        enabled: true
      tags:
        Name: ${env}
        Owner: ${other.id}
      rules:
        - id: one
        - id: ${logs.id}
      secretish:
        fn::secret: hunter2
      joined:
        fn::join:
          - "-"
          - [a, "${env}"]
      weird:
        fn::mystery: x
      pulumiStack: ${pulumi.stack}
      cyc: ${loop}
      flagged: ${flag}
      escaped: $${not-a-var}
      num: 5
    options:
      provider: ${west}
  ami:
    type: aws:ec2:getAmi
    get:
      id: ami-123
`

func TestReadPulumiYAML(t *testing.T) {
	set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(pulumiYAML), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Resources) != 2 || set.SyntaxErrors != 0 {
		t.Fatalf("%+v", set)
	}
	b := set.Resources[0]
	if b.Tool != ToolPulumi || b.Mode != ModeManaged || b.Name != "logs" || b.Type != "aws:s3/bucket:Bucket" || b.Provider != "west" {
		t.Fatalf("header %+v", b)
	}
	if set.Resources[1].Mode != ModeData {
		t.Fatal("get section is a data source")
	}
	for _, c := range []struct {
		path string
		want Class
		refs []string
		val  any
	}{
		{"bucket", ClassDefaulted, []string{"prefix"}, nil},
		{"acl", ClassLiteral, nil, "private"},
		{"region", ClassReference, []string{"region"}, nil},
		{"versioning.enabled", ClassLiteral, nil, true},
		{"tags.Name", ClassDefaulted, []string{"env"}, "dev"},
		{"tags.Owner", ClassUnknown, []string{"other.id"}, nil},
		{"rules[*].id", ClassLiteral, nil, "one"},
		{"rules[*].id", ClassReference, []string{"logs.id"}, nil},
		{"secretish", ClassLiteral, nil, nil}, // fn::secret: the class stays and the value is withheld
		{"joined", ClassExpression, []string{"env"}, nil},
		{"weird", ClassUnknown, nil, nil},
		{"pulumiStack", ClassExpression, nil, nil},
		{"cyc", ClassUnknown, []string{"loop"}, nil},
		{"flagged", ClassLiteral, []string{"flag"}, true},
		{"escaped", ClassLiteral, nil, "${not-a-var}"},
		{"num", ClassLiteral, nil, int64(5)},
	} {
		found := false
		for _, a := range b.Attrs {
			if a.Path == c.path && a.Class == c.want && (c.want == ClassUnknown || reflect.DeepEqual(a.Refs, c.refs)) {
				found = true
				if c.val != nil && !reflect.DeepEqual(a.Value, c.val) {
					t.Errorf("%s: value %#v, want %#v", c.path, a.Value, c.val)
				}
			}
		}
		if !found {
			t.Errorf("%s: no %s attribute with refs %v", c.path, c.want, c.refs)
		}
	}
}

func TestReadPulumiYAMLNotAProgram(t *testing.T) {
	for _, src := range []string{"", "name: x\nruntime: nodejs\n", "resources: [a]\n", "- a\n", "resources:\n  a:\n    properties: {}\n"} {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src))
		if err != nil || len(set.Resources) != 0 {
			t.Errorf("%q: %+v %v", src, set, err)
		}
	}
	if set, _ := ReadPulumiYAMLFile("Pulumi.yaml", []byte("a: [")); set.SyntaxErrors != 1 {
		t.Fatal("syntax error not counted")
	}
}

// fn::secret keeps the class of its argument whatever the argument's shape, and never its value. A list or mapping
// literal inside fn::secret reads literal, as the same literal does unwrapped, so a check for a hardcoded secret
// sees it.
func TestReadPulumiSecretKeepsTheClassOfAStructure(t *testing.T) {
	const src = `name: demo
runtime: yaml
variables:
  vl:
    fn::secret: [a, b]
  vm:
    fn::rawSecret: {k: v}
resources:
  r:
    type: aws:s3/bucket:Bucket
    properties:
      l1: [a, b]
      l2: {fn::secret: [a, b]}
      o2: {fn::secret: {k: v}}
      s: {fn::secret: hunter2}
      viaList: ${vl}
      viaMap: ${vm}
      e:
        fn::secret:
          fn::join: ["-", [a, b]]
`
	set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("%v %+v", err, set)
	}
	got := map[string]Attr{}
	for _, a := range set.Resources[0].Attrs {
		got[a.Path] = a
	}
	for path, want := range map[string]Class{"l2": ClassLiteral, "o2": ClassLiteral, "s": ClassLiteral, "viaList": ClassLiteral, "viaMap": ClassLiteral, "e": ClassExpression} {
		a, ok := got[path]
		if !ok || a.Class != want {
			t.Errorf("%s: %+v, want class %s", path, a, want)
		}
		if a.Value != nil {
			t.Errorf("%s: a secret's value reached the read: %#v", path, a.Value)
		}
	}
}
