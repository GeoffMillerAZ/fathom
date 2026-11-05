package iacfacts

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// IAC22, IAC23: pulumi-yaml v1.36.0 matches the fields of its records and the names of its fn:: functions without
// regard to case and warns that a later version will not. A reader that reads only the documented spelling returned a
// secret (config `SECRET: true`, `Fn::Secret`), read no resources (`RESOURCES:`), and read literal facts out of a
// function call (`Fn::Join`).
const pulumiCaseBase = `name: p
runtime: yaml
configuration:
  pw:
    type: string
    default: hunter2
    secret: true
variables:
  v: ${pw}
resources:
  r:
    type: a:b:C
    properties:
      cfg: ${pw}
      sec:
        fn::secret: hunter2
      raw:
        fn::rawSecret: hunter2
      joined:
        fn::join: [",", [a, b]]
      plain: kept
    options:
      provider: ${p}
  d:
    type: a:b:D
    get:
      id: x
`

// recase rewrites the keys of a Pulumi program with f, leaving values and the names of resources, variables and
// configuration alone: every line's `key:` and the `fn::` names.
func recase(src string, f func(string) string) string {
	// Only the fields of Pulumi's records and the fn:: names are recased: a property, a resource or a variable is a
	// name the program chose.
	fields := map[string]bool{"name": true, "runtime": true, "configuration": true, "config": true, "variables": true,
		"resources": true, "type": true, "default": true, "secret": true, "properties": true, "options": true,
		"provider": true, "get": true, "fn::secret": true, "fn::rawSecret": true, "fn::join": true}
	var out []string
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := line[:len(line)-len(trimmed)]
		i := strings.Index(trimmed, ": ")
		if i < 0 && strings.HasSuffix(trimmed, ":") {
			i = len(trimmed) - 1
		}
		if i < 0 || strings.HasPrefix(trimmed, "- ") || !fields[trimmed[:i]] {
			out = append(out, line)
			continue
		}
		out = append(out, indent+f(trimmed[:i])+trimmed[i:])
	}
	return strings.Join(out, "\n")
}

func TestPulumiKeysAndFunctionNamesAreCaseInsensitive(t *testing.T) {
	read := func(src string) *ResourceSet {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil {
			t.Fatal(err)
		}
		return set
	}
	want := read(pulumiCaseBase)
	if len(want.Resources) != 2 || want.SyntaxErrors != 0 {
		t.Fatalf("baseline: %+v", want)
	}
	for _, a := range want.Resources[0].Attrs {
		if v, _ := a.Value.(string); v == "hunter2" {
			t.Fatalf("the baseline returns a secret at %s", a.Path)
		}
	}
	for name, f := range map[string]func(string) string{
		"upper": strings.ToUpper,
		"title": func(s string) string { return strings.ToUpper(s[:1]) + s[1:] },
		"mixed": func(s string) string {
			b := []byte(s)
			for i := range b {
				if i%2 == 1 && b[i] >= 'a' && b[i] <= 'z' {
					b[i] -= 32
				}
			}
			return string(b)
		},
	} {
		got := read(recase(pulumiCaseBase, f))
		// A function name and a field read the same in any case. Only the fn:: spelling can differ in a path, and the
		// fixture has no attribute keyed by one.
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: reads differently\n got %+v\nwant %+v\nsource\n%s", name, got.Resources, want.Resources, recase(pulumiCaseBase, f))
		}
	}
}

func TestPulumiSecretsStayWithheldInEveryCase(t *testing.T) {
	for _, src := range []string{
		"configuration:\n  pw:\n    default: hunter2\n    SECRET: true\nresources:\n  r:\n    type: a:b:C\n    properties:\n      x: ${pw}\n",
		"configuration:\n  pw:\n    default: hunter2\n    secret: false\n    Secret: true\nresources:\n  r:\n    type: a:b:C\n    properties:\n      x: ${pw}\n",
		"resources:\n  r:\n    type: a:b:C\n    properties:\n      x: {Fn::Secret: hunter2}\n",
		"resources:\n  r:\n    type: a:b:C\n    properties:\n      x: {FN::SECRET: hunter2}\n",
		"resources:\n  r:\n    type: a:b:C\n    properties:\n      x: {fn::RAWSECRET: hunter2}\n",
	} {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil || len(set.Resources) != 1 {
			t.Fatalf("%v %+v", err, set)
		}
		for _, a := range set.Resources[0].Attrs {
			if a.Value != nil || strings.Contains(a.Path, "ecret") {
				t.Errorf("%q: %s returns %#v", src, a.Path, a.Value)
			}
		}
	}
	// {fn::secret: x, other: 1} is a plain object to pulumi-yaml, so its value is returned.
	set, _ := ReadPulumiYAMLFile("Pulumi.yaml", []byte("resources:\n  r:\n    type: a:b:C\n    properties:\n      x: {fn::secret: hunter2, other: 1}\n"), WithValues())
	if a := attrOf(t, set.Resources[0], "x.other"); fmt.Sprint(a.Value) != "1" {
		t.Errorf("%+v", a)
	}
}

// IAC20: node.get scanned the mapping once per interpolation; a config entry of 40,000 keys read by 80,000
// interpolations took 18 s.
func TestNodeLookupsAreIndexedForLargeMappings(t *testing.T) {
	n := &node{kind: nodeMap}
	for i := 0; i < 100_000; i++ {
		n.pairs = append(n.pairs, pair{key: &node{kind: nodeScalar, value: fmt.Sprintf("Key%d", i)}, val: &node{kind: nodeScalar, value: "v"}})
	}
	n.pairs = append(n.pairs, pair{key: &node{kind: nodeScalar, value: "default"}, val: &node{kind: nodeScalar, value: "d"}})
	within(t, 10*time.Second, "lookups", func() {
		for i := 0; i < 100_000; i++ {
			if n.get("default") == nil || n.getFold("DEFAULT") == nil || n.get("missing") != nil || n.getFold("missing") != nil {
				t.Error("lookup")
				return
			}
		}
	})
	if n.get("Key5") == nil || n.get("key5") != nil || n.getFold("KEY5") == nil {
		t.Error("get is exact and getFold is not")
	}
}
