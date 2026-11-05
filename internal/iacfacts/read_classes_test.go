package iacfacts

import (
	"testing"
)

func classOf(t *testing.T, set *ResourceSet, path string) Attr {
	t.Helper()
	if len(set.Resources) == 0 {
		t.Fatalf("no resources: %+v", set)
	}
	return attrOf(t, set.Resources[0], path)
}

// OpenTofu reports constant_value -1 for -1, -1.5, [-1, 2] and {a = -1}. from_port = -1 is common, and reading it
// as an expression would count against the class-agreement bar.
func TestReadTerraformNegativeNumbersAreLiteral(t *testing.T) {
	src := "resource \"t\" \"n\" {\n  a = -1\n  b = -1.5\n  c = [-1, 2]\n  d = {x = -1}\n  e = \"p-${-1}\"\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if a := classOf(t, set, "a"); a.Class != ClassLiteral || a.Value != int64(-1) {
		t.Errorf("a %+v", a)
	}
	if a := classOf(t, set, "b"); a.Class != ClassLiteral || a.Value != -1.5 {
		t.Errorf("b %+v", a)
	}
	if a := classOf(t, set, "c"); a.Class != ClassLiteral {
		t.Errorf("c %+v", a)
	} else if l, ok := a.Value.([]any); !ok || len(l) != 2 || l[0] != int64(-1) || l[1] != int64(2) {
		t.Errorf("c value %#v", a.Value)
	}
	if a := classOf(t, set, "d"); a.Class != ClassLiteral {
		t.Errorf("d %+v", a)
	} else if m, ok := a.Value.(map[string]any); !ok || m["x"] != int64(-1) {
		t.Errorf("d value %#v", a.Value)
	}
	if a := classOf(t, set, "e"); a.Class != ClassLiteral || a.Value != "p--1" {
		t.Errorf("e %+v", a)
	}
}

// These fold in OpenTofu too and read expression here. Section 5 exempts them from class agreement in writing.
func TestReadTerraformFoldableOperatorsStayExpression(t *testing.T) {
	src := "resource \"t\" \"n\" {\n  a = !true\n  b = 1 + 2\n  c = [for x in [\"a\"] : x]\n  d = -var.n\n  e = --1\n  f = -(1)\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a", "b", "c", "d", "e", "f"} {
		if a := classOf(t, set, p); a.Class != ClassExpression {
			t.Errorf("%s: %+v", p, a)
		}
	}
}

// The parser recovers from a broken expression with a placeholder or a partial value. What it hands back says
// nothing about what was written, so the attribute reads unknown and the error is counted.
func TestReadTerraformBrokenExpressionsReadUnknown(t *testing.T) {
	cases := map[string]string{
		"close paren":   "resource \"t\" \"n\" {\n  x = )\n  y = 1\n}\n",
		"at sign":       "resource \"t\" \"n\" {\n  x = @true\n  y = 1\n}\n",
		"two operands":  "resource \"t\" \"n\" {\n  x = a b\n  y = 1\n}\n",
		"dangling plus": "resource \"t\" \"n\" {\n  x = 1 +\n  y = 1\n}\n",
	}
	for name, src := range cases {
		set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if set.SyntaxErrors == 0 {
			t.Errorf("%s: not counted", name)
		}
		if a := classOf(t, set, "x"); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("%s: x %+v", name, a)
		}
		if a := classOf(t, set, "y"); a.Class != ClassLiteral {
			t.Errorf("%s: the neighbour must still read: %+v", name, a)
		}
	}
	// An unterminated string swallows what follows it: the attribute reads unknown, never "abc".
	for name, src := range map[string]string{
		"string":  "resource \"t\" \"n\" {\n  x = \"abc\n  y = 1\n}\n",
		"bracket": "resource \"t\" \"n\" {\n  x = [1, 2\n  y = 1\n}\n",
	} {
		set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
		if err != nil || set.SyntaxErrors == 0 {
			t.Fatalf("%s: %v %d", name, err, set.SyntaxErrors)
		}
		if a := classOf(t, set, "x"); a.Class != ClassUnknown || a.Value != nil {
			t.Errorf("%s: x %+v", name, a)
		}
	}
}

// One schema attribute has one spelling, whether a resource writes one block or two.
func TestReadTerraformBlockPathsAlwaysTakeStar(t *testing.T) {
	one, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  subject {\n    common_name = \"a\"\n  }\n  lifecycle {\n    prevent_destroy = true\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  subject {\n    common_name = \"a\"\n  }\n  subject {\n    common_name = \"b\"\n  }\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	for name, set := range map[string]*ResourceSet{"one": one, "two": two} {
		if a := classOf(t, set, "subject[*].common_name"); a.Class != ClassLiteral {
			t.Errorf("%s: %+v", name, a)
		}
	}
	classOf(t, one, "lifecycle.prevent_destroy")
}
