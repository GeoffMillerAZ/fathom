package iacfacts

import (
	"strings"
	"testing"
)

func cfnWithProps(props string) string {
	return "Resources:\n  B:\n    Type: AWS::S3::Bucket\n    Properties:\n" + props
}

// IAC9: a Fn::ForEach::Name beside other keys in Properties, or nested under a property, reads as literal facts at
// bogus paths (`Fn::ForEach::Tags[*]`) with no error counted, and the keys it generates are absent. cfn-lint, the
// oracle, flattens the loop into the mapping, so the reader treats it as it treats a Fn::Transform beside other keys:
// disclosed, counted, no facts at bogus paths.
func TestCloudFormationForEachBesideOtherKeysIsDisclosed(t *testing.T) {
	loop := "Fn::ForEach::Tags: [K, [a, b], {'Tag${K}': v}]"
	cases := map[string]struct {
		props      string
		unknownAt  string
		literalAt  string
		wantSyntax int
	}{
		"in Properties":    {"      BucketName: x\n      " + loop + "\n", "", "BucketName", 1},
		"under a property": {"      Tags:\n        Other: 1\n        " + loop + "\n", "Tags", "Tags.Other", 1},
		"deep":             {"      A:\n        B:\n          C: 1\n          " + loop + "\n", "A.B", "A.B.C", 1},
	}
	for name, c := range cases {
		set, err := ReadCloudFormationFile("t.yaml", []byte(cfnWithProps(c.props)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		r := set.Resources[0]
		for _, a := range r.Attrs {
			if strings.Contains(a.Path, "Fn::") || strings.Contains(a.Path, "Tag${K}") {
				t.Errorf("%s: a fact at a bogus path %q (%s)", name, a.Path, a.Class)
			}
		}
		if a := attrOf(t, r, c.unknownAt); a.Class != ClassUnknown {
			t.Errorf("%s: %q must read unknown: %+v", name, c.unknownAt, a)
		}
		if a := attrOf(t, r, c.literalAt); a.Class != ClassLiteral {
			t.Errorf("%s: %q: %+v", name, c.literalAt, a)
		}
		if set.SyntaxErrors != c.wantSyntax {
			t.Errorf("%s: syntax errors %d, want %d", name, set.SyntaxErrors, c.wantSyntax)
		}
	}
	// A loop that is the whole value is still an intrinsic, as before.
	set, err := ReadCloudFormationFile("t.yaml", []byte(cfnWithProps("      Tags:\n        "+loop+"\n")))
	if err != nil || set.SyntaxErrors != 0 {
		t.Fatalf("%v %+v", err, set)
	}
	if a := attrOf(t, set.Resources[0], "Tags"); a.Class != ClassExpression {
		t.Errorf("a lone Fn::ForEach is an expression: %+v", a)
	}
}

// N13: a key that is not a scalar has no name to put in a path. A complex key read path "" literal 1, which breaks
// Attr.Path's contract that an empty path reads unknown, and an alias key's path was the anchor's name.
func TestComplexYAMLKeysReadUnknownAtTheParent(t *testing.T) {
	for name, src := range map[string]string{
		"sequence key": cfnWithProps("      ? [a, b]\n      : 1\n      Size: 2\n"),
		"mapping key":  cfnWithProps("      ? {a: b}\n      : 1\n      Size: 2\n"),
		"alias key":    "x-k: &k name\n" + cfnWithProps("      *k : 1\n      Size: 2\n"),
	} {
		set, err := ReadCloudFormationFile("t.yaml", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		r := set.Resources[0]
		for _, a := range r.Attrs {
			if a.Path == "" && a.Class != ClassUnknown {
				t.Errorf("%s: an empty path must read unknown: %+v", name, a)
			}
			if a.Path == "name" || a.Path == "a" || a.Path == "k" {
				t.Errorf("%s: a fact at the key's text: %+v", name, a)
			}
		}
		if a := attrOf(t, r, ""); a.Class != ClassUnknown {
			t.Errorf("%s: %+v", name, a)
		}
		if a := attrOf(t, r, "Size"); a.Class != ClassLiteral {
			t.Errorf("%s: Size: %+v", name, a)
		}
		if set.SyntaxErrors != 1 {
			t.Errorf("%s: syntax errors %d", name, set.SyntaxErrors)
		}
	}
}

// N5: a merge key at the document root may carry Resources or Parameters, and counted nothing.
func TestRootMergeIsCounted(t *testing.T) {
	set, err := ReadCloudFormationFile("t.yaml", []byte("x-b: &b\n  Resources:\n    A:\n      Type: AWS::S3::Bucket\n<<: *b\n"))
	if err != nil || len(set.Resources) != 0 || set.SyntaxErrors != 1 {
		t.Errorf("cloudformation: %v resources %d syntax %d", err, len(set.Resources), set.SyntaxErrors)
	}
	set, err = ReadPulumiYAMLFile("Pulumi.yaml", []byte("x-b: &b\n  resources:\n    a:\n      type: aws:s3/bucket:Bucket\n<<: *b\n"))
	if err != nil || len(set.Resources) != 0 || set.SyntaxErrors != 1 {
		t.Errorf("pulumi: %v resources %d syntax %d", err, len(set.Resources), set.SyntaxErrors)
	}
}

// IAC10: `provider = tls.by[each.key]` is valid in OpenTofu and names the configuration tls.by (provider_config_key
// in `tofu show -json -config`). The reader accepted only a bare traversal, so the field read empty, which means "no
// meta-argument", and a type-prefix binding would pick the wrong provider.
func TestReadTerraformProviderWithInstanceKey(t *testing.T) {
	for expr, want := range map[string]string{
		"tls.by":                    "tls.by",
		"tls.by[each.key]":          "tls.by",
		`tls.by["k"]`:               "tls.by",
		"google-beta.by[each.key]":  "google-beta.by",
		"aws":                       "aws",
		`{ a = 1 }`:                 "",
		`"tls.by"`:                  "",
		"tls.by[each.key] ? a : b ": "",
	} {
		src := "resource \"google_x\" \"n\" {\n  provider = " + expr + "\n  for_each = var.r\n}\n"
		set, err := ReadTerraformFile("a.tf", []byte(src))
		if err != nil || len(set.Resources) != 1 {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := set.Resources[0].Provider; got != want {
			t.Errorf("provider = %s: Provider %q, want %q", expr, got, want)
		}
	}
}
