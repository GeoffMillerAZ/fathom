package main

import "testing"

const propsProgram = `name: p
runtime: yaml
config:
  size:
    type: string
    default: small
  zone:
    type: string
variables:
  fixed: abc
  viaCfg: ${size}
  loopA: ${loopB}
  loopB: ${loopA}
resources:
  bucket:
    type: aws:s3:Bucket
    properties:
      name: literal
      size: ${size}
      zone: ${zone}
      fixedName: ${fixed}
      viaConfig: ${viaCfg}
      cyc: ${loopA}
      stack: ${pulumi.stack}
      mixed: "x-${size}-${zone}"
      joined:
        fn::join: ["-", [a, b]]
      joinedRef:
        fn::join: ["-", [a, "${bucket2}"]]
      secret:
        fn::secret: shh
      tags:
        - key: k
          value: ${size}
      empty: {}
      none: []
  bucket2:
    type: aws:s3:Bucket
    properties:
      other: ${bucket.id}
`

func TestPropsLeaves(t *testing.T) {
	p := propsOf("Pulumi.yaml", []byte(propsProgram))
	if !p.OK {
		t.Fatalf("refused: %s", p.Error)
	}
	want := map[string]propLeaf{
		"name":          {Class: "constant"},
		"size":          {Class: "references", Defaultable: true},
		"zone":          {Class: "references"},
		"fixedName":     {Class: "constant"},
		"viaConfig":     {Class: "references", Defaultable: true},
		"cyc":           {Class: "unresolved"},
		"stack":         {Class: "empty"},
		"mixed":         {Class: "references"},
		"joined":        {Class: "empty"},
		"joinedRef":     {Class: "references"},
		"secret":        {Class: "constant"},
		"tags[*].key":   {Class: "constant"},
		"tags[*].value": {Class: "references", Defaultable: true},
		"empty":         {Class: "constant"},
		"none":          {Class: "constant"},
	}
	got := map[string]propLeaf{}
	for k, ls := range p.Resources["bucket"].Leaves {
		if len(ls) != 1 {
			t.Errorf("%s: %d leaves, want 1", k, len(ls))
		}
		got[k] = ls[0]
	}
	if len(got) != len(want) {
		t.Errorf("leaves: got %d, want %d: %v", len(got), len(want), got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: got %+v, want %+v", k, got[k], w)
		}
	}
	if o := p.Resources["bucket2"].Leaves["other"][0]; o.Class != "references" || o.Defaultable {
		t.Errorf("other: %+v", o)
	}
}

func TestPropsRefusedProgram(t *testing.T) {
	if p := propsOf("Pulumi.yaml", []byte("name: p\nruntime: yaml\nresources: [\n")); p.OK {
		t.Fatalf("a program that does not parse should be refused: %+v", p)
	}
}
