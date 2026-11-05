package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// MF1: a condition's references were copied into every resource that names it, and nothing charged the copies.
// 4,000 references on 2,000 resources (135 KB) held 196 MB; a 2 MiB file would have taken tens of GB.
func cfnConditionTemplate(refs, resources int) []byte {
	var items, res []string
	for i := 0; i < refs; i++ {
		items = append(items, fmt.Sprintf(`{"Ref": "R%d"}`, i))
	}
	for i := 0; i < resources; i++ {
		res = append(res, fmt.Sprintf(`"B%d": {"Type": "AWS::S3::Bucket", "Condition": "C"}`, i))
	}
	return []byte(`{"Conditions": {"C": {"Fn::Equals": [{"Fn::Join": ["", [` + strings.Join(items, ",") + `]]}, "x"]}},
"Resources": {` + strings.Join(res, ",") + `}}`)
}

func TestReadCloudFormationConditionRefsAreCharged(t *testing.T) {
	set, err := ReadCloudFormationFile("t.json", cfnConditionTemplate(100, 100))
	if err != nil || len(set.Resources) != 100 {
		t.Fatalf("a small condition fan-out reads: %v", err)
	}
	if m := set.Resources[0].Multiplicity; m.Kind != MultiCondition || len(m.Refs) != 101 {
		t.Fatalf("the condition keeps its name and references: %+v", m)
	}
	var over error
	if got := allocated(func() { _, over = ReadCloudFormationFile("t.json", cfnConditionTemplate(4000, 2000)) }); got > 512<<20 {
		t.Errorf("allocated %d MB", got>>20)
	}
	if !errors.Is(over, ErrLimit) {
		t.Fatalf("refs x resources past the budget must be refused: %v", over)
	}
}

// MF2: the HCL parser recurses once per pending `?` and once per open `%{if}` or `%{for}`, and copies the
// diagnostics of every level below it. 80 KB of `a?a?...` allocated 6.4 GB.
func TestParseHCLBoundedCountsTernariesAndDirectives(t *testing.T) {
	n := 4 * MaxDepth
	refused := map[string]string{
		"ternary":        "a" + strings.Repeat("?a", n),
		"ternary in ()":  "(" + "a" + strings.Repeat("?a", n) + ")",
		"if directive":   `"` + strings.Repeat("%{if a}", n) + `"`,
		"closed for":     `"` + strings.Repeat("%{for a in b}", n) + strings.Repeat("%{endfor}", n) + `"`,
		"heredoc if":     "<<EOT\n" + strings.Repeat("%{if a}", n) + "\nEOT",
		"mixed":          "a" + strings.Repeat("?a", n/2) + `"` + strings.Repeat("%{if a}", n/2) + `"`,
		"ternary at cap": "a" + strings.Repeat("?a", MaxDepth+1),
	}
	for name, e := range refused {
		if _, err := ReadTerraformFile("a.tf", tfWith(e)); !errors.Is(err, ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
		if body, _ := (&bindingReader{}).parse([]byte("x = "+e+"\n"), "z.tf"); body != nil {
			t.Errorf("%s: binding must skip the file", name)
		}
	}
	reads := map[string]string{
		"colon-closed chain": "a" + strings.Repeat("?b:a", 5*MaxDepth) + "?b:c",
		"closed directives":  `"` + strings.Repeat("%{if a}x%{endif}", 5*MaxDepth) + `"`,
		"separate strings":   `["` + strings.Repeat(`%{if a}x", "%{if a}`, 5*MaxDepth) + `x"]`,
		"object colons":      "{" + strings.Repeat("a: b,", 5*MaxDepth) + "}",
		"for colon":          "[for x in y : x]",
	}
	for name, e := range reads {
		if _, err := ReadTerraformFile("a.tf", tfWith(e)); errors.Is(err, ErrLimit) {
			t.Errorf("%s: a shape that does not nest must read: %v", name, err)
		}
	}
}
