package iac_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
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
