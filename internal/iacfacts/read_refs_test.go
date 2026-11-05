package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func cfnProps(arg string) []byte {
	return []byte(`{"Resources": {"B": {"Type": "AWS::S3::Bucket", "Properties": {"BucketName": ` + arg + `}}}}`)
}

// A Sub definition used twice per level, bottoming at a Ref: the memo made the calls linear but each level still
// appended both copies of the definition's references, so the list doubled per level. 22 levels (817 bytes)
// allocated 298 MB and 32 levels about 300 GB. The old doubling test bottomed at a literal, which has no references.
func TestReadCloudFormationSubRefsDoNotDouble(t *testing.T) {
	arg := `{"Ref": "P"}`
	for i := 0; i < 40; i++ {
		arg = `{"Fn::Sub": ["${a}${a}", {"a": ` + arg + `}]}`
	}
	within(t, 10*time.Second, "sub refs", func() {
		var set *ResourceSet
		var err error
		if got := allocated(func() { set, err = ReadCloudFormationFile("t.json", cfnProps(arg)) }); got > 16<<20 {
			t.Errorf("allocated %d MB", got>>20)
		}
		if err != nil || len(set.Resources[0].Attrs) != 1 {
			t.Fatalf("%v", err)
		}
		if a := set.Resources[0].Attrs[0]; a.Class != ClassReference || fmt.Sprint(a.Refs) != "[P]" {
			t.Errorf("%+v", a)
		}
	})
}

// One definition with 20,000 references used 1,000 times allocated 2 GB from 333 KB.
func TestReadCloudFormationSubRepeatedDefinitionIsMergedOnce(t *testing.T) {
	var items []string
	for i := 0; i < 20_000; i++ {
		items = append(items, fmt.Sprintf(`{"Ref": "R%d"}`, i))
	}
	def := `{"Fn::Join": ["", [` + strings.Join(items, ",") + `]]}`
	arg := `{"Fn::Sub": ["` + strings.Repeat("${d}", 1000) + `", {"d": ` + def + `}]}`
	within(t, 20*time.Second, "repeated definition", func() {
		var err error
		var set *ResourceSet
		if got := allocated(func() { set, err = ReadCloudFormationFile("t.json", cfnProps(arg)) }); got > 128<<20 {
			t.Errorf("allocated %d MB", got>>20)
		}
		if err != nil || len(set.Resources[0].Attrs) != 1 || len(set.Resources[0].Attrs[0].Refs) != 20_000 {
			t.Fatalf("%v", err)
		}
	})
}

// MaxValueBytes is per value: 400 attributes of "x" plus a value near 1 MiB are 420 MB of strings from a 1 MiB
// file, and 75,000 of them in a 2 MiB file would be tens of GB. Under WithValues the sum is bounded too.
func TestReadValuesAreBoundedInAll(t *testing.T) {
	big := strings.Repeat("v", 1_000_000)
	attrs := func(n int, f func(i int) string) string {
		var sb strings.Builder
		for i := 0; i < n; i++ {
			sb.WriteString(f(i))
		}
		return sb.String()
	}
	type reader func(n int) error
	readers := map[string]reader{
		"terraform": func(n int) error {
			src := "locals {\n  v = \"" + big + "\"\n}\nresource \"t\" \"n\" {\n" + attrs(n, func(i int) string { return fmt.Sprintf("  a%d = \"x${local.v}\"\n", i) }) + "}\n"
			_, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(src)}, ".", WithValues())
			return err
		},
		"cloudformation": func(n int) error {
			src := `{"Parameters": {"V": {"Type": "String", "Default": "` + big + `"}}, "Resources": {"B": {"Type": "AWS::S3::Bucket", "Properties": {` +
				strings.TrimSuffix(attrs(n, func(i int) string { return fmt.Sprintf(`"A%d": {"Fn::Sub": "x${V}"},`, i) }), ",") + `}}}}`
			_, err := ReadCloudFormationFile("t.json", []byte(src), WithValues())
			return err
		},
		"pulumi": func(n int) error {
			src := "variables:\n  v: \"" + big + "\"\nresources:\n  r:\n    type: aws:s3/bucket:Bucket\n    properties:\n" +
				attrs(n, func(i int) string { return fmt.Sprintf("      a%d: \"x${v}\"\n", i) })
			_, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
			return err
		},
	}
	for name, read := range readers {
		var err error
		if got := allocated(func() { err = read(400) }); !errors.Is(err, ErrLimit) || got > 128<<20 {
			t.Errorf("%s: 400 attributes: err %v, allocated %d MB", name, err, got>>20)
		}
		if err := read(10); err != nil {
			t.Errorf("%s: 10 attributes must read: %v", name, err)
		}
	}
}

// A local was classified again at every reference, and the node count only saw the reference: 4,000 uses of a
// 100,000-term local took 40 s and 45 GB with values off. A local is classified once.
func TestReadTerraformLocalIsClassifiedOnce(t *testing.T) {
	const terms, uses = 20_000, 1000
	var sb strings.Builder
	sb.WriteString("locals {\n  big = " + strings.Repeat("a.b + ", terms) + "1\n}\nresource \"t\" \"n\" {\n")
	for i := 0; i < uses; i++ {
		fmt.Fprintf(&sb, "  x%d = local.big\n", i)
	}
	sb.WriteString("}\n")
	var set *ResourceSet
	var err error
	got := allocated(func() { set, err = ReadTerraformModule(fstest.MapFS{"main.tf": file(sb.String())}, ".") })
	if err != nil || len(set.Resources) != 1 || len(set.Resources[0].Attrs) != uses {
		t.Fatalf("%v", err)
	}
	if got > 256<<20 {
		t.Errorf("allocated %d MB", got>>20)
	}
	if a := set.Resources[0].Attrs[0]; a.Class != ClassExpression || fmt.Sprint(a.Refs) != "[local.big]" {
		t.Errorf("%+v", a)
	}
}

// A cycle reads unknown at the reference that closes it, and an answer cut by a cycle is not kept for other uses.
func TestReadTerraformLocalCyclesAreNotMemoized(t *testing.T) {
	src := "locals {\n  a = \"x${local.b}\"\n  b = \"y${local.a}\"\n  c = \"z\"\n}\nresource \"t\" \"n\" {\n  p = local.a\n  q = local.b\n  r = local.c\n}\n"
	set, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(src)}, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range set.Resources[0].Attrs {
		want := ClassUnknown
		if a.Path == "r" {
			want = ClassLiteral
		}
		if a.Class != want {
			t.Errorf("%s: %s, want %s", a.Path, a.Class, want)
		}
	}
}

// The parser reports the first syntax error in a file and then recovers silently, so a later broken attribute read as
// a clean literal: `x = 1 2` was literal 1 and SyntaxErrors stayed at 1. An attribute at or after the first error
// reads unknown unless its own lines parse clean on their own.
func TestReadTerraformAttributesAfterTheFirstErrorAreChecked(t *testing.T) {
	src := "locals {\n  fine = \"f\"\n}\n" +
		"resource \"t\" \"n\" {\n  before = 1\n  a = )\n  x = 1 2\n  q = [1, 2 3]\n  o = {k = 1 z = 2}\n  t = true false\n  y = 1\n  r = local.fine\n" +
		"  lifecycle {\n    ignore_changes = [tags] junk\n  }\n}\n" +
		"locals {\n  l = 1 2\n  g = 3\n}\n" +
		"resource \"t\" \"m\" {\n  p = 1 2\n  ok = 2\n  viaL = local.l\n  viaG = local.g\n}\n"
	set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if set.SyntaxErrors == 0 {
		t.Fatal("not counted")
	}
	for name, res := range map[string]Resource{"n": set.Resources[0], "m": set.Resources[1]} {
		if res.Name != name {
			t.Fatalf("order: %s", res.Name)
		}
	}
	unknown := map[string][]string{"n": {"a", "x", "q", "o", "t", "lifecycle.ignore_changes"}, "m": {"p"}}
	literal := map[string]map[string]any{"n": {"before": int64(1), "y": int64(1)}, "m": {"ok": int64(2)}}
	for _, res := range set.Resources {
		for _, p := range unknown[res.Name] {
			if a := attrOf(t, res, p); a.Class != ClassUnknown || a.Value != nil {
				t.Errorf("%s.%s: %+v", res.Name, p, a)
			}
		}
		for p, v := range literal[res.Name] {
			if a := attrOf(t, res, p); a.Class != ClassLiteral || a.Value != v {
				t.Errorf("%s.%s: %+v", res.Name, p, a)
			}
		}
	}
	// Only the module read resolves locals: one the parser may have guessed reads unknown.
	mod, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(src)}, ".", WithValues())
	if err != nil {
		t.Fatal(err)
	}
	m := mod.Resources[1]
	if a := attrOf(t, m, "viaL"); a.Class != ClassUnknown || a.Value != nil {
		t.Errorf("viaL: %+v", a)
	}
	if a := attrOf(t, m, "viaG"); a.Value != int64(3) {
		t.Errorf("viaG: %+v", a)
	}
	if a := attrOf(t, mod.Resources[0], "r"); a.Value != "f" {
		t.Errorf("r: %+v", a)
	}
}
