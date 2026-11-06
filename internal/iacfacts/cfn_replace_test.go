package iacfacts

import (
	"strings"
	"testing"
	"time"
)

// IF-C4: what a registry schema's createOnlyProperties and conditionalCreateOnlyProperties say about a change.
// Every schema here is made up (IF-P7).

func cfnChangeCats(t *testing.T, extra map[string]any) []IaCCatalogInfo {
	t.Helper()
	dir := t.TempDir()
	importCFN(t, dir, cfnZip(t, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC), []string{"w.json"}, map[string][]byte{"w.json": cfnFixtureType("Example::Widget::Thing", extra)}), "us-east-1")
	cats, bad := ListIaCCatalogs(dir, false)
	if len(cats) != 1 || len(bad) != 0 {
		t.Fatalf("catalogs %d %v", len(cats), bad)
	}
	return cats
}

func cfnRes(t *testing.T, tmpl string) Resource {
	t.Helper()
	set, err := ReadCloudFormationFile("t.yaml", []byte(tmpl))
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("read: %v %v", err, set)
	}
	return set.Resources[0]
}

// change reads two templates, each with one resource, and compares them. No read carries values.
func change(t *testing.T, cats []IaCCatalogInfo, base, head string) CFNChange {
	t.Helper()
	return CheckCloudFormationChange(cats, "us-east-1", cfnRes(t, base), cfnRes(t, head), []byte(base), []byte(head))
}

func changeKinds(c CFNChange) string {
	var out []string
	for _, f := range c.Findings {
		out = append(out, f.Kind+":"+f.Property+":"+f.Change)
	}
	return strings.Join(out, " ")
}

const cfnWidget = `Resources:
  W:
    Type: Example::Widget::Thing
    Properties:
      Name: a
      Size: 3
      Tags:
        - Key: k
`

func TestCheckCloudFormationChange(t *testing.T) {
	cats := cfnChangeCats(t, nil)
	for _, c := range []struct {
		name, head, want string
	}{
		{"unchanged", cfnWidget, ""},
		{"create-only", strings.Replace(cfnWidget, "Name: a", "Name: b", 1), "forces-replacement:Name:changed"},
		{"conditional", strings.Replace(cfnWidget, "Size: 3", "Size: 4", 1), "may-replace:Size:changed"},
		{"both", strings.Replace(strings.Replace(cfnWidget, "Name: a", "Name: b", 1), "Size: 3", "Size: 4", 1), "forces-replacement:Name:changed may-replace:Size:changed"},
		{"unflagged property", strings.Replace(cfnWidget, "Key: k", "Key: z", 1), ""},
		{"removed", strings.Replace(cfnWidget, "      Size: 3\n", "", 1), "may-replace:Size:removed"},
		{"reference", strings.Replace(cfnWidget, "Name: a", "Name: !Ref P", 1), "forces-replacement:Name:changed"},
		{"expression text", strings.Replace(cfnWidget, "Name: a", "Name: !Sub \"x-${P}\"", 1), "forces-replacement:Name:changed"},
	} {
		got := change(t, cats, cfnWidget, c.head)
		if changeKinds(got) != c.want {
			t.Errorf("%s: got %q, want %q", c.name, changeKinds(got), c.want)
		}
		if !got.Binding.Bound || got.Binding.Version != CFNUnversioned {
			t.Errorf("%s: binding %+v", c.name, got.Binding)
		}
	}
	// A property new to the head reads as added.
	got := change(t, cats, strings.Replace(cfnWidget, "      Name: a\n", "", 1), cfnWidget)
	if changeKinds(got) != "forces-replacement:Name:added" {
		t.Fatalf("added: %q", changeKinds(got))
	}
	// A new line, a re-indented value and a moved property are no change.
	moved := strings.Replace(strings.Replace(cfnWidget, "      Name: a\n      Size: 3\n", "      Size: 3\n      Name: a\n", 1), "Resources:", "\nResources:", 1)
	if got := change(t, cats, cfnWidget, moved); changeKinds(got) != "" {
		t.Fatalf("a reorder: %q", changeKinds(got))
	}
	// Every finding cites the schema field and the blob it was read from.
	f := change(t, cats, cfnWidget, strings.Replace(cfnWidget, "Name: a", "Name: b", 1)).Findings[0]
	if f.Path != "createOnlyProperties" || f.Pointer != "/properties/Name" || len(f.Digest) != 64 || f.Span.StartLine != 5 {
		t.Fatalf("finding %+v", f)
	}
}

func TestCheckCloudFormationChangeNested(t *testing.T) {
	cats := cfnChangeCats(t, map[string]any{"createOnlyProperties": []string{"/properties/Tags/*/Key"}, "conditionalCreateOnlyProperties": []string{}})
	got := change(t, cats, cfnWidget, strings.Replace(cfnWidget, "Key: k", "Key: z", 1))
	if changeKinds(got) != "forces-replacement:Tags:changed" {
		t.Fatalf("nested: %q", changeKinds(got))
	}
	if got := change(t, cats, cfnWidget, strings.Replace(cfnWidget, "Name: a", "Name: b", 1)); len(got.Findings) != 0 {
		t.Fatalf("an unrelated change: %q", changeKinds(got))
	}
}

func TestCheckCloudFormationChangeStatesWhatItCannotSee(t *testing.T) {
	cats := cfnChangeCats(t, nil)
	// Properties the reader could not walk prove nothing either way.
	opaque := "Resources:\n  W:\n    Type: Example::Widget::Thing\n    Properties: !If [C, {Name: a}, {Name: b}]\n"
	got := change(t, cats, cfnWidget, opaque)
	if len(got.Findings) != 0 || len(got.Notes) == 0 || !strings.Contains(got.Notes[0], "could not be walked") {
		t.Fatalf("opaque head: %+v", got)
	}
	headB := strings.Replace(cfnWidget, "Name: a", "Name: b", 1)
	// No catalog: unbound, no finding.
	if got := change(t, nil, cfnWidget, headB); got.Binding.Bound || len(got.Findings) != 0 {
		t.Fatalf("no catalog: %+v", got)
	}
	// A type the schemas lack reads unbound.
	other := strings.ReplaceAll(cfnWidget, "Example::Widget::Thing", "Other::Vendor::Thing")
	if got := change(t, cats, other, strings.Replace(other, "Name: a", "Name: b", 1)); got.Binding.Bound || len(got.Findings) != 0 {
		t.Fatalf("unknown type: %+v", got)
	}
	// A different type is a different resource: the contract of one says nothing of the other.
	moved := strings.ReplaceAll(cfnWidget, "Example::Widget::Thing", "Example::Widget::Other")
	if got := change(t, cats, moved, cfnWidget); len(got.Findings) != 0 || len(got.Notes) == 0 {
		t.Fatalf("type change: %+v", got)
	}
}

func TestCFNReplacementFacts(t *testing.T) {
	cats := cfnChangeCats(t, nil)
	facts, b := CFNReplacementFacts(cats, "us-east-1", cfnRes(t, cfnWidget))
	if !b.Bound {
		t.Fatalf("binding %+v", b)
	}
	var got []string
	for _, f := range facts {
		got = append(got, f.Class+":"+f.Property)
	}
	if strings.Join(got, " ") != "replace:Name conditional:Size" {
		t.Fatalf("facts %v", got)
	}
	if facts, b := CFNReplacementFacts(nil, "", cfnRes(t, cfnWidget)); b.Bound || len(facts) != 0 {
		t.Fatalf("no catalog: %v %+v", facts, b)
	}
}

func TestCFNPointerPath(t *testing.T) {
	for in, want := range map[string]string{
		"/properties/Name":       "Name",
		"/properties/A/B":        "A.B",
		"/properties/Tags/*/Key": "Tags[*].Key",
		"/properties/A/*":        "A[*]",
		"/definitions/X":         "",
		"/properties":            "",
		"/properties/":           "",
		"properties/Name":        "",
		"/properties/A//B":       "",
		"/properties/*/B":        "",
		"":                       "",
	} {
		if got, ok := CFNPointerPath(in); got != want || ok != (want != "") {
			t.Errorf("CFNPointerPath(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

// A change is a change of value, not of spelling: a quoted scalar, the long and short spellings of an intrinsic
// and a JSON template rewritten as YAML claim nothing when no value moved. No read carries values, so the check
// normalizes what the span covers.
func TestCheckCloudFormationChangeIgnoresSpelling(t *testing.T) {
	cats := cfnChangeCats(t, nil)
	chg := func(baseFile, base, headFile, head string) CFNChange {
		t.Helper()
		read := func(file, src string) Resource {
			set, err := ReadCloudFormationFile(file, []byte(src))
			if err != nil || len(set.Resources) != 1 {
				t.Fatalf("read %s: %v %v", file, err, set)
			}
			return set.Resources[0]
		}
		return CheckCloudFormationChange(cats, "us-east-1", read(baseFile, base), read(headFile, head), []byte(base), []byte(head))
	}
	for name, h := range map[string]string{
		"double quotes": strings.Replace(cfnWidget, "Name: a", `Name: "a"`, 1),
		"single quotes": strings.Replace(cfnWidget, "Name: a", `Name: 'a'`, 1),
		"quoted number": strings.Replace(cfnWidget, "Size: 3", `Size: "3"`, 1),
	} {
		if got := chg("t.yaml", cfnWidget, "t.yaml", h); changeKinds(got) != "" {
			t.Errorf("%s: %q", name, changeKinds(got))
		}
	}
	// A real change under quotes is still a change.
	if got := chg("t.yaml", cfnWidget, "t.yaml", strings.Replace(cfnWidget, "Name: a", `Name: "b"`, 1)); changeKinds(got) != "forces-replacement:Name:changed" {
		t.Errorf("quoted change: %q", changeKinds(got))
	}
	const asJSON = `{"Resources": {"W": {"Type": "Example::Widget::Thing", "Properties": {
  "Name": {"Ref": "P"},
  "Size": {"Fn::Sub": "n-${Q}"},
  "Tags": [{"Key": "k"}]}}}}
`
	const asYAML = `Resources:
  W:
    Type: Example::Widget::Thing
    Properties:
      Name: !Ref P
      Size: !Sub "n-${Q}"
      Tags:
        - Key: k
`
	if got := chg("t.json", asJSON, "t.yaml", asYAML); changeKinds(got) != "" {
		t.Errorf("JSON to YAML: %q", changeKinds(got))
	}
	if got := chg("t.yaml", asYAML, "t.json", asJSON); changeKinds(got) != "" {
		t.Errorf("YAML to JSON: %q", changeKinds(got))
	}
	// A literal part of an intrinsic that changed is still a change.
	if got := chg("t.json", asJSON, "t.yaml", strings.Replace(asYAML, "n-${Q}", "m-${Q}", 1)); changeKinds(got) != "may-replace:Size:changed" {
		t.Errorf("changed Sub: %q", changeKinds(got))
	}
	if got := chg("t.json", asJSON, "t.yaml", strings.Replace(asYAML, "!Ref P", "!Ref R", 1)); changeKinds(got) != "forces-replacement:Name:changed" {
		t.Errorf("changed Ref: %q", changeKinds(got))
	}
}
