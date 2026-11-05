package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"
)

// N6: a one-label resource block, and a data source inside a check block, read as nothing with no error counted and
// nothing dropped, which contradicts ResourceSet.Dropped's promise that absence is never silent.
func TestReadTerraformUnreadBlocksAreCounted(t *testing.T) {
	for name, c := range map[string]struct {
		src     string
		syntax  int
		dropped []string
	}{
		"one-label resource": {"resource \"t\" {\n  a = 1\n}\n", 1, nil},
		"three labels":       {"resource \"t\" \"n\" \"x\" {}\n", 1, nil},
		"data one label":     {"data \"t\" {}\n", 1, nil},
		"module two labels":  {"module \"a\" \"b\" {}\n", 1, nil},
		"check data source":  {"check \"c\" {\n  data \"t\" \"n\" {}\n  assert {\n    condition = true\n    error_message = \"x\"\n  }\n}\n", 0, []string{"check.c.data.t.n"}},
		"check bad data":     {"check \"c\" {\n  data \"t\" {}\n}\n", 1, nil},
		"clean":              {"resource \"t\" \"n\" {}\nvariable \"v\" {}\nlocals { a = 1 }\n", 0, nil},
	} {
		set, err := ReadTerraformFile("a.tf", []byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if set.SyntaxErrors != c.syntax || strings.Join(set.Dropped, ",") != strings.Join(c.dropped, ",") {
			t.Errorf("%s: syntax %d dropped %v, want %d %v", name, set.SyntaxErrors, set.Dropped, c.syntax, c.dropped)
		}
	}
	// An override file's unread block counts too.
	set, err := ReadTerraformModule(fstest.MapFS{"main.tf": file("resource \"t\" \"n\" {}\n"), "x_override.tf": file("resource \"t\" {}\n")}, ".")
	if err != nil || set.SyntaxErrors != 1 {
		t.Errorf("override: %v syntax %d", err, set.SyntaxErrors)
	}
}

// N14: tofu.workspace and terraform.workspace are the same thing and read the same class.
func TestReadTerraformWorkspaceReadsTheSameForBothSpellings(t *testing.T) {
	var got []Attr
	for _, root := range []string{"terraform", "tofu"} {
		set, err := ReadTerraformFile("a.tf", tfWith(root+".workspace"))
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, attrOf(t, set.Resources[0], "x"))
	}
	if got[0].Class != got[1].Class || len(got[0].Refs) != len(got[1].Refs) || got[1].Class != ClassExpression || len(got[1].Refs) != 0 {
		t.Errorf("terraform.workspace %+v, tofu.workspace %+v", got[0], got[1])
	}
	set, err := ReadTerraformFile("a.tf", tfWith(`"ws-${tofu.workspace}"`))
	if err != nil || len(attrOf(t, set.Resources[0], "x").Refs) != 0 {
		t.Errorf("a template over tofu.workspace has no reference: %v %+v", err, set.Resources[0].Attrs)
	}
}

// N9: yaml.v3 counts NEL, U+2028 and U+2029 as line breaks, and the span index did not, so a later attribute read
// 7:1-7:1 in a 6-line file. The reader agrees with yaml.v3 on where the key is.
func TestSpansAgreeWithYAMLLineBreaks(t *testing.T) {
	for name, br := range map[string]string{"NEL": "\u0085", "LS": "\u2028", "PS": "\u2029"} {
		src := "Resources:\n  B:\n    Type: T\n    Properties:\n      A: 1" + br + "      C: 2\n      D: 3\n"
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var line, col int
		var find func(n *yaml.Node)
		find = func(n *yaml.Node) {
			if n.Kind == yaml.ScalarNode && n.Value == "C" {
				line, col = n.Line, n.Column
			}
			for _, c := range n.Content {
				find(c)
			}
		}
		find(&doc)
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil || set.SyntaxErrors != 0 {
			t.Fatalf("%s: %v syntax %d", name, err, set.SyntaxErrors)
		}
		a := attrOf(t, set.Resources[0], "C")
		if a.Span.StartLine != line || a.Span.StartCol != col || line == 0 {
			t.Errorf("%s: C spans %d:%d, yaml.v3 says %d:%d", name, a.Span.StartLine, a.Span.StartCol, line, col)
		}
		if d := attrOf(t, set.Resources[0], "D"); d.Span.StartLine != line+1 || d.Span.StartCol != 7 {
			t.Errorf("%s: D spans %d:%d", name, d.Span.StartLine, d.Span.StartCol)
		}
	}
	// JSON has no such breaks: the same bytes inside a string do not move later spans.
	set, err := ReadCloudFormationFile("t.json", []byte("{\"Resources\": {\"B\": {\"Type\": \"T\", \"Properties\": {\"A\": \"x\u2028y\", \"C\": 2}}}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a := attrOf(t, set.Resources[0], "C"); a.Span.StartLine != 1 {
		t.Errorf("JSON line %d, want 1", a.Span.StartLine)
	}
}

// N16: MaxPathBytes bounds one path and MaxAttrs the count, which allowed a gigabyte of paths: a deep nesting
// repeats its prefix in every leaf, and 184 MB came from a 592 KB file.
func TestReadPathBytesAreBoundedInAll(t *testing.T) {
	const depth = 110
	tpl := func(leaves int) []byte {
		var sb strings.Builder
		sb.WriteString(`{"Resources": {"B": {"Type": "T", "Properties": `)
		sb.WriteString(strings.Repeat(`{"a": `, depth))
		sb.WriteString("{")
		for i := 0; i < leaves; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `"l%05d":1`, i)
		}
		sb.WriteString("}" + strings.Repeat("}", depth) + "}}}")
		return []byte(sb.String())
	}
	if set, err := ReadCloudFormationFile("t.json", tpl(500)); err != nil || len(set.Resources[0].Attrs) != 500 {
		t.Fatalf("a small deep template reads: %v", err)
	}
	// 90,000 leaves of about 226 bytes are 20 MB of paths from a 1 MB file.
	if _, err := ReadCloudFormationFile("t.json", tpl(90_000)); !errors.Is(err, ErrLimit) {
		t.Fatalf("20 MB of paths must be refused: %v", err)
	}
}
