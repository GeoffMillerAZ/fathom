package extractor

import (
	"reflect"
	"testing"
)

// TestDocCrossRefRepoRoot pins the repo-root strategy (owner decision D4,
// 2026-09-28): a "./path" value resolves from the repository root, not the
// referring file's directory (GitHub Actions' `uses: ./x` convention); any
// other value (a registry or VCS address) is kept as written.
func TestDocCrossRefRepoRoot(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","detect":{},"entities":[
  {"select":"$","where":{"has_key":["runs"]},"kind":"target","source_kind":"GHAActionDef","name":".name"}],
 "cross_file":[{"from":"target","select":"$.runs.steps[*].uses","value":"$","resolve":"repo-root","edge":"uses_action"}]}`)
	src := "name: A\nruns:\n  using: composite\n  steps:\n    - uses: ./.github/actions/b\n    - uses: actions/checkout@v4\n    - uses: ./\n    - uses: ./../escape\n"
	r, err := x.ExtractDoc("s", ".github/actions/a/action.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range r.CrossFile {
		got = append(got, c.Raw+"=>"+c.Target)
	}
	want := []string{"./.github/actions/b=>.github/actions/b", "actions/checkout@v4=>actions/checkout@v4", "./=>.", "./../escape=>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
	if !reflect.DeepEqual(r.Roots, []string{"A"}) {
		t.Fatalf("roots = %q", r.Roots)
	}
}

// TestDocLabelSelectorJoin pins the bounded label-selector join's engine
// half (D4): an entity's name qualified by its kind and namespace, a
// cross_file ref restricted by `where` on its origin, the selector map
// recorded canonically with its scope, and the label sets a workload
// carries.
func TestDocLabelSelectorJoin(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","multi_document":true,"detect":{"require_keys":["kind","metadata"]},"entities":[
  {"select":"$","where":{"has_key":["kind","metadata"]},"kind":"resource","source_kind":"K8sWorkload","name":".metadata.name","qualify":[".kind",".metadata.namespace"]}],
 "cross_file":[{"from":"resource","where":{"equals":{".kind":"Service"}},"select":"$.spec.selector","value":"$","resolve":"label-selector","scope":".metadata.namespace","scope_default":"default","edge":"selects"}],
 "labels":[{"from":"resource","select":"$.spec.template.metadata.labels","scope":".metadata.namespace","scope_default":"default"}]}`)
	src := `apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector:
    tier: fe
    app: web
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
        tier: fe
`
	r, err := x.ExtractDoc("s", "k8s/web.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range r.Nodes {
		names = append(names, n.Name)
	}
	if want := []string{"Service/web", "Deployment/prod/web"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %q want %q", names, want)
	}
	if len(r.CrossFile) != 1 {
		t.Fatalf("cross = %+v (the Deployment's own selector must not match where)", r.CrossFile)
	}
	c := r.CrossFile[0]
	if c.From != "Service/web" || c.Strategy != "label-selector" || c.Target != "app=web,tier=fe" || c.Scope != "default" ||
		!reflect.DeepEqual(c.Labels, map[string]string{"app": "web", "tier": "fe"}) || c.Line != 6 {
		t.Fatalf("cross = %+v", c)
	}
	want := []DocLabelSet{{Entity: "Deployment/prod/web", Scope: "prod", Labels: map[string]string{"app": "web", "tier": "fe"}, Line: 21}}
	if !reflect.DeepEqual(r.Labels, want) {
		t.Fatalf("labels = %+v want %+v", r.Labels, want)
	}
	if !reflect.DeepEqual(r.Roots, []string{"Service/web", "Deployment/prod/web"}) {
		t.Fatalf("roots = %q", r.Roots)
	}
}
