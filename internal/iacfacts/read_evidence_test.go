package iacfacts

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestBuildReadEvidence(t *testing.T) {
	s := Span{File: "main.tf", StartLine: 2, StartCol: 3, EndLine: 2, EndCol: 20}
	observations := []ReadObservation{
		{Path: "main.tf", Read: true, Complete: true, Unknowns: []UnknownReason{{Code: UnknownExpression, Span: s, DecidableBy: []DecidableBy{DecideRender, DecidePlan, DecidePlan}}}},
		{Path: "defaults.tf", Read: true, Complete: true},
		{Path: "broken.tf", Read: true, Complete: false, Unknowns: []UnknownReason{{Code: UnknownSyntax, Span: Span{File: "broken.tf"}, DecidableBy: []DecidableBy{}}}},
		{Path: "missing.tf"},
	}
	want := ReadEvidence{Read: []string{"defaults.tf", "main.tf"}, Unread: []string{"broken.tf", "missing.tf"}, Unknowns: []UnknownReason{
		{Code: UnknownSyntax, Span: Span{File: "broken.tf"}, DecidableBy: []DecidableBy{}},
		{Code: UnknownExpression, Span: s, DecidableBy: []DecidableBy{DecidePlan, DecideRender}},
		{Code: UnknownUnreadInput, Span: Span{File: "missing.tf"}, DecidableBy: []DecidableBy{DecideContextFile}},
	}}
	got := BuildReadEvidence(observations)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("evidence = %#v, want %#v", got, want)
	}
	first, _ := json.Marshal(got)
	for i, j := 0, len(observations)-1; i < j; i, j = i+1, j-1 {
		observations[i], observations[j] = observations[j], observations[i]
	}
	second, _ := json.Marshal(BuildReadEvidence(observations))
	if !bytes.Equal(first, second) {
		t.Fatalf("order changes evidence: %s / %s", first, second)
	}
	// Incomplete wins even when another observation calls the same path complete.
	duplicate := BuildReadEvidence([]ReadObservation{{Path: "a.tf", Read: true, Complete: true}, {Path: "a.tf", Read: true}})
	if len(duplicate.Read) != 0 || !reflect.DeepEqual(duplicate.Unread, []string{"a.tf"}) || len(duplicate.Unknowns) == 0 {
		t.Fatalf("false closure: %+v", duplicate)
	}
	empty, _ := json.Marshal(BuildReadEvidence(nil))
	if string(empty) != `{"read":[],"unread":[],"unknowns":[]}` {
		t.Fatalf("empty = %s", empty)
	}
	if reflect.TypeOf(ReadObservation{}).NumField() != 4 {
		t.Fatal("observation shape grew")
	}
	t.Run("actual inputs", func(t *testing.T) {
		defaults, err := os.ReadFile("testdata/contracts/evidence/defaults.tf")
		if err != nil {
			t.Fatal(err)
		}
		broken, err := os.ReadFile("testdata/contracts/evidence/incomplete.tf")
		if err != nil {
			t.Fatal(err)
		}
		set, err := ReadTerraformModule(fstest.MapFS{"defaults.tf": {Data: defaults}, "broken.tf": {Data: broken}}, ".")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(set.Evidence.Read, []string{"defaults.tf"}) || !reflect.DeepEqual(set.Evidence.Unread, []string{"broken.tf"}) {
			t.Fatalf("actual files: %+v", set.Evidence)
		}
		if len(set.Evidence.Unknowns) == 0 || set.Evidence.Unknowns[0].Code != UnknownSyntax {
			t.Fatalf("syntax reason absent: %+v", set.Evidence)
		}
	})
}

// Unsupported and partial inputs must retain their names even when they return no resources.
func TestReadEvidencePartialAndEmptyInputs(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		read      func(string, []byte, ...Option) (*ResourceSet, error)
		complete  bool
		code      UnknownCode
	}{
		{"empty.tf", "", ReadTerraformFile, true, ""},
		{"empty.yaml", "", ReadCloudFormationFile, true, ""},
		{"empty-pulumi.yaml", "", ReadPulumiYAMLFile, true, ""},
		{"syntax.tf", "resource \"widget\" \"w\" {", ReadTerraformFile, false, UnknownSyntax},
		{"syntax.yaml", "Resources: [", ReadCloudFormationFile, false, UnknownSyntax},
		{"syntax-pulumi.yaml", "resources: [", ReadPulumiYAMLFile, false, UnknownSyntax},
		{"main.tf.json", `{"resource":{}}`, ReadTerraformJSONFile, false, UnknownUnsupportedForm},
		{"shape.yaml", "Resources: [unsupported]\n", ReadCloudFormationFile, false, UnknownUnsupportedForm},
		{"shape-pulumi.yaml", "resources: [unsupported]\n", ReadPulumiYAMLFile, false, UnknownUnsupportedForm},
		{"root.yaml", "[unsupported]\n", ReadCloudFormationFile, false, UnknownUnsupportedForm},
		{"root-pulumi.yaml", "[unsupported]\n", ReadPulumiYAMLFile, false, UnknownUnsupportedForm},
		{"dropped.yaml", "Resources:\n  Dropped: {}\n", ReadCloudFormationFile, false, UnknownUnsupportedForm},
		{"dropped-pulumi.yaml", "resources:\n  dropped: {}\n", ReadPulumiYAMLFile, false, UnknownUnsupportedForm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, err := tc.read(tc.name, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			e := set.Evidence
			if tc.complete {
				if !reflect.DeepEqual(e, ReadEvidence{Read: []string{tc.name}, Unread: []string{}, Unknowns: []UnknownReason{}}) {
					t.Fatalf("empty input lost: %+v", e)
				}
			} else {
				if !reflect.DeepEqual(e.Read, []string{}) || !reflect.DeepEqual(e.Unread, []string{tc.name}) {
					t.Fatalf("false closure: %+v", e)
				}
				found := false
				for _, u := range e.Unknowns {
					if u.Code == tc.code && u.Span.File == tc.name {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.code, e)
				}
			}
		})
	}
}

func TestReadEvidenceModuleIncludesAllInputsOnce(t *testing.T) {
	files := fstest.MapFS{
		"defaults.tf":   {Data: []byte("variable \"name\" { default = \"default-value-canary\" }\n")},
		"main.tf":       {Data: []byte("resource \"widget\" \"w\" { name = var.name }\n")},
		"extra.tf.json": {Data: []byte(`{"resource":{}}`)},
	}
	fsys := &evidenceCountingFS{FS: files, opens: map[string]int{}}
	set, err := ReadTerraformModule(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set.Evidence.Read, []string{"defaults.tf", "main.tf"}) || !reflect.DeepEqual(set.Evidence.Unread, []string{"extra.tf.json"}) {
		t.Fatalf("module inventory: %+v", set.Evidence)
	}
	for file := range files {
		if fsys.opens[file] != 1 {
			t.Fatalf("%s read %d times", file, fsys.opens[file])
		}
	}
	with, err := ReadTerraformModule(files, ".", WithValues())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set.Evidence, with.Evidence) {
		t.Fatalf("WithValues changes completeness: %+v / %+v", set.Evidence, with.Evidence)
	}
	empty, err := ReadTerraformModule(fstest.MapFS{}, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Evidence.Read) != 0 || empty.Evidence.Read == nil || empty.Evidence.Unread == nil || empty.Evidence.Unknowns == nil {
		t.Fatalf("empty module evidence: %+v", empty.Evidence)
	}
}

type evidenceCountingFS struct {
	fs.FS
	opens map[string]int
}

func (f *evidenceCountingFS) Open(name string) (fs.File, error) {
	f.opens[name]++
	return f.FS.Open(name)
}

func TestReadEvidenceReasonsDoNotAliasObservations(t *testing.T) {
	o := []ReadObservation{{Path: "a.tf", Read: true, Complete: true, Unknowns: []UnknownReason{{Code: UnknownExpression, Span: Span{File: "a.tf"}, DecidableBy: []DecidableBy{DecideRender, DecidePlan}}}}}
	before, _ := json.Marshal(o)
	e := BuildReadEvidence(o)
	after, _ := json.Marshal(o)
	if !bytes.Equal(before, after) {
		t.Fatal("builder mutates observations")
	}
	e.Unknowns[0].DecidableBy[0] = DecideContextFile
	after, _ = json.Marshal(o)
	if !bytes.Equal(before, after) {
		t.Fatal("builder aliases actions")
	}
	raw, _ := json.Marshal(BuildReadEvidence(o))
	var wire struct {
		Unknowns []struct {
			Span map[string]any `json:"span"`
		} `json:"unknowns"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Unknowns) != 1 || len(wire.Unknowns[0].Span) != 5 || wire.Unknowns[0].Span["file"] != "a.tf" || wire.Unknowns[0].Span["start_line"] != float64(0) {
		t.Fatalf("new span wire shape: %s", raw)
	}
}

func TestReadEvidenceNamesEveryDroppedSection(t *testing.T) {
	for _, tc := range []struct {
		file, src string
		read      func(string, []byte, ...Option) (*ResourceSet, error)
		lines     []int
	}{
		{"main.tf", "check \"c\" {\n data \"widget\" \"a\" {}\n data \"widget\" \"b\" {}\n}\n", ReadTerraformFile, []int{2, 3}},
		{"stack.yaml", "Resources:\n  A: {}\n  B: {}\n", ReadCloudFormationFile, []int{2, 3}},
		{"Pulumi.yaml", "resources:\n  a: {}\n  b: {}\n", ReadPulumiYAMLFile, []int{2, 3}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			set, err := tc.read(tc.file, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(set.Evidence.Unread, []string{tc.file}) {
				t.Fatalf("dropped input reads complete: %+v", set.Evidence)
			}
			var lines []int
			for _, u := range set.Evidence.Unknowns {
				if u.Code == UnknownUnsupportedForm && u.Span.File == tc.file {
					lines = append(lines, u.Span.StartLine)
				}
			}
			if !reflect.DeepEqual(lines, tc.lines) {
				t.Fatalf("dropped sections lost their spans: %v, want %v", lines, tc.lines)
			}
		})
	}
}
