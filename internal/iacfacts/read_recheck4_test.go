package iacfacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// The fourth recheck of the IaC reader (IAC12 to IAC28). Each test fails without its fix.

func jsonNumber(s string) json.Number { return json.Number(s) }

// IAC18: default mode copied the block prefix for every nested block and only emit checked its length, so a long
// block name over many empty blocks cost gigabytes with no attribute to refuse it.
func TestReadTerraformRefusesALongBlockPathWithNoAttributes(t *testing.T) {
	name := strings.Repeat("a", MaxPathBytes+10)
	src := "resource \"t\" \"n\" {\n  " + name + " {\n" + strings.Repeat("    b {}\n", 3000) + "  }\n}\n"
	if _, err := ReadTerraformFile("a.tf", []byte(src)); !errors.Is(err, ErrLimit) {
		t.Fatalf("want ErrLimit, got %v", err)
	}
	dyn := "resource \"t\" \"n\" {\n  dynamic \"" + name + "\" {\n    for_each = []\n    content {}\n  }\n}\n"
	if _, err := ReadTerraformFile("a.tf", []byte(dyn)); !errors.Is(err, ErrLimit) {
		t.Fatalf("a dynamic block name: want ErrLimit, got %v", err)
	}
	ok := "resource \"t\" \"n\" {\n  " + strings.Repeat("a", 100) + " {\n    b {}\n  }\n}\n"
	if _, err := ReadTerraformFile("a.tf", []byte(ok)); err != nil {
		t.Fatalf("a path under the bound reads: %v", err)
	}
}

// IAC19: a template over a 512 KB local built a real string that was thrown away when the list that held it was not
// a literal, and nothing charged it.
func TestReadTerraformChargesATemplateThatIsDiscarded(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("locals {\n  l9 = \"" + strings.Repeat("x", 512*1024) + "\"\n}\nresource \"t\" \"n\" {\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "  a%d = [\"${local.l9}x\", z.y]\n", i)
	}
	sb.WriteString("}\n")
	_, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(sb.String())}, ".", WithValues())
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("100 discarded templates of 512 KB each: want ErrLimit, got %v", err)
	}
	// One of them is fine, and is read.
	one := "locals {\n  l9 = \"" + strings.Repeat("x", 512*1024) + "\"\n}\nresource \"t\" \"n\" {\n  a = [\"${local.l9}x\", z.y]\n  b = \"${local.l9}x\"\n}\n"
	set, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(one)}, ".", WithValues())
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("one template reads: %v", err)
	}
	if v, _ := attrOf(t, set.Resources[0], "b").Value.(string); len(v) != 512*1024+1 {
		t.Errorf("a kept template keeps its value: %d bytes", len(v))
	}
}

// IAC21: the reference search reran over the whole expression at each nested splat source.
func TestReadTerraformWalksVariablesOnceForNestedSplats(t *testing.T) {
	const args = 130_000
	var sb strings.Builder
	names := rand.New(rand.NewSource(1)).Perm(args)
	for i, v := range names {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "v%05d", v)
	}
	expr := "f(" + sb.String() + ")"
	for i := 0; i < 63; i++ {
		expr = "(" + expr + ")[*]"
	}
	var set *ResourceSet
	var err error
	within(t, 60*time.Second, "nested splats", func() {
		set, err = ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  x = "+expr+"\n}\n"))
	})
	if err != nil {
		t.Fatalf("a file of %d KB with 63 nested splats must read: %v", len(expr)/1024, err)
	}
	if a := attrOf(t, set.Resources[0], "x"); a.Class != ClassExpression || len(a.Refs) != args {
		t.Fatalf("class %v, %d refs, want %d", a.Class, len(a.Refs), args)
	}
	// A splat over a source with its own variables, and steps with variables of their own, keeps both.
	set, err = ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  x = (f(p.q))[*].a[k.l]\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a := attrOf(t, set.Resources[0], "x"); strings.Join(a.Refs, ",") != "k.l,p.q" {
		t.Fatalf("refs %v", a.Refs)
	}
}

// IAC24: a literal's text was taken from the file being read, whatever file the literal was in. A variable default of
// 3.14 in vars.tf read as the 9.25 that sits at the same offset in main.tf.
func TestLiteralTextComesFromItsOwnFile(t *testing.T) {
	for _, tc := range []struct {
		name, decl, other string
		want              any
	}{
		{"float default", "variable \"r\" {\n  default = %s\n}\n", "9.25", 3.14},
		{"big local", "locals {\n  r = %s\n}\n", "12345678901234567891", jsonNumber("12345678901234567890")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lit := fmt.Sprint(tc.want)
			if n, ok := tc.want.(interface{ String() string }); ok {
				lit = n.String()
			}
			main := "resource \"t\" \"n\" {\n  #" + strings.Repeat("-", 60) + "\n  y = " + tc.other + "\n  x = " + map[bool]string{true: "var.r", false: "local.r"}[strings.HasPrefix(tc.decl, "variable")] + "\n}\n"
			at := strings.Index(main, tc.other)
			decl := fmt.Sprintf(tc.decl, lit)
			// Pad the other file so its literal sits at the same byte offset as main.tf's.
			decl = strings.Repeat(" ", at-strings.Index(decl, lit)) + decl
			if strings.Index(decl, lit) != at {
				t.Fatalf("setup: offsets %d and %d", strings.Index(decl, lit), at)
			}
			set := readModule(t, map[string]string{"main.tf": main, "vars.tf": decl}, WithValues())
			if got := attrOf(t, set.Resources[0], "x").Value; fmt.Sprintf("%v %T", got, got) != fmt.Sprintf("%v %T", tc.want, tc.want) {
				t.Fatalf("x = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// IAC25: override blocks, checked against tofu 1.13 `show -json -config`.
func TestOverrideBlocksNameWhatTheyReplace(t *testing.T) {
	base := func(inner string) string {
		return "resource \"t\" \"n\" {\n  subject {\n" + inner + "\n  }\n  keep = 1\n}\n"
	}
	over := func(inner string) string { return "resource \"t\" \"n\" {\n  subject {\n" + inner + "\n  }\n}\n" }
	paths := func(r Resource) string {
		var out []string
		for _, a := range r.Attrs {
			out = append(out, a.Path+"="+string(a.Class))
		}
		return strings.Join(out, " ")
	}
	t.Run("an empty override block replaces the base block", func(t *testing.T) {
		set := readModule(t, map[string]string{"main.tf": base(`common_name = "x"`), "main_override.tf": over("")}, WithValues())
		a := attrOf(t, set.Resources[0], "subject[*].common_name")
		if a.Class != ClassUnknown || a.Value != nil {
			t.Fatalf("the base value must not survive an override that replaces the block: %s", paths(set.Resources[0]))
		}
		if k := attrOf(t, set.Resources[0], "keep"); k.Class != ClassLiteral {
			t.Fatalf("an attribute the override does not name is kept: %s", paths(set.Resources[0]))
		}
	})
	t.Run("every override attribute is added when the base has no block", func(t *testing.T) {
		set := readModule(t, map[string]string{"main.tf": "resource \"t\" \"n\" {\n  keep = 1\n}\n", "main_override.tf": over("common_name = \"a\"\n    organization = \"b\"")})
		attrOf(t, set.Resources[0], "subject[*].common_name")
		attrOf(t, set.Resources[0], "subject[*].organization")
	})
	t.Run("a block over a block with other attributes names both", func(t *testing.T) {
		set := readModule(t, map[string]string{"main.tf": base(`common_name = "x"`), "main_override.tf": over(`organization = "o"`)}, WithValues())
		if a := attrOf(t, set.Resources[0], "subject[*].common_name"); a.Class != ClassUnknown {
			t.Fatalf("%s", paths(set.Resources[0]))
		}
		attrOf(t, set.Resources[0], "subject[*].organization")
	})
	t.Run("a second override file adds its own attributes", func(t *testing.T) {
		set := readModule(t, map[string]string{"main.tf": base(`common_name = "x"`), "a_override.tf": over(`organization = "o"`), "b_override.tf": over(`country = "c"`)})
		attrOf(t, set.Resources[0], "subject[*].organization")
		attrOf(t, set.Resources[0], "subject[*].country")
	})
}

// IAC27: a value past MaxValueBytes refuses the whole input in every reader, as the documentation says.
func TestAValuePastMaxValueBytesRefusesInEveryReader(t *testing.T) {
	big := strings.Repeat("x", MaxValueBytes+MaxValueBytes/2)
	cfn := "Resources:\n  B:\n    Type: T\n    Properties:\n      V: " + big + "\n"
	cfnRef := "Parameters:\n  P:\n    Type: String\n    Default: " + big + "\nResources:\n  B:\n    Type: T\n    Properties:\n      V: !Ref P\n"
	pul := "resources:\n  r:\n    type: a:b:C\n    properties:\n      v: " + big + "\n"
	for name, read := range map[string]func(opts ...Option) error{
		"cloudformation literal": func(o ...Option) error { _, err := ReadCloudFormationFile("t.yaml", []byte(cfn), o...); return err },
		"cloudformation !Ref":    func(o ...Option) error { _, err := ReadCloudFormationFile("t.yaml", []byte(cfnRef), o...); return err },
		"pulumi literal":         func(o ...Option) error { _, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(pul), o...); return err },
	} {
		if err := read(WithValues()); !errors.Is(err, ErrLimit) {
			t.Errorf("%s under WithValues: want ErrLimit, got %v", name, err)
		}
		if err := read(); err != nil {
			t.Errorf("%s without values reads: %v", name, err)
		}
	}
}
