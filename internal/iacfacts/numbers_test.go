package iacfacts

import (
	"encoding/json"
	"fmt"
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
