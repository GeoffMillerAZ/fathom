package iacfacts

import (
	"strings"
	"testing"
	"time"
)

const cfnUseTemplate = `Resources:
  Good:
    Type: Example::Widget::Thing
    Properties:
      Name: a
      Size: 3
      Tags:
        - Key: k
  Bad:
    Type: Example::Widget::Thing
    Properties:
      Name: b
      Arn: x
      OldKnob: true
      Colour: red
  Opaque:
    Type: Example::Widget::Thing
    Properties: !If [C, {Name: a}, {Name: b}]
  Func:
    Type: AWS::Serverless::Function
    Properties:
      Handler: h
  Third:
    Type: Other::Vendor::Thing
    Properties:
      X: 1
`

func cfnUseFixture(t *testing.T, extra map[string]any) ([]IaCCatalogInfo, map[string]Resource) {
	t.Helper()
	dir := t.TempDir()
	importCFN(t, dir, cfnZip(t, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC), []string{"w.json"}, map[string][]byte{"w.json": cfnFixtureType("Example::Widget::Thing", extra)}), "us-east-1")
	cats, bad := ListIaCCatalogs(dir, false)
	if len(cats) != 1 || len(bad) != 0 {
		t.Fatalf("catalogs %d %v", len(cats), bad)
	}
	set, err := ReadCloudFormationFile("t.yaml", []byte(cfnUseTemplate))
	if err != nil {
		t.Fatal(err)
	}
	rs := map[string]Resource{}
	for _, r := range set.Resources {
		rs[r.Name] = r
	}
	return cats, rs
}

func kinds(u CFNUse) []string {
	var out []string
	for _, f := range u.Findings {
		out = append(out, f.Kind+":"+f.Property)
	}
	return out
}

func TestCheckCloudFormationUse(t *testing.T) {
	cats, rs := cfnUseFixture(t, nil)

	good := CheckCloudFormationUse(cats, "us-east-1", rs["Good"])
	if !good.Binding.Bound || good.Binding.Version != "unversioned" || good.Binding.Date != "2026-10-30" || good.Binding.Region != "us-east-1" || len(good.Findings) != 0 {
		t.Fatalf("good: %+v", good)
	}
	if !strings.Contains(good.Binding.Note, "2026-10-30") || !strings.Contains(good.Binding.Note, "names no version") {
		t.Fatalf("the answer does not state the catalog's date: %q", good.Binding.Note)
	}

	bad := CheckCloudFormationUse(cats, "us-east-1", rs["Bad"])
	got := strings.Join(kinds(bad), " ")
	for _, want := range []string{"unknown-property:Colour", "missing-required:Size", "read-only-set:Arn", "deprecated:OldKnob"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if len(bad.Findings) != 4 {
		t.Fatalf("findings %v", got)
	}
	for _, f := range bad.Findings {
		if f.Path == "" || f.Digest == "" || f.Span.StartLine == 0 {
			t.Fatalf("a finding lacks provenance or a span: %+v", f)
		}
	}
	for _, f := range bad.Findings {
		if f.Kind == CFNReadOnlySet && (f.Path != "readOnlyProperties" || f.Pointer != "/properties/Arn" || f.Span.StartLine != 13) {
			t.Fatalf("read-only finding %+v", f)
		}
	}

	// A Properties the reader cannot walk proves neither an unknown nor a missing property.
	if u := CheckCloudFormationUse(cats, "us-east-1", rs["Opaque"]); !u.Binding.Bound || len(u.Findings) != 0 {
		t.Fatalf("opaque: %+v", u)
	}

	// Transform and third-party types read unbound, with a reason, and no finding.
	fn := CheckCloudFormationUse(cats, "us-east-1", rs["Func"])
	if fn.Binding.Bound || len(fn.Findings) != 0 || !strings.Contains(fn.Binding.Note, "transform") {
		t.Fatalf("serverless: %+v", fn)
	}
	third := CheckCloudFormationUse(cats, "us-east-1", rs["Third"])
	if third.Binding.Bound || len(third.Findings) != 0 || !strings.Contains(third.Binding.Note, "not in the catalog") {
		t.Fatalf("third party: %+v", third)
	}
}

func TestCheckCloudFormationUseSelectsCatalog(t *testing.T) {
	cats, rs := cfnUseFixture(t, nil)
	// An empty region reads the only CloudFormation catalog.
	if u := CheckCloudFormationUse(cats, "", rs["Good"]); !u.Binding.Bound || u.Binding.Region != "us-east-1" {
		t.Fatalf("empty region: %+v", u)
	}
	// Another region is unbound and names what is installed.
	u := CheckCloudFormationUse(cats, "eu-west-1", rs["Good"])
	if u.Binding.Bound || !strings.Contains(u.Binding.Note, "eu-west-1") || !strings.Contains(u.Binding.Note, "us-east-1") {
		t.Fatalf("other region: %+v", u)
	}
	// Two regions and no region named: unbound, never a guess.
	two := append([]IaCCatalogInfo{}, cats...)
	c2 := cats[0]
	c2.Manifest.Line = "eu-west-1"
	two = append(two, c2)
	if u := CheckCloudFormationUse(two, "", rs["Good"]); u.Binding.Bound || !strings.Contains(u.Binding.Note, "regions") {
		t.Fatalf("two regions: %+v", u)
	}
	// No catalog at all.
	if u := CheckCloudFormationUse(nil, "us-east-1", rs["Good"]); u.Binding.Bound || !strings.Contains(u.Binding.Note, "fathom iac import --region") {
		t.Fatalf("none: %+v", u)
	}
	// A Terraform resource is not this function's.
	tf := rs["Good"]
	tf.Tool = ToolTerraform
	if u := CheckCloudFormationUse(cats, "us-east-1", tf); u.Binding.Bound {
		t.Fatalf("terraform resource bound: %+v", u)
	}
}

func TestCheckCloudFormationUseOpenSchema(t *testing.T) {
	// A schema that allows additional properties, or says nothing, never yields an unknown property.
	for _, extra := range []map[string]any{{"additionalProperties": true}, {"additionalProperties": nil}} {
		cats, rs := cfnUseFixture(t, extra)
		u := CheckCloudFormationUse(cats, "us-east-1", rs["Bad"])
		if strings.Contains(strings.Join(kinds(u), " "), "unknown-property") {
			t.Fatalf("%v: %v", extra, kinds(u))
		}
	}
}
