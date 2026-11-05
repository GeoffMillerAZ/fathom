package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

// IAC32: MaxValueBytes says a value is counted as a consumer would serialise it, and MaxTotalValueBytes that it bounds
// every value kept, but neither counted a mapping's keys or the names of ignore_changes. A 900 KB key with ten
// doublings marshals to 900 MB, one key read by 2,000 attributes to 1,757 MB, with no refusal under WithValues.
func TestValueBudgetsCountKeysAndNames(t *testing.T) {
	key := strings.Repeat("k", 300_000)
	// One key past MaxValueBytes is a value past it.
	tf := "resource \"t\" \"n\" {\n  x = {\"" + strings.Repeat("k", MaxValueBytes+1) + "\" = 1}\n}\n"
	if _, err := ReadTerraformFile("a.tf", []byte(tf), WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("terraform, a key past MaxValueBytes: want ErrLimit, got %v", err)
	}
	// One 300 KB key read by 100 attributes is 30 MB of keys, past the 16 MiB total.
	var mod strings.Builder
	mod.WriteString("locals {\n  m = {\"" + key + "\" = 1}\n}\nresource \"t\" \"n\" {\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&mod, "  a%d = local.m\n", i)
	}
	mod.WriteString("}\n")
	if _, err := ReadTerraformModule(fstest.MapFS{"main.tf": {Data: []byte(mod.String())}}, ".", WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("terraform, one key read 100 times: want ErrLimit, got %v", err)
	}
	var pm strings.Builder
	// yaml.v3 refuses a simple key past 1,024 bytes, so the long key is an explicit one.
	pm.WriteString("variables:\n  m:\n    ? \"" + key + "\"\n    : 1\nresources:\n  r:\n    type: a:b:C\n    properties:\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&pm, "      a%d: ${m}\n", i)
	}
	if _, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(pm.String()), WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("pulumi, one key read 100 times: want ErrLimit, got %v", err)
	}
	// Without WithValues no value is built, and the same files read.
	if _, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(pm.String())); err != nil {
		t.Errorf("pulumi without values: %v", err)
	}
	// The names of ignore_changes are a value past MaxValueBytes at 110,000 names of ten bytes.
	var ic strings.Builder
	ic.WriteString("resource \"t\" \"n\" {\n  lifecycle {\n    ignore_changes = [\n")
	for i := 0; i < 110_000; i++ {
		fmt.Fprintf(&ic, "      name%06d,\n", i)
	}
	ic.WriteString("    ]\n  }\n}\n")
	if _, err := ReadTerraformFile("a.tf", []byte(ic.String()), WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("ignore_changes of 1.2 MB of names: want ErrLimit, got %v", err)
	}
	// Small keys and names still read and still return their values.
	set, err := ReadTerraformFile("a.tf", []byte("resource \"t\" \"n\" {\n  x = {a = 1, b = {c = 2}}\n  lifecycle {\n    ignore_changes = [tags]\n  }\n}\n"), WithValues())
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("%v %+v", err, set)
	}
}
