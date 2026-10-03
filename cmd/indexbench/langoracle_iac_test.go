package main

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestIaCCountNotes: what wave iac's dialects count but never extract
// reaches a reading's notes through lang-oracle: Terragrunt references no
// static reading resolves, by reason (extractor.TerragruntUnresolvedRefs,
// LB-F142), and a subject's YAML files whose Argo, Flux or Crossplane
// documents sit beside others (fusion.CRMixed, LB-F149). Every other dialect
// gets none.
func TestIaCCountNotes(t *testing.T) {
	tg := []core.SourceFile{
		{Path: "live/a/terragrunt.hcl", Content: []byte("include \"r\" {\n  path = get_env(\"ROOT\")\n}\nterraform {\n  source = \"git::https://example.com/org/m.git//app?ref=v1\"\n}\n")},
		{Path: "live/b/terragrunt.hcl", Content: []byte("terraform {\n  source = \"tfr:///terraform-aws-modules/vpc/aws?version=3.3.0\"\n}\n")},
		{Path: "live/root.hcl", Content: []byte("locals {\n  x = 1\n}\n")},
	}
	want := []string{"s: 3 Terragrunt references no static reading resolves, counted not guessed (TerragruntUnresolvedRefs): external:git 1, external:registry 1, function:get_env 1"}
	if got := iacCountNotes("terragrunt", "s", tg, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("terragrunt notes = %q, want %q", got, want)
	}
	if got := iacCountNotes("terragrunt", "s", tg[2:], nil); got != nil {
		t.Errorf("terragrunt with nothing unresolved: notes = %q, want none", got)
	}

	app := "apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: a\n  namespace: argocd\nspec:\n  project: default\n"
	ns := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: n\n"
	yamls := []core.SourceFile{
		{Path: "apps/mixed.yaml", Content: []byte(app + "---\n" + ns)},
		{Path: "apps/app.yaml", Content: []byte(app)},
		{Path: "k8s/ns.yaml", Content: []byte(ns)},
	}
	want = []string{"s: 1 files hold Argo, Flux or Crossplane documents beside other documents and stay Kubernetes, whole (fusion.CRMixed): apps/mixed.yaml"}
	for _, d := range []string{"argo", "flux", "crossplane"} {
		if got := iacCountNotes(d, "s", nil, yamls); !reflect.DeepEqual(got, want) {
			t.Errorf("%s notes = %q, want %q", d, got, want)
		}
	}
	if got := iacCountNotes("argo", "s", nil, yamls[1:]); got != nil {
		t.Errorf("argo with no mixed file: notes = %q, want none", got)
	}
	if got := iacCountNotes("kubernetes", "s", tg, yamls); got != nil {
		t.Errorf("kubernetes notes = %q, want none", got)
	}
}

// TestIaCCountNotes_Timoni (LB-F218): a bundle instance's module url no static
// reading resolves reaches the timoni reading's notes, by reason
// (extractor.TimoniUnresolvedRefs): an oci:// module, a url that leaves the
// repository, a url that is no string literal. A subject with every url
// resolved gets none.
func TestIaCCountNotes_Timoni(t *testing.T) {
	bundle := "bundle: {\n\tapiVersion: \"v1alpha1\"\n\tname: \"demo\"\n\tinstances: {\n" +
		"\t\tweb: module: url: \"file://../app\"\n" +
		"\t\tcache: module: url: \"oci://registry.example.test/modules/cache\"\n" +
		"\t\tclimb: module: url: \"file://../../../outside\"\n" +
		"\t\tdynamic: module: url: \"file://\\(_base)/app\"\n" +
		"\t}\n}\n\n_base: \"..\"\n"
	files := []core.SourceFile{
		{Path: "svc/bundles/bundle.cue", Content: []byte(bundle)},
		{Path: "svc/app/timoni.cue", Content: []byte("package main\n\ntimoni: apiVersion: \"v1alpha1\"\n")},
	}
	want := []string{"s: 3 Timoni module references no static reading resolves, counted not guessed (TimoniUnresolvedRefs): expression 1, external:oci 1, outside-repo 1"}
	if got := iacCountNotes("timoni", "s", files, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("timoni notes = %q, want %q", got, want)
	}
	if got := iacCountNotes("timoni", "s", files[1:], nil); got != nil {
		t.Errorf("timoni with nothing unresolved: notes = %q, want none", got)
	}
}
