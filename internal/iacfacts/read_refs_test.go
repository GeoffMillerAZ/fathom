package iacfacts

import (
	"fmt"
	"strings"
	"testing"
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
