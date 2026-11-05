package iacfacts

import (
	"strings"
	"testing"
	"testing/fstest"
)

// MF5: reader overrides in the JSON syntax, and a sensitive-only override.
func TestReadTerraformJSONOverrideOfModuleAndDynamic(t *testing.T) {
	base := "module \"m\" {\n  source = \"./a\"\n  n = 1\n}\nresource \"t\" \"r\" {\n  dynamic \"rule\" {\n    for_each = []\n    content {\n      p = 1\n    }\n  }\n}\n"
	over := `{"module": {"m": {"source": "./b", "n": 2}}, "resource": {"t": {"r": {"dynamic": {"rule": {"for_each": [], "content": {"p": 2}}}}}}}`
	set, err := ReadTerraformModule(fstest.MapFS{"main.tf": file(base), "main_override.tf.json": file(over)}, ".", WithValues())
	if err != nil || set.SyntaxErrors != 0 {
		t.Fatalf("%v syntax %d", err, set.SyntaxErrors)
	}
	for _, r := range set.Resources {
		switch r.Name {
		case "m":
			for _, p := range []string{"source", "n"} {
				if a := attrOf(t, r, p); a.Class != ClassUnknown || a.Value != nil {
					t.Errorf("module %s: the override sets it and tofu reads it differently: %+v", p, a)
				}
			}
		case "r":
			if a := attrOf(t, r, "rule[*].p"); a.Class != ClassUnknown || a.Value != nil {
				t.Errorf("rule[*].p: %+v", a)
			}
			for _, a := range r.Attrs {
				if a.Path == "dynamic" {
					t.Errorf("`dynamic` is not an attribute: %v", attrPaths(r))
				}
			}
		}
	}
}

func TestReadTerraformSensitiveOnlyOverride(t *testing.T) {
	base := "variable \"v\" {\n  default = \"hunter2\"\n}\nresource \"t\" \"r\" {\n  x = var.v\n}\n"
	value := func(files map[string]string) any {
		m := fstest.MapFS{"main.tf": file(base)}
		for k, v := range files {
			m[k] = file(v)
		}
		set, err := ReadTerraformModule(m, ".", WithValues())
		if err != nil {
			t.Fatal(err)
		}
		return attrOf(t, set.Resources[0], "x").Value
	}
	if v := value(nil); v != "hunter2" {
		t.Fatalf("without an override the default reads: %v", v)
	}
	for name, files := range map[string]map[string]string{
		"hcl":                   {"main_override.tf": "variable \"v\" {\n  sensitive = true\n}\n"},
		"hcl ephemeral":         {"main_override.tf": "variable \"v\" {\n  ephemeral = true\n}\n"},
		"hcl sorts before base": {"a_override.tf": "variable \"v\" {\n  sensitive = true\n}\n"},
		"hcl expression":        {"main_override.tf": "variable \"v\" {\n  sensitive = var.s\n}\n"},
		"json":                  {"main_override.tf.json": `{"variable": {"v": {"sensitive": true}}}`},
	} {
		if v := value(files); v != nil {
			t.Errorf("%s: a sensitive default must not be kept as a value: %v", name, v)
		}
	}
	// The override replaces the base's flag, so a literal false clears it.
	m := fstest.MapFS{"main.tf": file("variable \"v\" {\n  default = \"hunter2\"\n  sensitive = true\n}\nresource \"t\" \"r\" {\n  x = var.v\n}\n"),
		"main_override.tf": file("variable \"v\" {\n  sensitive = false\n}\n")}
	set, err := ReadTerraformModule(m, ".", WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if a := attrOf(t, set.Resources[0], "x"); a.Value != "hunter2" {
		t.Errorf("sensitive = false in the override clears the base flag: %+v", a)
	}
}

// MF6: absence in a template is never silent. A merge key, or a Fn::Transform beside other keys, may carry any key.
func TestTemplateMergeAndTransformAreCountedNotAbsent(t *testing.T) {
	unknownAt := func(t *testing.T, r Resource, path string) {
		t.Helper()
		if a := attrOf(t, r, path); a.Class != ClassUnknown {
			t.Errorf("%q: %+v", path, a)
		}
	}
	cfn := func(props string) *ResourceSet {
		src := "x-a: &a\n  Encrypted: true\nResources:\n  B:\n    Type: AWS::S3::Bucket\n    Properties:\n" + props
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		return set
	}
	t.Run("cfn merge in properties", func(t *testing.T) {
		set := cfn("      <<: *a\n      Size: 1\n")
		r := set.Resources[0]
		unknownAt(t, r, "")
		if a := attrOf(t, r, "Size"); a.Class != ClassLiteral {
			t.Errorf("Size: %+v", a)
		}
		for _, a := range r.Attrs {
			if a.Path == "<<" {
				t.Errorf("the merge key is not an attribute: %v", attrPaths(r))
			}
		}
		if set.SyntaxErrors != 1 {
			t.Errorf("syntax errors %d", set.SyntaxErrors)
		}
	})
	t.Run("cfn nested merge", func(t *testing.T) {
		set := cfn("      VersioningConfiguration:\n        <<: *a\n        Other: 1\n")
		unknownAt(t, set.Resources[0], "VersioningConfiguration")
		if set.SyntaxErrors != 1 {
			t.Errorf("syntax errors %d", set.SyntaxErrors)
		}
	})
	t.Run("cfn transform beside keys", func(t *testing.T) {
		set := cfn("      Fn::Transform:\n        Name: AWS::Include\n        Parameters:\n          Location: s3://x/y\n      Size: 1\n")
		r := set.Resources[0]
		unknownAt(t, r, "")
		for _, a := range r.Attrs {
			if strings.HasPrefix(a.Path, "Fn::Transform") {
				t.Errorf("the transform is not a literal attribute: %v", attrPaths(r))
			}
		}
		if set.SyntaxErrors != 1 {
			t.Errorf("syntax errors %d", set.SyntaxErrors)
		}
	})
	t.Run("cfn condition alias", func(t *testing.T) {
		src := "x-c: &c Cond\nResources:\n  B:\n    Type: AWS::S3::Bucket\n    Condition: *c\n"
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if m := set.Resources[0].Multiplicity; m.Kind != MultiCondition || m.Class != ClassUnknown || set.SyntaxErrors != 1 {
			t.Errorf("%+v syntax %d", m, set.SyntaxErrors)
		}
	})
	t.Run("cfn condition behind a merge", func(t *testing.T) {
		src := "x-b: &b\n  Condition: Cond\nResources:\n  B:\n    <<: *b\n    Type: AWS::S3::Bucket\n    Properties:\n      Size: 1\n"
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if m := set.Resources[0].Multiplicity; m.Kind != MultiCondition || m.Class != ClassUnknown || set.SyntaxErrors != 0 {
			t.Errorf("%+v syntax %d", m, set.SyntaxErrors)
		}
	})
	t.Run("pulumi merge in properties", func(t *testing.T) {
		src := "name: p\nruntime: yaml\nx-a: &a\n  encrypted: true\nresources:\n  b:\n    type: aws:s3/bucket:Bucket\n    properties:\n      <<: *a\n      size: 1\n"
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		unknownAt(t, set.Resources[0], "")
		if set.SyntaxErrors != 1 {
			t.Errorf("syntax errors %d", set.SyntaxErrors)
		}
	})
	t.Run("a mapping with neither reads as before", func(t *testing.T) {
		set := cfn("      Size: 1\n      Nested:\n        A: 1\n")
		if set.SyntaxErrors != 0 || len(set.Resources[0].Attrs) != 2 {
			t.Errorf("syntax %d attrs %v", set.SyntaxErrors, attrPaths(set.Resources[0]))
		}
	})
}
