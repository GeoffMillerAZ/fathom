package iac_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

func TestResourceReadEvidence(t *testing.T) {
	ctx := context.Background()
	defaults, err := os.ReadFile("../../../internal/iacfacts/testdata/contracts/evidence/defaults.tf")
	if err != nil {
		t.Fatal(err)
	}
	main := []byte(`resource "widget_thing" "a" {
 name = upper(var.name)
}
`)
	for _, module := range []bool{false, true} {
		var got *iac.ResourceSet
		var inner *iacfacts.ResourceSet
		if module {
			files := fstest.MapFS{"infra/defaults.tf": {Data: defaults}, "infra/main.tf": {Data: main}}
			got, err = iac.ReadModule(ctx, files, "infra")
			if err != nil {
				t.Fatal(err)
			}
			inner, err = iacfacts.ReadTerraformModule(files, "infra")
		} else {
			got, err = iac.ReadFile(ctx, "main.tf", main)
			if err != nil {
				t.Fatal(err)
			}
			inner, err = iacfacts.ReadTerraformFile("main.tf", main)
		}
		if err != nil {
			t.Fatal(err)
		}
		assertEvidenceCopy(t, got, inner)
		want := []string{"main.tf"}
		if module {
			want = []string{"infra/defaults.tf", "infra/main.tf"}
		}
		if !reflect.DeepEqual(got.Evidence.Read, want) || len(got.Evidence.Unread) != 0 || len(got.Evidence.Unknowns) != 1 {
			t.Fatalf("Terraform evidence: %+v", got.Evidence)
		}
		u := got.Evidence.Unknowns[0]
		if u.Code != iac.UnknownExpression || !reflect.DeepEqual(u.DecidableBy, []iac.DecidableBy{iac.DecidePlan}) || u.Span != got.Resources[0].Attrs[0].Span {
			t.Fatalf("reason: %+v", u)
		}
	}
	for _, tc := range []struct {
		fixture, path string
		read          func(string, []byte, ...iacfacts.Option) (*iacfacts.ResourceSet, error)
	}{
		{"template.yaml", "stack.yaml", iacfacts.ReadCloudFormationFile}, {"pulumi.yaml", "Pulumi.yaml", iacfacts.ReadPulumiYAMLFile},
	} {
		src, err := os.ReadFile("../../../internal/iacfacts/testdata/contracts/evidence/" + tc.fixture)
		if err != nil {
			t.Fatal(err)
		}
		inner, err := tc.read(tc.path, src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := iac.ReadFile(ctx, tc.path, src)
		if err != nil {
			t.Fatal(err)
		}
		assertEvidenceCopy(t, got, inner)
		if !reflect.DeepEqual(got.Evidence.Read, []string{tc.path}) || len(got.Evidence.Unread) != 0 || len(got.Evidence.Unknowns) != 1 {
			t.Fatalf("%s: %+v", tc.path, got.Evidence)
		}
		u := got.Evidence.Unknowns[0]
		if u.Code != iac.UnknownExpression || !reflect.DeepEqual(u.DecidableBy, []iac.DecidableBy{iac.DecideRender}) || u.Span != got.Resources[0].Attrs[0].Span {
			t.Fatalf("%s reason: %+v", tc.path, u)
		}
	}
}

func assertEvidenceCopy(t *testing.T, got *iac.ResourceSet, inner *iacfacts.ResourceSet) {
	t.Helper()
	a, _ := json.Marshal(got.Evidence)
	b, _ := json.Marshal(inner.Evidence)
	if !bytes.Equal(a, b) {
		t.Fatalf("conversion drops evidence: %s / %s", a, b)
	}
	raw, _ := json.Marshal(got)
	if bytes.Contains(raw, []byte("default-value-canary")) || bytes.Contains(a, []byte("null")) {
		t.Fatalf("values/null leaked: %s", raw)
	}
}

// Absolute caller names remain accepted and retained by legacy spans. Only the new
// evidence uses a logical input name, including early returns with no resources.
func TestResourceReadEvidenceLogicalPaths(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		unread    bool
	}{
		{"main.tf", "resource \"widget\" \"w\" {\n name = upper(var.name)\n}\n", false},
		{"stack.yaml", "Resources:\n  W:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: !Sub '${Name}-bucket'\n", false},
		{"Pulumi.yaml", "name: sample\nruntime: yaml\nresources:\n  w:\n    type: aws:s3:Bucket\n    properties:\n      bucket: {fn::join: [\"-\", [a, b]]}\n", false},
		{"defaults.tf", "variable \"name\" { default = \"default-value-canary\" }\n", false},
		{"defaults.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nParameters:\n  Name: {Type: String, Default: default-value-canary}\n", false},
		{"Pulumi.yml", "name: sample\nruntime: yaml\nconfiguration:\n  name: {default: default-value-canary}\n", false},
		{"empty.tf", "", false},
		{"broken.tf", "resource \"widget\" \"w\" {", true},
		{"broken.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources: [", true},
		{"Pulumi.yaml", "resources: [", true},
		{"main.tf.json", `{"resource":{}}`, true},
		{"dropped.tf", "check \"c\" {\n data \"widget\" \"a\" {}\n}\n", true},
		{"dropped.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Dropped: {}\n", true},
		{"Pulumi.yaml", "resources:\n  dropped: {}\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// This is a classifier name, never a file-system read.
			absolute := filepath.Join(string(filepath.Separator), "synthetic", "project", tc.name)
			relative, err := iac.ReadFile(context.Background(), tc.name, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			got, err := iac.ReadFile(context.Background(), absolute, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Evidence, relative.Evidence) {
				t.Fatalf("absolute evidence = %+v, relative = %+v", got.Evidence, relative.Evidence)
			}
			if tc.unread && (!reflect.DeepEqual(got.Evidence.Unread, []string{tc.name}) || len(got.Evidence.Unknowns) == 0) {
				t.Fatalf("incomplete input lost: %+v", got.Evidence)
			}
			for _, r := range got.Resources {
				if r.Span.File != absolute {
					t.Fatalf("legacy resource span changed: %+v", r.Span)
				}
				for _, a := range r.Attrs {
					if a.Span.File != absolute {
						t.Fatalf("legacy attribute span changed: %+v", a.Span)
					}
				}
			}
			with, err := iac.ReadFile(context.Background(), absolute, []byte(tc.src), iac.WithValues())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Evidence, with.Evidence) {
				t.Fatal("values changed evidence")
			}
			raw, _ := json.Marshal(got.Evidence)
			if bytes.Contains(raw, []byte("synthetic")) || bytes.Contains(raw, []byte("default-value-canary")) {
				t.Fatalf("host path/value leaked: %s", raw)
			}
		})
	}
}

func TestResourceReadEvidenceForeignAbsolutePaths(t *testing.T) {
	for _, prefix := range []string{
		`C:/synthetic/project/`, `C:\synthetic\project\`, `\\synthetic-server\share\`,
		`//synthetic-server/share/`, `/synthetic/project/`,
		`./C:/synthetic/`, `../synthetic/`, `relative/`, ``,
	} {
		for _, unread := range []bool{false, true} {
			name := "main.tf"
			src := []byte("resource \"widget\" \"w\" {\n name = upper(var.name)\n}\n")
			if unread {
				name += ".json"
				src = []byte(`{"resource":{}}`)
			}
			input := prefix + name
			t.Run(input, func(t *testing.T) {
				logical := name
				if prefix == "./C:/synthetic/" || prefix == "../synthetic/" || prefix == "relative/" {
					logical = input
				}
				want := iac.ReadEvidence{Read: []string{logical}, Unread: []string{}, Unknowns: []iac.UnknownReason{{
					Code:        iac.UnknownExpression,
					Span:        iac.Span{File: logical, StartLine: 2, StartCol: 2, EndLine: 2, EndCol: 24},
					DecidableBy: []iac.DecidableBy{iac.DecidePlan},
				}}}
				if unread {
					want.Read, want.Unread = []string{}, []string{logical}
					want.Unknowns[0].Code = iac.UnknownUnsupportedForm
					want.Unknowns[0].Span = iac.Span{File: logical}
				}
				for _, options := range [][]iac.Option{nil, {iac.WithValues()}} {
					got, err := iac.ReadFile(context.Background(), input, src, options...)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got.Evidence, want) {
						t.Fatalf("evidence = %+v, want %+v", got.Evidence, want)
					}
					if unread {
						if len(got.Resources) != 0 {
							t.Fatalf("unsupported input produced resources: %+v", got.Resources)
						}
					} else if len(got.Resources) != 1 || len(got.Resources[0].Attrs) != 1 ||
						got.Resources[0].Span.File != filepath.ToSlash(input) ||
						got.Resources[0].Attrs[0].Span.File != filepath.ToSlash(input) {
						t.Fatalf("legacy resource/attribute spans changed: %+v", got.Resources)
					}
				}
			})
		}
	}
}
