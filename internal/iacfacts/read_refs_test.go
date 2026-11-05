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
