package iacfacts

import "testing"

// IAC5: a secrecy flag is off only when it is the literal false. The CloudFormation and Pulumi readers cleared it for
// anything but the text `true`, so `NoEcho: yes` (CloudFormation parses YAML 1.1, where yaml.v3 reads a string),
// `NoEcho: !Ref D`, `NoEcho: 1`, an alias, a merge inside the parameter and Pulumi `secret: ${x}` returned the default
// under WithValues. Terraform's rule, kept in tfVar, is the rule: anything the reader cannot tell from true is true.
func TestCloudFormationNoEchoIsOffOnlyForLiteralFalse(t *testing.T) {
	cases := []struct {
		name, noEcho string
		anchor       bool
		withheld     bool
	}{
		{"true", "NoEcho: true", false, true},
		{"quoted true", `NoEcho: "true"`, false, true},
		{"True", "NoEcho: True", false, true},
		{"yes", "NoEcho: yes", false, true},
		{"no", "NoEcho: no", false, true},
		{"1", "NoEcho: 1", false, true},
		{"!Ref", "NoEcho: !Ref D", false, true},
		{"alias", "NoEcho: *a", true, true},
		{"empty", "NoEcho:", false, true},
		{"mapping", "NoEcho: {a: b}", false, true},
		{"merge in the parameter", "<<: *m", true, true},
		{"false", "NoEcho: false", false, false},
		{"False", "NoEcho: False", false, false},
		{"quoted false", `NoEcho: "false"`, false, false},
		{"absent", "Description: d", false, false},
	}
	for _, c := range cases {
		src := "Parameters:\n  P:\n    Type: String\n    Default: hunter2\n    " + c.noEcho + "\nResources:\n  B:\n    Type: T\n    Properties:\n      A: !Ref P\n      S: !Sub 'x-${P}'\n"
		switch {
		case c.noEcho == "NoEcho: *a":
			src = "x-a: &a true\n" + src
		case c.noEcho == "<<: *m":
			src = "x-m: &m\n  Description: d\n" + src
		}
		set, err := ReadCloudFormationFile("t.yaml", []byte(src), WithValues())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, p := range []string{"A", "S"} {
			if withheld := attrOf(t, set.Resources[0], p).Value == nil; withheld != c.withheld {
				t.Errorf("%s: %s withheld = %v, want %v", c.name, p, withheld, c.withheld)
			}
		}
	}
}

func TestPulumiSecretIsOffOnlyForLiteralFalse(t *testing.T) {
	cases := []struct {
		name, secret string
		withheld     bool
	}{
		{"true", "secret: true", true},
		{"quoted true", `secret: "true"`, true},
		{"yes", "secret: yes", true},
		{"1", "secret: 1", true},
		{"tagged", "secret: !Foo x", true},
		{"interpolation", "secret: ${x}", true},
		{"empty", "secret:", true},
		{"merge in the entry", "<<: *m", true},
		{"false", "secret: false", false},
		{"False", "secret: False", false},
		{"absent", "type: string", false},
	}
	for _, c := range cases {
		src := "config:\n  c:\n    default: hunter2\n    " + c.secret + "\nresources:\n  r:\n    type: a:b:C\n    properties:\n      p: ${c}\n      q: x-${c}\n"
		if c.secret == "<<: *m" {
			src = "x-m: &m\n  type: string\n" + src
		}
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, p := range []string{"p", "q"} {
			if withheld := attrOf(t, set.Resources[0], p).Value == nil; withheld != c.withheld {
				t.Errorf("%s: %s withheld = %v, want %v", c.name, p, withheld, c.withheld)
			}
		}
	}
}
