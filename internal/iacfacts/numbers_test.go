package iacfacts

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"testing/fstest"
)

// IAC11: Attr.Value is public, and a number's type differed by reader and could not be serialised. Terraform 1e999
// was +Inf, which json.Marshal rejects, so one attribute would fail a JSON output whole; 12345678901234567890 was
// float64 1.2345678901234567e+19 in Terraform and in CloudFormation JSON and a string in CloudFormation YAML; YAML
// 1e400 was a string. One rule (numberValue) now holds for every reader.
var numberCases = []struct {
	text string
	want any
}{
	{"1.5", 1.5},
	{"12345678901234567890", json.Number("12345678901234567890")},
	{"1e999", "1e999"},
	{"1e400", "1e400"},
	{"7", int64(7)},
	{"-3", int64(-3)},
	{"0.1", 0.1},
}

func numberReads(t *testing.T, text string) map[string]any {
	t.Helper()
	got := map[string]any{}
	tf, err := ReadTerraformModule(fstest.MapFS{"main.tf": file("resource \"t\" \"n\" {\n  v = " + text + "\n}\n")}, ".", WithValues())
	if err != nil {
		t.Fatalf("%s: terraform: %v", text, err)
	}
	got["terraform"] = attrOf(t, tf.Resources[0], "v").Value
	for name, src := range map[string]string{
		"cloudformation yaml": "Resources:\n  B:\n    Type: T\n    Properties:\n      V: " + text + "\n",
		"cloudformation json": `{"Resources": {"B": {"Type": "T", "Properties": {"V": ` + text + `}}}}`,
	} {
		set, err := ReadCloudFormationFile("t."+strings.Fields(name)[1], []byte(src), WithValues())
		if err != nil || set.SyntaxErrors != 0 {
			t.Fatalf("%s %s: %v syntax %d", name, text, err, set.SyntaxErrors)
		}
		got[name] = attrOf(t, set.Resources[0], "V").Value
	}
	for name, src := range map[string]string{
		"pulumi yaml": "resources:\n  r:\n    type: a:b:C\n    properties:\n      v: " + text + "\n",
		"pulumi json": `{"resources": {"r": {"type": "a:b:C", "properties": {"v": ` + text + `}}}}`,
	} {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil || set.SyntaxErrors != 0 {
			t.Fatalf("%s %s: %v syntax %d", name, text, err, set.SyntaxErrors)
		}
		got[name] = attrOf(t, set.Resources[0], "v").Value
	}
	return got
}

func TestNumbersReadTheSameInEveryReader(t *testing.T) {
	for _, c := range numberCases {
		for reader, v := range numberReads(t, c.text) {
			if v != c.want || fmt.Sprintf("%T", v) != fmt.Sprintf("%T", c.want) {
				t.Errorf("%s in %s: %#v (%T), want %#v (%T)", c.text, reader, v, v, c.want, c.want)
			}
		}
	}
}

// Every attribute of every reader serialises, and a number keeps its digits.
func TestNumberAttributesMarshal(t *testing.T) {
	for _, c := range numberCases {
		tf, _ := ReadTerraformModule(fstest.MapFS{"main.tf": file("resource \"t\" \"n\" {\n  v = " + c.text + "\n  l = [" + c.text + ", -" + c.text + "]\n  o = { k = " + c.text + " }\n}\n")}, ".", WithValues())
		for _, a := range tf.Resources[0].Attrs {
			b, err := json.Marshal(a)
			if err != nil {
				t.Errorf("%s: attribute %s does not marshal: %v", c.text, a.Path, err)
			}
			if a.Path == "v" && !strings.Contains(string(b), strings.Trim(fmt.Sprint(c.want), `"`)) && c.text != "1e999" && c.text != "1e400" {
				t.Errorf("%s: marshalled %s lost its digits", c.text, b)
			}
		}
	}
	// 1e999 and 1e400 keep their text, as a string.
	for _, text := range []string{"1e999", "1e400"} {
		if v := numberReads(t, text)["terraform"]; v != text {
			t.Errorf("%s: %#v", text, v)
		}
	}
}

func TestNumberValueRule(t *testing.T) {
	for in, want := range map[string]any{
		"1.0": 1.0, "1e3": 1000.0, ".5": 0.5, "0x10": int64(16), "1e-999": json.Number("1e-999"),
		"0.1000000000000000055511151231257827": json.Number("0.1000000000000000055511151231257827"),
		"inf":                                  "inf", "nan": "nan", ".inf": ".inf", "-1e999": "-1e999",
	} {
		if got := numberValue(in); got != want {
			t.Errorf("%q: %#v (%T), want %#v (%T)", in, got, got, want, want)
		}
	}
}

// IAC26: Terraform read 1.0, 1e3 and -0.0 as int64 (cty keeps the value and not the spelling) while every other reader
// returned float64. The rule is by spelling: an integer written without a fraction or an exponent is an int64.
func TestNumberSpellingsOfOneIntegerReadAlike(t *testing.T) {
	for _, tc := range []struct {
		text string
		want any
	}{
		{"1.0", float64(1)}, {"1e3", float64(1000)}, {"1E3", float64(1000)}, {"2.0e0", float64(2)}, {"0e5", float64(0)},
		{"-0.0", math.Copysign(0, -1)}, {"1e18", float64(1e18)}, {"9223372036854775807", int64(9223372036854775807)},
		{"1e19", 1e19}, {"1.50", 1.5}, {"1e-3", 0.001}, {"0", int64(0)}, {"-0", int64(0)},
	} {
		for reader, v := range numberReads(t, tc.text) {
			if fmt.Sprintf("%v %T", v, v) != fmt.Sprintf("%v %T", tc.want, tc.want) {
				t.Errorf("%s in %s: %#v (%T), want %#v (%T)", tc.text, reader, v, v, tc.want, tc.want)
			}
		}
	}
}

// IAC31: the IAC26 fix made 1e6 a float64, and scalarString printed a float64 with 'g', so "a${1e6}" read a1e+06 where
// tofu prints a1000000. cty prints a number positionally, and so must the reader, from a literal or a default.
func TestTerraformInterpolatesNumbersPositionally(t *testing.T) {
	read := func(src string) Attr {
		set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
		if err != nil || len(set.Resources) != 1 {
			t.Fatalf("%v %+v", err, set)
		}
		return attrOf(t, set.Resources[0], "x")
	}
	for expr, want := range map[string]string{
		"1e6": "a1000000", "1000000.0": "a1000000", "1234567.5": "a1234567.5", "1e21": "a1000000000000000000000",
		"0.00001": "a0.00001", "1.5": "a1.5", "100": "a100", "1e-7": "a0.0000001",
		"12345678901234567890": "a12345678901234567890", "0.1000000000000000055511151231257827": "a0.1000000000000000055511151231257827",
	} {
		a := read(fmt.Sprintf("resource \"t\" \"n\" {\n  x = \"a${%s}\"\n}\n", expr))
		if a.Value != want {
			t.Errorf("a${%s}: %#v, want %q", expr, a.Value, want)
		}
	}
	mod, err := ReadTerraformModule(fstest.MapFS{"main.tf": {Data: []byte("variable \"t\" {\n  default = 1e6\n}\nresource \"t\" \"n\" {\n  x = \"${var.t}s\"\n}\n")}}, ".", WithValues())
	if err != nil || len(mod.Resources) != 1 {
		t.Fatalf("%v %+v", err, mod)
	}
	if a := attrOf(t, mod.Resources[0], "x"); a.Value != "1000000s" || a.Class != ClassDefaulted {
		t.Errorf("a defaulted 1e6: %+v", a)
	}
	// An exponent far past a float64's would print as a million digits: no string is known, and the read stays fast.
	b := read("resource \"t\" \"n\" {\n  x = \"a${1e-99999999}\"\n}\n")
	if b.Value != nil {
		t.Errorf("1e-99999999 must not format: %#v", b.Value)
	}
}
