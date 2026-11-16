package main

import (
	"bytes"
	"encoding/json"
	"os"
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
