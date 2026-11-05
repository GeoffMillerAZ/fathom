package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func tfWith(expr string) []byte {
	return []byte("resource \"t\" \"n\" {\n  x = " + expr + "\n}\n")
}

// A run of prefix operators makes the HCL parser recurse once per operator, and Go's stack overflow is a fatal
// error no recover can catch. 300 KB of `!` or `-` killed the process before parseHCLBounded.
func TestReadTerraformOperatorRunsAreRefused(t *testing.T) {
	const n = 300_000
	shapes := map[string]string{
		"bang":       strings.Repeat("!", n) + "true",
		"minus":      strings.Repeat("-", n) + "1",
		"mixed":      strings.Repeat("!-", n/2) + "1",
		"parens+ops": strings.Repeat("(", 100) + strings.Repeat("!", 100) + "true" + strings.Repeat(")", 100),
	}
	for name, e := range shapes {
		_, err := ReadTerraformFile("a.tf", tfWith(e))
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
	}
	// Under the bound the run still reads.
	set, err := ReadTerraformFile("a.tf", tfWith(strings.Repeat("!", MaxDepth-1)+"true"))
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("a run under the bound must read: %v", err)
	}
	// Cap + 1.
	if _, err := ReadTerraformFile("a.tf", tfWith(strings.Repeat("!", MaxDepth+1)+"true")); !errors.Is(err, ErrLimit) {
		t.Fatalf("cap+1: %v", err)
	}
}

// The binding reader parses repository files too, up to 1 MiB each, and had no guard at all.
func TestBindingOperatorRunsAreSkippedAndDisclosed(t *testing.T) {
	for name, run := range map[string]string{"bang": "!", "minus": "-", "paren": "("} {
		pad := strings.Repeat(run, maxBindingFileBytes-64)
		fsys := fstest.MapFS{"main.tf": file(reusableModule), "z.tf": file("x = " + pad + "\n")}
		b := bindBlock(fsys, awsCatalogs, IaCBlockRef{Dir: ".", Type: "aws_lb_listener"})
		if !strings.Contains(strings.Join(b.Disclosures, "|"), "past the read bound") {
			t.Errorf("%s: %v", name, b.Disclosures)
		}
	}
}

// within fails the test when fn runs longer than d. The exponential shapes below do not finish in minutes.
func within(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s: still running after %v", name, d)
	}
}

// Fn::Base64 nested 100 deep: classifying an intrinsic walked its argument twice, a Fibonacci curve that had not
// finished in 20 s on a 1.5 KB file.
func TestReadCloudFormationNestedIntrinsicsAreLinear(t *testing.T) {
	arg := `{"Ref": "X"}`
	for i := 0; i < 100; i++ {
		arg = `{"Fn::Base64": ` + arg + `}`
	}
	src := []byte(`{"Resources": {"B": {"Type": "AWS::S3::Bucket", "Properties": {"BucketName": ` + arg + `}}}}`)
	within(t, 10*time.Second, "depth 100", func() {
		set, err := ReadCloudFormationFile("t.json", src)
		if err != nil || len(set.Resources) != 1 || len(set.Resources[0].Attrs) != 1 {
			t.Errorf("%v %+v", err, set)
			return
		}
		a := set.Resources[0].Attrs[0]
		if a.Class != ClassExpression || len(a.Refs) != 1 || a.Refs[0] != "X" {
			t.Errorf("%+v", a)
		}
	})
}

func cfnSubDoubling(levels int) string {
	arg := `"x"`
	for i := 0; i < levels; i++ {
		arg = `{"Fn::Sub": ["${a}${a}", {"a": ` + arg + `}]}`
	}
	return `{"Resources": {"B": {"Type": "AWS::S3::Bucket", "Properties": {"BucketName": ` + arg + `}}}}`
}

// A Sub definition used twice per level doubles the work per level without a memo.
func TestReadCloudFormationSubDoublingChainIsLinear(t *testing.T) {
	within(t, 10*time.Second, "sub chain", func() {
		set, err := ReadCloudFormationFile("t.json", []byte(cfnSubDoubling(40)))
		if err != nil || len(set.Resources) != 1 || len(set.Resources[0].Attrs) != 1 {
			t.Errorf("%v %+v", err, set)
		}
	})
}

// v0: "${v1}${v1}" doubled the work per variable: a 627-byte file took 29 s and 23 GB.
func pulumiDoubling(n int) []byte {
	var sb strings.Builder
	sb.WriteString("variables:\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "  v%d: \"${v%d}${v%d}\"\n", i, i+1, i+1)
	}
	fmt.Fprintf(&sb, "  v%d: x\nresources:\n  r:\n    type: aws:s3/bucket:Bucket\n    properties:\n      bucket: \"${v0}\"\n", n)
	return []byte(sb.String())
}

func TestReadPulumiVariableDoublingChainIsLinear(t *testing.T) {
	within(t, 10*time.Second, "variable chain", func() {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", pulumiDoubling(60))
		if err != nil || len(set.Resources) != 1 || len(set.Resources[0].Attrs) != 1 {
			t.Errorf("%v %+v", err, set)
			return
		}
		if c := set.Resources[0].Attrs[0].Class; c != ClassLiteral {
			t.Errorf("class %s", c)
		}
	})
}

// Under WithValues the same chains would build a 2^60 byte value. A value past MaxValueBytes refuses the whole
// input, and a value under it still reads.
func TestReadValuesPastTheCapAreRefused(t *testing.T) {
	within(t, 10*time.Second, "values", func() {
		if _, err := ReadPulumiYAMLFile("Pulumi.yaml", pulumiDoubling(60), WithValues()); !errors.Is(err, ErrLimit) {
			t.Errorf("pulumi: %v", err)
		}
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", pulumiDoubling(10), WithValues())
		if err != nil || set.Resources[0].Attrs[0].Value != strings.Repeat("x", 1<<10) {
			t.Errorf("pulumi under the cap: %v", err)
		}

		// 2 KB, then 200 copies (400 KB, under the cap), then 3 copies of that (1.2 MB, over it): few nodes, a big value.
		tf := "locals {\n  l0 = \"" + strings.Repeat("x", 2000) + "\"\n  l1 = \"" + strings.Repeat("${local.l0}", 200) +
			"\"\n  l2 = \"${local.l1}${local.l1}${local.l1}\"\n}\nresource \"t\" \"n\" {\n  x = local.l2\n}\n"
		fsys := fstest.MapFS{"main.tf": file(tf)}
		if _, err := ReadTerraformModule(fsys, ".", WithValues()); !errors.Is(err, ErrLimit) {
			t.Errorf("terraform: %v", err)
		}
		if set, err := ReadTerraformModule(fsys, "."); err != nil || len(set.Resources) != 1 {
			t.Errorf("terraform without values must read: %v", err)
		}

		if _, err := ReadCloudFormationFile("t.json", []byte(cfnSubDoubling(35)), WithValues()); !errors.Is(err, ErrLimit) {
			t.Errorf("cloudformation: %v", err)
		}
	})
}

// The visit budget refuses the whole read and never truncates.
func TestReadVisitBudgetSpends(t *testing.T) {
	c := &cfnReader{}
	p := &pulumiReader{}
	for i := 0; i < MaxReadVisits; i++ {
		if !c.spend() || !p.spend() {
			t.Fatalf("budget spent early at %d", i)
		}
	}
	if c.spend() || !c.over || p.spend() || !p.over {
		t.Fatal("cap+1 must spend the budget")
	}
}
