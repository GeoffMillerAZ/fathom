package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

func TestResourcesCarryReadEvidence(t *testing.T) {
	root := t.TempDir()
	defaults, err := os.ReadFile("../../internal/iacfacts/testdata/contracts/evidence/defaults.tf")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "infra/defaults.tf", string(defaults))
	write(t, root, "infra/main.tf", "resource \"widget_thing\" \"a\" {\n name = upper(var.name)\n}\n")
	args := []string{"-root", root, "-module", "infra", "infra/main.tf"}
	_, sc, supervised, se := runResources(t, args...)
	_, ic, inproc, ie := runResources(t, append([]string{"-in-process"}, args...)...)
	if sc != 0 || ic != 0 {
		t.Fatalf("exits %d/%d: %s %s", sc, ic, se, ie)
	}
	if !bytes.Equal(supervised, inproc) {
		t.Fatalf("transport mismatch:\n%s\n%s", supervised, inproc)
	}
	var doc struct {
		Files []struct {
			Result iac.ResourceSet `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal(supervised, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Files) != 2 {
		t.Fatalf("files: %s", supervised)
	}
	for i, f := range doc.Files {
		want := []string{"infra/main.tf"}
		if i == 0 {
			want = []string{"infra/defaults.tf", "infra/main.tf"}
		}
		e := f.Result.Evidence
		if !reflect.DeepEqual(e.Read, want) || len(e.Unread) != 0 || len(e.Unknowns) != 1 {
			t.Fatalf("evidence %d: %+v", i, e)
		}
		u := e.Unknowns[0]
		if u.Code != iac.UnknownExpression || !reflect.DeepEqual(u.DecidableBy, []iac.DecidableBy{iac.DecidePlan}) || u.Span.File != "infra/main.tf" || u.Span.StartLine != 2 {
			t.Fatalf("reason: %+v", u)
		}
	}
	if bytes.Contains(supervised, []byte("default-value-canary")) || bytes.Contains(supervised, []byte(`"value"`)) {
		t.Fatalf("value leaked: %s", supervised)
	}
}

// Exercise the actual command and supervised child with accepted absolute inputs
// outside -root, conserving the preexisting entry and resource span names.
func TestResourcesCarryReadEvidenceOutsideRoot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		name, src string
		unread    bool
	}{
		{"main.tf", "resource \"widget\" \"w\" {\n name = upper(var.name)\n}\n", false},
		{"stack.yaml", "Resources:\n  W:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: !Sub '${Name}-bucket'\n", false},
		{"Pulumi.yaml", "resources:\n  w:\n    type: aws:s3:Bucket\n    properties:\n      bucket: {fn::join: [\"-\", [a, b]]}\n", false},
		{"defaults.tf", "variable \"name\" { default = \"default-value-canary\" }\n", false},
		{"main.tf.json", `{"resource":{}}`, true},
		{"broken.tf", "resource \"widget\" \"w\" {", true},
	} {
		write(t, outside, tc.name, tc.src)
		t.Run(tc.name, func(t *testing.T) {
			absolute := filepath.Join(outside, tc.name)
			args := []string{"-root", root, absolute}
			_, sc, supervised, se := runResources(t, args...)
			_, ic, inproc, ie := runResources(t, append([]string{"-in-process"}, args...)...)
			if sc != 0 || ic != 0 {
				t.Fatalf("exits %d/%d: %s %s", sc, ic, se, ie)
			}
			if !bytes.Equal(supervised, inproc) {
				t.Fatalf("transport mismatch: %s / %s", supervised, inproc)
			}
			var doc struct {
				Files []struct {
					Path   string
					Result iac.ResourceSet
				}
			}
			if err := json.Unmarshal(supervised, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Files) != 1 || doc.Files[0].Path != filepath.ToSlash(absolute) {
				t.Fatalf("legacy entry changed: %s", supervised)
			}
			set := doc.Files[0].Result
			wantRead, wantUnread := []string{tc.name}, []string{}
			if tc.unread {
				wantRead, wantUnread = []string{}, []string{tc.name}
			}
			if !reflect.DeepEqual(set.Evidence.Read, wantRead) || !reflect.DeepEqual(set.Evidence.Unread, wantUnread) {
				t.Fatalf("inventory: %+v", set.Evidence)
			}
			if tc.unread && len(set.Evidence.Unknowns) == 0 {
				t.Fatalf("unknowns lost: %+v", set.Evidence)
			}
			for _, u := range set.Evidence.Unknowns {
				if u.Span.File != tc.name {
					t.Fatalf("reason path: %+v", u)
				}
			}
			for _, r := range set.Resources {
				if r.Span.File != filepath.ToSlash(absolute) {
					t.Fatalf("legacy span changed: %+v", r.Span)
				}
			}
			raw, _ := json.Marshal(set.Evidence)
			if bytes.Contains(raw, []byte(filepath.ToSlash(outside))) || bytes.Contains(supervised, []byte("default-value-canary")) || bytes.Contains(supervised, []byte(`"value"`)) {
				t.Fatalf("host path/value leaked: %s", raw)
			}
		})
	}
}
